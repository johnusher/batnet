package hub

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Review 2026-10-22: the panel's server side (hub.go, kit.go, logbook.go, maps.go).

// A saved log goes to the LAN only with the hub PIN, and only a real log file
// under the logs folder is served, whatever the name asks for.
func TestReviewLogDownloadNeedsThePIN(t *testing.T) {
	dir := t.TempDir()
	l := NewLogbook(dir, nil, nil)
	if err := l.Start(); err != nil {
		t.Fatal(err)
	}
	name := l.Status().File
	l.Stop()
	os.WriteFile(filepath.Join(dir, "other.jsonl"), []byte("{}\n"), 0o640)
	h := New(Config{HubPIN: "246810", Log: l}).Handler()
	get := func(path, from string) int {
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = from
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := get("/hub/logs/"+name, "192.168.1.5:4000"); c != 401 {
		t.Errorf("the LAN without the PIN: %d", c)
	}
	if c := get("/hub/logs/"+name+"?pin=246810", "192.168.1.5:4000"); c != 200 {
		t.Errorf("the LAN with the PIN: %d", c)
	}
	if c := get("/hub/logs/"+name, "127.0.0.1:4000"); c != 200 {
		t.Errorf("the screen: %d", c)
	}
	for _, p := range []string{"/hub/logs/other.jsonl", "/hub/logs/last-hello", "/hub/logs/log-..%2F..%2Fhub.json"} {
		if c := get(p+"?pin=246810", "192.168.1.5:4000"); c != 404 {
			t.Errorf("%s: %d", p, c)
		}
	}
}

// A map is public data: served with range requests and no PIN, but only a
// map the hub has, by its area ID.
func TestReviewMapFileServesRanges(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "berlin.pmtiles"), []byte("0123456789"), 0o644)
	os.WriteFile(filepath.Join(dir, "berlin.pmtiles.part"), []byte("partial"), 0o644)
	m := NewMaps(MapsConfig{Dir: dir, Latest: func(context.Context) (string, string, error) { return "", "", nil },
		Extract: func(context.Context, string, string, [4]float64, int) error { return nil },
		Free:    func(string) (int64, error) { return 10 << 30, nil }})
	h := New(Config{HubPIN: "246810", Maps: m}).Handler()
	req := httptest.NewRequest("GET", "/hub/maps/berlin.pmtiles", nil)
	req.RemoteAddr = "192.168.1.5:4000"
	req.Header.Set("Range", "bytes=2-4")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 206 || rec.Body.String() != "234" || rec.Header().Get("Content-Range") != "bytes 2-4/10" {
		t.Errorf("range: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Range"))
	}
	for _, p := range []string{"/hub/maps/paris.pmtiles", "/hub/maps/berlin.pmtiles.part", "/hub/maps/berlin.json", "/hub/maps/..%2Fhub.json"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != 404 {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
}
