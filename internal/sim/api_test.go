package sim

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAPI(t *testing.T) {
	var a API
	if err := a.Load([]byte(street)); err != nil {
		t.Fatal(err)
	}
	a.Step(90_000) // 90 s: the 1-minute post has happened
	var st struct {
		Now   int64  `json:"now"`
		Clock string `json:"clock"`
		Nodes []struct {
			Name    string `json:"name"`
			Entries int    `json:"entries"`
		} `json:"nodes"`
		Links  [][2]string `json:"links"`
		Frames []any       `json:"frames"`
		Posts  []struct {
			House   string `json:"house"`
			Arrived int    `json:"arrived"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(a.State(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Now != 90*Second || st.Clock != "20:01:30" || len(st.Nodes) != 3 || len(st.Links) == 0 || len(st.Frames) == 0 || len(st.Posts) == 0 {
		t.Fatalf("state %+v", st)
	}
	if err := a.Post("Miller", "from the page", 8); err != nil {
		t.Fatal(err)
	}
	if err := a.Power("Fern", false); err != nil {
		t.Fatal(err)
	}
	if err := a.Post("Nobody", "x", 5); err == nil {
		t.Fatal("post from an unknown house accepted")
	}
	var r Report
	if err := json.Unmarshal(a.Report(), &r); err != nil || r.Scenario != "street" {
		t.Fatalf("report %+v, %v", r, err)
	}
	if err := a.Load([]byte(`{`)); err == nil {
		t.Fatal("bad scenario accepted")
	}
}

// Review focus 4.
func TestSimEvents(t *testing.T) {
	var a API
	if err := a.Load([]byte(street)); err != nil {
		t.Fatal(err)
	}
	var people []struct {
		House  string
		People []struct{ ID, Name string }
	}
	json.Unmarshal(a.People(), &people)
	fern := people[1]
	tok, err := a.Login(fern.House, fern.People[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if ev := a.Events(fern.House, tok); string(ev) != "[]" {
		t.Fatalf("first events call should subscribe and return nothing: %s", ev)
	}
	status, _, body := a.HTTP(fern.House, "POST", "/api/post", `{"conv":"everyone","text":"from the sim phone"}`, tok)
	if status != 200 || !strings.Contains(body, "from the sim phone") {
		t.Fatalf("post: %d %s", status, body)
	}
	ev := a.Events(fern.House, tok)
	if !strings.Contains(string(ev), "from the sim phone") {
		t.Fatalf("events: %s", ev)
	}
	if again := a.Events(fern.House, tok); string(again) != "[]" {
		t.Fatalf("a drained queue repeated: %s", again)
	}
	a.Step(5000)
	miller := people[0]
	t2, _ := a.Login(miller.House, miller.People[0].ID)
	_, _, msgs := a.HTTP(miller.House, "GET", "/api/messages?conv=everyone", "", t2)
	if !strings.Contains(msgs, "from the sim phone") {
		t.Fatalf("Miller doesn't see Fern's post after 5 s: %s", msgs)
	}
}
