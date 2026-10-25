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
)

// NewSightings starts with none.
func NewSightings() *Sightings { return &Sightings{m: map[string]*Sighting{}} }

// Event records a packet heard from another radio.
func (s *Sightings) Event(e meshtastic.Event) {
	if e.Kind == "packet" && !e.Local {
		s.add(e.From, e.Relay, e.Hops, e.At)
	}
}

// Garbled records a packet heard but not readable: its header still says who
// sent it and who passed it on.
func (s *Sightings) Garbled(u meshtastic.Unreadable) { s.add(u.From, u.Relay, u.Hops, u.At) }

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
	s.mu.Lock()
	defer s.mu.Unlock()
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
