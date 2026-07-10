package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.bug.st/serial"
	"github.com/simon/dewey/internal/nfc"
)

type settingsData struct {
	NFCPort      string
	NFCPorts     []string
	NFCConnected bool
	APIKey       string
	APIKeySet    bool
	NFCSuccess   string
	APISuccess   string
	NFCError     string
	APIError     string
}

func listSerialPorts() []string {
	ports, _ := serial.GetPortsList()
	return ports
}

func containsPort(ports []string, port string) bool {
	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false
}

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nfcPort, _ := s.store.GetSetting(ctx, "nfc_port")
	apiKey, _ := s.store.GetSetting(ctx, "google_books_key")

	ports := listSerialPorts()
	if nfcPort != "" && !containsPort(ports, nfcPort) {
		ports = append([]string{nfcPort}, ports...)
	}

	reader := s.getNFC()
	d := settingsData{
		NFCPort:      nfcPort,
		NFCPorts:     ports,
		NFCConnected: reader != nil && reader.Connected(),
		APIKey:       apiKey,
		APIKeySet:    apiKey != "",
	}

	switch r.URL.Query().Get("saved") {
	case "nfc":
		d.NFCSuccess = "NFC-Einstellungen gespeichert."
	case "api":
		d.APISuccess = "API-Schlüssel gespeichert."
	}

	render(w, "settings", d)
}

func (s *Server) handleSettingsNFCSave(w http.ResponseWriter, r *http.Request) {
	port := strings.TrimSpace(r.FormValue("port"))

	if err := s.store.SetSetting(r.Context(), "nfc_port", port); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	old := s.getNFC()
	if old != nil {
		old.Stop()
	}

	var newReader *nfc.Reader
	if port != "" {
		newReader = nfc.New(port, s.logger)
		// context.Background() is fine here: Reader owns its lifecycle via
		// Stop(), called both above (swapping readers) and during graceful
		// app shutdown — it doesn't need to react to a request context or a
		// long-lived context stored on Server.
		newReader.Start(context.Background())
	}
	s.setNFC(newReader)

	http.Redirect(w, r, "/settings?saved=nfc", http.StatusSeeOther)
}

func (s *Server) handleSettingsNFCPorts(w http.ResponseWriter, r *http.Request) {
	ports := listSerialPorts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"ports": ports})
}

func (s *Server) handleSettingsNFCTest(w http.ResponseWriter, r *http.Request) {
	port := strings.TrimSpace(r.FormValue("port"))

	type result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}

	w.Header().Set("Content-Type", "application/json")

	if port == "" {
		json.NewEncoder(w).Encode(result{OK: false, Error: "Kein Port ausgewählt"})
		return
	}

	sp, err := serial.Open(port, &serial.Mode{BaudRate: 115200})
	if err != nil {
		json.NewEncoder(w).Encode(result{OK: false, Error: err.Error()})
		return
	}
	sp.Close()
	json.NewEncoder(w).Encode(result{OK: true})
}

func (s *Server) handleSettingsAPISave(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.FormValue("key"))

	// Don't overwrite an existing key with an empty submission
	if key == "" {
		existing, _ := s.store.GetSetting(r.Context(), "google_books_key")
		if existing != "" {
			http.Redirect(w, r, "/settings?saved=api", http.StatusSeeOther)
			return
		}
	}

	if err := s.store.SetSetting(r.Context(), "google_books_key", key); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings?saved=api", http.StatusSeeOther)
}

func (s *Server) handleSettingsAPICheck(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.FormValue("key"))

	type result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}

	w.Header().Set("Content-Type", "application/json")

	if key == "" {
		json.NewEncoder(w).Encode(result{OK: false, Error: "Kein Schlüssel eingegeben"})
		return
	}

	apiURL := "https://www.googleapis.com/books/v1/volumes?q=isbn:9783551551672&key=" + url.QueryEscape(key)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, nil)
	if err != nil {
		json.NewEncoder(w).Encode(result{OK: false, Error: err.Error()})
		return
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		json.NewEncoder(w).Encode(result{OK: false, Error: fmt.Sprintf("Nicht erreichbar: %v", err)})
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		json.NewEncoder(w).Encode(result{OK: true})
	case 400, 403:
		json.NewEncoder(w).Encode(result{OK: false, Error: fmt.Sprintf("Ungültiger Schlüssel (HTTP %d)", resp.StatusCode)})
	case 429:
		json.NewEncoder(w).Encode(result{OK: false, Error: "Anfragelimit erreicht"})
	default:
		json.NewEncoder(w).Encode(result{OK: false, Error: fmt.Sprintf("Unerwarteter Status: %d", resp.StatusCode)})
	}
}
