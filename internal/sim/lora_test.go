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
	s.RunUntil(100 * Ms) // past the carrier-sense delay
	if b.BusyUntil() != dur {
		t.Fatalf("b sees the channel busy until %d, want %d", b.BusyUntil(), dur)
	}
}

// A rebroadcast that finds the channel busy waits for it, instead of being dropped.
func TestRebroadcastWaitsForBusyChannel(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 2)
	air.BaseLoss, air.EdgeLoss = 0, 0
	heard := 0
	a := air.Add("a", 0, 0, nil)
	air.Add("relay", 2800, 0, nil)
	air.Add("b", 5600, 0, func(f []byte) {
		if len(f) == 60 { // a's packet, not the noise
			heard++
		}
	})
	noisy := air.Add("noisy", 2800, 2000, nil) // heard by the relay, not by a or b: keeps the relay's channel busy
	a.Transmit(make([]byte, 60), lora.AirtimeUs(60, air.Preset))
	dur := lora.AirtimeUs(60, air.Preset)
	s.At(dur, func() { noisy.Transmit(make([]byte, 190), lora.AirtimeUs(190, air.Preset)) }) // ~1.9 s, longer than the relay's 0.2-1.2 s delay
	s.RunUntil(30 * Second)
	if heard != 1 {
		t.Fatalf("b heard %d times; the relay must rebroadcast after the busy channel clears", heard)
	}
}

// Review I4: two identical frames sent minutes apart are two packets, both heard.
func TestIdenticalFramesAreSeparatePackets(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	air.BaseLoss, air.EdgeLoss = 0, 0
	heard := 0
	a := air.Add("a", 0, 0, nil)
	air.Add("b", 1000, 0, func([]byte) { heard++ })
	frame := []byte("the same beacon twice")
	a.Transmit(frame, lora.AirtimeUs(len(frame), air.Preset))
	s.At(15*Minute, func() { a.Transmit(frame, lora.AirtimeUs(len(frame), air.Preset)) })
	s.RunUntil(20 * Minute)
	if heard != 2 {
		t.Fatalf("b heard %d of 2 identical transmissions", heard)
	}
}

// B15: a packet is rebroadcast at most MaxHops times (Meshtastic's hop
// limit), so a chain of relays doesn't carry it for ever.
func TestHopLimit(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 3)
	air.BaseLoss, air.EdgeLoss = 0, 0
	heard := map[string]int{}
	a := air.Add("a", 0, 0, nil)
	for i := 1; i <= 5; i++ { // relays 2.8 km apart: each hears only its neighbours
		name := string(rune('a' + i))
		air.Add(name, float64(i)*2800, 0, func([]byte) { heard[name]++ })
	}
	a.Transmit(make([]byte, 60), lora.AirtimeUs(60, air.Preset))
	s.RunUntil(2 * Minute)
	for i := 1; i <= MaxHops+1; i++ { // within reach of hop 0..MaxHops
		if heard[string(rune('a'+i))] != 1 {
			t.Fatalf("radio %d hops away heard it %d times, want once", i, heard[string(rune('a'+i))])
		}
	}
	if heard[string(rune('a'+MaxHops+2))] != 0 {
		t.Fatalf("a radio %d hops away heard the packet: no hop limit", MaxHops+2)
	}
}

// B14: the duty check reads the air's own tally of what each radio sent, so a
// sender that misreports its airtime is still caught.
func TestAirTalliesAirtimeItself(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	a := air.Add("a", 0, 0, nil)
	dur := lora.AirtimeUs(100, air.Preset)
	a.Transmit(make([]byte, 100), dur) // straight to the air: a.Duty was never told
	s.RunUntil(Second)
	if a.Duty.Used(s.Now()) != 0 {
		t.Fatal("the test needs a transmission the radio's own meter missed")
	}
	if got := air.Used(a, s.Now()); got != dur {
		t.Fatalf("the air tallied %d µs for a, want %d", got, dur)
	}
}

// Review I5: a radio needs a moment to notice another one has started.
func TestCarrierSenseTakesTime(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	a, b := air.Add("a", 0, 0, nil), air.Add("b", 1000, 0, nil)
	a.Transmit(make([]byte, 50), lora.AirtimeUs(50, air.Preset))
	s.RunUntil(10 * Ms)
	if b.BusyUntil() > s.Now() {
		t.Fatal("b noticed a's transmission 10 ms after it began")
	}
	s.RunUntil(100 * Ms)
	if b.BusyUntil() <= s.Now() {
		t.Fatal("b still hears a free channel 100 ms into a's transmission")
	}
}
