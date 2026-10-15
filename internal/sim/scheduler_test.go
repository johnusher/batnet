package sim

import (
	"reflect"
	"testing"
)

func TestOrderAndTies(t *testing.T) {
	var s Scheduler
	var got []string
	s.At(30, func() { got = append(got, "c") })
	s.At(10, func() { got = append(got, "a") })
	s.At(10, func() { got = append(got, "b") }) // same time: scheduling order
	s.RunUntil(100)
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("order %v", got)
	}
	if s.Now() != 100 {
		t.Fatalf("Now = %d, want 100", s.Now())
	}
}

func TestNestedAndPast(t *testing.T) {
	var s Scheduler
	var got []int64
	s.At(5, func() {
		s.After(5, func() { got = append(got, s.Now()) })
		s.At(1, func() { got = append(got, s.Now()) }) // in the past: runs now
	})
	s.RunUntil(4)
	if len(got) != 0 || s.Pending() != 1 {
		t.Fatalf("ran early: %v, pending %d", got, s.Pending())
	}
	s.RunUntil(20)
	if !reflect.DeepEqual(got, []int64{5, 10}) {
		t.Fatalf("times %v, want [5 10]", got)
	}
}
