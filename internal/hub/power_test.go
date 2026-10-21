package hub

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPowerScreenSleepsAndWakes(t *testing.T) {
	var mu sync.Mutex
	var units []string
	run := func(_ context.Context, name string, args ...string) (string, error) {
		mu.Lock()
		units = append(units, name+" "+strings.Join(args, " "))
		mu.Unlock()
		return "", nil
	}
	p := NewPower(PowerConfig{Run: run, Settings: PowerSettings{Mode: PowerMedium}})
	clk := &fakeClock{t: time.Date(2026, 10, 21, 22, 0, 0, 0, time.UTC)}
	p.now = clk.now
	p.lastAct = clk.t
	p.apply()
	mu.Lock()
	if len(units) != 1 || units[0] != "systemctl --no-ask-password start batnet-power@medium.service" {
		t.Fatalf("%q", units)
	}
	mu.Unlock()
	clk.t = clk.t.Add(9 * time.Minute)
	p.check()
	if st := p.Status(); !st.ScreenOn || st.SleepsIn != 60 {
		t.Fatalf("%+v", st)
	}
	clk.t = clk.t.Add(2 * time.Minute)
	p.check()
	if p.Status().ScreenOn {
		t.Fatal("the screen did not sleep after 10 minutes")
	}
	if got := p.WaitScreen(context.Background(), "on", time.Second); got != "off" {
		t.Fatal(got)
	}
	// A kiosk waiting for a change hears the wake at once.
	done := make(chan string)
	go func() { done <- p.WaitScreen(context.Background(), "off", 5*time.Second) }()
	time.Sleep(20 * time.Millisecond)
	p.Wake("direct")
	select {
	case got := <-done:
		if got != "on" {
			t.Fatal(got)
		}
	case <-time.After(time.Second):
		t.Fatal("the wake did not reach the kiosk")
	}
	// Low power: reduced screen, and a message does not wake it.
	p.Configure(PowerSettings{Mode: PowerLow})
	clk.t = clk.t.Add(3 * time.Minute)
	p.check()
	p.Wake("direct")
	if st := p.Status(); st.ScreenOn || st.PollSeconds != 30 || !st.Reduced {
		t.Fatalf("%+v", st)
	}
	p.Activity()
	if got := p.WaitScreen(context.Background(), "off", time.Second); got != "on reduced" {
		t.Fatal(got)
	}
	// High: never sleeps.
	p.Configure(PowerSettings{Mode: PowerHigh})
	clk.t = clk.t.Add(24 * time.Hour)
	p.check()
	if st := p.Status(); !st.ScreenOn || st.SleepsIn != -1 {
		t.Fatalf("%+v", st)
	}
	if p.Configure(PowerSettings{Mode: "turbo"}) == nil {
		t.Fatal("turbo accepted")
	}
}
