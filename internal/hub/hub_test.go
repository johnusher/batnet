package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	for i := 0; i < failsAllowed-1; i++ { // one wrong already
		post(h, "/hub/api/mode", "10.0.0.0:5000", "000000", `{"mode":"off"}`, true)
	}
	if code, _ := post(h, "/hub/api/mode", "10.0.0.1:5000", "246810", `{"mode":"off"}`, true); code != 429 {
		t.Errorf("after %d wrong PINs, even the right one waits: %d", failsAllowed, code)
	}
	if code, _ := post(h, "/hub/api/mode", "127.0.0.1:5000", "", `{"mode":"off"}`, true); code != 200 {
		t.Errorf("the screen still works during a lockout: %d", code)
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
