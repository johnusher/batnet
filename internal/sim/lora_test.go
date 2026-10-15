package sim

import (
	"testing"

	"github.com/johnusher/batnet/internal/lora"
)

func TestLoRaReachAndAirtime(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	air.BaseLoss, air.EdgeLoss = 0, 0
	var gotNear, gotFar []int64
	a := air.Add("a", 0, 0, nil)
	air.Add("near", 2000, 0, func([]byte) { gotNear = append(gotNear, s.Now()) })
	air.Add("far", 9000, 0, func([]byte) { gotFar = append(gotFar, s.Now()) })
	dur := lora.AirtimeUs(100, air.Preset)
	a.Transmit(make([]byte, 100), dur)
	s.RunUntil(10 * Second)
	if len(gotNear) != 1 || gotNear[0] != dur {
		t.Fatalf("near heard %v, want once at %d", gotNear, dur)
	}
	if len(gotFar) != 0 {
		t.Fatal("a radio 9 km away heard a LongFast packet")
	}
}

func TestCollision(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	air.BaseLoss, air.EdgeLoss = 0, 0
	got := 0
	a := air.Add("a", 0, 0, nil)
	b := air.Add("b", 2000, 0, nil)
	air.Add("mid", 1000, 0, func([]byte) { got++ })
	dur := lora.AirtimeUs(100, air.Preset)
	a.Transmit(make([]byte, 100), dur)
	b.Transmit(make([]byte, 100), dur) // same time: they collide at mid
	s.RunUntil(10 * Second)
	if got != 0 || air.Stats.Collided == 0 {
		t.Fatalf("mid heard %d packets, collided %d", got, air.Stats.Collided)
	}
}

func TestManagedFloodingExtendsReach(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 2)
	air.BaseLoss, air.EdgeLoss = 0, 0
	heard := 0
	a := air.Add("a", 0, 0, nil)
	air.Add("relay", 2800, 0, nil) // a Meshtastic user in between
	air.Add("b", 5600, 0, func([]byte) { heard++ })
	a.Transmit(make([]byte, 60), lora.AirtimeUs(60, air.Preset))
	s.RunUntil(30 * Second)
	if heard != 1 {
		t.Fatalf("b heard the packet %d times, want once (through the relay)", heard)
	}
	if air.Stats.Rebroadcasts > 2 {
		t.Fatalf("%d rebroadcasts for one packet: flooding is storming", air.Stats.Rebroadcasts)
	}
}

func TestBusyUntil(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	a, b := air.Add("a", 0, 0, nil), air.Add("b", 1000, 0, nil)
	dur := lora.AirtimeUs(100, air.Preset)
	a.Transmit(make([]byte, 100), dur)
	if b.BusyUntil() != dur {
		t.Fatalf("b sees the channel busy until %d, want %d", b.BusyUntil(), dur)
	}
}
