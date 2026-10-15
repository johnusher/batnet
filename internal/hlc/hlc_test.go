package hlc

import "testing"

func TestNowIncreases(t *testing.T) {
	c := New(func() int64 { return 1000 })
	a, b := c.Now(), c.Now()
	if !a.Less(b) {
		t.Fatalf("second Now %v not after first %v", b, a)
	}
}

func TestNowFollowsPhysicalClock(t *testing.T) {
	ms := int64(1000)
	c := New(func() int64 { return ms })
	c.Now()
	ms = 5000
	if got := c.Now(); got.Wall != 5000 || got.Logical != 0 {
		t.Fatalf("got %+v, want {5000 0}", got)
	}
}

func TestObserveMovesPastRemote(t *testing.T) {
	c := New(func() int64 { return 1000 })
	remote := Timestamp{Wall: 9000, Logical: 7}
	c.Observe(remote)
	if got := c.Now(); !remote.Less(got) {
		t.Fatalf("Now %v not after observed %v", got, remote)
	}
}

// Review focus 4: a Pi that booted with its clock at 1970 still orders new
// events after everything it has seen.
func TestWrongClockStillOrders(t *testing.T) {
	c := New(func() int64 { return 0 })
	seen := Timestamp{Wall: 1_790_000_000_000}
	c.Observe(seen)
	a := c.Now()
	b := c.Now()
	if !seen.Less(a) || !a.Less(b) {
		t.Fatalf("order broken: seen %v, a %v, b %v", seen, a, b)
	}
}

func TestLess(t *testing.T) {
	cases := []struct {
		a, b Timestamp
		want bool
	}{
		{Timestamp{1, 0}, Timestamp{2, 0}, true},
		{Timestamp{2, 0}, Timestamp{1, 9}, false},
		{Timestamp{1, 1}, Timestamp{1, 2}, true},
		{Timestamp{1, 2}, Timestamp{1, 2}, false},
	}
	for _, c := range cases {
		if got := c.a.Less(c.b); got != c.want {
			t.Errorf("%v.Less(%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
