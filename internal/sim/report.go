package sim

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// CheckResult is one check's verdict.
type CheckResult struct {
	Kind   string
	Pass   bool
	Detail string
}

// Report summarises a run.
type Report struct {
	Scenario                                 string
	Seed                                     int64
	Houses, Relays, Posts, Deliveries        int
	LatencyP50Ms, LatencyP95Ms, LatencyMaxMs float64
	Frames, FramesLost                       int
	MaxChannelUse                            float64
	Checks                                   []CheckResult
	Pass                                     bool
	Expect, Because                          string
}

// OK is whether the run matches its expectation: it passes, or it fails and
// was marked as a known failure. A known failure that passes is not OK, so the
// mark gets removed.
func (r *Report) OK() bool { return r.Pass == (r.Expect != "fail") }

// Run builds and runs a scenario to its end and judges it.
func Run(sc *Scenario) (*Report, *Village, error) {
	v, err := sc.Build()
	if err != nil {
		return nil, nil, err
	}
	v.RunFor(int64(sc.Duration))
	return Judge(sc, v), v, nil
}

// Judge measures a finished (or paused) village against the scenario's checks.
func Judge(sc *Scenario, v *Village) *Report {
	r := &Report{Scenario: sc.Name, Seed: sc.Seed, Posts: len(v.Posts), Frames: v.Air.Stats.Frames,
		FramesLost: v.Air.Stats.Lost, MaxChannelUse: v.Air.Stats.MaxUtil, Pass: true, Expect: sc.Expect, Because: sc.Because}
	for _, n := range v.Nodes {
		if n.Kind == "house" {
			r.Houses++
		} else {
			r.Relays++
		}
	}
	var lat []float64
	for _, p := range v.Posts {
		for h, at := range p.Arrived {
			if h != p.House {
				r.Deliveries++
				lat = append(lat, float64(at-p.Created)/float64(Ms))
			}
		}
	}
	sort.Float64s(lat)
	if len(lat) > 0 {
		r.LatencyP50Ms = lat[len(lat)/2]
		r.LatencyP95Ms = lat[min(len(lat)-1, len(lat)*95/100)]
		r.LatencyMaxMs = lat[len(lat)-1]
	}
	group := map[string]int{} // node name → final WiFi group
	for i, g := range v.Air.Groups() {
		for _, n := range g {
			group[n.Name] = i
		}
	}
	for _, c := range sc.Checks {
		var res CheckResult
		switch c.Kind {
		case "converged":
			res = checkConverged(v, group)
		case "delivered":
			res = checkDelivered(v, int64(c.Within))
		}
		r.Checks = append(r.Checks, res)
		r.Pass = r.Pass && res.Pass
	}
	return r
}

// keysHash summarises every entry a node holds (not just its gap-free prefix).
func keysHash(n *Node) [32]byte {
	all, _ := n.Store.All()
	keys := make([]string, 0, len(all))
	for _, e := range all {
		keys = append(keys, fmt.Sprintf("%x/%d", e.Key.Origin[:], e.Key.Seq))
	}
	sort.Strings(keys)
	return sha256.Sum256([]byte(strings.Join(keys, ",")))
}

func checkConverged(v *Village, group map[string]int) CheckResult {
	hashes := map[int][32]byte{}
	for _, n := range v.Nodes {
		g, ok := group[n.Name]
		if !ok {
			continue // no power at the end
		}
		h := keysHash(n)
		if first, seen := hashes[g]; !seen {
			hashes[g] = h
		} else if first != h {
			return CheckResult{Kind: "converged", Detail: fmt.Sprintf("%s holds different entries from the rest of its WiFi group", n.Name)}
		}
	}
	return CheckResult{Kind: "converged", Pass: true, Detail: fmt.Sprintf("%d WiFi groups each agree", len(hashes))}
}

// requiredFrom is the earliest time house h was connected to any node holding
// p since it got it: from then on, h should get p. False if never.
func requiredFrom(v *Village, p *Post, h string) (int64, bool) {
	best, found := int64(0), false
	for holder, at := range p.Arrived {
		if holder == h {
			continue
		}
		if t, ok := v.ConnectedFrom(holder, h, at); ok && (!found || t < best) {
			best, found = t, true
		}
	}
	return best, found
}

// checkDelivered: every house that was connected (directly or through a node
// holding the post) to a holder must get the post within the limit, counted
// from that connection. A post that could not yet have arrived by the end is
// pending, not missing. A sender that ends the run without power is still judged.
func checkDelivered(v *Village, within int64) CheckResult {
	end := v.Sch.Now()
	grace := within
	if grace <= 0 {
		grace = Minute
	}
	missing, late, pending := 0, 0, 0
	example := ""
	for _, p := range v.Posts {
		for _, n := range v.Nodes {
			if n.Kind != "house" || n.Name == p.House {
				continue
			}
			from, ok := requiredFrom(v, p, n.Name)
			at, got := p.Arrived[n.Name]
			switch {
			case !ok:
				continue // never connected to anyone holding it
			case got && at <= from:
				continue // it arrived before the connection we measure from
			case !got && from+grace > end:
				pending++
			case !got:
				missing++
				if example == "" {
					example = fmt.Sprintf("%q from %s never reached %s", p.Text, p.House, n.Name)
				}
			case within > 0 && at-from > within:
				late++
				if example == "" {
					example = fmt.Sprintf("%q from %s reached %s %.1f s after they were connected", p.Text, p.House, n.Name, float64(at-from)/float64(Second))
				}
			}
		}
	}
	res := CheckResult{Kind: "delivered", Pass: missing == 0 && late == 0}
	res.Detail = fmt.Sprintf("%d missing, %d late, %d still on their way at the end (limit %s, counted from when the house was connected to a holder)", missing, late, pending, fmtDur(within))
	if example != "" {
		res.Detail += "; e.g. " + example
	}
	return res
}

func fmtDur(us int64) string {
	if us <= 0 {
		return "none"
	}
	return fmt.Sprintf("%gs", float64(us)/float64(Second))
}

// String is the report as people read it.
func (r *Report) String() string {
	var b strings.Builder
	verdict := "PASS"
	switch {
	case !r.Pass && r.Expect == "fail":
		verdict = "KNOWN FAIL"
	case !r.Pass:
		verdict = "FAIL"
	case r.Expect == "fail":
		verdict = "PASS, but marked as a known failure: remove the mark"
	}
	fmt.Fprintf(&b, "%s  %s (seed %d): %d houses, %d relays, %d posts, %d deliveries\n", verdict, r.Scenario, r.Seed, r.Houses, r.Relays, r.Posts, r.Deliveries)
	fmt.Fprintf(&b, "      delivery time: median %.0f ms, slowest 5%% %.0f ms, worst %.0f ms\n", r.LatencyP50Ms, r.LatencyP95Ms, r.LatencyMaxMs)
	fmt.Fprintf(&b, "      WiFi frames: %d, lost to receivers: %d, busiest channel %.0f%%\n", r.Frames, r.FramesLost, r.MaxChannelUse*100)
	if r.Expect == "fail" {
		fmt.Fprintf(&b, "      known problem: %s\n", r.Because)
	}
	for _, c := range r.Checks {
		mark := "ok  "
		if !c.Pass {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "      %s %s: %s\n", mark, c.Kind, c.Detail)
	}
	return b.String()
}
