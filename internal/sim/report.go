package sim

import (
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
}

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
		FramesLost: v.Air.Stats.Lost, MaxChannelUse: v.Air.Stats.MaxUtil, Pass: true}
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
			res = checkDelivered(v, group, int64(c.Within))
		}
		r.Checks = append(r.Checks, res)
		r.Pass = r.Pass && res.Pass
	}
	return r
}

func checkConverged(v *Village, group map[string]int) CheckResult {
	hashes := map[int][32]byte{}
	for _, n := range v.Nodes {
		g, ok := group[n.Name]
		if !ok {
			continue // no power at the end
		}
		vec, _ := n.Store.Vector()
		h := vec.Hash()
		if first, seen := hashes[g]; !seen {
			hashes[g] = h
		} else if first != h {
			return CheckResult{Kind: "converged", Detail: fmt.Sprintf("%s differs from the rest of its WiFi group", n.Name)}
		}
	}
	return CheckResult{Kind: "converged", Pass: true, Detail: fmt.Sprintf("%d WiFi groups each agree", len(hashes))}
}

func checkDelivered(v *Village, group map[string]int, within int64) CheckResult {
	missing, late := 0, 0
	example := ""
	for _, p := range v.Posts {
		g, ok := group[p.House]
		if !ok {
			continue // sender off at the end: its group is unknown
		}
		for _, n := range v.Nodes {
			if n.Kind != "house" || n.Name == p.House {
				continue
			}
			if gg, ok := group[n.Name]; !ok || gg != g {
				continue
			}
			at, got := p.Arrived[n.Name]
			switch {
			case !got:
				missing++
				if example == "" {
					example = fmt.Sprintf("%q from %s never reached %s", p.Text, p.House, n.Name)
				}
			case within > 0:
				// count from when sender and receiver were connected, so a
				// split that heals is judged on how fast the house catches up
				from, ok := v.ConnectedFrom(p.House, n.Name, p.Created)
				if ok && at-from > within {
					late++
					if example == "" {
						example = fmt.Sprintf("%q from %s reached %s %.1f s after they were connected", p.Text, p.House, n.Name, float64(at-from)/float64(Second))
					}
				}
			}
		}
	}
	res := CheckResult{Kind: "delivered", Pass: missing == 0 && late == 0}
	res.Detail = fmt.Sprintf("%d missing, %d late (limit %s, counted from when sender and house were connected)", missing, late, fmtDur(within))
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
	if !r.Pass {
		verdict = "FAIL"
	}
	fmt.Fprintf(&b, "%s  %s (seed %d): %d houses, %d relays, %d posts, %d deliveries\n", verdict, r.Scenario, r.Seed, r.Houses, r.Relays, r.Posts, r.Deliveries)
	fmt.Fprintf(&b, "      delivery time: median %.0f ms, slowest 5%% %.0f ms, worst %.0f ms\n", r.LatencyP50Ms, r.LatencyP95Ms, r.LatencyMaxMs)
	fmt.Fprintf(&b, "      WiFi frames: %d, lost to receivers: %d, busiest channel %.0f%%\n", r.Frames, r.FramesLost, r.MaxChannelUse*100)
	for _, c := range r.Checks {
		mark := "ok  "
		if !c.Pass {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "      %s %s: %s\n", mark, c.Kind, c.Detail)
	}
	return b.String()
}
