package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/simon/dewey/internal/db"
	"github.com/simon/dewey/internal/nfc"
	"github.com/simon/dewey/internal/seed"
	"github.com/simon/dewey/internal/store"
	"github.com/simon/dewey/internal/web"
)

// Config holds everything main needs to start the app, gathered from flags
// and environment variables in one place so the rest of startup is linear
// and testable independent of flag.Parse/os.Getenv.
type Config struct {
	Addr     string
	DBPath   string
	SeedMock bool
}

func loadConfig() Config {
	addr := flag.String("addr", envOr("DEWEY_ADDR", "127.0.0.1:8080"), "listen address")
	dbPath := flag.String("db", envOr("DEWEY_DB_PATH", "dewey.db"), "path to sqlite database")
	seedMock := flag.Bool("seed", os.Getenv("DEWEY_SEED") == "1", "seed the database with mock data on startup")
	flag.Parse()
	return Config{Addr: *addr, DBPath: *dbPath, SeedMock: *seedMock}
}

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(cfg, logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(cfg Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	s := store.New(database)

	if cfg.SeedMock {
		if err := seed.Run(ctx, database); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
		logger.Info("Mockdaten eingespielt")
	}

	// Load NFC port from settings
	nfcPort, _ := s.GetSetting(ctx, "nfc_port")
	var nfcReader *nfc.Reader
	if nfcPort != "" {
		nfcReader = nfc.New(nfcPort, logger)
		nfcReader.Start(ctx)
		logger.Info("NFC-Reader gestartet", "port", nfcPort)
	} else {
		logger.Info("NFC nicht konfiguriert – über Einstellungen einrichten")
	}

	handler := web.NewServer(s, nfcReader, logger)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// No WriteTimeout: /nfc/events is a Server-Sent-Events stream that
		// holds its connection open indefinitely by design.
		IdleTimeout: 60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server: %w", err)
		}
	case <-ctx.Done():
		logger.Info("Beende Server …")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("Server shutdown", "error", err)
		}
	}

	if nfcReader != nil {
		nfcReader.Stop()
	}
	logger.Info("Server beendet")
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
