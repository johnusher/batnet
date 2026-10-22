package prio

import (
	"fmt"
	"sync"
	"testing"
)

func TestForPost(t *testing.T) {
	cases := []struct {
		c        Choice
		audience string
		resend   bool
		want     Level
	}{
		{"normal", "everyone", false, 5},
		{"normal", "dm", false, 6},
		{"normal", "public", false, 4},
		{"normal", "everyone", true, 4},
		{"low", "everyone", false, 2},
		{"low", "dm", false, 3},
		{"high", "everyone", false, 8},
		{"high", "public", true, 8}, // High never drops below 8
		{"high", "dm", false, 8},    // nor above: 9 is the admin's Alert (B13)
		{"alert", "everyone", false, 9},
		{"", "everyone", false, 5}, // empty means normal
	}
	for _, c := range cases {
		if got := ForPost(c.c, c.audience, c.resend); got != c.want {
			t.Errorf("ForPost(%q, %q, %v) = %d, want %d", c.c, c.audience, c.resend, got, c.want)
		}
	}
}

func TestEffective(t *testing.T) {
	min := int64(60_000_000)
	cases := []struct {
		base   Level
		waited int64
		want   Level
	}{
		{Normal, 0, 5}, {Normal, 19 * min, 5}, {Normal, 20 * min, 6}, {Normal, 41 * min, 7},
		{Normal, 300 * min, 7}, // ageing stops at 7
		{Low, 60 * min, 5},
		{High, 300 * min, 8}, {Alert, 0, 9},
		{Housekeeping, 0, 1},
	}
	for _, c := range cases {
		if got := Effective(c.base, c.waited); got != c.want {
			t.Errorf("Effective(%d, %d min) = %d, want %d", c.base, c.waited/min, got, c.want)
		}
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(3)
	hour := int64(3_600_000_000)
	for i := 0; i < 3; i++ {
		if !l.Allow("anna", int64(i)) {
			t.Fatalf("High %d refused", i+1)
		}
	}
	if l.Allow("anna", 10) {
		t.Fatal("a fourth High in the hour allowed")
	}
	if !l.Allow("ben", 10) {
		t.Fatal("another person's High refused")
	}
	if !l.Allow("anna", hour+1) {
		t.Fatal("High refused after the hour passed")
	}
}

// The house server calls Allow from every request at once.
func TestLimiterUnderParallelUse(t *testing.T) {
	l := NewLimiter(3)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if l.Allow("anna", int64(i)) {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
			l.Allow(fmt.Sprintf("person-%d", i), int64(i))
		}(i)
	}
	wg.Wait()
	if allowed != 3 {
		t.Fatalf("%d of 40 parallel High posts allowed, want 3", allowed)
	}
}
