// Package prio turns a person's Low / Normal / High (or an admin's Alert) into
// the 0-9 level the radio queue uses (decision 0015).
package prio

import "sync"

// Level is 0-9; higher goes first.
type Level uint8

const (
	Housekeeping Level = 1 // directory cards and housekeeping
	Low          Level = 2
	Normal       Level = 5
	Beacon       Level = 6 // sync beacons keep recovery working
	AgedCap      Level = 7 // waiting never lifts a message past this
	High         Level = 8
	Alert        Level = 9
)

// Choice is what a person picked.
type Choice string

const hourUs = int64(3_600_000_000)

// ForPost is a post's level: the choice, +1 for a direct message, -1 for a
// public post or a resend; High is always 8, Alert always 9. Nothing a
// person chooses reaches 9: that level is the admin's Alert alone.
func ForPost(c Choice, audience string, resend bool) Level {
	if c == "alert" {
		return Alert
	}
	if c == "high" {
		return High
	}
	base := map[Choice]int{"low": int(Low), "normal": int(Normal), "": int(Normal)}[c]
	if base == 0 {
		base = int(Normal)
	}
	if audience == "dm" {
		base++
	}
	if audience == "public" {
		base--
	}
	if resend {
		base--
	}
	switch {
	case base >= int(High):
		base = int(High) - 1
	case base < 0:
		base = 0
	}
	return Level(base)
}

// Effective raises a waiting message one level per 20 minutes, up to 7;
// High and Alert stay as they are.
func Effective(base Level, waitedUs int64) Level {
	if base >= High {
		return base
	}
	l := int(base) + int(waitedUs/(20*60_000_000))
	if l > int(AgedCap) {
		if base > AgedCap {
			return base
		}
		return AgedCap
	}
	return Level(l)
}

// Limiter allows each person PerHour messages in any sliding hour. It is
// safe for concurrent use: the house server calls it from every request.
type Limiter struct {
	PerHour int
	mu      sync.Mutex
	sent    map[string][]int64
}

// NewLimiter returns a limiter of perHour per person.
func NewLimiter(perHour int) *Limiter { return &Limiter{PerHour: perHour, sent: map[string][]int64{}} }

// Allow records and allows a message, or refuses it.
func (l *Limiter) Allow(person string, nowUs int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.sent) > 1000 { // forget people whose hour has passed, so the table can't grow forever
		for p, ts := range l.sent {
			if len(ts) == 0 || nowUs-ts[len(ts)-1] >= hourUs {
				delete(l.sent, p)
			}
		}
	}
	keep := l.sent[person][:0]
	for _, t := range l.sent[person] {
		if nowUs-t < hourUs {
			keep = append(keep, t)
		}
	}
	if len(keep) >= l.PerHour {
		l.sent[person] = keep
		return false
	}
	l.sent[person] = append(keep, nowUs)
	return true
}
