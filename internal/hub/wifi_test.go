package hub

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeNM pretends to be nmcli and iw for one WiFi device.
type fakeNM struct {
	mu       sync.Mutex
	conns    map[string]string // name -> SSID
	prio     map[string]int
	active   string
	homeHere bool // the home network is in range
	clients  int
	calls    []string
}

func newFakeNM() *fakeNM {
	return &fakeNM{conns: map[string]string{"home": "Home Net"}, prio: map[string]int{}, homeHere: true, active: "home"}
}

func (f *fakeNM) run(_ context.Context, name string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if name == "/usr/sbin/iw" {
		return strings.Repeat("Station aa:bb:cc:dd:ee:ff (on wlan0)\n", f.clients), nil
	}
	a := strings.Join(args, " ")
	switch {
	case strings.Contains(a, "device show"):
		if f.active == "" {
			return "GENERAL.STATE:30 (disconnected)\nGENERAL.CONNECTION:", nil
		}
		return "GENERAL.STATE:100 (connected)\nGENERAL.CONNECTION:" + f.active, nil
	case strings.Contains(a, "connection show id"):
		n := args[len(args)-1]
		return f.conns[n], nil
	case strings.Contains(a, "connection show"):
		var b strings.Builder
		for n := range f.conns {
			act := "no"
			if n == f.active {
				act = "yes"
			}
			fmt.Fprintf(&b, "%s:802-11-wireless:%d:%s\n", n, f.prio[n], act)
		}
		return b.String(), nil
	case strings.Contains(a, "connection delete"):
		n := args[len(args)-1]
		if _, ok := f.conns[n]; !ok {
			return "", errors.New("Error: unknown connection")
		}
		delete(f.conns, n)
		return "", nil
	case strings.Contains(a, "connection add"):
		f.conns[OwnWiFiName] = "batnet-test"
		return "", nil
	case strings.Contains(a, "connection up"):
		n := args[len(args)-1]
		if _, ok := f.conns[n]; !ok {
			return "", errors.New("Error: unknown connection")
		}
		if n != OwnWiFiName && !f.homeHere {
			return "", errors.New("Error: no network with SSID")
		}
		f.active = n
		return "", nil
	case strings.Contains(a, "connection down"):
		f.active = ""
		if f.homeHere {
			f.active = "home" // NetworkManager connects by itself
		}
		return "", nil
	case strings.Contains(a, "wifi list"):
		if f.active == "home" {
			return "*:Home Net:70:WPA2\n :Other:40:WPA2", nil
		}
		return "", nil
	}
	return "", nil
}

func (f *fakeNM) set(fn func(f *fakeNM)) { f.mu.Lock(); fn(f); f.mu.Unlock() }

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newTestWiFi(f *fakeNM) (*WiFi, *fakeClock) {
	clk := &fakeClock{t: time.Date(2026, 10, 21, 20, 0, 0, 0, time.UTC)}
	w := NewWiFi(WiFiConfig{Run: f.run, Password: "testpass123", DefaultSSID: "batnet-test"})
	w.now = clk.now
	w.lastHome = clk.t
	w.step = 0
	w.homeWait = time.Millisecond
	return w, clk
}

func TestWiFiFallbackStartsOwnAndComesBack(t *testing.T) {
	f := newFakeNM()
	w, clk := newTestWiFi(f)
	w.ensureOwn()
	w.Tick()
	if st := w.Status(true); st.Mode != "home" || st.SSID != "Home Net" || st.Signal != 70 {
		t.Fatalf("at start: %+v", st)
	}
	// The power goes: the router with it.
	f.set(func(f *fakeNM) { f.homeHere, f.active = false, "" })
	clk.t = clk.t.Add(time.Minute)
	w.Tick()
	if st := w.Status(true); st.Mode != "none" || st.FallbackIn != 120 {
		t.Fatalf("a minute later: mode %s, fallback in %d", st.Mode, st.FallbackIn)
	}
	clk.t = clk.t.Add(3 * time.Minute)
	w.Tick()
	st := w.Status(true)
	if st.Mode != "own" || st.StartedBy != "auto" || st.Password != "testpass123" {
		t.Fatalf("after 4 minutes: %+v", st)
	}
	if w.Status(false).Password != "" {
		t.Fatal("the password must only go to the hub's own screen")
	}
	// Someone is on it: it stays, however long.
	f.set(func(f *fakeNM) { f.clients = 1 })
	clk.t = clk.t.Add(time.Hour)
	w.Tick()
	if w.Status(false).Mode != "own" {
		t.Fatal("left its own WiFi with a device on it")
	}
	// The power is back and nobody is on the hub's WiFi: 10 minutes later it goes home.
	f.set(func(f *fakeNM) { f.clients, f.homeHere = 0, true })
	w.Tick()
	clk.t = clk.t.Add(11 * time.Minute)
	w.Tick()
	if st := w.Status(false); st.Mode != "home" {
		t.Fatalf("did not go back home: %+v", st)
	}
}

func TestWiFiTryHomeRestartsOwnWhenHomeIsMissing(t *testing.T) {
	f := newFakeNM()
	w, clk := newTestWiFi(f)
	w.ensureOwn()
	f.set(func(f *fakeNM) { f.homeHere, f.active = false, "" })
	w.Tick()
	clk.t = clk.t.Add(4 * time.Minute)
	w.Tick()
	clk.t = clk.t.Add(11 * time.Minute)
	w.Tick() // tries home, finds nothing, starts its own again
	if st := w.Status(false); st.Mode != "own" || st.StartedBy != "auto" {
		t.Fatalf("after a failed try: %+v", st)
	}
}

func TestWiFiBackHomeWins(t *testing.T) {
	f := newFakeNM()
	w, clk := newTestWiFi(f)
	w.ensureOwn()
	f.set(func(f *fakeNM) { f.homeHere, f.active = false, "" })
	if err := w.StartOwn("you"); err != nil {
		t.Fatal(err)
	}
	if st := w.Status(false); st.Mode != "own" || st.StartedBy != "you" {
		t.Fatalf("%+v", st)
	}
	// Started by hand: no return by itself, however long.
	f.set(func(f *fakeNM) { f.homeHere = true })
	clk.t = clk.t.Add(time.Hour)
	w.Tick()
	if w.Status(false).Mode != "own" {
		t.Fatal("left the WiFi the owner started")
	}
	// The owner goes back while the router is still off: no fallback until home is seen.
	f.set(func(f *fakeNM) { f.homeHere = false })
	if err := w.BackHome(); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(time.Hour)
	w.Tick()
	if st := w.Status(false); st.Mode != "none" || !st.Waiting || st.FallbackIn != -1 {
		t.Fatalf("the fallback overrode the owner: %+v", st)
	}
	// Home comes back, then goes again: the fallback is armed once more.
	f.set(func(f *fakeNM) { f.homeHere, f.active = true, "home" })
	w.Tick()
	f.set(func(f *fakeNM) { f.homeHere, f.active = false, "" })
	clk.t = clk.t.Add(4 * time.Minute)
	w.Tick()
	if st := w.Status(false); st.Mode != "own" {
		t.Fatalf("fallback not re-armed: %+v", st)
	}
}

func TestWiFiNoFallbackWhenOff(t *testing.T) {
	f := newFakeNM()
	w, clk := newTestWiFi(f)
	w.set.NoFallback = true
	f.set(func(f *fakeNM) { f.homeHere, f.active = false, "" })
	clk.t = clk.t.Add(time.Hour)
	w.Tick()
	if st := w.Status(false); st.Mode != "none" || st.FallbackIn != -1 {
		t.Fatalf("%+v", st)
	}
}

func TestWiFiNeighboursRemovesPassword(t *testing.T) {
	f := newFakeNM()
	w, _ := newTestWiFi(f)
	if err := w.Configure(WiFiSettings{Neighbours: true}); err != nil {
		t.Fatal(err)
	}
	var add string
	for _, c := range f.calls {
		if strings.Contains(c, "connection add") {
			add = c
		}
	}
	if add == "" || strings.Contains(add, "wifi-sec") {
		t.Fatalf("open WiFi made with a password: %q", add)
	}
	if !strings.Contains(add, "ap-isolation yes") || !strings.Contains(add, "autoconnect no") {
		t.Fatalf("missing isolation or autoconnect no: %q", add)
	}
	if w.Status(true).Password != "" {
		t.Fatal("an open WiFi has no password to show")
	}
	if err := w.Configure(WiFiSettings{SSID: " bad"}); err == nil {
		t.Fatal("a name with a leading space was accepted")
	}
}

func TestWiFiJoinFailureLeavesNothing(t *testing.T) {
	f := newFakeNM()
	w, _ := newTestWiFi(f)
	orig := f.run
	failing := func(ctx context.Context, name string, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "wifi connect") {
			f.set(func(f *fakeNM) { f.conns["Cafe"] = "Cafe" })
			return "", errors.New("Error: Connection activation failed: Secrets were required")
		}
		return orig(ctx, name, args...)
	}
	w.run = failing
	err := w.Join("Cafe", "wrongpass1", false)
	if err == nil || !strings.Contains(err.Error(), "Could not join Cafe") {
		t.Fatalf("err %v", err)
	}
	if _, ok := f.conns["Cafe"]; ok {
		t.Fatal("a failed join stayed saved")
	}
	if err := w.Join("Cafe", "short", false); err == nil {
		t.Fatal("a 5-letter password was accepted")
	}
}

func TestSplitTerse(t *testing.T) {
	got := splitTerse(`*:My\:Net:70:WPA2`)
	if len(got) != 4 || got[1] != "My:Net" {
		t.Fatalf("%q", got)
	}
	if unescape(`a\:b\\c`) != `a:b\c` {
		t.Fatal(unescape(`a\:b\\c`))
	}
	if !OnOwnWiFi("10.42.0.17:5555") || OnOwnWiFi("192.168.1.2:80") {
		t.Fatal("OnOwnWiFi")
	}
}
