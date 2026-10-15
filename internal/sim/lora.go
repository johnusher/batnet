package sim

import (
	"math"
	"math/rand"

	"github.com/johnusher/batnet/internal/lora"
)

// LoRaAir is the 868 MHz channel in virtual time: one preset for everyone,
// range and distance loss, collisions, and Meshtastic's managed flooding.
type LoRaAir struct {
	Preset     lora.Preset
	BaseLoss   float64
	EdgeLoss   float64 // extra loss at the edge of range, growing with distance cubed (default 0.3)
	Stats      LoRaStats
	OnTransmit func(from string, at, durUs int64, heard []string)

	sch    *Scheduler
	rnd    *rand.Rand
	radios []*LoRaRadio
	live   []transmission // recent and current packets, for collisions
	nextID uint64         // each transmission is a new packet (Meshtastic's from + id)
}

// carrierSense is how long a radio takes to notice another has started.
const carrierSense = 50 * Ms

// LoRaStats counts packets since the start.
type LoRaStats struct{ Packets, Delivered, Collided, Lost, Rebroadcasts int }

type transmission struct {
	from       *LoRaRadio
	start, end int64
}

// LoRaRadio is one node's LoRa radio (a batnet gateway or a Meshtastic user).
type LoRaRadio struct {
	Name    string
	X, Y    float64
	On      bool
	Duty    *lora.DutyMeter
	air     *LoRaAir
	onFrame func([]byte)
	seen    map[uint64]bool // packets heard (for managed flooding)
	pending map[uint64]bool // rebroadcasts waiting
}

// NewLoRaAir returns an empty channel for one preset.
func NewLoRaAir(s *Scheduler, p lora.Preset, seed int64) *LoRaAir {
	return &LoRaAir{Preset: p, BaseLoss: 0.02, EdgeLoss: 0.3, sch: s, rnd: rand.New(rand.NewSource(seed))}
}

// Add puts a radio at (x, y); onFrame receives what it hears (nil for none).
func (a *LoRaAir) Add(name string, x, y float64, onFrame func([]byte)) *LoRaRadio {
	r := &LoRaRadio{Name: name, X: x, Y: y, On: true, Duty: lora.NewDutyMeter(), air: a, onFrame: onFrame, seen: map[uint64]bool{}, pending: map[uint64]bool{}}
	a.radios = append(a.radios, r)
	return r
}

// Reach reports whether two radios hear each other directly.
func (a *LoRaAir) Reach(p, q *LoRaRadio) bool {
	return math.Hypot(p.X-q.X, p.Y-q.Y) <= a.Preset.RangeM
}

// Transmit starts sending a frame now (lora.Radio).
func (r *LoRaRadio) Transmit(frame []byte, durUs int64) {
	if !r.On {
		return
	}
	r.air.nextID++
	id := r.air.nextID
	r.seen[id] = true
	r.air.send(r, append([]byte(nil), frame...), durUs, id)
}

// BusyUntil is when the last packet this radio can hear ends (lora.Radio).
// Another radio's packet is noticed only carrierSense after it starts.
func (r *LoRaRadio) BusyUntil() int64 {
	var until int64
	now := r.air.sch.Now()
	for _, t := range r.air.live {
		noticed := t.from == r || (r.air.Reach(t.from, r) && now-t.start >= carrierSense)
		if noticed && t.end > until {
			until = t.end
		}
	}
	return until
}

func (a *LoRaAir) send(from *LoRaRadio, frame []byte, dur int64, id uint64) {
	now := a.sch.Now()
	keep := a.live[:0]
	for _, t := range a.live {
		if t.end > now-10*Second {
			keep = append(keep, t)
		}
	}
	a.live = append(keep, transmission{from: from, start: now, end: now + dur})
	a.Stats.Packets++
	var heard []string
	for _, r := range a.radios {
		if r == from || !r.On || !a.Reach(from, r) {
			continue
		}
		r, d := r, math.Hypot(from.X-r.X, from.Y-r.Y)
		heard = append(heard, r.Name)
		a.sch.At(now+dur, func() { a.arrive(from, r, frame, id, now, now+dur, d) })
	}
	if a.OnTransmit != nil {
		a.OnTransmit(from.Name, now, dur, heard)
	}
}

func (a *LoRaAir) arrive(from, r *LoRaRadio, frame []byte, id uint64, start, end int64, d float64) {
	if !r.On {
		return
	}
	for _, t := range a.live {
		if t.from != from && t.start < end && t.end > start && (t.from == r || a.Reach(t.from, r)) {
			a.Stats.Collided++
			return
		}
	}
	if a.rnd.Float64() < a.BaseLoss+a.EdgeLoss*math.Pow(d/a.Preset.RangeM, 3) {
		a.Stats.Lost++
		return
	}
	if r.seen[id] {
		delete(r.pending, id) // someone else rebroadcast it first: managed flooding stands down
		return
	}
	r.seen[id] = true
	a.Stats.Delivered++
	if r.onFrame != nil {
		r.onFrame(frame)
	}
	r.pending[id] = true
	// Meshtastic's contention window: the farther the radio (weaker signal), the
	// sooner it rebroadcasts; near radios usually hear that and stand down.
	delay := 200*Ms + int64((1-math.Min(1, d/a.Preset.RangeM))*2000)*Ms + a.rnd.Int63n(300*Ms)
	a.sch.After(delay, func() { a.rebroadcast(r, frame, id, 0) })
}

// rebroadcast sends a heard packet on once (managed flooding): it stands down
// if someone else rebroadcast it meanwhile, waits for a busy channel (a few
// times), and never exceeds the radio's duty limit.
func (a *LoRaAir) rebroadcast(r *LoRaRadio, frame []byte, id uint64, tries int) {
	if !r.pending[id] || !r.On {
		return
	}
	dur := lora.AirtimeUs(len(frame), a.Preset)
	now := a.sch.Now()
	if busy := r.BusyUntil(); busy > now {
		if tries < 5 {
			a.sch.At(busy+1+a.rnd.Int63n(2*Second), func() { a.rebroadcast(r, frame, id, tries+1) })
		} else {
			delete(r.pending, id)
		}
		return
	}
	delete(r.pending, id)
	if r.Duty.AllowedAt(now, dur) > now {
		return
	}
	r.Duty.Record(now, dur)
	a.Stats.Rebroadcasts++
	a.send(r, frame, dur, id)
}
