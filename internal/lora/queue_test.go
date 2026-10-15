package lora

import (
	"testing"

	"github.com/johnusher/batnet/internal/prio"
)

func TestQueueOrder(t *testing.T) {
	var q Queue
	min := int64(60_000_000)
	q.Push(&Item{Key: "low-old", Base: prio.Low, Enqueued: 0})
	q.Push(&Item{Key: "normal", Base: prio.Normal, Enqueued: 50 * min})
	q.Push(&Item{Key: "high", Base: prio.High, Enqueued: 55 * min})
	q.Push(&Item{Key: "normal-older", Base: prio.Normal, Enqueued: 45 * min})
	if q.Push(&Item{Key: "high"}) {
		t.Fatal("duplicate key queued")
	}
	now := 60 * min
	var got []string
	for q.Len() > 0 {
		it := q.Peek(now)
		got = append(got, it.Key)
		q.Remove(it.Key)
	}
	// low-old has waited an hour: 2 + 3 = 5, ties with Normal but is oldest
	want := []string{"high", "low-old", "normal-older", "normal"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order %v, want %v", got, want)
		}
	}
}

// Review focus 2.
func TestDutyMeter(t *testing.T) {
	d := NewDutyMeter()
	sec := int64(1_000_000)
	for i := int64(0); i < 300; i++ {
		d.Record(i*10*sec, 1200*1000) // 300 x 1.2 s = 360 s in the first 50 min
	}
	now := 50 * 60 * sec
	if d.Used(now) != 360*sec {
		t.Fatalf("used %d", d.Used(now))
	}
	at := d.AllowedAt(now, 1200*1000)
	if at <= now {
		t.Fatal("a radio at its limit may transmit")
	}
	if at != 60*60*sec+1200*1000 { // the first transmission (0 to 1.2 s) ages out an hour later
		t.Fatalf("allowed at %d", at)
	}
}
