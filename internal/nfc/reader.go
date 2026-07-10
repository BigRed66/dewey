package nfc

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// Reader liest TAG:-Zeilen vom seriellen Port des ESP8266 und verteilt die UIDs.
type Reader struct {
	port   string
	baud   int
	logger *slog.Logger

	mu        sync.Mutex
	subs      map[chan string]struct{}
	connSubs  map[chan bool]struct{}
	connected bool
	lastUID   string
	lastAt    time.Time
	cancel    context.CancelFunc
}

func New(port string, logger *slog.Logger) *Reader {
	return &Reader{
		port:     port,
		baud:     115200,
		logger:   logger,
		subs:     make(map[chan string]struct{}),
		connSubs: make(map[chan bool]struct{}),
	}
}

// Start startet den Reader als Hintergrund-Goroutine. Ruft Stop auf allen laufenden Goroutinen auf.
func (r *Reader) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.cancel = cancel
	r.mu.Unlock()
	go r.run(ctx)
}

// Stop beendet den laufenden Reader.
func (r *Reader) Stop() {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.mu.Unlock()
}

// Port gibt den konfigurierten seriellen Port zurück.
func (r *Reader) Port() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.port
}

func (r *Reader) run(ctx context.Context) {
	for {
		if err := r.readLoop(ctx); err != nil {
			r.logger.Warn("nfc read loop failed, retrying", "error", err, "retry_in", "5s")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (r *Reader) readLoop(ctx context.Context) error {
	r.mu.Lock()
	port := r.port
	r.mu.Unlock()

	sp, err := serial.Open(port, &serial.Mode{BaudRate: r.baud})
	if err != nil {
		return err
	}
	defer sp.Close()

	// 100ms Timeout damit wir ctx.Done() regelmäßig prüfen können
	if err := sp.SetReadTimeout(100 * time.Millisecond); err != nil {
		return err
	}

	r.logger.Info("nfc connected", "port", port)
	r.setConnected(true)
	defer r.setConnected(false)

	buf := make([]byte, 64)
	var line []byte

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, err := sp.Read(buf)
		if n == 0 && err != nil {
			// Timeout → einfach weiter warten
			continue
		}
		if err != nil {
			return err
		}

		for _, b := range buf[:n] {
			switch b {
			case '\r':
				// ignorieren
			case '\n':
				s := strings.TrimSpace(string(line))
				line = line[:0]
				if !strings.HasPrefix(s, "TAG:") {
					continue
				}
				uid := strings.TrimSpace(strings.TrimPrefix(s, "TAG:"))
				if uid == "" {
					continue
				}
				r.logger.Info("nfc tag read", "uid", uid)
				r.broadcast(uid)
			default:
				line = append(line, b)
			}
		}
	}
}

func (r *Reader) broadcast(uid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastUID = uid
	r.lastAt = time.Now()
	for ch := range r.subs {
		select {
		case ch <- uid:
		default:
		}
	}
}

// Subscribe gibt einen Channel zurück der bei jedem neuen Tag eine UID erhält.
func (r *Reader) Subscribe() (ch chan string, unsub func()) {
	ch = make(chan string, 4)
	r.mu.Lock()
	r.subs[ch] = struct{}{}
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subs, ch)
		r.mu.Unlock()
	}
}

// Connected meldet, ob der serielle Port gerade tatsächlich geöffnet ist —
// im Gegensatz dazu ist ein *Reader bereits dann nicht nil, wenn nur ein
// Port konfiguriert wurde (siehe Settings), auch wenn die Hardware fehlt.
func (r *Reader) Connected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.connected
}

// SubscribeStatus gibt einen Channel zurück, der den aktuellen
// Verbindungsstatus sofort und danach bei jeder Änderung erhält.
func (r *Reader) SubscribeStatus() (ch chan bool, unsub func()) {
	ch = make(chan bool, 4)
	r.mu.Lock()
	r.connSubs[ch] = struct{}{}
	ch <- r.connected
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.connSubs, ch)
		r.mu.Unlock()
	}
}

func (r *Reader) setConnected(v bool) {
	r.mu.Lock()
	if r.connected == v {
		r.mu.Unlock()
		return
	}
	r.connected = v
	subs := make([]chan bool, 0, len(r.connSubs))
	for ch := range r.connSubs {
		subs = append(subs, ch)
	}
	r.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- v:
		default:
		}
	}
}

// LastTag gibt die zuletzt gelesene UID und den Zeitpunkt zurück.
func (r *Reader) LastTag() (string, time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastUID, r.lastAt
}
