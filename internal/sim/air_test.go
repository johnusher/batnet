package sim

import (
	"context"
	"testing"

	"github.com/johnusher/batnet/internal/meshsync"
)

var _ meshsync.Link = (*AirNode)(nil)

func TestWiFiRange(t *testing.T) {
	cases := []struct {
		pi, radio string
		want      float64
	}{{"Pi 4", "builtin", 120}, {"Zero 2 W", "builtin", 100}, {"Zero W", "builtin", 80}, {"Pi 3", "dongle", 350}, {"Pi 5", "pole", 450}, {"Pi 4", "none", 0}}
	for _, c := range cases {
		if got := WiFiRange(c.pi, c.radio); got != c.want {
			t.Errorf("WiFiRange(%q, %q) = %v, want %v", c.pi, c.radio, got, c.want)
		}
	}
}

func listen(n *AirNode) *[]meshsync.Packet {
	var got []meshsync.Packet
	n.SetDeliver(func(p meshsync.Packet) { got = append(got, p) })
	return &got
}

func TestFloodingThroughARelay(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 1)
	air.BaseLoss = 0
	a := air.Add("a", 0, 0, 350)
	r := air.Add("relay", 300, 0, 350)
	b := air.Add("b", 600, 0, 350)
	if air.Linked(a, b) || !air.Linked(a, r) {
		t.Fatal("wrong links")
	}
	gotR, gotB := listen(r), listen(b)
	a.Broadcast([]byte("hello"))
	if len(*gotB) != 0 {
		t.Fatal("delivered before any virtual time passed")
	}
	s.RunUntil(Second)
	if len(*gotR) != 1 || len(*gotB) != 1 || (*gotB)[0].From != "a" {
		t.Fatalf("relay got %d, b got %d", len(*gotR), len(*gotB))
	}
}

func TestPowerOff(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 1)
	air.BaseLoss = 0
	a, b := air.Add("a", 0, 0, 350), air.Add("b", 100, 0, 350)
	got := listen(b)
	b.SetHandler(func(string, []byte) []byte { return []byte("ok") })
	b.On = false
	a.Broadcast([]byte("x"))
	s.RunUntil(Second)
	if len(*got) != 0 {
		t.Fatal("a node without power received a frame")
	}
	if _, err := a.Request(context.Background(), "b", nil); err == nil {
		t.Fatal("a node without power answered a request")
	}
	b.On = true
	if resp, err := a.Request(context.Background(), "b", nil); err != nil || string(resp) != "ok" {
		t.Fatalf("after power returned: %q, %v", resp, err)
	}
}

func TestBusyChannelLosesAndDelays(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 3)
	air.BaseLoss = 0
	nodes := []*AirNode{}
	for i := 0; i < 10; i++ {
		nodes = append(nodes, air.Add(string(rune('a'+i)), float64(i*30), 0, 350))
	}
	var last int64
	nodes[9].SetDeliver(func(meshsync.Packet) { last = s.Now() })
	for i := 0; i < 400; i++ { // a burst: 400 frames of 300 bytes at the same instant
		nodes[i%9].Broadcast(make([]byte, 300))
	}
	s.RunUntil(Minute)
	if air.Stats.Lost == 0 {
		t.Fatal("no frames lost on a saturated channel")
	}
	if last < Second {
		t.Fatalf("last delivery at %d µs: a saturated channel should delay frames past 1 s", last)
	}
	if air.Stats.MaxUtil < 0.9 {
		t.Fatalf("max utilisation %.2f, want near 1", air.Stats.MaxUtil)
	}
}

func TestSeparateGroupsSeparateChannels(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 1)
	air.BaseLoss = 0
	a, b := air.Add("a", 0, 0, 120), air.Add("b", 100, 0, 120)
	c, d := air.Add("c", 5000, 0, 120), air.Add("d", 5100, 0, 120)
	for i := 0; i < 300; i++ {
		a.Broadcast(make([]byte, 300)) // keep group 1 busy
	}
	var at int64 = -1
	d.SetDeliver(func(meshsync.Packet) { at = s.Now() })
	c.Broadcast([]byte("quiet over here"))
	s.RunUntil(Minute)
	if at < 0 || at > 10*Ms {
		t.Fatalf("group 2's frame arrived at %d µs; group 1's traffic must not delay it", at)
	}
	if g := air.Groups(); len(g) != 2 || len(g[0]) != 2 || len(g[1]) != 2 {
		t.Fatalf("groups %v", g)
	}
	_ = b
}

// Review: repair requests use airtime on the group's channel and can fail when it is busy.
func TestRequestsCostAirtime(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 4)
	air.BaseLoss = 0
	a, b := air.Add("a", 0, 0, 350), air.Add("b", 100, 0, 350)
	b.SetHandler(func(string, []byte) []byte { return make([]byte, 5000) })
	if _, err := a.Request(context.Background(), "b", make([]byte, 100)); err != nil {
		t.Fatal(err)
	}
	if air.Stats.RequestBytes != 5100 || air.Stats.MaxUtil == 0 {
		t.Fatalf("a request and its answer used no airtime: %+v", air.Stats)
	}
	for i := 0; i < 2000; i++ { // saturate the channel
		a.Broadcast(make([]byte, 300))
	}
	failed := 0
	for i := 0; i < 50; i++ {
		if _, err := a.Request(context.Background(), "b", make([]byte, 100)); err != nil {
			failed++
		}
	}
	if failed == 0 {
		t.Fatal("no request failed on a saturated channel")
	}
}
