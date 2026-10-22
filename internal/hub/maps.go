package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Offline maps (decision 0032): vector map extracts from Protomaps' daily
// build of OpenStreetMap, kept on the hub, so the map shows streets with no
// internet. The `pmtiles` tool cuts an area out of the build over HTTP range
// requests; the panel draws it with protomaps-leaflet.

// MapArea is an area the hub can keep a map of.
type MapArea struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Bounds [4]float64 `json:"bounds"` // west, south, east, north
	Have   bool       `json:"have"`
	Bytes  int64      `json:"bytes,omitempty"`
	Build  string     `json:"build,omitempty"` // the OpenStreetMap build it came from, e.g. 20261022
	At     time.Time  `json:"at,omitempty"`    // when the hub got it
}

// MapsStatus is the offline maps' part of the panel's status.
type MapsStatus struct {
	Areas   []MapArea `json:"areas"`
	Tool    bool      `json:"tool"`              // the pmtiles tool is installed
	Getting string    `json:"getting,omitempty"` // the area being fetched
	SoFar   int64     `json:"soFar,omitempty"`   // bytes of it so far
	Error   string    `json:"error,omitempty"`
}

// MapMaxZoom is the closest zoom kept: single streets and house blocks.
const MapMaxZoom = 15

// HereID is the area around the hub itself, about 55 km across.
const HereID = "here"

var cityAreas = []MapArea{
	{ID: "berlin", Name: "Berlin", Bounds: [4]float64{13.05, 52.30, 13.80, 52.70}},
	{ID: "london", Name: "London", Bounds: [4]float64{-0.55, 51.27, 0.30, 51.70}},
	{ID: "paris", Name: "Paris", Bounds: [4]float64{2.05, 48.70, 2.65, 49.00}},
	{ID: "barcelona", Name: "Barcelona", Bounds: [4]float64{1.95, 41.27, 2.30, 41.50}},
}

// hereBounds is a box about 55 km across centred on w.
func hereBounds(w Where) [4]float64 {
	dLat := 0.25
	dLon := dLat / math.Max(math.Cos(w.Lat*math.Pi/180), 0.2)
	r := func(x float64) float64 { return math.Round(x*1e4) / 1e4 }
	return [4]float64{r(w.Lon - dLon), r(w.Lat - dLat), r(w.Lon + dLon), r(w.Lat + dLat)}
}

const buildsURL = "https://build-metadata.protomaps.dev/builds.json"

// MapsConfig wires the offline maps; nil functions get the real ones.
type MapsConfig struct {
	Dir   string
	Where func() *Where
	// Latest returns the newest build's URL and name (20261022).
	Latest func(ctx context.Context) (url, build string, err error)
	// Extract cuts bounds out of src into dst.
	Extract func(ctx context.Context, src, dst string, bounds [4]float64, maxZoom int) error
	// Free is the free space where the maps go, in bytes.
	Free func(dir string) (int64, error)
}

// Maps keeps the hub's offline maps.
type Maps struct {
	cfg MapsConfig

	mu      sync.Mutex
	getting string
	err     string
	lastTry time.Time // the last automatic try for the area round the hub
}

// minFree is the room a map fetch leaves for the logs and the rest.
const minFree = 2 << 30

// NewMaps returns the offline maps kept in cfg.Dir.
func NewMaps(cfg MapsConfig) *Maps {
	if cfg.Latest == nil {
		cfg.Latest = latestBuild
	}
	if cfg.Extract == nil {
		cfg.Extract = pmtilesExtract
	}
	if cfg.Free == nil {
		cfg.Free = freeSpace
	}
	if cfg.Where == nil {
		cfg.Where = func() *Where { return nil }
	}
	return &Maps{cfg: cfg}
}

var mapIDRe = regexp.MustCompile(`^[a-z]+$`)

// File is the path of an area's map, or "" if the hub has none.
func (m *Maps) File(id string) string {
	if !mapIDRe.MatchString(id) {
		return ""
	}
	p := filepath.Join(m.cfg.Dir, id+".pmtiles")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

type mapMeta struct {
	Name   string     `json:"name"`
	Bounds [4]float64 `json:"bounds"`
	Build  string     `json:"build"`
	At     time.Time  `json:"at"`
}

func (m *Maps) meta(id string) (mapMeta, bool) {
	b, err := os.ReadFile(filepath.Join(m.cfg.Dir, id+".json"))
	if err != nil {
		return mapMeta{}, false
	}
	var mm mapMeta
	return mm, json.Unmarshal(b, &mm) == nil
}

// area is what the hub knows of an area: one of the cities, or round the hub.
func (m *Maps) area(id string) (MapArea, bool) {
	for _, a := range cityAreas {
		if a.ID == id {
			return a, true
		}
	}
	if id == HereID {
		if w := m.cfg.Where(); w != nil {
			return MapArea{ID: HereID, Name: "Around this hub", Bounds: hereBounds(*w)}, true
		}
	}
	return MapArea{}, false
}

// Status lists the areas: round the hub first, then the cities.
func (m *Maps) Status() MapsStatus {
	var areas []MapArea
	if a, ok := m.area(HereID); ok {
		areas = append(areas, a)
	} else if mm, ok := m.meta(HereID); ok {
		areas = append(areas, MapArea{ID: HereID, Name: "Around this hub", Bounds: mm.Bounds})
	}
	areas = append(areas, cityAreas...)
	for i, a := range areas {
		p := m.File(a.ID)
		if p == "" {
			continue
		}
		mm, ok := m.meta(a.ID)
		if !ok {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil { // removed between the two looks
			continue
		}
		// The map has what was cut, which for round the hub may be an older place.
		areas[i].Have, areas[i].Bytes, areas[i].Build, areas[i].At, areas[i].Bounds = true, fi.Size(), mm.Build, mm.At, mm.Bounds
	}
	_, err := exec.LookPath("pmtiles")
	m.mu.Lock()
	defer m.mu.Unlock()
	st := MapsStatus{Areas: areas, Tool: err == nil, Getting: m.getting, Error: m.err}
	if m.getting != "" {
		if fi, err := os.Stat(filepath.Join(m.cfg.Dir, m.getting+".pmtiles.part")); err == nil {
			st.SoFar = fi.Size()
		}
	}
	return st
}

var errMapBusy = errors.New("A map is being fetched already: one at a time.")

// Get fetches an area's map in the background (or a newer one over it).
func (m *Maps) Get(id string) error {
	a, ok := m.area(id)
	if !ok {
		if id == HereID {
			return errors.New("Mark where this hub is on the map first.")
		}
		return errors.New("No such area.")
	}
	m.mu.Lock()
	if m.getting != "" {
		m.mu.Unlock()
		return errMapBusy
	}
	m.getting, m.err = id, ""
	m.mu.Unlock()
	go func() {
		err := m.fetch(a)
		m.mu.Lock()
		m.getting = ""
		if err != nil {
			m.err = fmt.Sprintf("%s: %v", a.Name, err)
			log.Printf("maps: %s", m.err)
		} else {
			log.Printf("maps: got %s", a.Name)
		}
		m.mu.Unlock()
	}()
	return nil
}

func (m *Maps) fetch(a MapArea) error {
	if err := os.MkdirAll(m.cfg.Dir, 0o755); err != nil {
		return err
	}
	if free, err := m.cfg.Free(m.cfg.Dir); err == nil && free < minFree {
		return fmt.Errorf("not enough room on the card (%d MB free; a map needs it to stay above %d MB)", free>>20, minFree>>20)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	src, build, err := m.cfg.Latest(ctx)
	if err != nil {
		return fmt.Errorf("no internet, or the map server did not answer (%v)", err)
	}
	part := filepath.Join(m.cfg.Dir, a.ID+".pmtiles.part")
	defer os.Remove(part)
	if err := m.cfg.Extract(ctx, src, part, a.Bounds, MapMaxZoom); err != nil {
		return err
	}
	b, _ := json.Marshal(mapMeta{Name: a.Name, Bounds: a.Bounds, Build: build, At: time.Now()})
	if err := os.WriteFile(filepath.Join(m.cfg.Dir, a.ID+".json"), b, 0o644); err != nil {
		return err
	}
	return os.Rename(part, filepath.Join(m.cfg.Dir, a.ID+".pmtiles"))
}

// Remove deletes an area's map.
func (m *Maps) Remove(id string) error {
	if !mapIDRe.MatchString(id) {
		return errors.New("No such area.")
	}
	m.mu.Lock()
	busy := m.getting == id
	m.mu.Unlock()
	if busy {
		return errMapBusy
	}
	os.Remove(filepath.Join(m.cfg.Dir, id+".json"))
	if err := os.Remove(filepath.Join(m.cfg.Dir, id+".pmtiles")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Run fetches the map round the hub by itself, once its place is marked and
// no map it has covers it, whenever there is internet (tried at most hourly).
func (m *Maps) Run(ctx context.Context) {
	// A fetch cut short by a power cut leaves its part file: sweep it.
	if parts, _ := filepath.Glob(filepath.Join(m.cfg.Dir, "*.pmtiles.part")); len(parts) > 0 {
		for _, p := range parts {
			os.Remove(p)
		}
	}
	t := time.NewTicker(2 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		w := m.cfg.Where()
		if w == nil || m.covered(*w) {
			continue
		}
		if _, err := exec.LookPath("pmtiles"); err != nil {
			continue
		}
		m.mu.Lock()
		due := m.getting == "" && time.Since(m.lastTry) > time.Hour
		if due {
			m.lastTry = time.Now()
		}
		m.mu.Unlock()
		if due {
			m.Get(HereID)
		}
	}
}

// covered: a map the hub has holds w, with 10 km to spare.
func (m *Maps) covered(w Where) bool {
	for _, a := range m.Status().Areas {
		b := a.Bounds
		if a.Have && w.Lat-0.09 > b[1] && w.Lat+0.09 < b[3] && w.Lon-0.15 > b[0] && w.Lon+0.15 < b[2] {
			return true
		}
	}
	return false
}

func latestBuild(ctx context.Context) (string, string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, buildsURL, nil)
	c := &http.Client{Timeout: 20 * time.Second}
	res, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("builds list: %s", res.Status)
	}
	var builds []struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&builds); err != nil {
		return "", "", err
	}
	if len(builds) == 0 {
		return "", "", errors.New("no builds listed")
	}
	key := builds[len(builds)-1].Key
	if !regexp.MustCompile(`^\d{8}\.pmtiles$`).MatchString(key) {
		return "", "", fmt.Errorf("odd build name %q", key)
	}
	return "https://build.protomaps.com/" + key, strings.TrimSuffix(key, ".pmtiles"), nil
}

func pmtilesExtract(ctx context.Context, src, dst string, b [4]float64, maxZoom int) error {
	bbox := fmt.Sprintf("--bbox=%g,%g,%g,%g", b[0], b[1], b[2], b[3])
	cmd := exec.CommandContext(ctx, "pmtiles", "extract", src, dst, bbox, fmt.Sprintf("--maxzoom=%d", maxZoom), "--download-threads=2")
	out, err := cmd.CombinedOutput()
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return fmt.Errorf("pmtiles: %v: %s", err, lines[len(lines)-1])
	}
	return nil
}

func freeSpace(dir string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
