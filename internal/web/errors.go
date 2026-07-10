package web

import (
	"errors"
	"net/http"

	"github.com/simon/dewey/internal/store"
)

// writeError responds with a status code and a safe, German message derived
// from err, logging the full error server-side first. Sentinel domain
// errors from the store package get a specific status and friendly text;
// anything else — SQL failures, driver errors — gets a generic message, so
// raw internal error text never reaches the browser (a librarian shouldn't
// see driver strings).
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, message := s.friendlyError(r, err)
	http.Error(w, message, status)
}

// writeErrorPrefixed is writeError with a leading "what were we doing"
// prefix, e.g. "Buch konnte nicht aktualisiert werden".
func (s *Server) writeErrorPrefixed(w http.ResponseWriter, r *http.Request, prefix string, err error) {
	status, message := s.friendlyError(r, err)
	http.Error(w, prefix+": "+message, status)
}

// friendlyError maps err to an HTTP status and a safe, German message,
// logging the full error server-side first if it isn't a recognized domain
// error (i.e. it's an unexpected internal/db failure). Handlers that render
// a flash-message page instead of an HTTP error response (e.g.
// studentsWithError) use this directly to avoid leaking err's raw text.
func (s *Server) friendlyError(r *http.Request, err error) (status int, message string) {
	status, message = errorResponse(err)
	if status == http.StatusInternalServerError {
		s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	}
	return status, message
}

func errorResponse(err error) (status int, message string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound, "Nicht gefunden."
	case errors.Is(err, store.ErrBookOnLoan):
		return http.StatusConflict, "Es sind noch Exemplare ausgeliehen."
	case errors.Is(err, store.ErrClassOnLoan):
		return http.StatusConflict, "Diese Klasse hat Schüler mit einer laufenden Ausleihe."
	case errors.Is(err, store.ErrStudentOnLoan):
		return http.StatusConflict, "Dieser Schüler hat eine laufende Ausleihe."
	case errors.Is(err, store.ErrLoanReserved):
		return http.StatusConflict, "Ein anderer Schüler hat dieses Buch vorgemerkt."
	case errors.Is(err, store.ErrInvalidDueDate):
		return http.StatusBadRequest, "Das Rückgabedatum muss in der Zukunft liegen."
	case errors.Is(err, store.ErrReservationExists):
		return http.StatusConflict, "Dieser Schüler hat das Buch bereits vorgemerkt."
	default:
		return http.StatusInternalServerError, "Ein unerwarteter Fehler ist aufgetreten. Bitte versuchen Sie es erneut."
	}
}
