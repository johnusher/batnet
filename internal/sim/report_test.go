package sim

import (
	"strings"
	"testing"
)

func TestStreetPasses(t *testing.T) {
	sc, _ := LoadScenario([]byte(street))
	r, _, err := Run(sc)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Pass {
		t.Fatalf("street failed:\n%s", r)
	}
	if r.Posts == 0 || r.Deliveries == 0 || r.LatencyP50Ms <= 0 {
		t.Fatalf("empty report: %+v", r)
	}
	if !strings.Contains(r.String(), "converged") {
		t.Fatalf("report text lacks the checks:\n%s", r)
	}
}

func TestCheckCatchesAMiss(t *testing.T) {
	sc, _ := LoadScenario([]byte(street))
	v, _ := sc.Build()
	v.RunFor(int64(sc.Duration))
	// tamper: pretend Okafor never got the first post
	delete(v.Posts[0].Arrived, "Okafor")
	if r := Judge(sc, v); r.Pass {
		t.Fatal("a missing delivery passed the checks")
	}
}

// Review focus 1.
func TestRunTwiceSameReport(t *testing.T) {
	sc, _ := LoadScenario([]byte(street))
	a, _, _ := Run(sc)
	b, _, _ := Run(sc)
	if a.String() != b.String() {
		t.Fatalf("two runs differ:\n%s\n---\n%s", a, b)
	}
}
