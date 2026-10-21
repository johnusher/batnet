package hub

import (
	"context"
	"encoding/binary"
	"sync"
	"testing"
	"time"
)

func TestChimeIsAWAV(t *testing.T) {
	for k := range chimes {
		b := Chime(k)
		if string(b[:4]) != "RIFF" || string(b[8:16]) != "WAVEfmt " || string(b[36:40]) != "data" {
			t.Fatalf("%s: not a WAV header", k)
		}
		if n := binary.LittleEndian.Uint32(b[40:44]); int(n) != len(b)-44 {
			t.Fatalf("%s: data size %d of %d", k, n, len(b)-44)
		}
		if binary.LittleEndian.Uint32(b[24:28]) != sampleRate {
			t.Fatalf("%s: rate", k)
		}
	}
}

func TestSoundCards(t *testing.T) {
	c, ok := parseCardLine(" 0 [Headphones     ]: bcm2835_headpho - bcm2835 Headphones")
	if !ok || c.ID != "Headphones" || c.Kind != "jack" || c.Name != "bcm2835 Headphones" {
		t.Fatalf("%+v", c)
	}
	u, _ := parseCardLine(" 3 [Device         ]: USB-Audio - USB Audio Device")
	h, _ := parseCardLine(" 1 [vc4hdmi0       ]: vc4-hdmi - vc4-hdmi-0")
	if u.Kind != "usb" || h.Kind != "hdmi" {
		t.Fatalf("%+v %+v", u, h)
	}
	if _, ok := parseCardLine("                      bcm2835 Headphones"); ok {
		t.Fatal("a second line read as a card")
	}
	cards := []SoundCard{c, h, u}
	if pickCard(cards, "auto").Kind != "usb" || pickCard(cards, "jack").Kind != "jack" || pickCard(cards[:2], "auto").Kind != "jack" {
		t.Fatal("pickCard")
	}
}

func TestQuietHours(t *testing.T) {
	s := DefaultSound
	s.Quiet = true
	at := func(h, m int) time.Time { return time.Date(2026, 10, 21, h, m, 0, 0, time.Local) }
	if !quietNow(s, at(23, 0)) || !quietNow(s, at(3, 0)) || quietNow(s, at(7, 0)) || quietNow(s, at(12, 0)) {
		t.Fatal("22:00-07:00 wrong")
	}
	s.QuietFrom, s.QuietTo = "13:00", "14:00"
	if !quietNow(s, at(13, 30)) || quietNow(s, at(14, 30)) {
		t.Fatal("13-14 wrong")
	}
}

type played struct {
	mu   sync.Mutex
	devs []string
}

func TestSoundNotifyRules(t *testing.T) {
	p := &played{}
	s := NewSound(SoundConfig{Settings: DefaultSound})
	s.cards = func() []SoundCard { return []SoundCard{{ID: "Headphones", Kind: "jack", Name: "jack"}} }
	s.play = func(_ context.Context, dev string, wav []byte) error {
		p.mu.Lock()
		p.devs = append(p.devs, dev)
		p.mu.Unlock()
		return nil
	}
	s.speak = func(context.Context, string) ([]byte, error) { return Chime("delivered"), nil }
	clk := &fakeClock{t: time.Date(2026, 10, 21, 12, 0, 0, 0, time.Local)}
	s.now = clk.now
	s.Notify("public", "Anna", "hi")
	s.Notify("public", "Ben", "hi") // within 30 s: no second chime
	s.Notify("delivered", "", "")   // off by default
	s.Notify("direct", "Anna", "hi")
	if len(s.q) != 2 {
		t.Fatalf("queued %d, want 2", len(s.q))
	}
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	defer cancel()
	for i := 0; i < 100 && len(s.q) > 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	p.mu.Lock()
	if len(p.devs) != 2 || p.devs[0] != "plughw:CARD=Headphones,DEV=0" {
		t.Fatalf("played %v", p.devs)
	}
	p.mu.Unlock()
	// Quiet hours keep only messages for the hub.
	set := DefaultSound
	set.Quiet = true
	s.Configure(set)
	clk.t = time.Date(2026, 10, 21, 23, 0, 0, 0, time.Local)
	s.Notify("reply", "Anna", "")
	if len(s.q) != 0 {
		t.Fatal("a reply sounded in quiet hours")
	}
	if err := s.Configure(SoundSettings{Output: "hdmi", Volume: 50, QuietFrom: "22:00", QuietTo: "07:00"}); err == nil {
		t.Fatal("hdmi accepted")
	}
}
