package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/simon/dewey/internal/db"
	"github.com/simon/dewey/internal/nfc"
	"github.com/simon/dewey/internal/seed"
	"github.com/simon/dewey/internal/store"
	"github.com/simon/dewey/internal/web"
)

func main() {
	addr := flag.String("addr", envOr("DEWEY_ADDR", "127.0.0.1:8080"), "listen address")
	dbPath := flag.String("db", envOr("DEWEY_DB_PATH", "dewey.db"), "path to sqlite database")
	seedMock := flag.Bool("seed", os.Getenv("DEWEY_SEED") == "1", "seed the database with mock data on startup")
	flag.Parse()

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	s := store.New(database)
	ctx := context.Background()

	if *seedMock {
		if err := seed.Run(ctx, database); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Printf("info: Mockdaten eingespielt")
	}

	// Load NFC port from settings
	nfcPort, _ := s.GetSetting(ctx, "nfc_port")
	var nfcReader *nfc.Reader
	if nfcPort != "" {
		nfcReader = nfc.New(nfcPort)
		nfcReader.Start(ctx)
		log.Printf("info: NFC-Reader gestartet auf %s", nfcPort)
	} else {
		log.Printf("info: NFC nicht konfiguriert – über Einstellungen einrichten")
	}

	handler := web.NewServer(s, ctx, nfcReader)

	log.Printf("listening on http://%s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
