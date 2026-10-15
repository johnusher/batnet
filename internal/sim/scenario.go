package sim

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Duration is virtual time in µs, written in JSON like "90s" or "5m".
type Duration int64

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"5m\": %s", b)
	}
	x, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("duration %q: %v", s, err)
	}
	*d = Duration(x.Microseconds())
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal((time.Duration(d) * time.Microsecond).String())
}

// Scenario is a village, what its people do, and what must hold.
type Scenario struct {
	Name     string      `json:"name"`
	Seed     int64       `json:"seed"`
	Duration Duration    `json:"duration"`
	Start    string      `json:"start"`
	Houses   []HouseSpec `json:"houses"`
	Relays   []RelaySpec `json:"relays"`
	Generate *Generate   `json:"generate"`
	Traffic  Traffic     `json:"traffic"`
	Events   []Event     `json:"events"`
	Checks   []Check     `json:"checks"`
}

// Generate places houses in hamlets, like the JavaScript model.
type Generate struct {
	Houses      int     `json:"houses"`
	Hamlets     int     `json:"hamlets"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Spread      float64 `json:"spread"`
	DongleShare float64 `json:"dongleShare"`
	MaxPeople   int     `json:"maxPeople"`
}

// Traffic is background chatter: everyone posts at this rate, at random.
type Traffic struct {
	PerPersonPerHour float64 `json:"perPersonPerHour"`
}

// Event is something scripted: a post, a power change, or a burst.
type Event struct {
	At     Duration `json:"at"`
	Kind   string   `json:"kind"` // post, power, burst
	House  string   `json:"house"`
	Text   string   `json:"text"`
	Prio   uint8    `json:"prio"`
	On     *bool    `json:"on"`
	For    Duration `json:"for"`
	Near   *Circle  `json:"near"`
	Within Duration `json:"within"`
}

// Circle is an area in metres.
type Circle struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	R float64 `json:"r"`
}

// Check is something that must hold at the end of the run.
type Check struct {
	Kind   string   `json:"kind"` // converged, delivered
	Within Duration `json:"within"`
}

var houseNames = []string{"Miller", "Fern", "Okafor", "Novak", "Haddad", "Brandt", "Kowalski", "Moreau", "Lindqvist", "Santos", "Petrov", "Weber", "Osei", "Tanaka", "Rossi", "Byrne", "Hoffmann", "Dubois", "Varga", "Nakamura"}

var chatter = []string{"Water point open until 18:00", "Anyone have spare batteries?", "Road to the bridge is blocked", "Community kitchen at the school from 12:00", "Generator at the fire station charges phones 9-11", "Please boil tap water"}

// LoadScenario parses and checks a scenario file.
func LoadScenario(b []byte) (*Scenario, error) {
	var sc Scenario
	if err := json.Unmarshal(b, &sc); err != nil {
		return nil, fmt.Errorf("scenario: %v", err)
	}
	if sc.Duration <= 0 {
		return nil, fmt.Errorf("scenario %q: duration missing", sc.Name)
	}
	if len(sc.Houses) == 0 && (sc.Generate == nil || sc.Generate.Houses <= 0) {
		return nil, fmt.Errorf("scenario %q: no houses and no generator", sc.Name)
	}
	for _, c := range sc.Checks {
		if c.Kind != "converged" && c.Kind != "delivered" {
			return nil, fmt.Errorf("scenario %q: unknown check %q", sc.Name, c.Kind)
		}
	}
	return &sc, nil
}

// epochMs turns Start ("20:00") into a wall clock on a fixed day.
func (sc *Scenario) epochMs() int64 {
	base := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	if h, m, ok := strings.Cut(sc.Start, ":"); ok {
		hh, _ := strconv.Atoi(h)
		mm, _ := strconv.Atoi(m)
		base = base.Add(time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute)
	}
	return base.UnixMilli()
}

func (sc *Scenario) houses() []HouseSpec {
	if sc.Generate == nil {
		return sc.Houses
	}
	g := sc.Generate
	rnd := rand.New(rand.NewSource(sc.Seed*7919 + 1))
	type pt struct{ x, y float64 }
	centres := make([]pt, max(1, g.Hamlets))
	for i := range centres {
		centres[i] = pt{g.Width * (0.15 + 0.7*rnd.Float64()), g.Height * (0.15 + 0.7*rnd.Float64())}
	}
	pis := []string{"Zero 2 W", "Pi 3", "Pi 4", "Pi 4", "Pi 5"}
	out := make([]HouseSpec, 0, g.Houses)
	for i := 0; i < g.Houses; i++ {
		c := centres[i%len(centres)]
		ang, dist := rnd.Float64()*2*math.Pi, g.Spread*math.Sqrt(rnd.Float64())
		name := houseNames[i%len(houseNames)]
		if i >= len(houseNames) {
			name = fmt.Sprintf("%s %d", name, i/len(houseNames)+1)
		}
		wifi := "builtin"
		if rnd.Float64() < g.DongleShare {
			wifi = "dongle"
		}
		out = append(out, HouseSpec{Name: name, X: math.Round(c.x + dist*math.Cos(ang)), Y: math.Round(c.y + dist*math.Sin(ang)),
			Pi: pis[rnd.Intn(len(pis))], WiFi: wifi, People: 1 + rnd.Intn(max(1, g.MaxPeople))})
	}
	return out
}

// Build makes the village and schedules traffic and events; nothing has run yet.
func (sc *Scenario) Build() (*Village, error) {
	v, err := NewVillage(sc.Seed, sc.epochMs())
	if err != nil {
		return nil, err
	}
	for _, h := range sc.houses() {
		if h.People == 0 {
			h.People = 1
		}
		if _, err := v.AddHouse(h); err != nil {
			return nil, err
		}
	}
	for _, r := range sc.Relays {
		if _, err := v.AddRelay(r); err != nil {
			return nil, err
		}
	}
	for i, e := range sc.Events {
		if err := sc.schedule(v, e); err != nil {
			return nil, fmt.Errorf("scenario %q, event %d: %v", sc.Name, i+1, err)
		}
	}
	sc.scheduleTraffic(v)
	v.Start()
	return v, nil
}

func (sc *Scenario) schedule(v *Village, e Event) error {
	at := int64(e.At)
	prio := e.Prio
	if prio == 0 {
		prio = 5
	}
	switch e.Kind {
	case "post":
		if v.Node(e.House) == nil {
			return fmt.Errorf("no house %q", e.House)
		}
		v.Sch.At(at, func() { v.Post(e.House, e.Text, prio) })
	case "power":
		if v.Node(e.House) == nil || e.On == nil {
			return fmt.Errorf("power needs a known house and \"on\"")
		}
		on := *e.On
		v.Sch.At(at, func() { v.SetPower(e.House, on) })
		if e.For > 0 {
			v.Sch.At(at+int64(e.For), func() { v.SetPower(e.House, !on) })
		}
	case "burst":
		if e.Near == nil || e.Within <= 0 {
			return fmt.Errorf("burst needs \"near\" and \"within\"")
		}
		for _, n := range v.Nodes {
			if n.Kind != "house" || math.Hypot(n.House.X-e.Near.X, n.House.Y-e.Near.Y) > e.Near.R {
				continue
			}
			for p := 0; p < n.House.People; p++ {
				name := n.Name
				v.Sch.At(at+v.rnd.Int63n(int64(e.Within)), func() { v.Post(name, e.Text, prio) })
			}
		}
	default:
		return fmt.Errorf("unknown event kind %q", e.Kind)
	}
	return nil
}

// scheduleTraffic gives every person a Poisson stream of posts.
func (sc *Scenario) scheduleTraffic(v *Village) {
	rate := sc.Traffic.PerPersonPerHour
	if rate <= 0 {
		return
	}
	mean := float64(time.Hour.Microseconds()) / rate
	for _, n := range v.Nodes {
		if n.Kind != "house" {
			continue
		}
		for p := 0; p < n.House.People; p++ {
			name := n.Name
			var next func()
			next = func() {
				if v.Node(name).Radio.On {
					v.Post(name, chatter[v.rnd.Intn(len(chatter))], 5)
				}
				v.Sch.After(int64(v.rnd.ExpFloat64()*mean), next)
			}
			v.Sch.After(int64(v.rnd.ExpFloat64()*mean), next)
		}
	}
}
