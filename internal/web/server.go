package web

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/simon/dewey/internal/nfc"
	"github.com/simon/dewey/internal/store"
)

type Server struct {
	store *store.Store
	ctx   context.Context // long-lived context for background goroutines
	mu    sync.RWMutex
	nfc   *nfc.Reader // nil wenn kein NFC-Reader konfiguriert; protected by mu
}

func (s *Server) getNFC() *nfc.Reader {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nfc
}

func (s *Server) setNFC(r *nfc.Reader) {
	s.mu.Lock()
	s.nfc = r
	s.mu.Unlock()
}

func NewServer(s *store.Store, ctx context.Context, nfcReader *nfc.Reader) http.Handler {
	srv := &Server{store: s, ctx: ctx, nfc: nfcReader}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/classes", http.StatusSeeOther)
	})

	r.Get("/favicon.ico", serveStatic("static/favicon.ico", "image/x-icon"))
	r.Get("/favicon.svg", serveStatic("static/favicon.svg", "image/svg+xml"))
	r.Get("/apple-touch-icon.png", serveStatic("static/apple-touch-icon.png", "image/png"))

	r.Get("/classes", srv.handleClassesList)
	r.Post("/classes", srv.handleClassesCreate)
	r.Post("/classes/{id}", srv.handleClassesUpdate)
	r.Post("/classes/{id}/delete", srv.handleClassesDelete)

	r.Get("/students", srv.handleStudentsList)
	r.Post("/students", srv.handleStudentsCreate)
	r.Get("/students/{id}", srv.handleStudentDetail)
	r.Post("/students/{id}", srv.handleStudentsUpdate)
	r.Post("/students/{id}/delete", srv.handleStudentsDelete)

	r.Get("/books", srv.handleBooksList)
	r.Get("/books/new", srv.handleBooksNew)
	r.Post("/books/lookup", srv.handleBooksLookup)
	r.Post("/books", srv.handleBooksCreate)
	r.Get("/books/{id}", srv.handleBookDetail)
	r.Post("/books/{id}", srv.handleBooksUpdate)
	r.Post("/books/{id}/delete", srv.handleBooksDelete)
	r.Post("/copies/{id}/delete", srv.handleCopyDelete)

	r.Get("/loans", srv.handleLoansList)
	r.Get("/loans/checkout", srv.handleLoansCheckoutForm)
	r.Post("/loans/checkout", srv.handleLoansCheckout)
	r.Post("/loans/return", srv.handleLoansReturn)
	r.Post("/loans/{id}/prolong", srv.handleLoanProlong)

	r.Post("/reservations", srv.handleReservationsCreate)
	r.Post("/reservations/{id}/cancel", srv.handleReservationsCancel)

	r.Get("/nfc/events", srv.handleNFCEvents)
	r.Get("/nfc/scan", srv.handleNFCScan)

	r.Get("/settings", srv.handleSettingsGet)
	r.Post("/settings/nfc", srv.handleSettingsNFCSave)
	r.Get("/settings/nfc/ports", srv.handleSettingsNFCPorts)
	r.Post("/settings/nfc/test", srv.handleSettingsNFCTest)
	r.Post("/settings/api", srv.handleSettingsAPISave)
	r.Post("/settings/api/check", srv.handleSettingsAPICheck)

	return r
}
