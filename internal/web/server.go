// Package web is the house Pi's HTTP server: the captive portal, the app, its
// JSON API and a server-sent-events stream (spec §8).
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/auth"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
	"github.com/johnusher/batnet/internal/state"
)

//go:embed app
var appFS embed.FS

// Config wires a server to its house node.
type Config struct {
	House        string
	Author       *ledger.Author
	State        *state.State
	Publish      func(*ledger.Entry)
	Now          func() time.Time
	SecureCookie bool
	HighPerHour  int
}

// Server serves one house.
type Server struct {
	cfg      Config
	sessions *auth.Sessions
	lockout  *auth.Lockout
	high     *prio.Limiter
	mu       sync.Mutex // serialises writes to the log
	hub      *hub
}

// New returns a server.
func New(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.HighPerHour == 0 {
		cfg.HighPerHour = 3
	}
	if cfg.Publish == nil {
		cfg.Publish = func(*ledger.Entry) {}
	}
	return &Server{cfg: cfg, sessions: auth.NewSessions(), lockout: auth.NewLockout(), high: prio.NewLimiter(cfg.HighPerHour), hub: newHub()}
}

// Write appends an entry from this house, applies it, publishes it, and tells listeners.
func (s *Server) Write(t ledger.Type, p prio.Level, body any) (*ledger.Entry, error) {
	s.mu.Lock()
	e, err := s.cfg.Author.Append(t, uint8(p), body)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	s.cfg.State.Apply(e)
	s.cfg.Publish(e)
	s.notify(e)
	return e, nil
}

// OnEntry takes an entry that arrived from the mesh or the radio.
func (s *Server) OnEntry(e *ledger.Entry) {
	s.cfg.State.Apply(e)
	s.notify(e)
}

func (s *Server) notify(e *ledger.Entry) {
	switch e.Type {
	case ledger.TypeMsgPost, ledger.TypeAlert:
		s.hub.message(s.cfg.State, state.AccountID(e.Key))
	default:
		s.hub.state()
	}
}

// Bootstrap is first boot: announce the house, create its approved admin and groups.
func (s *Server) Bootstrap(admin, pin string, groups []string) (string, error) {
	if !auth.ValidPIN(pin) {
		return "", fmt.Errorf("web: the admin PIN must be 6 digits")
	}
	if _, err := s.Write(ledger.TypeNodeAnnounce, prio.Housekeeping, map[string]string{"house": s.cfg.House}); err != nil {
		return "", err
	}
	h, err := auth.HashPIN(pin)
	if err != nil {
		return "", err
	}
	req, err := s.Write(ledger.TypeAccountRequest, prio.Normal, map[string]any{"name": admin, "pinHash": h, "admin": true})
	if err != nil {
		return "", err
	}
	id := state.AccountID(req.Key)
	if _, err := s.Write(ledger.TypeAccountApprove, prio.Normal, map[string]string{"account": id}); err != nil {
		return "", err
	}
	for _, g := range groups {
		slug := strings.ToLower(strings.ReplaceAll(g, " ", "-"))
		if _, err := s.Write(ledger.TypeGroupCreate, prio.Housekeeping, map[string]string{"id": slug, "name": g}); err != nil {
			return "", err
		}
	}
	return id, nil
}

// Handler routes every request.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.captive(mux)
	s.api(mux)
	sub, _ := fs.Sub(appFS, "app")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))
	return securityHeaders(mux)
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'")
		h.ServeHTTP(w, r)
	})
}

// ---- server-sent events ----

type listener struct {
	account string
	ch      chan []byte
}

type hub struct {
	mu        sync.Mutex
	listeners map[*listener]bool
}

func newHub() *hub { return &hub{listeners: map[*listener]bool{}} }

func (h *hub) add(account string) *listener {
	l := &listener{account: account, ch: make(chan []byte, 64)}
	h.mu.Lock()
	h.listeners[l] = true
	h.mu.Unlock()
	return l
}

func (h *hub) remove(l *listener) {
	h.mu.Lock()
	delete(h.listeners, l)
	h.mu.Unlock()
}

func (h *hub) send(l *listener, b []byte) {
	select {
	case l.ch <- b:
	default: // a slow phone misses events; it reloads conversations on reconnect
	}
}

func (h *hub) message(st *state.State, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := st.MessageByID(id)
	if m == nil {
		return
	}
	for l := range h.listeners {
		if st.Visible(m, l.account) {
			b, _ := json.Marshal(m)
			h.send(l, []byte("event: message\ndata: "+string(b)+"\n\n"))
		}
	}
}

func (h *hub) state() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for l := range h.listeners {
		h.send(l, []byte("event: state\ndata: {}\n\n"))
	}
}
