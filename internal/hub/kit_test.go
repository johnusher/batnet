package hub

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestOpenWiFiLocksThePanelAndTerminalIsLocalOnly(t *testing.T) {
	f := newFakeNM()
	w, _ := newTestWiFi(f)
	w.set.Neighbours = true
	f.set(func(f *fakeNM) { f.conns[OwnWiFiName] = "batnet-test"; f.active = OwnWiFiName })
	w.Tick()
	if !w.OpenToAll() {
		t.Fatal("not open")
	}
	var mu sync.Mutex
	var ran []string
	run := func(_ context.Context, name string, args ...string) (string, error) {
		mu.Lock()
		ran = append(ran, name+" "+strings.Join(args, " "))
		mu.Unlock()
		return "", nil
	}
	s := New(Config{House: "Miller", HubPIN: "246810", WiFi: w, Run: run})
	h := s.Handler()
	status := func(from, pin string) int {
		req := httptest.NewRequest("GET", "/hub/api/status", nil)
		req.RemoteAddr = from
		if pin != "" {
			req.Header.Set("X-Hub-PIN", pin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := status("10.42.0.20:4000", ""); c != 401 {
		t.Errorf("a neighbour read the panel: %d", c)
	}
	if c := status("10.42.0.20:4000", "246810"); c != 200 {
		t.Errorf("with the PIN: %d", c)
	}
	if c := status("127.0.0.1:4000", ""); c != 200 {
		t.Errorf("the hub's screen: %d", c)
	}
	if c := status("192.168.1.5:4000", ""); c != 200 {
		t.Errorf("the home LAN reads as before: %d", c)
	}
	if code, out := post(h, "/hub/api/system", "192.168.1.5:4000", "246810", `{"op":"terminal"}`, true); code != 403 {
		t.Errorf("a terminal from the LAN: %d %v", code, out)
	}
	if code, _ := post(h, "/hub/api/system", "127.0.0.1:4000", "", `{"op":"terminal"}`, true); code != 200 {
		t.Errorf("a terminal on the screen: %d", code)
	}
	mu.Lock()
	if len(ran) != 1 || ran[0] != "systemctl --no-ask-password restart batnet-console.service" {
		t.Errorf("%q", ran)
	}
	mu.Unlock()
	// The password goes to the screen only, and only for a closed WiFi.
	w.set.Neighbours = false
	_, out := post(h, "/hub/api/wifi", "192.168.1.5:4000", "246810", `{"op":"settings","afterMinutes":"5"}`, true)
	if out["password"] != nil {
		t.Errorf("the password went to the LAN: %v", out)
	}
	_, out = post(h, "/hub/api/wifi", "127.0.0.1:4000", "", `{"op":"settings"}`, true)
	if out["password"] != "testpass123" || out["afterMinutes"] != float64(5) {
		t.Errorf("the screen: %v", out)
	}
}
