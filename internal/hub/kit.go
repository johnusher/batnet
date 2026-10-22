package hub

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/johnusher/batnet/internal/translate"
)

// The power-cut kit's part of the panel (plan 5a): the hub's own WiFi, sound,
// the LED and button, power modes, devices, the translator, the console and
// the Pi's own buttons.

func (s *Server) kitRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /hub/api/wifi", s.actionR(s.wifi))
	mux.HandleFunc("POST /hub/api/sound", s.action(s.sound))
	mux.HandleFunc("POST /hub/api/light", s.action(s.light))
	mux.HandleFunc("POST /hub/api/power", s.action(s.power))
	mux.HandleFunc("POST /hub/api/translate", s.action(s.translate))
	mux.HandleFunc("POST /hub/api/system", s.actionR(s.system))
	mux.HandleFunc("POST /hub/api/seen", s.seen)
	mux.HandleFunc("GET /hub/api/screen", s.screen)
	mux.HandleFunc("POST /hub/api/maps", s.action(s.maps))
	mux.HandleFunc("GET /hub/maps/{file}", s.mapFile)
}

// maps: op get or remove, area (an ID from the status).
func (s *Server) maps(b map[string]string) (any, error) {
	m := s.cfg.Maps
	if m == nil {
		return nil, errNoKit
	}
	var err error
	switch b["op"] {
	case "get":
		err = m.Get(b["area"])
	case "remove":
		err = m.Remove(b["area"])
	default:
		return nil, &failure{http.StatusBadRequest, "Get or remove?"}
	}
	if err != nil {
		return nil, &failure{http.StatusBadRequest, err.Error()}
	}
	return m.Status(), nil
}

// mapFile serves a map, with range requests (the panel reads only the tiles
// it shows). Maps are public data: no PIN.
func (s *Server) mapFile(w http.ResponseWriter, r *http.Request) {
	p := ""
	if id, ok := strings.CutSuffix(r.PathValue("file"), ".pmtiles"); ok && s.cfg.Maps != nil {
		p = s.cfg.Maps.File(id)
	}
	if p == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, p)
}

// readAllowed: on the hub's own WiFi open to neighbours, reading the panel
// needs the hub PIN too (it shows the hub's messages).
func (s *Server) readAllowed(w http.ResponseWriter, r *http.Request) bool {
	if local(r) || s.cfg.WiFi == nil || !s.cfg.WiFi.OpenToAll() || !OnOwnWiFi(r.RemoteAddr) {
		return true
	}
	pin := r.Header.Get("X-Hub-PIN")
	if pin == "" {
		pin = r.URL.Query().Get("pin")
	}
	if err := s.checkPIN(pin, r.RemoteAddr); err != nil {
		writeJSON(w, err.code, map[string]any{"error": err.msg, "locked": true})
		return false
	}
	return true
}

func (s *Server) kitStatus(out *statusOut) {
	if s.cfg.WiFi != nil {
		st := s.cfg.WiFi.Status(out.Local)
		out.WiFi = &st
	}
	if s.cfg.Sound != nil {
		st := s.cfg.Sound.Status()
		out.Sound = &st
	}
	if s.cfg.Light != nil {
		st := s.cfg.Light.Status()
		out.Light = &st
	}
	if s.cfg.Power != nil {
		st := s.cfg.Power.Status()
		out.Power = &st
	}
	if s.cfg.Devices != nil {
		st := s.cfg.Devices.Status()
		out.Devices = &st
	}
	if s.cfg.Translator != nil {
		st := s.cfg.Translator.Status()
		out.Translate = &st
	}
	if s.cfg.Notifier != nil {
		out.Unread = s.cfg.Notifier.Unread()
	}
	if s.cfg.Maps != nil {
		st := s.cfg.Maps.Status()
		out.Maps = &st
	}
}

// boolOr reads "true"/"false" from b[k], or keeps cur.
func boolOr(b map[string]string, k string, cur bool) bool {
	if v, ok := b[k]; ok {
		return v == "true"
	}
	return cur
}

func intOr(b map[string]string, k string, cur int) (int, error) {
	v, ok := b[k]
	if !ok {
		return cur, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return cur, errors.New("Not a number: " + k + ".")
	}
	return n, nil
}

func strOr(b map[string]string, k string, cur string) string {
	if v, ok := b[k]; ok {
		return strings.TrimSpace(v)
	}
	return cur
}

var errNoKit = &failure{http.StatusNotFound, "This hub doesn't have that."}

// wifi: op own, home, scan, join (ssid, password, hidden), forget (name),
// default (name), settings (auto, afterMinutes, neighbours, ssid).
func (s *Server) wifi(r *http.Request, b map[string]string) (any, error) {
	w := s.cfg.WiFi
	if w == nil {
		return nil, errNoKit
	}
	var err error
	switch b["op"] {
	case "own":
		err = w.StartOwn("you")
	case "home":
		err = w.BackHome()
	case "scan":
		list, err := w.Scan()
		if err != nil {
			return nil, err
		}
		return map[string]any{"networks": list}, nil
	case "join":
		err = w.Join(b["ssid"], b["password"], b["hidden"] == "true")
	case "forget":
		err = w.Forget(b["name"])
	case "default":
		err = w.MakeDefault(b["name"])
	case "settings":
		cur := w.Settings()
		n := cur
		n.NoFallback = !boolOr(b, "auto", !cur.NoFallback)
		n.Neighbours = boolOr(b, "neighbours", cur.Neighbours)
		n.SSID = strOr(b, "ssid", cur.SSID)
		if n.AfterMinutes, err = intOr(b, "afterMinutes", cur.AfterMinutes); err == nil {
			err = w.Configure(n)
		}
	default:
		return nil, errors.New("Unknown WiFi action.")
	}
	if err != nil {
		return nil, err
	}
	return w.Status(local(r)), nil
}

// sound: op test, or settings (any of SoundSettings' fields).
func (s *Server) sound(b map[string]string) (any, error) {
	so := s.cfg.Sound
	if so == nil {
		return nil, errNoKit
	}
	if b["op"] == "test" {
		so.Test()
		return so.Status(), nil
	}
	c := so.Settings()
	n := SoundSettings{On: boolOr(b, "on", c.On), Output: strOr(b, "output", c.Output),
		Direct: boolOr(b, "direct", c.Direct), Public: boolOr(b, "public", c.Public), Replies: boolOr(b, "replies", c.Replies),
		Delivered: boolOr(b, "delivered", c.Delivered), Speak: boolOr(b, "speak", c.Speak), ReadAloud: boolOr(b, "readAloud", c.ReadAloud),
		Quiet: boolOr(b, "quiet", c.Quiet), QuietFrom: strOr(b, "quietFrom", c.QuietFrom), QuietTo: strOr(b, "quietTo", c.QuietTo)}
	var err error
	if n.Volume, err = intOr(b, "volume", c.Volume); err != nil {
		return nil, err
	}
	if err := so.Configure(n); err != nil {
		return nil, err
	}
	return so.Status(), nil
}

// light: op test, or settings (led, button: a GPIO number or -1; keepBlinking, public).
func (s *Server) light(b map[string]string) (any, error) {
	l := s.cfg.Light
	if l == nil {
		return nil, errNoKit
	}
	if b["op"] == "test" {
		l.Flash("test")
		return l.Status(), nil
	}
	c := l.Settings()
	n := LightSettings{KeepBlinking: boolOr(b, "keepBlinking", c.KeepBlinking), Public: boolOr(b, "public", c.Public)}
	var err error
	if n.LED, err = intOr(b, "led", c.LED); err != nil {
		return nil, err
	}
	if n.Button, err = intOr(b, "button", c.Button); err != nil {
		return nil, err
	}
	if err := l.Configure(n); err != nil {
		return nil, err
	}
	return l.Status(), nil
}

// power: mode (high, medium, low), reduced.
func (s *Server) power(b map[string]string) (any, error) {
	p := s.cfg.Power
	if p == nil {
		return nil, errNoKit
	}
	c := p.Settings()
	n := PowerSettings{Mode: strOr(b, "mode", c.Mode), Reduced: boolOr(b, "reduced", c.Reduced)}
	if err := p.Configure(n); err != nil {
		return nil, err
	}
	return p.Status(), nil
}

// translate: text, from ("" guesses), to ("" the hub's setting, else en).
func (s *Server) translate(b map[string]string) (any, error) {
	t := s.cfg.Translator
	if t == nil {
		return nil, errNoKit
	}
	text := b["text"]
	from, to := b["from"], b["to"]
	if to == "" && s.cfg.Settings != nil {
		to = s.cfg.Settings().TranslateTo
	}
	if to == "" {
		to = "en"
	}
	if from == "" {
		from = translate.Guess(text)
		if from == "" || from == to {
			// Not sure: try the other installed language into this one.
			for _, p := range t.Pairs() {
				if f, tt, _ := strings.Cut(p, "-"); tt == to && f != to {
					from = f
					break
				}
			}
		}
	}
	if from == "" || from == to {
		return nil, errors.New("This looks like it is in " + translate.Name(to) + " already.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := t.Translate(ctx, from, to, text)
	if err != nil {
		return nil, err
	}
	return map[string]string{"text": out, "from": from, "to": to, "fromName": translate.Name(from), "toName": translate.Name(to)}, nil
}

// system: op terminal (the hub's own screen only), reboot, poweroff, clock (ms
// since 1970, from the device's own clock).
func (s *Server) system(r *http.Request, b map[string]string) (any, error) {
	run := s.cfg.Run
	if run == nil {
		return nil, errNoKit
	}
	switch b["op"] {
	case "terminal":
		if !local(r) {
			return nil, &failure{http.StatusForbidden, "The terminal opens on the hub's own screen only."}
		}
		if err := StartUnit(run, "batnet-console.service", true); err != nil {
			return nil, err
		}
		return map[string]string{"note": "The screen shows a login now. Ctrl+Alt+F1 comes back here."}, nil
	case "reboot", "poweroff":
		op := b["op"]
		time.AfterFunc(time.Second, func() { runFor(run, 30*time.Second, "systemctl", "--no-ask-password", op) })
		if op == "reboot" {
			return map[string]string{"note": "Restarting: back in about a minute."}, nil
		}
		return map[string]string{"note": "Switching off. Wait until the green light stops blinking before you unplug it."}, nil
	case "clock":
		ms, err := strconv.ParseInt(b["ms"], 10, 64)
		t := time.UnixMilli(ms)
		if err != nil || t.Year() < 2026 || t.Year() > 2100 {
			return nil, errors.New("That is not a time.")
		}
		runFor(run, 15*time.Second, "timedatectl", "--no-ask-password", "set-ntp", "false")
		_, err = runFor(run, 15*time.Second, "timedatectl", "--no-ask-password", "set-time", t.Local().Format("2006-01-02 15:04:05"))
		// timesyncd moves the clock forward to its saved time when it starts:
		// save the new time first, or a clock set back would jump ahead again.
		StartUnit(run, "batnet-clock.service", false)
		runFor(run, 15*time.Second, "timedatectl", "--no-ask-password", "set-ntp", "true")
		if err != nil {
			return nil, errors.New("The clock was not set: " + err.Error())
		}
		return map[string]string{"note": "The hub's clock is set: " + t.Local().Format("15:04, 2 Jan 2006") + "."}, nil
	}
	return nil, errors.New("Unknown action.")
}

// seen: the messages were read on a panel (no PIN: it only stops the LED).
func (s *Server) seen(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Hub") != "1" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Not from the panel."})
		return
	}
	if s.cfg.Notifier != nil {
		s.cfg.Notifier.Seen()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// screen is the kiosk's long poll (the hub itself only): it answers "on",
// "on reduced" or "off" as soon as that differs from ?state=, or after 50 s.
func (s *Server) screen(w http.ResponseWriter, r *http.Request) {
	if !local(r) {
		http.Error(w, "for the hub's own screen", http.StatusForbidden)
		return
	}
	tok := "on"
	if s.cfg.Power != nil {
		tok = s.cfg.Power.WaitScreen(r.Context(), r.URL.Query().Get("state"), 50*time.Second)
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(tok + "\n"))
}
