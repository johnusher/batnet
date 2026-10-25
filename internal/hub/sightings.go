package hub

import (
	"sort"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
)

// Sightings remembers, for each radio, how its packets reach this hub: by
// which last relay and through how few relays. The panel's guess of where a
// radio with no shared place is (for the hub's owner only) is worked out from
// them and from the radios whose place is known.
type Sightings struct {
	mu sync.Mutex
	m  map[string]*Sighting
	// A garbled copy's header may be garbled too: it counts only once a
	// readable copy of the same packet (ID) names the same sender, before
	// or within copyWindow after it.
	read    map[uint32]seenPacket
	waiting map[uint32][]meshtastic.Unreadable
}

type seenPacket struct {
	from string
	at   time.Time
}

// Sighting is how one radio's packets reached this hub.
type Sighting struct {
	// Via counts packets by the last radio that passed them on (the last
	// byte of its ID, as two hex digits); "direct" is none.
	Via  map[string]int `json:"via"`
	Hops int            `json:"hops"` // the fewest relays any packet came through
	N    int            `json:"n"`
	At   time.Time      `json:"at"`
}

const (
	keepSightings = 21 * 24 * time.Hour // as long as the zone's readings
	maxSightings  = 3000                // radios
	maxVia        = 16                  // last relays per radio
	maxHops       = 7                   // Meshtastic's hop limit
	copyWindow    = 2 * time.Minute     // copies of one packet come within seconds
	maxWaiting    = 2000                // garbled copies waiting for a readable one
)

// NewSightings starts with none.
func NewSightings() *Sightings {
	return &Sightings{m: map[string]*Sighting{}, read: map[uint32]seenPacket{}, waiting: map[uint32][]meshtastic.Unreadable{}}
}

// Event records a packet heard from another radio.
func (s *Sightings) Event(e meshtastic.Event) {
	if e.Kind != "packet" || e.Local {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.add(e.From, e.Relay, e.Hops, e.At)
	if e.ID == 0 {
		return
	}
	s.prune()
	s.read[e.ID] = seenPacket{e.From, time.Now()}
	for _, u := range s.waiting[e.ID] {
		if u.From == e.From {
			s.add(u.From, u.Relay, u.Hops, u.At)
		}
	}
	delete(s.waiting, e.ID)
}

// Garbled records a packet heard but not readable: its header says who sent
// it and who passed it on, once a readable copy bears the sender out.
func (s *Sightings) Garbled(u meshtastic.Unreadable) {
	if u.ID == 0 || u.From == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if p, ok := s.read[u.ID]; ok {
		if p.from == u.From {
			s.add(u.From, u.Relay, u.Hops, u.At)
		}
		return
	}
	if u.At.IsZero() {
		u.At = time.Now()
	}
	if len(s.waiting) < maxWaiting {
		s.waiting[u.ID] = append(s.waiting[u.ID], u)
	}
}

// prune forgets packets older than copyWindow.
func (s *Sightings) prune() {
	for id, p := range s.read {
		if time.Since(p.at) > copyWindow {
			delete(s.read, id)
		}
	}
	for id, us := range s.waiting {
		if len(us) == 0 || time.Since(us[0].At) > copyWindow {
			delete(s.waiting, id)
		}
	}
}

// add counts one copy; s.mu is held.
func (s *Sightings) add(from, relay string, hops int, at time.Time) {
	if from == "" || hops < 0 || hops > maxHops {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	via := relay
	if hops == 0 {
		via = "direct"
	}
	g := s.m[from]
	if g == nil {
		if len(s.m) >= maxSightings {
			s.dropOldest()
		}
		g = &Sighting{Via: map[string]int{}, Hops: hops}
		s.m[from] = g
	}
	if via != "" {
		if _, ok := g.Via[via]; !ok && len(g.Via) >= maxVia {
			least := ""
			for k, n := range g.Via {
				if least == "" || n < g.Via[least] || (n == g.Via[least] && k < least) {
					least = k
				}
			}
			delete(g.Via, least)
		}
		g.Via[via]++
	}
	g.Hops = min(g.Hops, hops)
	g.N++
	if at.After(g.At) {
		g.At = at
	}
}

func (s *Sightings) dropOldest() {
	ids := make([]string, 0, len(s.m))
	for id := range s.m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return s.m[ids[i]].At.Before(s.m[ids[j]].At) })
	for _, id := range ids[:len(ids)/10+1] {
		delete(s.m, id)
	}
}

// Saved returns a copy of the sightings, those older than three weeks left out.
func (s *Sightings) Saved() map[string]Sighting {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]Sighting, len(s.m))
	for id, g := range s.m {
		if time.Since(g.At) > keepSightings {
			delete(s.m, id)
			continue
		}
		c := *g
		c.Via = make(map[string]int, len(g.Via))
		for k, n := range g.Via {
			c.Via[k] = n
		}
		out[id] = c
	}
	return out
}

// Load puts back saved sightings (at start).
func (s *Sightings) Load(m map[string]Sighting) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, g := range m {
		if g.Via == nil {
			g.Via = map[string]int{}
		}
		s.m[id] = &g
	}
}
