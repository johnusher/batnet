package hub

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOfflineMaps(t *testing.T) {
	dir := t.TempDir()
	var where *Where
	var cut [4]float64
	online, free := true, int64(10<<30)
	m := NewMaps(MapsConfig{Dir: dir, Where: func() *Where { return where },
		Latest: func(context.Context) (string, string, error) {
			if !online {
				return "", "", errors.New("no route to host")
			}
			return "https://build.example/20261022.pmtiles", "20261022", nil
		},
		Extract: func(_ context.Context, src, dst string, b [4]float64, z int) error {
			cut = b
			return os.WriteFile(dst, []byte("PMTiles map bytes"), 0o644)
		},
		Free: func(string) (int64, error) { return free, nil }})
	wait := func() MapsStatus {
		t.Helper()
		for i := 0; i < 200; i++ {
			if st := m.Status(); st.Getting == "" {
				return st
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("still fetching")
		return MapsStatus{}
	}

	// No place marked: the cities only, and no map round the hub.
	if st := m.Status(); len(st.Areas) != 4 || st.Areas[0].ID != "berlin" || st.Areas[0].Have {
		t.Fatalf("areas %+v", st.Areas)
	}
	if err := m.Get(HereID); err == nil {
		t.Error("a map round a hub with no place")
	}

	if err := m.Get("berlin"); err != nil {
		t.Fatal(err)
	}
	st := wait()
	if b := st.Areas[0]; !b.Have || b.Build != "20261022" || b.Bytes != 17 || st.Error != "" {
		t.Errorf("berlin %+v, %q", b, st.Error)
	}
	if m.File("berlin") == "" || m.File("../berlin") != "" || m.File("paris") != "" {
		t.Error("files")
	}

	// Round the hub: a box about 55 km across, centred on it.
	where = &Where{Lat: 52.5, Lon: 13.4}
	if err := m.Get(HereID); err != nil {
		t.Fatal(err)
	}
	wait()
	if cut[1] != 52.25 || cut[3] != 52.75 || cut[0] > 12.99 || cut[2] < 13.81 {
		t.Errorf("round the hub %v", cut)
	}
	if !m.covered(*where) || m.covered(Where{Lat: 41.4, Lon: 2.15}) {
		t.Error("covered")
	}

	// No internet, no room: said, and nothing half-written left.
	online = false
	m.Get("paris")
	if st := wait(); !strings.Contains(st.Error, "no internet") || m.File("paris") != "" {
		t.Errorf("offline: %q", st.Error)
	}
	online, free = true, 1<<30
	m.Get("paris")
	if st := wait(); !strings.Contains(st.Error, "not enough room") {
		t.Errorf("full card: %q", st.Error)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 4 { // berlin and here, each with its .json
		t.Errorf("files left: %v", entries)
	}

	// The panel serves a map with ranges, and removes it.
	s := New(Config{Maps: m})
	h := s.Handler()
	req := httptest.NewRequest("GET", "/hub/maps/berlin.pmtiles", nil)
	req.Header.Set("Range", "bytes=0-6")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 206 || w.Body.String() != "PMTiles" {
		t.Errorf("range: %d %q", w.Code, w.Body.String())
	}
	for _, p := range []string{"/hub/maps/paris.pmtiles", "/hub/maps/berlin.json", "/hub/maps/..%2fhub.json"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 404 {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
	if code, out := post(h, "/hub/api/maps", "127.0.0.1:5000", "", `{"op":"remove","area":"berlin"}`, true); code != 200 || m.File("berlin") != "" {
		t.Errorf("remove: %d %v", code, out)
	}
}
