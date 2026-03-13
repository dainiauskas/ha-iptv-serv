package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	port    = ":8080"
	workers = 8 // concurrent HTTP checks; lower = less CPU on small devices
)

var (
	sourceM3Us      []string            // for combined playlist
	playlistMap     map[string]string   // slug or "0","1"... -> URL for single playlist lookup
	epgURL          string              // optional EPG XMLTV URL from config
	validateStreams bool                // if true, check each stream with GET; if false, skip (faster, less CPU)
)

// EPG cache for xmltv.php
var (
	epgProgrammes []epgProgramme     // programmes with channel = our stream_id
	epgCacheTime  time.Time
	epgCacheMu    sync.RWMutex
	epgCacheTTL   = 6 * time.Hour
)

// Xtream cache: combined validated channel list for player_api and stream redirects
var (
	xtreamCache     []Channel
	xtreamCacheTime time.Time
	xtreamCacheMu   sync.RWMutex
	xtreamCacheTTL  = 30 * time.Minute // rebuild less often to reduce CPU
)

type Channel struct {
	Metadata string
	URL      string
}

// PlaylistEntry matches Hass.io schema: name (URL slug) and url
type PlaylistEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Options struct {
	Playlists       []PlaylistEntry `json:"playlists"`
	EpgURL          string          `json:"epg_url"`
	ValidateStreams bool            `json:"validate_streams"`
}

type epgProgramme struct {
	Channel string
	Start   string
	Stop    string
	Title   string
	Desc    string
}

// xmltvProgramme for parsing XMLTV
type xmltvProgramme struct {
	Start   string `xml:"start,attr"`
	Stop    string `xml:"stop,attr"`
	Channel string `xml:"channel,attr"`
	Title   string `xml:"title"`
	Desc    string `xml:"desc"`
}

type xmltvRoot struct {
	XMLName    xml.Name         `xml:"tv"`
	Programmes []xmltvProgramme `xml:"programme"`
}

func main() {
	loadConfig()
	// Preload Xtream cache in background so first player_api request is fast
	go func() {
		getXtreamChannels()
	}()

	http.HandleFunc("/playlist.m3u", combinedPlaylistHandler)
	http.HandleFunc("/playlist/", singlePlaylistHandler)
	http.HandleFunc("/player_api.php", xtreamPlayerAPIHandler)
	http.HandleFunc("/xmltv.php", xtreamXMLTVHandler)
	http.HandleFunc("/get.php", xtreamGetStreamHandler)
	http.HandleFunc("/live/", xtreamLiveStreamHandler)

	fmt.Printf("Local IPTV proxy server started. Listening on %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func loadConfig() {
	file, err := os.Open("/data/options.json")
	if err != nil {
		log.Println("Hass.io config file not found, using default playlists")
		setDefaultPlaylists()
		return
	}
	defer file.Close()

	var opts Options
	if err := json.NewDecoder(file).Decode(&opts); err != nil {
		log.Println("Failed to read Hass.io options:", err)
		setDefaultPlaylists()
		return
	}

	if len(opts.Playlists) > 0 {
		sourceM3Us = make([]string, 0, len(opts.Playlists))
		playlistMap = make(map[string]string)
		for i, p := range opts.Playlists {
			if p.URL == "" {
				continue
			}
			sourceM3Us = append(sourceM3Us, p.URL)
			slug := slugify(p.Name)
			if slug == "" {
				slug = strconv.Itoa(i)
			}
			playlistMap[slug] = p.URL
			// allow index lookup for backward compatibility
			playlistMap[strconv.Itoa(i)] = p.URL
		}
	}
	epgURL = strings.TrimSpace(opts.EpgURL)
	validateStreams = opts.ValidateStreams
}

func setDefaultPlaylists() {
	sourceM3Us = []string{
		"https://iptv-org.github.io/iptv/languages/lit.m3u",
		"https://iptv-org.github.io/iptv/languages/rus.m3u",
	}
	playlistMap = map[string]string{
		"lit": "https://iptv-org.github.io/iptv/languages/lit.m3u",
		"0":   "https://iptv-org.github.io/iptv/languages/lit.m3u",
		"rus": "https://iptv-org.github.io/iptv/languages/rus.m3u",
		"1":   "https://iptv-org.github.io/iptv/languages/rus.m3u",
	}
}

// slugify makes a URL-safe slug from name: lowercase, spaces to hyphens, letters and numbers only
func slugify(s string) string {
	var b strings.Builder
	needHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r == ' ' || r == '-':
			needHyphen = true
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if needHyphen && b.Len() > 0 {
				b.WriteRune('-')
			}
			needHyphen = false
			b.WriteRune(r)
		}
	}
	return b.String()
}

// fetchChannelsFromURL downloads one M3U and returns parsed channels
func fetchChannelsFromURL(sourceURL string) ([]Channel, error) {
	resp, err := http.Get(sourceURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var channels []Channel
	scanner := bufio.NewScanner(resp.Body)
	var currentMeta string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXTINF") {
			currentMeta = line
		} else if strings.HasPrefix(line, "http") && currentMeta != "" {
			channels = append(channels, Channel{Metadata: currentMeta, URL: line})
			currentMeta = ""
		}
	}
	return channels, scanner.Err()
}

func writeM3U(w http.ResponseWriter, channels []Channel) {
	w.Header().Set("Content-Type", "audio/x-mpegurl")
	w.Write([]byte("#EXTM3U\n"))
	for _, ch := range channels {
		w.Write([]byte(ch.Metadata + "\n" + ch.URL + "\n"))
	}
}

// combinedPlaylistHandler serves one merged playlist from all configured sources
func combinedPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	// HEAD: quick 200 so apps (e.g. IPTVX) can verify the URL without waiting for full generation
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "audio/x-mpegurl")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Reuse shared cache so we don't run full fetch+validate on every request (reduces CPU)
	channels := getXtreamChannels()
	if channels == nil {
		http.Error(w, "Failed to reach any sources", http.StatusBadGateway)
		return
	}
	log.Printf("Combined playlist: serving %d channels from cache", len(channels))
	writeM3U(w, channels)
}

// singlePlaylistHandler serves one playlist by name or index: /playlist/lit.m3u or /playlist/0.m3u
func singlePlaylistHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/playlist/")
	if path == r.URL.Path || !strings.HasSuffix(path, ".m3u") {
		http.NotFound(w, r)
		return
	}
	key := strings.TrimSuffix(path, ".m3u")
	if key == "" {
		http.NotFound(w, r)
		return
	}

	sourceURL, ok := playlistMap[key]
	if !ok {
		// try slugified key (e.g. user sent "Lietuviski" -> lookup "lietuviski")
		sourceURL, ok = playlistMap[slugify(key)]
	}
	if !ok {
		http.NotFound(w, r)
		return
	}

	// HEAD: quick 200 so apps (e.g. IPTVX) can verify the URL
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "audio/x-mpegurl")
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("Request: single playlist %q, source: %s", key, sourceURL)

	channels, err := fetchChannelsFromURL(sourceURL)
	if err != nil {
		log.Printf("Failed to fetch source %s: %v", sourceURL, err)
		http.Error(w, "Failed to reach source", http.StatusBadGateway)
		return
	}

	if len(channels) == 0 {
		http.Error(w, "Source has no channels", http.StatusBadGateway)
		return
	}

	if validateStreams {
		channels = validateChannelsConcurrently(channels)
	}
	writeM3U(w, channels)
	log.Printf("Single playlist %q: %d channels", key, len(channels))
}

var tvgIDRe = regexp.MustCompile(`tvg-id="([^"]*)"`)

// parseTvgID extracts tvg-id from #EXTINF metadata for EPG channel matching
func parseTvgID(meta string) string {
	if m := tvgIDRe.FindStringSubmatch(meta); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// parseChannelNameFromMetadata extracts display name from #EXTINF:-1 tvg-id="..." , Name
func parseChannelNameFromMetadata(meta string) string {
	if idx := strings.LastIndex(meta, ","); idx >= 0 && idx+1 < len(meta) {
		return strings.TrimSpace(meta[idx+1:])
	}
	return "Channel"
}

// getEpgProgrammes returns EPG programmes for our channels; uses cache, rebuilds if expired
func getEpgProgrammes(channels []Channel) []epgProgramme {
	if epgURL == "" {
		return nil
	}
	epgCacheMu.RLock()
	if time.Since(epgCacheTime) < epgCacheTTL && len(epgProgrammes) > 0 {
		out := epgProgrammes
		epgCacheMu.RUnlock()
		return out
	}
	epgCacheMu.RUnlock()

	epgCacheMu.Lock()
	defer epgCacheMu.Unlock()
	if time.Since(epgCacheTime) < epgCacheTTL && len(epgProgrammes) > 0 {
		return epgProgrammes
	}

	// Build tvg-id -> our stream_id (1-based) map
	tvgToStreamID := make(map[string]string)
	for i, ch := range channels {
		tid := parseTvgID(ch.Metadata)
		if tid != "" {
			tvgToStreamID[tid] = strconv.Itoa(i + 1)
		}
	}
	if len(tvgToStreamID) == 0 {
		return nil
	}

	resp, err := http.Get(epgURL)
	if err != nil {
		log.Printf("EPG fetch failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var root xmltvRoot
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&root); err != nil {
		log.Printf("EPG parse failed: %v", err)
		return nil
	}

	var out []epgProgramme
	for _, p := range root.Programmes {
		sid, ok := tvgToStreamID[p.Channel]
		if !ok {
			continue
		}
		out = append(out, epgProgramme{
			Channel: sid,
			Start:   p.Start,
			Stop:    p.Stop,
			Title:   p.Title,
			Desc:    p.Desc,
		})
	}
	epgProgrammes = out
	epgCacheTime = time.Now()
	log.Printf("EPG: loaded %d programmes for xmltv", len(out))
	return epgProgrammes
}

// getXtreamChannels returns cached combined channel list; rebuilds if expired
func getXtreamChannels() []Channel {
	xtreamCacheMu.RLock()
	if time.Since(xtreamCacheTime) < xtreamCacheTTL && len(xtreamCache) > 0 {
		ch := xtreamCache
		xtreamCacheMu.RUnlock()
		return ch
	}
	xtreamCacheMu.RUnlock()

	xtreamCacheMu.Lock()
	defer xtreamCacheMu.Unlock()
	// double-check after acquiring write lock
	if time.Since(xtreamCacheTime) < xtreamCacheTTL && len(xtreamCache) > 0 {
		return xtreamCache
	}

	var channels []Channel
	for _, sourceURL := range sourceM3Us {
		ch, err := fetchChannelsFromURL(sourceURL)
		if err != nil {
			log.Printf("Xtream cache: failed to fetch %s: %v", sourceURL, err)
			continue
		}
		channels = append(channels, ch...)
	}
	if len(channels) == 0 {
		return nil
	}
	var result []Channel
	if validateStreams {
		result = validateChannelsConcurrently(channels)
		log.Printf("Xtream cache: built list of %d channels (validated)", len(result))
	} else {
		result = channels
		log.Printf("Xtream cache: built list of %d channels (no validation)", len(result))
	}
	xtreamCache = result
	xtreamCacheTime = time.Now()
	return xtreamCache
}

// xtreamPlayerAPIHandler serves Xtream Codes player_api.php (no auth, local use)
func xtreamPlayerAPIHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")
	if username == "" {
		username = "local"
	}
	if password == "" {
		password = "local"
	}

	action := r.URL.Query().Get("action")
	host := r.Host
	if r.TLS != nil {
		host = "https://" + host
	} else {
		host = "http://" + host
	}

	// No action: return user_info + server_info immediately (no channel fetch = fast)
	if action == "" {
		userInfo := map[string]interface{}{
			"username": username, "password": password,
			"message": "", "auth": 1, "status": "Active",
			"exp_date": nil, "is_trial": "0", "active_cons": 0,
			"created_at": "", "max_connections": 0,
			"allowed_output_formats": []string{"ts", "m3u8"},
		}
		serverInfo := map[string]interface{}{
			"url": host, "port": "8080", "server_protocol": "http",
			"timezone": "UTC", "timestamp_now": time.Now().Unix(),
			"time_now": time.Now().Format("2006-01-02 15:04:05"),
			"rtmp_port": "", "https_port": "",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user_info": userInfo, "server_info": serverInfo,
		})
		return
	}

	// Actions that need channel list (use cache; may be slow on first request if preload not done)
	channels := getXtreamChannels()
	if channels == nil {
		// Return 200 with empty list so app does not show "error retrieving"; user can retry
		w.Header().Set("Content-Type", "application/json")
		switch action {
		case "get_live_categories":
			json.NewEncoder(w).Encode([]interface{}{})
		case "get_live_streams":
			json.NewEncoder(w).Encode([]interface{}{})
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{})
		}
		return
	}

	switch action {
	case "get_live_categories":
		out := []map[string]interface{}{
			{"category_id": "1", "category_name": "Live", "parent_id": 0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
		return
	case "get_live_streams":
		out := make([]map[string]interface{}, 0, len(channels))
		for i, ch := range channels {
			name := parseChannelNameFromMetadata(ch.Metadata)
			out = append(out, map[string]interface{}{
				"num":                i + 1,
				"name":               name,
				"stream_type":        "live",
				"stream_id":          i + 1,
				"stream_icon":       "",
				"epg_channel_id":     "",
				"added":              "",
				"category_id":        "1",
				"tv_archive":         0,
				"direct_source":      "",
				"tv_archive_duration": 0,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
		return
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{})
		return
	}
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// xtreamXMLTVHandler serves xmltv.php with channel list and EPG programmes (if epg_url configured)
func xtreamXMLTVHandler(w http.ResponseWriter, r *http.Request) {
	channels := getXtreamChannels()
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<tv>\n"))
	if channels != nil {
		for i, ch := range channels {
			name := parseChannelNameFromMetadata(ch.Metadata)
			id := strconv.Itoa(i + 1)
			w.Write([]byte("  <channel id=\"" + id + "\">\n    <display-name>" + xmlEscape(name) + "</display-name>\n  </channel>\n"))
		}
		// EPG programmes (channel id = our stream_id)
		programmes := getEpgProgrammes(channels)
		for _, p := range programmes {
			title := xmlEscape(p.Title)
			desc := xmlEscape(p.Desc)
			w.Write([]byte("  <programme start=\"" + p.Start + "\" stop=\"" + p.Stop + "\" channel=\"" + p.Channel + "\">\n"))
			w.Write([]byte("    <title>" + title + "</title>\n"))
			if desc != "" {
				w.Write([]byte("    <desc>" + desc + "</desc>\n"))
			}
			w.Write([]byte("  </programme>\n"))
		}
	}
	w.Write([]byte("</tv>\n"))
}

// xtreamGetStreamHandler redirects get.php?stream_id=N to the channel stream URL
func xtreamGetStreamHandler(w http.ResponseWriter, r *http.Request) {
	streamIDStr := r.URL.Query().Get("stream_id")
	if streamIDStr == "" {
		http.NotFound(w, r)
		return
	}
	streamID, err := strconv.Atoi(streamIDStr)
	if err != nil || streamID < 1 {
		http.NotFound(w, r)
		return
	}
	channels := getXtreamChannels()
	if channels == nil || streamID > len(channels) {
		http.NotFound(w, r)
		return
	}
	url := channels[streamID-1].URL
	http.Redirect(w, r, url, http.StatusFound)
}

// xtreamLiveStreamHandler redirects /live/username/password/stream_id to the channel stream URL
func xtreamLiveStreamHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/live/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	streamIDStr := parts[len(parts)-1]
	streamID, err := strconv.Atoi(streamIDStr)
	if err != nil || streamID < 1 {
		http.NotFound(w, r)
		return
	}
	channels := getXtreamChannels()
	if channels == nil || streamID > len(channels) {
		http.NotFound(w, r)
		return
	}
	url := channels[streamID-1].URL
	http.Redirect(w, r, url, http.StatusFound)
}

func validateChannelsConcurrently(channels []Channel) []Channel {
	var wg sync.WaitGroup
	chInput := make(chan Channel, len(channels))
	chOutput := make(chan Channel, len(channels))

	// HTTP client with strict timeout
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	// Start workers: use GET and read only a little (many streams return 405 for HEAD)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ch := range chInput {
				req, err := http.NewRequest("GET", ch.URL, nil)
				if err != nil {
					continue
				}
				resp, err := client.Do(req)
				if err != nil {
					if resp != nil {
						resp.Body.Close()
					}
					continue
				}
				if resp.StatusCode == http.StatusOK {
					// Consume only first bytes so we don't download the full stream
					io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
					chOutput <- ch
				}
				resp.Body.Close()
			}
		}()
	}

	// Send tasks to workers
	for _, ch := range channels {
		chInput <- ch
	}
	close(chInput)

	// Wait for all workers to finish
	wg.Wait()
	close(chOutput)

	// Collect results
	var valid []Channel
	for ch := range chOutput {
		valid = append(valid, ch)
	}
	
	return valid
}
