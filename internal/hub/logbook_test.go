package hub

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
)

func TestLogbook(t *testing.T) {
	dir := t.TempDir()
	saved := []bool{}
	l := NewLogbook(dir, meshtastic.New("127.0.0.1:1"), func(on bool) error { saved = append(saved, on); return nil })
	l.Event(meshtastic.Event{Kind: "packet", From: "!a1b2c3d4"}) // off: not logged
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	lat := 52.5
	l.Event(meshtastic.Event{At: time.Now(), Kind: "packet", From: "!a1b2c3d4", Port: "position", Lat: &lat, SNR: -7.25})
	l.Event(meshtastic.Event{At: time.Now(), Kind: "packet", From: "!a1b2c3d4", Port: "text", Text: "hi"})
	st := l.Status()
	if !st.On || st.Packets != 2 || st.Radios != 1 || len(st.Files) != 1 {
		t.Fatalf("status %+v", st)
	}
	name := st.File
	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 || !saved[0] || saved[1] {
		t.Errorf("persisted %v", saved)
	}
	if l.Path(name) == "" || l.Path("../hub.json") != "" || l.Path("node.key") != "" {
		t.Error("Path lets the wrong files through")
	}
	f, _ := os.Open(l.Path(name))
	defer f.Close()
	var kinds []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("line %q: %v", sc.Text(), err)
		}
		if m["t"] == nil || m["up"] == nil {
			t.Errorf("line without times: %v", m)
		}
		kinds = append(kinds, m["kind"].(string))
		if m["port"] == "position" && m["lat"] != 52.5 {
			t.Errorf("position %v", m)
		}
	}
	if len(kinds) != 4 || kinds[0] != "start" || kinds[3] != "stop" {
		t.Errorf("kinds %v", kinds)
	}
}
