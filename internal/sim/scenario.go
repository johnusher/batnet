package sim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/johnusher/batnet/internal/lora"
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
	Name       string           `json:"name"`
	About      string           `json:"about"`
	Expect     string           `json:"expect"`  // "pass" (default) or "fail": a known problem, see Because
	Because    string           `json:"because"` // why a scenario is expected to fail
	Seed       int64            `json:"seed"`
	Duration   Duration         `json:"duration"`
	Start      string           `json:"start"`
	Preset     string           `json:"preset"` // LoRa modem preset, default LongFast
	Meshtastic []MeshtasticUser `json:"meshtastic"`
	Houses     []HouseSpec      `json:"houses"`
	Relays     []RelaySpec      `json:"relays"`
	Generate   *Generate        `json:"generate"`
	Traffic    Traffic          `json:"traffic"`
	Events     []Event          `json:"events"`
	Checks     []Check          `json:"checks"`
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

// MeshtasticUser is someone's own Meshtastic radio: it relays LoRa packets.
type MeshtasticUser struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
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

var houseNames = []string{"Usher", "Shemie", "Dittrich", "Green", "Willett", "Seigner", "Chasserot", "Miller", "Fern", "Okafor", "Novak", "Haddad", "Brandt", "Kowalski", "Moreau", "Lindqvist", "Santos", "Petrov", "Weber", "Osei", "Tanaka", "Rossi", "Byrne", "Hoffmann", "Dubois", "Varga", "Nakamura"}

var chatter = []string{"Water point open until 18:00", "Anyone have spare batteries?", "Road to the bridge is blocked", "Community kitchen at the school from 12:00", "Generator at the fire station charges phones 9-11", "Please boil tap water"}

// LoadScenario parses and checks a scenario file.
func LoadScenario(b []byte) (*Scenario, error) {
	var sc Scenario
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sc); err != nil {
		return nil, fmt.Errorf("scenario: %v", err)
	}
	if err := sc.validate(); err != nil {
		return nil, fmt.Errorf("scenario %q: %v", sc.Name, err)
	}
	if sc.Duration <= 0 {
		return nil, fmt.Errorf("scenario %q: duration missing", sc.Name)
	}
	if len(sc.Houses) == 0 && (sc.Generate == nil || sc.Generate.Houses <= 0) {
		return nil, fmt.Errorf("scenario %q: no houses and no generator", sc.Name)
	}
	for _, c := range sc.Checks {
		if c.Kind != "converged" && c.Kind != "delivered" && c.Kind != "duty" {
			return nil, fmt.Errorf("scenario %q: unknown check %q", sc.Name, c.Kind)
		}
	}
	return &sc, nil
}

// Limits that keep a scenario from exhausting memory or time.
const (
	maxHouses      = 2000
	maxHamlets     = 200
	maxPeople      = 200
	maxRatePerHour = 600
)

func (sc *Scenario) validate() error {
	if _, ok := lora.Presets[sc.preset()]; !ok {
		return fmt.Errorf("unknown LoRa preset %q", sc.Preset)
	}
	if sc.Expect != "" && sc.Expect != "pass" && sc.Expect != "fail" {
		return fmt.Errorf("expect must be \"pass\" or \"fail\"")
	}
	if sc.Start != "" {
		if _, err := time.Parse("15:04", sc.Start); err != nil {
			return fmt.Errorf("start %q is not a time like \"20:00\"", sc.Start)
		}
	}
	if g := sc.Generate; g != nil {
		if g.Houses < 1 || g.Houses > maxHouses {
			return fmt.Errorf("generate.houses must be 1-%d", maxHouses)
		}
		if g.Hamlets < 0 || g.Hamlets > maxHamlets || g.MaxPeople < 0 || g.MaxPeople > maxPeople {
			return fmt.Errorf("generate.hamlets must be 0-%d and maxPeople 0-%d", maxHamlets, maxPeople)
		}
		if g.Width <= 0 || g.Height <= 0 || g.Spread < 0 || g.DongleShare < 0 || g.DongleShare > 1 {
			return fmt.Errorf("generate needs width and height above 0, spread 0 or more, dongleShare 0-1")
		}
	}
	if len(sc.Houses) > maxHouses {
		return fmt.Errorf("at most %d houses", maxHouses)
	}
	for _, h := range sc.Houses {
		if h.People < 0 || h.People > maxPeople {
			return fmt.Errorf("house %q: people must be 0-%d", h.Name, maxPeople)
		}
	}
	if r := sc.Traffic.PerPersonPerHour; r < 0 || r > maxRatePerHour {
		return fmt.Errorf("traffic.perPersonPerHour must be 0-%d", maxRatePerHour)
	}
	for i, e := range sc.Events {
		if e.At < 0 || e.For < 0 || e.Within < 0 {
			return fmt.Errorf("event %d: times must not be negative", i+1)
		}
	}
	return nil
}

func (sc *Scenario) preset() string {
	if sc.Preset == "" {
		return "LongFast"
	}
	return sc.Preset
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
	v, err := NewVillage(sc.Seed, sc.epochMs(), lora.Presets[sc.preset()])
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
	for _, m := range sc.Meshtastic {
		v.AddMeshtastic(m.Name, m.X, m.Y)
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
		if n := v.Node(e.House); n == nil || n.Kind != "house" {
			return fmt.Errorf("no house %q (relays don't post)", e.House)
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
				v.Sch.After(1+int64(v.rnd.ExpFloat64()*mean), next) // at least 1 µs apart
			}
			v.Sch.After(1+int64(v.rnd.ExpFloat64()*mean), next)
		}
	}
}
