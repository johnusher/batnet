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
	high     *prio.Limiter // High posts per person per hour
	posts    *prio.Limiter // posts of any kind per person per hour
	tries    *prio.Limiter // sign-in attempts per address per hour, whatever the name
	mu       sync.Mutex    // serialises writes to the log
	hub      *hub
	argon    chan struct{} // at most 2 PIN hashes at once: each takes 19 MiB, a Pi Zero has 512 MB

	signupMu sync.Mutex
	signups  map[string][]time.Time // by client address; "" is the whole house

	clientMu sync.Mutex
	clients  map[string]clientPost // account + "\x00" + clientId → the message it made
}

// clientPost remembers which message a phone's clientId made, so a retry (the
// response was lost) returns it instead of posting again. Kept here, not in the
// log: only this house's phones retry here, and the log crosses the radio.
type clientPost struct {
	msg string
	at  time.Time
}

const clientKeep = 24 * time.Hour

// rememberClient records a post's clientId, forgetting ones older than a day.
func (s *Server) rememberClient(k, msg string) {
	now := s.cfg.Now()
	if s.clients == nil {
		s.clients = map[string]clientPost{}
	}
	if len(s.clients) >= 1000 {
		for old, c := range s.clients {
			if now.Sub(c.at) > clientKeep {
				delete(s.clients, old)
			}
		}
	}
	s.clients[k] = clientPost{msg: msg, at: now}
}

// Sign-up limits on the open hotspot (review of plan 4a).
const (
	signupsPerAddrPerHour  = 3
	postsPerHour           = 120 // one person's posts; past it the log, every house's memory and the radio fill up
	signInTriesPerHour     = 200 // from one address, under any name: each costs a PIN hash
	signupsPerHousePerHour = 20
	maxPendingPerHouse     = 30
)

// allowSignup records a sign-up from addr if it is within the limits.
func (s *Server) allowSignup(addr string) bool {
	s.signupMu.Lock()
	defer s.signupMu.Unlock()
	now := s.cfg.Now()
	recent := func(k string) []time.Time {
		keep := s.signups[k][:0]
		for _, t := range s.signups[k] {
			if now.Sub(t) < time.Hour {
				keep = append(keep, t)
			}
		}
		s.signups[k] = keep
		return keep
	}
	if len(recent(addr)) >= signupsPerAddrPerHour || len(recent("")) >= signupsPerHousePerHour ||
		len(s.cfg.State.Pending(s.cfg.Author.ID())) >= maxPendingPerHouse {
		return false
	}
	s.signups[addr] = append(s.signups[addr], now)
	s.signups[""] = append(s.signups[""], now)
	return true
}

// hashSlot runs f while holding one of the two PIN-hashing slots.
func (s *Server) hashSlot(f func()) {
	s.argon <- struct{}{}
	defer func() { <-s.argon }()
	f()
}

var dummyOnce struct {
	sync.Once
	hash string
}

// dummyHash is a hash of no one's PIN, checked when a sign-in names nobody,
// so that a wrong name costs the same time as a wrong PIN (B23). Made once per
// process: every house server shares it.
func dummyHash() string {
	dummyOnce.Do(func() {
		h, err := auth.HashPIN("000000")
		if err != nil {
			panic(err) // crypto/rand does not fail on supported platforms
		}
		dummyOnce.hash = h
	})
	return dummyOnce.hash
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
	return &Server{cfg: cfg, sessions: auth.NewSessions(), lockout: auth.NewLockout(), high: prio.NewLimiter(cfg.HighPerHour),
		posts: prio.NewLimiter(postsPerHour), tries: prio.NewLimiter(signInTriesPerHour), hub: newHub(),
		argon: make(chan struct{}, 2), signups: map[string][]time.Time{}}
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
		s.hub.message(s.cfg.State, state.AccountID(e.Key), s.alive)
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.ServeHTTP(w, r)
	})
}

// ---- server-sent events ----

type listener struct {
	account, token string
	ch             chan []byte
	done           chan struct{} // closed when the session ends
}

// alive: the listener's session still exists and its account is still approved.
func (s *Server) alive(l *listener) bool {
	id, ok := s.sessions.Get(l.token, s.cfg.Now())
	return ok && id == l.account && s.cfg.State.Approved(l.account)
}

type hub struct {
	mu        sync.Mutex
	listeners map[*listener]bool
}

func newHub() *hub { return &hub{listeners: map[*listener]bool{}} }

// maxStreamsPerAccount bounds the live streams one account may hold open (B26):
// a phone, a laptop and a kiosk or two, not a loop that opens thousands.
const maxStreamsPerAccount = 8

// add opens a stream; nil if the account already has its share.
func (h *hub) add(account, token string) *listener {
	h.mu.Lock()
	defer h.mu.Unlock()
	open := 0
	for l := range h.listeners {
		if l.account == account {
			open++
		}
	}
	if open >= maxStreamsPerAccount {
		return nil
	}
	l := &listener{account: account, token: token, ch: make(chan []byte, 64), done: make(chan struct{})}
	h.listeners[l] = true
	return l
}

func (h *hub) remove(l *listener) {
	h.mu.Lock()
	delete(h.listeners, l)
	h.mu.Unlock()
}

// end closes every stream of a session (sign-out).
func (h *hub) end(token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for l := range h.listeners {
		if l.token == token {
			close(l.done)
			delete(h.listeners, l)
		}
	}
}

func (h *hub) send(l *listener, b []byte) {
	select {
	case l.ch <- b:
	default: // a slow phone misses events; it reloads conversations on reconnect
	}
}

func (h *hub) message(st *state.State, id string, alive func(*listener) bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := st.MessageByID(id)
	if m == nil {
		return
	}
	b, _ := json.Marshal(m)
	for l := range h.listeners {
		if !alive(l) {
			close(l.done)
			delete(h.listeners, l)
			continue
		}
		if st.Visible(m, l.account) {
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

// State is the house's view of the village (the simulator and tests read it).
func (s *Server) State() *state.State { return s.cfg.State }

// NewSession signs a person in without a PIN. The simulator uses it to pick a
// persona; nothing reachable over the network calls it.
func (s *Server) NewSession(account string) string { return s.sessions.New(account, s.cfg.Now()) }

// Event is one item of a session's live stream.
type Event struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

// Subscribe opens a live stream for a session without HTTP (the simulator
// polls it). drain returns what arrived since the last call; stop ends it.
func (s *Server) Subscribe(token string) (drain func() []Event, stop func(), err error) {
	id, ok := s.sessions.Get(token, s.cfg.Now())
	if !ok || !s.cfg.State.Approved(id) {
		return nil, nil, fmt.Errorf("web: no approved session")
	}
	l := s.hub.add(id, token)
	if l == nil {
		return nil, nil, fmt.Errorf("web: too many open streams for this account")
	}
	drain = func() []Event {
		var out []Event
		for {
			select {
			case b := <-l.ch:
				name, data := "", ""
				for _, line := range strings.Split(string(b), "\n") {
					if v, ok := strings.CutPrefix(line, "event: "); ok {
						name = v
					}
					if v, ok := strings.CutPrefix(line, "data: "); ok {
						data = v
					}
				}
				out = append(out, Event{Name: name, Data: data})
			default:
				return out
			}
		}
	}
	return drain, func() { s.hub.remove(l) }, nil
}
