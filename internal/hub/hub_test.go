package hub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
)

func post(h http.Handler, path, from, pin, body string, xhub bool) (int, map[string]any) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.RemoteAddr = from
	req.Header.Set("Content-Type", "application/json")
	if xhub {
		req.Header.Set("X-Hub", "1")
	}
	if pin != "" {
		req.Header.Set("X-Hub-PIN", pin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	out := map[string]any{}
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestButtonsNeedThePINFromTheLAN(t *testing.T) {
	var got string
	s := New(Config{House: "Miller", HubPIN: "246810", AdminPIN: "135791", SetMode: func(m string) error { got = m; return nil }})
	h := s.Handler()

	if code, _ := post(h, "/hub/api/mode", "127.0.0.1:5000", "", `{"mode":"mock"}`, true); code != 200 || got != "mock" {
		t.Fatalf("the screen: %d %q", code, got)
	}
	if code, _ := post(h, "/hub/api/mode", "127.0.0.1:5000", "", `{"mode":"mock"}`, false); code != 403 {
		t.Errorf("without X-Hub: %d", code)
	}
	if code, _ := post(h, "/hub/api/mode", "127.0.0.1:5000", "", `{"mode":"party"}`, true); code != 400 {
		t.Errorf("bad mode: %d", code)
	}
	got = ""
	if code, _ := post(h, "/hub/api/mode", "10.0.0.0:5000", "", `{"mode":"real"}`, true); code != 401 || got != "" {
		t.Errorf("LAN without PIN: %d %q", code, got)
	}
	if code, _ := post(h, "/hub/api/mode", "10.0.0.0:5000", "246810", `{"mode":"real"}`, true); code != 200 || got != "real" {
		t.Errorf("LAN with PIN: %d %q", code, got)
	}
	for i := 0; i < failsAllowed; i++ { // the empty PIN above was not a guess
		post(h, "/hub/api/mode", "10.0.0.0:5000", "000000", `{"mode":"off"}`, true)
	}
	if code, _ := post(h, "/hub/api/mode", "10.0.0.0:5000", "246810", `{"mode":"off"}`, true); code != 429 {
		t.Errorf("after %d wrong PINs, even the right one waits: %d", failsAllowed, code)
	}
	// The lockout is the guesser's, not the household's: another device still gets in.
	if code, _ := post(h, "/hub/api/mode", "10.0.0.1:5000", "246810", `{"mode":"off"}`, true); code != 200 {
		t.Errorf("a neighbour's wrong PINs locked the owner's phone out: %d", code)
	}
	if code, _ := post(h, "/hub/api/mode", "127.0.0.1:5000", "", `{"mode":"off"}`, true); code != 200 {
		t.Errorf("the screen still works during a lockout: %d", code)
	}
	// A guesser who changes address runs out too.
	for i := 0; i < failsAllowedAll; i++ {
		post(h, "/hub/api/mode", fmt.Sprintf("10.0.%d.%d:5000", i/250+1, i%250+1), "000000", `{"mode":"off"}`, true)
	}
	if code, _ := post(h, "/hub/api/mode", "10.9.9.9:5000", "246810", `{"mode":"off"}`, true); code != 429 {
		t.Errorf("after %d wrong PINs from everywhere, a fresh address still gets a guess: %d", failsAllowedAll, code)
	}
}

func TestStatusShowsPINsOnlyOnTheScreen(t *testing.T) {
	h := New(Config{House: "Miller", HubPIN: "246810", AdminPIN: "135791"}).Handler()
	for _, c := range []struct {
		from string
		pins bool
	}{{"127.0.0.1:1", true}, {"[::1]:1", true}, {"10.0.0.0:1", false}} {
		req := httptest.NewRequest("GET", "/hub/api/status", nil)
		req.RemoteAddr = c.from
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var out statusOut
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if (out.PINs != nil) != c.pins || out.Local != c.pins || out.House != "Miller" {
			t.Errorf("%s: %+v", c.from, out)
		}
		if strings.Contains(w.Body.String(), "246810") != c.pins {
			t.Errorf("%s: PIN in the body = %v", c.from, !c.pins)
		}
	}
	if code, _ := post(h, "/hub/api/send", "127.0.0.1:1", "", `{"text":"hi"}`, true); code != 400 {
		t.Errorf("send with no radio: %d", code)
	}
}

func TestPanelIsServed(t *testing.T) {
	h := New(Config{}).Handler()
	for _, p := range []string{"/hub/", "/hub/panel.js", "/hub/panel.css"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 200 || w.Body.Len() < 100 {
			t.Errorf("%s: %d, %d bytes", p, w.Code, w.Body.Len())
		}
	}
}

func TestSendToOneRadioAndTraffic(t *testing.T) {
	tr := NewTraffic(func() string { return "MediumFast" })
	tr.Busy()
	h := New(Config{Radio: meshtastic.New("127.0.0.1:1"), Traffic: tr}).Handler()
	for _, c := range []struct{ body, err string }{
		{`{"text":"hi","to":"a1b2c3d4"}`, "not a radio's ID"},
		{`{"text":"hi","to":"!zz"}`, "not a radio's ID"},
		{`{"text":"hi","to":"!a1b2c3d4","replyId":"x"}`, "Bad reply"},
		{`{"text":"hi","to":"!a1b2c3d4","replyId":"77"}`, "radio not connected"},
	} {
		code, out := post(h, "/hub/api/send", "127.0.0.1:1", "", c.body, true)
		if msg, _ := out["error"].(string); code != 400 || !strings.Contains(msg, c.err) {
			t.Errorf("%s: %d %v", c.body, code, out)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/hub/api/traffic?since=0", nil))
	var out TrafficOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Seq != 1 || len(out.Frames) != 1 || out.Limit != -12.5 {
		t.Errorf("traffic %s %v", w.Body.String(), err)
	}
}

func TestSettings(t *testing.T) {
	hs := HubSettings{}
	h := New(Config{Radio: meshtastic.New("127.0.0.1:1"),
		Settings: func() HubSettings { return hs }, SaveSettings: func(n HubSettings) error { hs = n; return nil }}).Handler()
	if code, _ := post(h, "/hub/api/settings", "127.0.0.1:1", "", `{"helloMinutes":"60","lat":"52.5","lon":"13.4"}`, true); code != 200 || hs.HelloMinutes != 60 || hs.Where == nil || hs.Where.Lat != 52.5 {
		t.Fatalf("save: %d %+v", code, hs)
	}
	if code, _ := post(h, "/hub/api/settings", "127.0.0.1:1", "", `{"lat":"95","lon":"13.4"}`, true); code != 400 || hs.Where.Lat != 52.5 {
		t.Errorf("a place off the map: %d %+v", code, hs)
	}
	if code, _ := post(h, "/hub/api/settings", "127.0.0.1:1", "", `{"lat":"","lon":""}`, true); code != 200 || hs.Where != nil {
		t.Errorf("forget the place: %d %+v", code, hs)
	}
	if code, _ := post(h, "/hub/api/settings", "10.0.0.1:1", "", `{"helloMinutes":"5"}`, true); code != 401 || hs.HelloMinutes != 60 {
		t.Errorf("from the LAN without the PIN: %d %+v", code, hs)
	}
	for _, c := range []struct{ path, body, err string }{
		{"/hub/api/radio", `{"hopLimit":"x"}`, "number"},
		{"/hub/api/radio", `{"preset":"MediumFast"}`, "not arrived yet"},
		{"/hub/api/owner", `{"long":"lorabatnet","short":"🦇"}`, "radio not connected"},
	} {
		code, out := post(h, c.path, "127.0.0.1:1", "", c.body, true)
		if msg, _ := out["error"].(string); code != 400 || !strings.Contains(msg, c.err) {
			t.Errorf("%s %s: %d %v", c.path, c.body, code, out)
		}
	}
}

// Starting the zone again keeps the old readings aside and forgets them (B68).
func TestZoneStartsAgain(t *testing.T) {
	radio := meshtastic.New("127.0.0.1:1")
	radio.LoadLinks(map[string]meshtastic.SavedLink{"!a1b2c3d4": {SNRs: []float32{-8}, Count: 1, At: time.Now()}})
	dir := t.TempDir()
	h := New(Config{DataDir: dir, Radio: radio}).Handler()
	code, out := post(h, "/hub/api/zone", "127.0.0.1:5000", "", `{}`, true)
	if code != 200 || out["forgot"] != 1.0 {
		t.Fatalf("%d %v", code, out)
	}
	if n := len(radio.SavedLinks()); n != 0 {
		t.Errorf("%d readings left", n)
	}
	kept, _ := filepath.Glob(filepath.Join(dir, "links-before-*.json"))
	if len(kept) != 1 {
		t.Fatalf("kept aside: %v", kept)
	}
	if b, _ := os.ReadFile(kept[0]); !strings.Contains(string(b), "!a1b2c3d4") {
		t.Errorf("kept %s", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, LinksFile)); string(b) != "{}\n" {
		t.Errorf("the saved readings are still %q", b)
	}
}
