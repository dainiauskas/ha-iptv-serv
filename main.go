package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sourceM3U = "https://iptv-org.github.io/iptv/languages/rus.m3u"
	port      = ":8080"
	workers   = 20 // Optimalus sinchroninių HTTP užklausų skaičius
)

type Channel struct {
	Metadata string
	URL      string
}

func main() {
	http.HandleFunc("/playlist.m3u", playlistHandler)

	fmt.Printf("Vietinis IPTV tarpinis serveris paleistas. Klausoma prievado %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func playlistHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Gauta užklausa iš Apple TV. Pradedamas šaltinio parsiuntimas ir filtravimas...")

	resp, err := http.Get(sourceM3U)
	if err != nil {
		http.Error(w, "Nepavyko pasiekti šaltinio", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var channels []Channel
	scanner := bufio.NewScanner(resp.Body)
	var currentMeta string

	// 1. Parsuojame M3U failą srautiniu būdu
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXTINF") {
			currentMeta = line
		} else if strings.HasPrefix(line, "http") && currentMeta != "" {
			channels = append(channels, Channel{Metadata: currentMeta, URL: line})
			currentMeta = "" // Atstatome sekančiam kanalui
		}
	}

	// 2. Filtruojame kanalus naudojant darbininkų telkinį (Worker Pool)
	validChannels := validateChannelsConcurrently(channels)

	// 3. Generuojame atsaką
	w.Header().Set("Content-Type", "audio/x-mpegurl")
	w.Write([]byte("#EXTM3U\n"))
	for _, ch := range validChannels {
		w.Write([]byte(ch.Metadata + "\n" + ch.URL + "\n"))
	}

	log.Printf("Apdorojimas baigtas. Atiduota veikiančių kanalų: %d iš %d", len(validChannels), len(channels))
}

func validateChannelsConcurrently(channels []Channel) []Channel {
	var wg sync.WaitGroup
	chInput := make(chan Channel, len(channels))
	chOutput := make(chan Channel, len(channels))

	// Konfigūruojamas HTTP klientas su griežtu Timeout
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	// Paleidžiame darbininkus (Workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ch := range chInput {
				// Atliekame HEAD užklausą – siunčiame tik antraštes, kad taupytume srautą
				req, err := http.NewRequest("HEAD", ch.URL, nil)
				if err != nil {
					continue
				}

				resp, err := client.Do(req)
				if err == nil && resp.StatusCode == http.StatusOK {
					chOutput <- ch // Srautas veikia
				}
				if resp != nil {
					resp.Body.Close()
				}
			}
		}()
	}

	// Paskirstome užduotis
	for _, ch := range channels {
		chInput <- ch
	}
	close(chInput)

	// Laukiame, kol visi darbininkai baigs darbą
	wg.Wait()
	close(chOutput)

	// Surenkame rezultatus
	var valid []Channel
	for ch := range chOutput {
		valid = append(valid, ch)
	}

	return valid
}
