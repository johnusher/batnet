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

// A post sent while the street was split counts from the moment sender and
// receiver are connected again, not from when it was sent.
func TestDeliveredCountsFromReconnection(t *testing.T) {
	sc, err := LoadScenario([]byte(`{"name":"split","seed":1,"duration":"5m",
	  "houses":[{"name":"A","x":0,"y":0,"pi":"Pi 4","wifi":"dongle"},
	            {"name":"B","x":300,"y":0,"pi":"Pi 4","wifi":"dongle"},
	            {"name":"C","x":600,"y":0,"pi":"Pi 4","wifi":"dongle"}],
	  "events":[{"at":"10s","kind":"power","house":"B","on":false,"for":"2m"},
	            {"at":"20s","kind":"post","house":"A","text":"during the split"}],
	  "checks":[{"kind":"delivered","within":"30s"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	r, v, err := Run(sc)
	if err != nil {
		t.Fatal(err)
	}
	at := v.Posts[0].Arrived["C"]
	if at < 130*Second {
		t.Fatalf("C got the post at %d µs, before B (the only path) was back", at)
	}
	if !r.Pass {
		t.Fatalf("a post that arrived soon after the split healed was judged late:\n%s", r)
	}
}
