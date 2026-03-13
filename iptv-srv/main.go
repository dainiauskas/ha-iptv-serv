package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	port    = ":8080"
	workers = 20 // number of concurrent HTTP check workers
)

var (
	sourceM3Us   []string            // for combined playlist
	playlistMap  map[string]string   // slug or "0","1"... -> URL for single playlist lookup
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
	Playlists []PlaylistEntry `json:"playlists"`
}

func main() {
	loadConfig()
	http.HandleFunc("/playlist.m3u", combinedPlaylistHandler)
	http.HandleFunc("/playlist/", singlePlaylistHandler)

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

	log.Println("Request: combined playlist – fetching and filtering sources...")

	var channels []Channel
	for _, sourceURL := range sourceM3Us {
		ch, err := fetchChannelsFromURL(sourceURL)
		if err != nil {
			log.Printf("Failed to fetch source %s: %v", sourceURL, err)
			continue
		}
		channels = append(channels, ch...)
	}

	if len(channels) == 0 {
		http.Error(w, "Failed to reach any sources", http.StatusBadGateway)
		return
	}

	validChannels := validateChannelsConcurrently(channels)
	writeM3U(w, validChannels)
	log.Printf("Combined playlist: %d working channels out of %d", len(validChannels), len(channels))
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

	validChannels := validateChannelsConcurrently(channels)
	writeM3U(w, validChannels)
	log.Printf("Single playlist %q: %d working channels out of %d", key, len(validChannels), len(channels))
}

func validateChannelsConcurrently(channels []Channel) []Channel {
	var wg sync.WaitGroup
	chInput := make(chan Channel, len(channels))
	chOutput := make(chan Channel, len(channels))

	// HTTP client with strict timeout
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ch := range chInput {
				// HEAD request to check stream without downloading
				req, err := http.NewRequest("HEAD", ch.URL, nil)
				if err != nil {
					continue
				}
				
				resp, err := client.Do(req)
				if err == nil && resp.StatusCode == http.StatusOK {
					chOutput <- ch // stream is reachable
				}
				if resp != nil {
					resp.Body.Close()
				}
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
