// Package hlc is a hybrid logical clock: wall-clock milliseconds plus a
// counter, so events order correctly even when a Pi's clock is wrong.
package hlc

import (
	"math"
	"sync"
)

const (
	// MaxSkew is how far ahead of a plausible local clock a received stamp may be
	// and still move this clock: a Pi that booted in 2099, or a forged stamp,
	// must not drag the village's clocks along.
	MaxSkew = 24 * 60 * 60 * 1000 // 1 day, in ms
	// MinPlausible is 2026-01-01 UTC in ms. A clock below it is wrong (a Pi 4
	// after a power cut starts near 1970), so the node follows the village.
	MinPlausible = 1_767_225_600_000
)

// Timestamp orders events across nodes.
type Timestamp struct {
	Wall    int64  // milliseconds since the Unix epoch, as best known
	Logical uint32 // breaks ties within one millisecond
}

// Less reports whether t is before o.
func (t Timestamp) Less(o Timestamp) bool {
	if t.Wall != o.Wall {
		return t.Wall < o.Wall
	}
	return t.Logical < o.Logical
}

// Clock hands out timestamps later than every timestamp it has produced or seen.
type Clock struct {
	mu   sync.Mutex
	phys func() int64
	last Timestamp
}

// New returns a clock reading physical time from phys (milliseconds).
func New(phys func() int64) *Clock { return &Clock{phys: phys} }

// Now returns a timestamp for a local event.
func (c *Clock) Now() Timestamp {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch p := c.phys(); {
	case p > c.last.Wall:
		c.last = Timestamp{Wall: p}
	case c.last.Logical == math.MaxUint32:
		c.last = Timestamp{Wall: c.last.Wall + 1} // full counter: move on a millisecond, never wrap
	default:
		c.last.Logical++
	}
	return c.last
}

// Observe merges a timestamp received from another node. With a plausible
// local clock, a stamp more than MaxSkew ahead of it is ignored.
func (c *Clock) Observe(r Timestamp) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p := c.phys(); p >= MinPlausible && r.Wall > p+MaxSkew {
		return
	}
	if c.last.Less(r) {
		c.last = r
	}
}
