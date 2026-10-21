// Package hub is the control panel of a batnet hub: one page, on the Pi's
// HDMI screen and on the LAN, that shows the radio, the radios nearby, the
// WiFi mesh and the machine, and switches the mesh between off, mock and real.
package hub

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
)

//go:embed panel
var panelFS embed.FS

// Modes of the WiFi mesh.
const (
	ModeOff  = "off"  // this hub alone: no mesh
	ModeMock = "mock" // pretend houses in memory, for trying things out
	ModeReal = "real" // batman-adv on the USB WiFi stick
)

// ValidMode reports whether m is a mesh mode.
func ValidMode(m string) bool { return m == ModeOff || m == ModeMock || m == ModeReal }

// Peer is another house on the mesh.
type Peer struct {
	Name   string `json:"name"`
	Addr   string `json:"addr,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// MeshStatus is what the mesh side reports.
type MeshStatus struct {
	Mode    string `json:"mode"`
	OK      bool   `json:"ok"`
	State   string `json:"state"` // a few plain words
	Detail  string `json:"detail,omitempty"`
	Iface   string `json:"iface,omitempty"`
	Addr    string `json:"addr,omitempty"`
	Peers   []Peer `json:"peers"`
	Entries int    `json:"entries"` // entries in this hub's log
}

// Config wires the panel to the rest of batnetd.
type Config struct {
	House   string
	DataDir string
	Radio   *meshtastic.Client // nil: no radio configured
	Mesh    func() MeshStatus
	// SetMode saves the new mode and restarts batnetd in it.
	SetMode func(mode string) error
	// HubPIN unlocks the panel's buttons from the LAN; the screen on the Pi
	// itself needs none. AdminPIN is the house app's first admin's PIN; both
	// are shown on the screen only.
	HubPIN, AdminPIN string
	AppURL           string
}

// Server serves the panel.
type Server struct {
	cfg Config

	mu       sync.Mutex
	failures []time.Time
}

// New returns a panel server.
func New(cfg Config) *Server { return &Server{cfg: cfg} }

const (
	maxBody      = 4 << 10
	failWindow   = 10 * time.Minute
	failsAllowed = 5
)

// Handler serves the panel under /hub/.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(panelFS, "panel")
	mux.Handle("GET /hub/", http.StripPrefix("/hub/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("GET /hub/api/ping", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /hub/api/status", s.status)
	mux.HandleFunc("POST /hub/api/send", s.action(s.send))
	mux.HandleFunc("POST /hub/api/hello", s.action(s.hello))
	mux.HandleFunc("POST /hub/api/mode", s.action(s.mode))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

// local is a request from the Pi itself (the HDMI screen).
func local(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type statusOut struct {
	House  string             `json:"house"`
	Local  bool               `json:"local"`
	PINs   map[string]string  `json:"pins,omitempty"`
	AppURL string             `json:"appURL"`
	System System             `json:"system"`
	Radio  *meshtastic.Status `json:"radio"`
	Mesh   MeshStatus         `json:"mesh"`
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	out := statusOut{House: s.cfg.House, Local: local(r), AppURL: s.cfg.AppURL, System: readSystem(s.cfg.DataDir)}
	if out.Local {
		out.PINs = map[string]string{"hub": s.cfg.HubPIN, "admin": s.cfg.AdminPIN}
	}
	if s.cfg.Radio != nil {
		st := s.cfg.Radio.Status()
		if len(st.Texts) > 100 {
			st.Texts = st.Texts[len(st.Texts)-100:]
		}
		out.Radio = &st
	}
	if s.cfg.Mesh != nil {
		out.Mesh = s.cfg.Mesh()
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

type failure struct {
	code int
	msg  string
}

func (f *failure) Error() string { return f.msg }

// action wraps a button's handler: JSON only, with the X-Hub header (so a
// page on another site cannot post here without a CORS preflight, which this
// server never answers), and the hub PIN unless the request is local.
func (s *Server) action(f func(body map[string]string) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Hub") != "1" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Not from the panel."})
			return
		}
		if !local(r) {
			if err := s.checkPIN(r.Header.Get("X-Hub-PIN")); err != nil {
				writeJSON(w, err.code, map[string]string{"error": err.msg})
				return
			}
		}
		body := map[string]string{}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bad request."})
			return
		}
		out, err := f(body)
		if err != nil {
			code := http.StatusBadRequest
			var fe *failure
			if errors.As(err, &fe) {
				code = fe.code
			}
			msg := err.Error()
			msg = strings.TrimPrefix(msg, "meshtastic: ")
			writeJSON(w, code, map[string]string{"error": msg})
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// checkPIN allows five wrong PINs in ten minutes from the whole LAN, then
// none until the oldest ages out.
func (s *Server) checkPIN(pin string) *failure {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	kept := s.failures[:0]
	for _, t := range s.failures {
		if now.Sub(t) < failWindow {
			kept = append(kept, t)
		}
	}
	s.failures = kept
	if len(s.failures) >= failsAllowed {
		return &failure{http.StatusTooManyRequests, "Too many wrong PINs. Try again in a few minutes, or use the screen on the hub."}
	}
	if s.cfg.HubPIN == "" || subtle.ConstantTimeCompare([]byte(pin), []byte(s.cfg.HubPIN)) != 1 {
		s.failures = append(s.failures, now)
		return &failure{http.StatusUnauthorized, "The hub PIN is shown on the hub's screen."}
	}
	return nil
}

func (s *Server) send(b map[string]string) (any, error) {
	if s.cfg.Radio == nil {
		return nil, meshtastic.ErrOffline
	}
	text := strings.TrimSpace(b["text"])
	if err := s.cfg.Radio.SendText(text, 0); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (s *Server) hello(map[string]string) (any, error) {
	if s.cfg.Radio == nil {
		return nil, meshtastic.ErrOffline
	}
	if err := s.cfg.Radio.SayHello(); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (s *Server) mode(b map[string]string) (any, error) {
	m := b["mode"]
	if !ValidMode(m) {
		return nil, errors.New("The mode is off, mock or real.")
	}
	if s.cfg.SetMode == nil {
		return nil, errors.New("This hub cannot change mode.")
	}
	if err := s.cfg.SetMode(m); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "restarting": true}, nil
}
