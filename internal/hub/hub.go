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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
	"github.com/johnusher/batnet/internal/translate"
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
	Log              *Logbook // nil: no logging
	Traffic          *Traffic // nil: no traffic view
	// Settings and SaveSettings are the hub's own settings (nil: fixed).
	Settings     func() HubSettings
	SaveSettings func(HubSettings) error
	// The power-cut kit (plan 5a); each nil when the hub has none.
	WiFi       *WiFi
	Sound      *Sound
	Light      *Light
	Power      *Power
	Devices    *Devices
	Translator *translate.Client
	Notifier   *Notifier
	// Run runs the system's commands: reboot, power off, the clock, the console.
	Run Runner
}

// HubSettings are the hub's own settings the panel can change.
type HubSettings struct {
	// HelloMinutes is the time between the log's hellos: 0 means every 5
	// minutes, negative none.
	HelloMinutes int `json:"helloMinutes"`
	// Where is where the hub stands, for the map and distances. It stays on
	// the hub and is never sent by radio.
	Where *Where `json:"where,omitempty"`
	// TranslateTo is the language messages are translated into ("": en);
	// TranslateAuto translates them without being asked.
	TranslateTo   string `json:"translateTo,omitempty"`
	TranslateAuto bool   `json:"translateAuto,omitempty"`
}

// Where is a place on the map.
type Where struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
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
	keepTexts    = 300 // messages the panel gets: public and to this hub
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
	mux.HandleFunc("GET /hub/api/traffic", s.traffic)
	mux.HandleFunc("POST /hub/api/send", s.action(s.send))
	mux.HandleFunc("POST /hub/api/hello", s.action(s.hello))
	mux.HandleFunc("POST /hub/api/mode", s.action(s.mode))
	mux.HandleFunc("POST /hub/api/log", s.action(s.logOnOff))
	mux.HandleFunc("POST /hub/api/radio", s.action(s.radio))
	mux.HandleFunc("POST /hub/api/owner", s.action(s.owner))
	mux.HandleFunc("POST /hub/api/settings", s.action(s.settings))
	mux.HandleFunc("GET /hub/logs/{name}", s.download)
	s.kitRoutes(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://tile.openstreetmap.org; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
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
	Log    *LogStatus         `json:"log,omitempty"`
	// Settings: the hub's own, and what the radio settings may be.
	Settings   *HubSettings `json:"settings,omitempty"`
	Presets    []string     `json:"presets"`
	MaxTxPower int          `json:"maxTxPower"`
	// The power-cut kit.
	WiFi      *WiFiStatus       `json:"wifi,omitempty"`
	Sound     *SoundStatus      `json:"sound,omitempty"`
	Light     *LightStatus      `json:"light,omitempty"`
	Power     *PowerStatus      `json:"power,omitempty"`
	Devices   *DevicesStatus    `json:"devices,omitempty"`
	Translate *translate.Status `json:"translate,omitempty"`
	Unread    int               `json:"unread"`
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if !s.readAllowed(w, r) {
		return
	}
	out := statusOut{House: s.cfg.House, Local: local(r), AppURL: s.cfg.AppURL, System: readSystem(s.cfg.DataDir),
		Presets: meshtastic.Presets, MaxTxPower: meshtastic.MaxTxPower}
	if s.cfg.Settings != nil {
		hs := s.cfg.Settings()
		out.Settings = &hs
	}
	if out.Local {
		out.PINs = map[string]string{"hub": s.cfg.HubPIN, "admin": s.cfg.AdminPIN}
	}
	if s.cfg.Radio != nil {
		st := s.cfg.Radio.Status()
		if len(st.Texts) > keepTexts {
			st.Texts = st.Texts[len(st.Texts)-keepTexts:]
		}
		out.Radio = &st
	}
	if s.cfg.Mesh != nil {
		out.Mesh = s.cfg.Mesh()
	}
	if s.cfg.Log != nil {
		ls := s.cfg.Log.Status()
		out.Log = &ls
	}
	s.kitStatus(&out)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) traffic(w http.ResponseWriter, r *http.Request) {
	if !s.readAllowed(w, r) {
		return
	}
	if s.cfg.Traffic == nil {
		writeJSON(w, http.StatusOK, TrafficOut{Frames: []Frame{}})
		return
	}
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	writeJSON(w, http.StatusOK, s.cfg.Traffic.Since(since))
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
	return s.actionR(func(_ *http.Request, b map[string]string) (any, error) { return f(b) })
}

// actionR is action for a handler that needs the request (local or not).
func (s *Server) actionR(f func(r *http.Request, body map[string]string) (any, error)) http.HandlerFunc {
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
		out, err := f(r, body)
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

// send sends a text to everyone, or with "to" (a node ID such as !a1b2c3d4)
// to one radio; "replyId" makes it an answer to that message.
func (s *Server) send(b map[string]string) (any, error) {
	if s.cfg.Radio == nil {
		return nil, meshtastic.ErrOffline
	}
	to := uint32(meshtastic.Broadcast)
	if id := b["to"]; id != "" && id != "everyone" {
		n, err := strconv.ParseUint(strings.TrimPrefix(id, "!"), 16, 32)
		if err != nil || !strings.HasPrefix(id, "!") || len(id) != 9 {
			return nil, errors.New("That is not a radio's ID (like !a1b2c3d4).")
		}
		to = uint32(n)
	}
	var reply uint64
	if r := b["replyId"]; r != "" {
		var err error
		if reply, err = strconv.ParseUint(r, 10, 32); err != nil {
			return nil, errors.New("Bad reply.")
		}
	}
	t, err := s.cfg.Radio.Send(to, 0, strings.TrimSpace(b["text"]), uint32(reply))
	if err != nil {
		return nil, err
	}
	return t, nil
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

func (s *Server) logOnOff(b map[string]string) (any, error) {
	if s.cfg.Log == nil {
		return nil, errors.New("This hub has no log.")
	}
	var err error
	if b["on"] == "true" {
		err = s.cfg.Log.Start()
	} else {
		err = s.cfg.Log.Stop()
	}
	if err != nil {
		return nil, err
	}
	return s.cfg.Log.Status(), nil
}

// radio changes the radio's LoRa settings: preset, hopLimit, txPower, txEnabled.
func (s *Server) radio(b map[string]string) (any, error) {
	if s.cfg.Radio == nil {
		return nil, meshtastic.ErrOffline
	}
	rs := meshtastic.RadioSettings{Preset: b["preset"]}
	if v := b["hopLimit"]; v != "" {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, errors.New("Relays is a number.")
		}
		h := uint32(n)
		rs.HopLimit = &h
	}
	if v := b["txPower"]; v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return nil, errors.New("Transmit power is a number.")
		}
		p := int32(n)
		rs.TxPower = &p
	}
	if v := b["txEnabled"]; v != "" {
		on := v == "true"
		rs.TxEnabled = &on
	}
	if err := s.cfg.Radio.SetRadio(rs); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "restarting": true}, nil
}

// owner renames this hub's radio: long and short.
func (s *Server) owner(b map[string]string) (any, error) {
	if s.cfg.Radio == nil {
		return nil, meshtastic.ErrOffline
	}
	if err := s.cfg.Radio.SetOwner(b["long"], b["short"]); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

// settings changes the hub's own settings: helloMinutes; lat and lon (both
// empty: forget where the hub is).
func (s *Server) settings(b map[string]string) (any, error) {
	if s.cfg.Settings == nil || s.cfg.SaveSettings == nil {
		return nil, errors.New("This hub's settings are fixed.")
	}
	hs := s.cfg.Settings()
	if v, ok := b["helloMinutes"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n > 24*60 {
			return nil, errors.New("Hello every: a number of minutes.")
		}
		hs.HelloMinutes = n
	}
	if la, lo := b["lat"], b["lon"]; la != "" || lo != "" {
		lat, err1 := strconv.ParseFloat(la, 64)
		lon, err2 := strconv.ParseFloat(lo, 64)
		if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return nil, errors.New("That is not a place on the map.")
		}
		hs.Where = &Where{Lat: lat, Lon: lon}
	} else if _, ok := b["lat"]; ok {
		hs.Where = nil
	}
	if v, ok := b["translateTo"]; ok {
		if v != "" && !regexp.MustCompile(`^[a-z]{2,3}$`).MatchString(v) {
			return nil, errors.New("Translate into: a language code like en.")
		}
		hs.TranslateTo = v
	}
	if v, ok := b["translateAuto"]; ok {
		hs.TranslateAuto = v == "true"
	}
	if err := s.cfg.SaveSettings(hs); err != nil {
		return nil, err
	}
	return hs, nil
}

// download serves a saved log; from the LAN it needs the hub PIN (?pin=).
func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	if !local(r) {
		if err := s.checkPIN(r.URL.Query().Get("pin")); err != nil {
			http.Error(w, err.msg, err.code)
			return
		}
	}
	p := ""
	if s.cfg.Log != nil {
		p = s.cfg.Log.Path(r.PathValue("name"))
	}
	if p == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", `attachment; filename="`+r.PathValue("name")+`"`)
	http.ServeFile(w, r, p)
}
