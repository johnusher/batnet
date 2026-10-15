package sim

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/johnusher/batnet/internal/lora"
)

// CheckResult is one check's verdict.
type CheckResult struct {
	Kind   string
	Pass   bool
	Detail string
}

// Report summarises a run.
type Report struct {
	Scenario                                    string
	Seed                                        int64
	Houses, Relays, Posts, Deliveries           int
	LatencyP50Ms, LatencyP95Ms, LatencyMaxMs    float64
	Frames, FramesLost                          int
	LoRaPackets, LoRaCollided, LoRaRebroadcasts int
	LoRaMaxDutyPct                              float64
	MaxChannelUse                               float64
	Skipped                                     []string // scripted posts that could not be made, with why
	Checks                                      []CheckResult
	Pass                                        bool
	Expect, Because                             string
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
		FramesLost: v.Air.Stats.Lost, MaxChannelUse: v.Air.Stats.MaxUtil, Pass: true, Expect: sc.Expect, Because: sc.Because, Skipped: v.Skipped,
		LoRaPackets: v.LoRa.Stats.Packets, LoRaCollided: v.LoRa.Stats.Collided, LoRaRebroadcasts: v.LoRa.Stats.Rebroadcasts,
		LoRaMaxDutyPct: float64(v.MaxDutyUs) * 100 / float64(lora.DutyLimitUs)}
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
		case "duty":
			res = CheckResult{Kind: "duty", Pass: v.MaxDutyUs <= lora.DutyLimitUs,
				Detail: fmt.Sprintf("busiest radio used %.0f%% of its 360 s in an hour", float64(v.MaxDutyUs)*100/float64(lora.DutyLimitUs))}
		}
		r.Checks = append(r.Checks, res)
		r.Pass = r.Pass && res.Pass
	}
	return r
}

// inFlightMs: entries younger than this at the end are still spreading, and
// converged doesn't judge them.
const inFlightMs = 2 * 60 * 1000

// keysHash summarises every entry a node holds (not just its gap-free prefix)
// that was written before cutoff (wall ms).
func keysHash(n *Node, cutoff int64) [32]byte {
	all, _ := n.Store.All()
	keys := make([]string, 0, len(all))
	for _, e := range all {
		if e.TS.Wall > cutoff {
			continue
		}
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
		h := keysHash(n, v.EpochMs+v.Sch.Now()/Ms-inFlightMs)
		if first, seen := hashes[g]; !seen {
			hashes[g] = h
		} else if first != h {
			return CheckResult{Kind: "converged", Detail: fmt.Sprintf("%s holds different entries from the rest of its WiFi group", n.Name)}
		}
	}
	return CheckResult{Kind: "converged", Pass: true, Detail: fmt.Sprintf("%d WiFi groups each agree", len(hashes))}
}

// connectedUs is how long, before until, house h was connected (in the same
// group of the snapshots: WiFi links and LoRa links between powered radios)
// to any node holding p, counted from the moment that node got it. Time while
// no holder was reachable (the holder's power was cut, the group was split)
// doesn't count: spec §13 says messages wait and arrive on return.
func connectedUs(v *Village, p *Post, h string, until int64) int64 {
	var total int64
	for i, s := range v.groups {
		lo, hi := s.T, until
		if i+1 < len(v.groups) {
			hi = min(hi, v.groups[i+1].T)
		}
		if hi <= lo {
			continue
		}
		g, here := s.Group[h]
		if !here {
			continue
		}
		earliest, found := int64(0), false
		for holder, at := range p.Arrived {
			if holder == h || s.Group[holder] != g {
				continue
			}
			if _, powered := s.Group[holder]; powered && (!found || at < earliest) {
				earliest, found = at, true
			}
		}
		if found && max(lo, earliest) < hi {
			total += hi - max(lo, earliest)
		}
	}
	return total
}

// firstConnectedUs is the first time h was connected to a holder of p (after
// that holder got it), and whether h still is at time until.
func firstConnectedUs(v *Village, p *Post, h string, until int64) (first int64, stillAtEnd bool) {
	found := false
	for i, s := range v.groups {
		lo, hi := s.T, until
		if i+1 < len(v.groups) {
			hi = min(hi, v.groups[i+1].T)
		}
		if hi < lo {
			continue
		}
		g, here := s.Group[h]
		if !here {
			stillAtEnd = false
			continue
		}
		connected := false
		for holder, at := range p.Arrived {
			if holder == h || s.Group[holder] != g {
				continue
			}
			if _, powered := s.Group[holder]; powered && at < hi {
				connected = true
				if !found || max(lo, at) < first {
					first, found = max(lo, at), true
				}
			}
		}
		stillAtEnd = connected
	}
	return first, stillAtEnd
}

// checkDelivered: every house that was connected (directly or through a node
// holding the post) to a holder must get the post within the limit of
// connected time. A post that could not yet have arrived by the end is
// pending, not missing. A house whose holders are all dark at the end is
// judged on the clock since it was first connected: a sender that goes dark
// for good can't hide a lost post behind "still on its way".
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
			at, got := p.Arrived[n.Name]
			if !got {
				at = end
			}
			conn := connectedUs(v, p, n.Name, at)
			if !got && conn > 0 {
				if first, still := firstConnectedUs(v, p, n.Name, end); !still {
					conn = end - first // every holder dark at the end: the clock since the first connection
				}
			}
			switch {
			case conn == 0:
				continue // never connected to anyone holding it, or got it in the same instant
			case !got && conn <= grace:
				pending++
			case !got:
				missing++
				if example == "" {
					example = fmt.Sprintf("%q from %s never reached %s", p.Text, p.House, n.Name)
				}
			case within > 0 && conn > within:
				late++
				if example == "" {
					example = fmt.Sprintf("%q from %s reached %s after %.1f s connected to a holder", p.Text, p.House, n.Name, float64(conn)/float64(Second))
				}
			}
		}
	}
	res := CheckResult{Kind: "delivered", Pass: missing == 0 && late == 0}
	res.Detail = fmt.Sprintf("%d missing, %d late, %d still on their way at the end (limit %s of time connected to a holder)", missing, late, pending, fmtDur(within))
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
	if r.LoRaPackets > 0 {
		fmt.Fprintf(&b, "      LoRa packets: %d, collided at receivers: %d, rebroadcasts: %d, busiest radio %.0f%% of its hourly limit\n", r.LoRaPackets, r.LoRaCollided, r.LoRaRebroadcasts, r.LoRaMaxDutyPct)
	}
	if n := len(r.Skipped); n > 0 {
		fmt.Fprintf(&b, "      scripted posts not made: %d (e.g. %s)\n", n, r.Skipped[0])
	}
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
