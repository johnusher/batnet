package hub

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
)

func TestSightings(t *testing.T) {
	s := NewSightings()
	s.Garbled(meshtastic.Unreadable{ID: 7, From: "!a1b2c3d4", Hops: 2, Relay: "50"}) // before its readable copy
	s.Event(meshtastic.Event{Kind: "packet", ID: 7, From: "!a1b2c3d4", Hops: 3, Relay: "50"})
	s.Event(meshtastic.Event{Kind: "packet", ID: 8, From: "!a1b2c3d4", Hops: 1, Relay: "07"})
	s.Garbled(meshtastic.Unreadable{ID: 8, From: "!a1b2c3d4", Hops: 4, Relay: "28"}) // after it
	s.Garbled(meshtastic.Unreadable{ID: 8, From: "!00000000", Hops: 1, Relay: "28"}) // its header garbled
	s.Garbled(meshtastic.Unreadable{ID: 9, From: "!a1b2c3d4", Hops: 0, Relay: "d4"}) // no readable copy
	s.Event(meshtastic.Event{Kind: "packet", From: "!deadbeef", Hops: 0, Relay: "ef"})
	s.Event(meshtastic.Event{Kind: "packet", From: "!00000000", Local: true})          // our own radio
	s.Garbled(meshtastic.Unreadable{ID: 10, From: "!00000000", Hops: -3, Relay: "50"}) // a broken header
	s.Event(meshtastic.Event{Kind: "sent", From: "!00000000", Hops: 0, Relay: "50"})   // sent, not heard
	s.Garbled(meshtastic.Unreadable{ID: 11, Hops: 1, Relay: "50"})                     // no sender
	s.Event(meshtastic.Event{Kind: "packet", From: "!00000000", Hops: 2, At: time.Now().Add(-22 * 24 * time.Hour)})

	got := s.Saved()
	if len(got) != 2 {
		t.Fatalf("radios %v", got)
	}
	a := got["!a1b2c3d4"]
	if a.N != 4 || a.Hops != 1 || a.Via["50"] != 2 || a.Via["07"] != 1 || a.Via["28"] != 1 || a.Via["direct"] != 0 || a.At.IsZero() {
		t.Errorf("!a1b2c3d4: %+v", a)
	}
	if d := got["!deadbeef"]; d.Via["direct"] != 1 || d.Hops != 0 || len(d.Via) != 1 {
		t.Errorf("heard straight: %+v", d)
	}

	// Saved is a copy, and Load puts it back.
	a.Via["50"] = 99
	b, _ := json.Marshal(got)
	var back map[string]Sighting
	json.Unmarshal(b, &back)
	s2 := NewSightings()
	s2.Load(back)
	if g := s2.Saved()["!a1b2c3d4"]; g.Via["50"] != 99 || s.Saved()["!a1b2c3d4"].Via["50"] != 2 {
		t.Errorf("load %+v, original %+v", g, s.Saved()["!a1b2c3d4"])
	}
}

func TestSightingsBounded(t *testing.T) {
	s := NewSightings()
	for i := range maxVia + 4 {
		for range i + 1 {
			s.add("!a1b2c3d4", fmt.Sprintf("%02x", i), 2, time.Time{})
		}
	}
	if v := s.Saved()["!a1b2c3d4"].Via; len(v) != maxVia || v["00"] != 0 || v[fmt.Sprintf("%02x", maxVia+3)] == 0 {
		t.Errorf("via %v", v)
	}
	start := time.Now().Add(-time.Hour)
	for i := range maxSightings + 1 {
		s.add(fmt.Sprintf("!%08x", i), "50", 2, start.Add(time.Duration(i)*time.Millisecond))
	}
	got := s.Saved()
	if len(got) > maxSightings {
		t.Errorf("%d radios kept", len(got))
	}
	if _, ok := got["!00000000"]; ok {
		t.Error("the oldest radio was kept")
	}
	if _, ok := got[fmt.Sprintf("!%08x", maxSightings)]; !ok {
		t.Error("the newest radio was dropped")
	}
}

func TestSightingsNeedThePIN(t *testing.T) {
	s := NewSightings()
	s.add("!a1b2c3d4", "50", 2, time.Time{})
	h := New(Config{HubPIN: "246810", Sightings: s}).Handler()
	for _, c := range []struct {
		from, pin string
		code      int
	}{{"127.0.0.1:1", "", 200}, {"10.0.0.5:1", "", 401}, {"10.0.0.5:1", "111111", 401}, {"10.0.0.5:1", "246810", 200}} {
		req := httptest.NewRequest("GET", "/hub/api/sightings", nil)
		req.RemoteAddr = c.from
		if c.pin != "" {
			req.Header.Set("X-Hub-PIN", c.pin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var out struct{ Radios map[string]Sighting }
		json.Unmarshal(w.Body.Bytes(), &out)
		if w.Code != c.code || (c.code == 200) != (out.Radios["!a1b2c3d4"].N == 1) {
			t.Errorf("%s pin %q: %d %s", c.from, c.pin, w.Code, w.Body)
		}
	}
}
