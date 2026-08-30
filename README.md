# Dewey

A small library management system for a primary school. It runs as a single
self-contained binary, stores everything in a local SQLite file, and is
operated through a plain server-rendered web UI in the browser.

Books are checked in and out by scanning an NFC/RFID tag glued into the book,
using a cheap ESP8266 + RC522 reader that talks to the server over USB serial.
Scanning a tag pops up the right dialog straight away: an unknown tag offers to
register the book, a shelved book offers to lend it, a lent book offers to take
it back.

**The user interface is in German**, since that is what the school it was
built for needed.

## Honest disclaimer

**This project was written essentially entirely by AI** (Claude). I described
what I needed, reviewed the result, and iterated until it worked. I did not
hand-write the code and I would not claim it as a model of Go craftsmanship.

That said: **it works, and it has been in real use for the case I built it
for.** It is published here so that anyone with the same problem — a small
school library that needs lending, pupils, classes and tag scanning without
paying for commercial software — can take it and not have to spend the effort
again. Fork it, change it, run it. No warranty of any kind.

## What it does

- **Classes and pupils** — maintain classes and their pupils, ordered by
  surname, with search and per-class filtering.
- **Books and copies** — one book record can have several physical copies,
  each with its own NFC tag and shelf position.
- **ISBN lookup** — enter an ISBN and the book's title, author and cover are
  fetched automatically from the Google Books API.
- **Lending** — check out, return and extend loans; see what is out and to whom.
- **Reservations** — queue a pupil for a copy that is currently lent out.
- **Shelf positions** — record where in the library a copy physically lives.
- **CSV import** — bulk-import pupils and classes from a spreadsheet export.
- **NFC scanning** — live tag events streamed to the browser via
  Server-Sent Events, so any open page reacts to a scan immediately.

## Requirements

- Go 1.26 or newer (no cgo needed — the SQLite driver is pure Go)
- Optional, for tag scanning: a NodeMCU ESP8266 with an RC522 RFID module
- Optional, for the e2e tests: Node.js

## Running it

```bash
go build -o dewey .
./dewey
```

Then open <http://127.0.0.1:8080>.

The database is created and migrated automatically on first start.

### Options

| Flag | Environment variable | Default | Meaning |
|---|---|---|---|
| `-addr` | `DEWEY_ADDR` | `127.0.0.1:8080` | Listen address |
| `-db` | `DEWEY_DB_PATH` | `dewey.db` | Path to the SQLite file |
| `-seed` | `DEWEY_SEED=1` | off | Fill the database with mock data on startup |

To try it out without touching real data, start it with a throwaway database:

```bash
./dewey -db /tmp/demo.db -seed
```

The Google Books API key and the NFC serial port are configured at runtime
under **Einstellungen** (Settings) in the web UI and stored in the database —
they are not build-time settings.

## The NFC reader

The `firmware/` directory holds a small PlatformIO sketch for a NodeMCU v2
(ESP8266) with an MFRC522 reader and a buzzer. It prints one `TAG:<uid>` line
per scan over serial at 115200 baud; the server reads that and forwards it to
the browser. Any device that emits the same line format would work just as well.

```bash
cd firmware
pio run -t upload
```

Pick the serial port in the settings page afterwards.

## Development

```bash
go test ./...      # unit tests
npm install        # once
npm run test:e2e   # Playwright end-to-end tests
```

The e2e suite starts its own server against a temporary seeded database, so it
never touches your real one.

## Layout

```
main.go              startup, config, graceful shutdown
internal/db          SQLite connection and schema migrations
internal/store       data access per entity
internal/web         HTTP handlers and html/template views
internal/nfc         serial reader, tag event fan-out
internal/booksapi    ISBN lookup against Google Books
internal/csvimport   CSV parsing for pupil/class import
internal/seed        mock data for demos and tests
firmware/            ESP8266 + RC522 sketch
e2e/                 Playwright tests
```

## Licence

None specified — treat it as "do what you like with it, at your own risk".
