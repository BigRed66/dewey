package web

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleNFCScan(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uid")
	if uid == "" {
		http.Error(w, "uid fehlt", http.StatusBadRequest)
		return
	}

	result, err := s.store.ScanByNFC(r.Context(), uid)
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	type response struct {
		Found       bool   `json:"found"`
		BookID      int64  `json:"book_id,omitempty"`
		Title       string `json:"title,omitempty"`
		Author      string `json:"author,omitempty"`
		CoverURL    string `json:"cover_url,omitempty"`
		NFCTagID    string `json:"nfc_tag_id,omitempty"`
		OnLoan      bool   `json:"on_loan"`
		StudentName string `json:"student_name,omitempty"`
		DueDate     string `json:"due_date,omitempty"`
		DaysLeft    int    `json:"days_left,omitempty"`
		DaysOverdue int    `json:"days_overdue,omitempty"`
		Overdue     bool   `json:"overdue,omitempty"`
	}

	if !result.Found {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{Found: false})
		return
	}

	resp := response{
		Found:    true,
		BookID:   result.Book.ID,
		Title:    result.Book.Title,
		Author:   result.Book.Author,
		CoverURL: result.Book.CoverURL,
		NFCTagID: result.Copy.NFCTagID,
		OnLoan:   result.OnLoan,
	}
	if result.OnLoan {
		resp.StudentName = result.StudentName
		resp.DueDate = result.DueAt.Format("02.01.2006")
		resp.DaysLeft = result.DaysLeft
		resp.DaysOverdue = result.DaysOverdue
		resp.Overdue = result.Overdue
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleNFCEvents streamt RFID-Tag-UIDs als Server-Sent Events.
// Der Browser auf books/new lauscht darauf und füllt das Tag-ID-Feld automatisch.
func (s *Server) handleNFCEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	nfcReader := s.getNFC()
	if nfcReader == nil {
		// No reader configured — hold connection with a slow retry so the
		// browser doesn't hammer the server (Firefox ignores JS backoff on 503).
		fmt.Fprintf(w, "retry: 10000\n\n")
		flusher.Flush()
		<-r.Context().Done()
		return
	}

	tagCh, unsubTag := nfcReader.Subscribe()
	defer unsubTag()

	// A reader being configured doesn't mean the serial port is actually
	// open — stream the real connection state so the browser doesn't show
	// "ready" while the hardware is unplugged or unreachable.
	statusCh, unsubStatus := nfcReader.SubscribeStatus()
	defer unsubStatus()

	for {
		select {
		case connected := <-statusCh:
			if connected {
				fmt.Fprintf(w, "event: nfc_ready\ndata: ok\n\n")
			} else {
				fmt.Fprintf(w, "event: nfc_disconnected\ndata: ok\n\n")
			}
			flusher.Flush()
		case uid := <-tagCh:
			fmt.Fprintf(w, "data: %s\n\n", uid)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
