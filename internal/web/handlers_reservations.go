package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/simon/dewey/internal/store"
)

// redirectTarget returns the page to send the user back to after a
// prolong/reservation action — the forms on /loans, /students/{id} and
// /books/{id} all carry a hidden "redirect" field so the result banner
// shows up wherever the action was triggered from.
func redirectTarget(r *http.Request, fallback string) string {
	if v := r.FormValue("redirect"); v != "" {
		return v
	}
	return fallback
}

// redirectWithMessage does a PRG redirect carrying a success/error message
// as a query param, so the target page's GET handler can render it inline.
func redirectWithMessage(w http.ResponseWriter, r *http.Request, path string, ok bool, msg string) {
	q := url.Values{}
	if ok {
		q.Set("success", msg)
	} else {
		q.Set("error", msg)
	}
	http.Redirect(w, r, path+"?"+q.Encode(), http.StatusSeeOther)
}

func (s *Server) handleReservationsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	bookID, _ := strconv.ParseInt(r.FormValue("book_id"), 10, 64)
	studentID, _ := strconv.ParseInt(r.FormValue("student_id"), 10, 64)
	redirect := redirectTarget(r, "/loans")

	if bookID == 0 || studentID == 0 {
		redirectWithMessage(w, r, redirect, false, "Bitte Buch und Schüler auswählen.")
		return
	}

	if _, err := s.store.CreateReservation(r.Context(), bookID, studentID); err != nil {
		msg := "Vormerkung fehlgeschlagen: " + err.Error()
		if errors.Is(err, store.ErrReservationExists) {
			msg = "Dieser Schüler hat das Buch bereits vorgemerkt."
		}
		redirectWithMessage(w, r, redirect, false, msg)
		return
	}
	redirectWithMessage(w, r, redirect, true, "Vormerkung gespeichert.")
}

func (s *Server) handleReservationsCancel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect := redirectTarget(r, "/loans")

	if err := s.store.CancelReservation(r.Context(), id); err != nil {
		redirectWithMessage(w, r, redirect, false, "Vormerkung konnte nicht storniert werden: "+err.Error())
		return
	}
	redirectWithMessage(w, r, redirect, true, "Vormerkung storniert.")
}
