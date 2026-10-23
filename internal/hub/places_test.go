package hub

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
	"github.com/johnusher/batnet/internal/seal"
)

func TestPlaces(t *testing.T) {
	now := time.Date(2026, 10, 23, 14, 0, 0, 0, time.UTC)
	key, _ := seal.NewVillageKey()
	sealer, _ := seal.NewSealer(key, seal.PurposePlaces)
	at := func(h int) time.Time { return now.Add(time.Duration(h) * time.Hour) }
	pos := func(lat float64, h int) *meshtastic.NodePosition {
		return &meshtastic.NodePosition{Lat: lat, Lon: 13.5, AccuracyM: 91, At: at(h)}
	}
	first := NewPlaces(PlacesConfig{Hub: "aa", Name: func() string { return "Miller House" }, Sealer: sealer, Now: func() time.Time { return now },
		Nodes: func() []meshtastic.Node {
			return []meshtastic.Node{
				{ID: "!place01", Self: true, Position: pos(52.1, -1)},
				{ID: "!place02", Position: pos(52.2, -2)},
				{ID: "!place03", Position: pos(52.3, -1)},
				{ID: "!place04"}, // never said where it is
			}
		}})
	second := NewPlaces(PlacesConfig{Hub: "bb", Sealer: sealer, Now: func() time.Time { return now }})

	box, err := first.box()
	if err != nil {
		t.Fatal(err)
	}
	if first.receive(box); first.Count() != 0 {
		t.Error("a hub took its own places")
	}
	second.receive(box)
	if second.Count() != 3 {
		t.Fatalf("the second hub got %d places, want 3", second.Count())
	}

	other, _ := seal.NewVillageKey()
	stranger, _ := seal.NewSealer(other, seal.PurposePlaces)
	third := NewPlaces(PlacesConfig{Hub: "cc", Sealer: stranger, Now: func() time.Time { return now }})
	if third.receive(box); third.Count() != 0 {
		t.Error("a hub of another village read the places")
	}

	// The second hub's radio heard !place02 since, and has never heard where !place03 is.
	nodes := []meshtastic.Node{
		{ID: "!hubradio", Self: true},
		{ID: "!place02", Position: pos(52.25, -1)},
		{ID: "!place03"},
		{ID: "!place05"},
	}
	second.Fill(nodes)
	if nodes[0].Position != nil || nodes[3].Position != nil {
		t.Errorf("filled a place nobody shared: %+v %+v", nodes[0].Position, nodes[3].Position)
	}
	if p := nodes[1].Position; p.Lat != 52.25 || p.From != "" {
		t.Errorf("its own newer place was replaced: %+v", p)
	}
	if p := nodes[2].Position; p == nil || p.Lat != 52.3 || p.From != "Miller House" || p.AccuracyM != 91 {
		t.Errorf("borrowed place %+v", p)
	}

	// A newer place from another hub wins over an older one heard here.
	old := []meshtastic.Node{{ID: "!place02", Position: pos(52.0, -5)}}
	second.Fill(old)
	if p := old[0].Position; p.Lat != 52.2 || p.From != "Miller House" {
		t.Errorf("newer borrowed place %+v", p)
	}

	// Nonsense inside a good seal is dropped.
	bad, _ := sealer.Seal([]byte(`{"hub":"dd","house":"x","places":{"!place09":{"lat":95,"lon":13,"at":"2026-10-23T13:00:00Z"},"nope":{"lat":52,"lon":13,"at":"2026-10-23T13:00:00Z"},"!place0a":{"lat":52,"lon":13,"at":"2026-10-25T13:00:00Z"}}}`))
	if second.receive(bad); second.Count() != 3 {
		t.Errorf("took bad places: %d", second.Count())
	}
}

func TestPlacesOverUDP(t *testing.T) {
	key, _ := seal.NewVillageKey()
	sealer, _ := seal.NewSealer(key, seal.PurposePlaces)
	free := func() int {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		return c.LocalAddr().(*net.UDPAddr).Port
	}
	pa, pb := free(), free()
	to := func(p int) func() []string {
		return func() []string { return []string{"127.0.0.1:" + strconv.Itoa(p)} }
	}
	a := NewPlaces(PlacesConfig{Hub: "aa", Name: func() string { return "Miller House" }, Sealer: sealer, Port: pa, To: to(pb), Every: 50 * time.Millisecond,
		Nodes: func() []meshtastic.Node {
			return []meshtastic.Node{{ID: "!place02", Position: &meshtastic.NodePosition{Lat: 52.2, Lon: 13.5, At: time.Now()}}}
		}})
	b := NewPlaces(PlacesConfig{Hub: "bb", Sealer: sealer, Port: pb, To: to(pa), Every: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)
	go b.Run(ctx)
	for deadline := time.Now().Add(5 * time.Second); b.Count() == 0; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("no places arrived")
		}
	}
}
