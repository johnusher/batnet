// Package hlc is a hybrid logical clock: wall-clock milliseconds plus a
// counter, so events order correctly even when a Pi's clock is wrong.
package hlc

import "sync"

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
	if p := c.phys(); p > c.last.Wall {
		c.last = Timestamp{Wall: p}
	} else {
		c.last.Logical++
	}
	return c.last
}

// Observe merges a timestamp received from another node.
func (c *Clock) Observe(r Timestamp) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last.Less(r) {
		c.last = r
	}
}
