// Package sim runs a village of real batnet nodes in virtual time, over a
// model of the physical world, for scenario tests and the simulator page.
package sim

import "container/heap"

// Units of virtual time.
const (
	Ms     int64 = 1000
	Second int64 = 1000 * Ms
	Minute int64 = 60 * Second
)

// Scheduler runs callbacks in virtual time (microseconds), in time order and,
// for equal times, in the order they were scheduled.
type Scheduler struct {
	now int64
	seq uint64
	q   events
}

type event struct {
	at  int64
	seq uint64
	fn  func()
}

type events []event

func (e events) Len() int { return len(e) }
func (e events) Less(i, j int) bool {
	if e[i].at != e[j].at {
		return e[i].at < e[j].at
	}
	return e[i].seq < e[j].seq
}
func (e events) Swap(i, j int) { e[i], e[j] = e[j], e[i] }
func (e *events) Push(x any)   { *e = append(*e, x.(event)) }
func (e *events) Pop() any {
	old := *e
	x := old[len(old)-1]
	*e = old[:len(old)-1]
	return x
}

// Now is the current virtual time.
func (s *Scheduler) Now() int64 { return s.now }

// At runs fn at time t (now, if t is in the past).
func (s *Scheduler) At(t int64, fn func()) {
	if t < s.now {
		t = s.now
	}
	s.seq++
	heap.Push(&s.q, event{at: t, seq: s.seq, fn: fn})
}

// After runs fn d from now.
func (s *Scheduler) After(d int64, fn func()) { s.At(s.now+d, fn) }

// RunUntil runs every event due by t, then sets the clock to t. It returns how many ran.
func (s *Scheduler) RunUntil(t int64) int {
	n := 0
	for len(s.q) > 0 && s.q[0].at <= t {
		e := heap.Pop(&s.q).(event)
		s.now = e.at
		e.fn()
		n++
	}
	if t > s.now {
		s.now = t
	}
	return n
}

// Pending is the number of events not yet run.
func (s *Scheduler) Pending() int { return len(s.q) }
