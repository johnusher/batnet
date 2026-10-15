package sim

import (
	"strings"
	"testing"
)

const street = `{
  "name": "street", "seed": 3, "duration": "10m", "start": "20:00",
  "houses": [
    {"name": "Miller", "x": 0,   "y": 0, "pi": "Pi 4", "wifi": "dongle", "people": 3},
    {"name": "Fern",   "x": 250, "y": 0, "pi": "Pi 4", "wifi": "dongle", "people": 2},
    {"name": "Okafor", "x": 500, "y": 0, "pi": "Pi 5", "wifi": "dongle", "people": 4}
  ],
  "traffic": {"perPersonPerHour": 6},
  "events": [
    {"at": "1m", "kind": "post", "house": "Fern", "text": "Bread at the bakery", "prio": 8},
    {"at": "2m", "kind": "power", "house": "Okafor", "on": false, "for": "3m"},
    {"at": "6m", "kind": "burst", "near": {"x": 250, "y": 0, "r": 400}, "within": "30s", "text": "Did you hear that bang?"}
  ],
  "checks": [{"kind": "converged"}, {"kind": "delivered", "within": "2m"}]
}`

func TestLoadAndBuild(t *testing.T) {
	sc, err := LoadScenario([]byte(street))
	if err != nil {
		t.Fatal(err)
	}
	if sc.Duration != Duration(10*Minute) || len(sc.Events) != 3 || sc.Events[1].For != Duration(3*Minute) {
		t.Fatalf("parsed %+v", sc)
	}
	v, err := sc.Build()
	if err != nil {
		t.Fatal(err)
	}
	v.RunFor(int64(sc.Duration))
	burst := 0
	for _, p := range v.Posts {
		if p.Text == "Did you hear that bang?" {
			burst++
		}
	}
	if burst != 9 { // everyone in the three houses
		t.Fatalf("burst posts = %d, want 9", burst)
	}
	if v.Node("Okafor").PoweredAll {
		t.Fatal("power event did not run")
	}
}

func TestGenerator(t *testing.T) {
	sc, err := LoadScenario([]byte(`{"name":"gen","seed":5,"duration":"1m",
	  "generate":{"houses":30,"hamlets":3,"width":1400,"height":900,"spread":150,"dongleShare":0.5,"maxPeople":6}}`))
	if err != nil {
		t.Fatal(err)
	}
	v, err := sc.Build()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(v.Nodes); n != 30 {
		t.Fatalf("generated %d houses, want 30", n)
	}
	again, _ := sc.Build()
	if v.Nodes[17].House != again.Nodes[17].House {
		t.Fatal("the generator is not deterministic")
	}
}

// Review focus 5.
func TestScenarioErrors(t *testing.T) {
	for name, js := range map[string]string{
		"bad json":           `{`,
		"bad duration":       `{"name":"x","duration":"soon","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}]}`,
		"no houses":          `{"name":"x","duration":"1m"}`,
		"unknown house":      `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"events":[{"at":"1s","kind":"post","house":"B","text":"x"}]}`,
		"unknown kind":       `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"events":[{"at":"1s","kind":"party"}]}`,
		"unknown check":      `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"checks":[{"kind":"vibes"}]}`,
		"negative generator": `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"generate":{"houses":-1}}`,
		"huge generator":     `{"name":"x","duration":"1m","generate":{"houses":100000000,"width":10,"height":10}}`,
		"huge traffic":       `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"traffic":{"perPersonPerHour":1e13}}`,
		"post from a relay":  `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"relays":[{"name":"R","pi":"Zero 2 W","wifi":"pole"}],"events":[{"at":"1s","kind":"post","house":"R","text":"x"}]}`,
		"negative time":      `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"events":[{"at":"-5s","kind":"post","house":"A","text":"x"}]}`,
		"bad start":          `{"name":"x","duration":"1m","start":"teatime","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}]}`,
		"unknown field":      `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"evnts":[]}`,
		// B17: Meshtastic users are checked like houses
		"nameless meshtastic":   `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"meshtastic":[{"x":1,"y":1}]}`,
		"meshtastic named as a house": `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"meshtastic":[{"name":"A","x":1,"y":1}]}`,
		"meshtastic twice":          `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"meshtastic":[{"name":"M","x":1,"y":1},{"name":"M","x":2,"y":2}]}`,
	} {
		sc, err := LoadScenario([]byte(js))
		if err == nil {
			_, err = sc.Build()
		}
		if err == nil {
			t.Errorf("%s: no error", name)
		} else if strings.Contains(err.Error(), "panic") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
