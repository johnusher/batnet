package sim

import (
	"strings"
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
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

// Review: a gap in the middle of the log must not hide a difference between houses.
func TestConvergedSeesPastGaps(t *testing.T) {
	sc, _ := LoadScenario([]byte(`{"name":"gap","seed":1,"duration":"1m",
	  "houses":[{"name":"A","x":0,"y":0,"pi":"Pi 4","wifi":"dongle"},{"name":"B","x":100,"y":0,"pi":"Pi 4","wifi":"dongle"}],
	  "checks":[{"kind":"converged"}]}`))
	v, _ := sc.Build()
	x := v.Node("A")
	e1, _ := x.Author.Append(ledger.TypeMsgPost, 5, "one")
	x.Author.Append(ledger.TypeMsgPost, 5, "two") // stays only on A
	e3, _ := x.Author.Append(ledger.TypeMsgPost, 5, "three")
	b := v.Node("B")
	b.Store.Put(e1)
	b.Store.Put(e3)                       // B: 1 and 3; A: 1, 2, 3
	x.Radio.On, b.Radio.On = false, false // no repair: judge as is
	v.Air.nodes[0].On, v.Air.nodes[1].On = true, true
	if r := Judge(sc, v); r.Pass {
		t.Fatal("converged passed although B lacks entry 2")
	}
}

// Review: a post from a house that ends the run without power is still checked,
// and a post made just before the end is pending, not missing.
func TestDeliveredJudgesDarkSendersAndPending(t *testing.T) {
	sc, err := LoadScenario([]byte(`{"name":"dark","seed":1,"duration":"3m",
	  "houses":[{"name":"A","x":0,"y":0,"pi":"Pi 4","wifi":"dongle"},{"name":"B","x":100,"y":0,"pi":"Pi 4","wifi":"dongle"}],
	  "events":[{"at":"10s","kind":"post","house":"A","text":"before the dark"},
	            {"at":"20s","kind":"power","house":"A","on":false},
	            {"at":"2m59s","kind":"post","house":"B","text":"just before the end"}],
	  "checks":[{"kind":"delivered","within":"30s"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := sc.Build()
	v.RunFor(int64(sc.Duration))
	if r := Judge(sc, v); !r.Pass {
		t.Fatalf("should pass:\n%s", r)
	}
	delete(v.Posts[0].Arrived, "B") // pretend B never got A's post
	if r := Judge(sc, v); r.Pass {
		t.Fatal("a lost post from a sender that went dark passed")
	}
}

// A scenario can be marked as a known failure: OK while it fails, not OK once
// it passes (so the fix is noticed and the mark removed).
func TestExpectFail(t *testing.T) {
	base := `{"name":"x","seed":1,"duration":"1m","houses":[{"name":"A","x":0,"y":0,"pi":"Pi 4","wifi":"dongle"}],"checks":[{"kind":"converged"}]`
	sc, err := LoadScenario([]byte(base + `,"expect":"fail","because":"testing"}`))
	if err != nil {
		t.Fatal(err)
	}
	r, _, _ := Run(sc)
	if !r.Pass || r.OK() {
		t.Fatalf("a passing scenario marked expect-fail must not be OK: pass=%v ok=%v", r.Pass, r.OK())
	}
	sc2, _ := LoadScenario([]byte(base + `}`))
	if r2, _, _ := Run(sc2); !r2.OK() {
		t.Fatal("a passing scenario without a mark must be OK")
	}
	if _, err := LoadScenario([]byte(base + `,"expect":"maybe"}`)); err == nil {
		t.Fatal("expect must be pass or fail")
	}
}
