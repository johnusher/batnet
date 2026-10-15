package sim

import (
	"encoding/json"
	"errors"
	"time"
)

// API is what the simulator page calls through WebAssembly.
type API struct {
	sc     *Scenario
	v      *Village
	frames []frameJSON
}

type frameJSON struct {
	From string   `json:"from"`
	To   []string `json:"to"`
	At   int64    `json:"at"`
}

// Load builds a scenario's village at time 0.
func (a *API) Load(scenarioJSON []byte) error {
	sc, err := LoadScenario(scenarioJSON)
	if err != nil {
		return err
	}
	v, err := sc.Build()
	if err != nil {
		return err
	}
	a.sc, a.v, a.frames = sc, v, nil
	v.Air.OnBroadcast = func(from string, to []string, at int64) {
		if len(a.frames) < 300 {
			a.frames = append(a.frames, frameJSON{From: from, To: to, At: at})
		}
	}
	return nil
}

var errNotLoaded = errors.New("sim: no scenario loaded")

// Step advances virtual time by ms milliseconds.
func (a *API) Step(ms int64) {
	if a.v != nil {
		a.v.RunFor(ms * Ms)
	}
}

// State is everything the page draws, as JSON. Frames are those since the last call.
func (a *API) State() []byte {
	if a.v == nil {
		return []byte(`{"error":"no scenario loaded"}`)
	}
	v := a.v
	type nodeJSON struct {
		Name    string  `json:"name"`
		Kind    string  `json:"kind"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		On      bool    `json:"on"`
		Range   float64 `json:"range"`
		Pi      string  `json:"pi"`
		WiFi    string  `json:"wifi"`
		People  int     `json:"people"`
		Entries int     `json:"entries"`
		Group   int     `json:"group"`
	}
	group := map[string]int{}
	for i, g := range v.Air.Groups() {
		for _, n := range g {
			group[n.Name] = i
		}
	}
	st := struct {
		Now    int64       `json:"now"`
		Clock  string      `json:"clock"`
		End    int64       `json:"end"`
		Nodes  []nodeJSON  `json:"nodes"`
		Links  [][2]string `json:"links"`
		Frames []frameJSON `json:"frames"`
		Posts  []any       `json:"posts"`
		Stats  any         `json:"stats"`
	}{Now: v.Sch.Now(), End: int64(a.sc.Duration), Frames: a.frames}
	st.Clock = time.UnixMilli(v.EpochMs + v.Sch.Now()/Ms).UTC().Format("15:04:05")
	for _, n := range v.Nodes {
		all, _ := n.Store.All()
		g, ok := group[n.Name]
		if !ok {
			g = -1
		}
		nj := nodeJSON{Name: n.Name, Kind: n.Kind, X: n.Radio.X, Y: n.Radio.Y, On: n.Radio.On, Range: n.Radio.Range, Entries: len(all), Group: g}
		if n.Kind == "house" {
			nj.Pi, nj.WiFi, nj.People = n.House.Pi, n.House.WiFi, n.House.People
		}
		st.Nodes = append(st.Nodes, nj)
	}
	for i, p := range v.Air.nodes {
		for _, q := range v.Air.nodes[i+1:] {
			if v.Air.Linked(p, q) {
				st.Links = append(st.Links, [2]string{p.Name, q.Name})
			}
		}
	}
	from := max(0, len(v.Posts)-30)
	for _, p := range v.Posts[from:] {
		st.Posts = append(st.Posts, map[string]any{"house": p.House, "text": p.Text, "prio": p.Prio, "created": p.Created, "arrived": len(p.Arrived) - 1})
	}
	r := Judge(a.sc, v)
	st.Stats = map[string]any{"posts": r.Posts, "deliveries": r.Deliveries, "p50": r.LatencyP50Ms, "p95": r.LatencyP95Ms,
		"frames": r.Frames, "lost": r.FramesLost, "channel": r.MaxChannelUse}
	a.frames = nil
	b, _ := json.Marshal(st)
	return b
}

// Post makes a house post now.
func (a *API) Post(house, text string, prio int) error {
	if a.v == nil {
		return errNotLoaded
	}
	if prio < 0 || prio > 9 {
		return errors.New("sim: priority must be 0-9")
	}
	_, err := a.v.Post(house, text, uint8(prio))
	return err
}

// Power cuts or restores a node's power.
func (a *API) Power(name string, on bool) error {
	if a.v == nil {
		return errNotLoaded
	}
	return a.v.SetPower(name, on)
}

// Report judges the village as it stands now.
func (a *API) Report() []byte {
	if a.v == nil {
		return []byte(`{"error":"no scenario loaded"}`)
	}
	b, _ := json.Marshal(Judge(a.sc, a.v))
	return b
}
