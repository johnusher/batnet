package lora

import (
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

// Item is something waiting to go on air: one message, as one or more frames.
type Item struct {
	Key      string
	Kind     Kind
	Base     prio.Level
	Enqueued int64
	Frames   [][]byte
	Next     int // frames already sent

	entry *ledger.Entry // for entries: what to put back if another gateway's copy never completes
}

// Queue orders items by effective level (with ageing), then by age.
type Queue struct {
	items []*Item
	keys  map[string]bool
}

// Push adds an item unless one with the same key is waiting.
func (q *Queue) Push(it *Item) bool {
	if q.keys == nil {
		q.keys = map[string]bool{}
	}
	if q.keys[it.Key] {
		return false
	}
	q.keys[it.Key] = true
	q.items = append(q.items, it)
	return true
}

// Has reports whether key is waiting.
func (q *Queue) Has(key string) bool { return q.keys[key] }

// Remove drops key; false if it wasn't there.
func (q *Queue) Remove(key string) bool {
	if !q.keys[key] {
		return false
	}
	delete(q.keys, key)
	for i, it := range q.items {
		if it.Key == key {
			q.items = append(q.items[:i], q.items[i+1:]...)
			break
		}
	}
	return true
}

// Peek is the item to send next.
func (q *Queue) Peek(nowUs int64) *Item {
	var best *Item
	var bestLevel prio.Level
	for _, it := range q.items {
		l := prio.Effective(it.Base, nowUs-it.Enqueued)
		if best == nil || l > bestLevel || (l == bestLevel && it.Enqueued < best.Enqueued) {
			best, bestLevel = it, l
		}
	}
	return best
}

// Len is how many items wait.
func (q *Queue) Len() int { return len(q.items) }

// DutyMeter tracks a radio's airtime over a sliding hour.
type DutyMeter struct {
	LimitUs, WindowUs int64
	spans             [][2]int64
}

// NewDutyMeter allows 360 s in any hour.
func NewDutyMeter() *DutyMeter { return &DutyMeter{LimitUs: DutyLimitUs, WindowUs: HourUs} }

// Record notes a transmission.
func (d *DutyMeter) Record(startUs, durUs int64) { d.spans = append(d.spans, [2]int64{startUs, durUs}) }

// Used is the airtime in the hour before now.
func (d *DutyMeter) Used(nowUs int64) int64 {
	keep := d.spans[:0]
	var used int64
	for _, s := range d.spans {
		if s[0]+s[1] <= nowUs-d.WindowUs {
			continue
		}
		keep = append(keep, s)
		used += s[1]
	}
	d.spans = keep
	return used
}

// AllowedAt is the earliest time a transmission of durUs fits under the limit.
func (d *DutyMeter) AllowedAt(nowUs, durUs int64) int64 {
	used := d.Used(nowUs)
	if used+durUs <= d.LimitUs {
		return nowUs
	}
	for _, s := range d.spans {
		used -= s[1]
		if used+durUs <= d.LimitUs {
			return s[0] + s[1] + d.WindowUs
		}
	}
	return nowUs + d.WindowUs
}
