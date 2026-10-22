package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
)

const (
	sampleEvery = 30 * time.Second
	// HelloEvery is how often a running log says hello by default: one
	// NodeInfo packet, about a second of airtime, well inside the 10% duty
	// cycle. At this pace every radio that hears it answers (askBelow), so a
	// hub that stays put for long says hello less often (SetHelloEvery).
	HelloEvery = meshtastic.HelloGap
	// askBelow: hellos more often than this ask the radios to answer.
	askBelow = 30 * time.Minute
)

// Logbook writes everything the radio hears to a JSON-lines file, one per
// run, for trips such as a bike ride: every packet with its signal, a sample
// of the radio and the Pi every 30 s, and a hello every 5 minutes. Once on, it
// stays on across restarts until stopped.
type Logbook struct {
	dir     string
	radio   *meshtastic.Client
	persist func(on bool) error
	boot    string
	every   time.Duration // between hellos; 0 means no hellos

	mu      sync.Mutex
	f       *os.File
	name    string
	started time.Time
	lines   int
	packets int
	hellos  int
	unread  int
	heard   map[string]bool
	lastErr string
}

// LogStatus is what the panel shows.
type LogStatus struct {
	On      bool      `json:"on"`
	File    string    `json:"file,omitempty"`
	Started time.Time `json:"started,omitempty"`
	Lines   int       `json:"lines"`
	Packets int       `json:"packets"`
	Hellos  int       `json:"hellos"`
	Radios  int       `json:"radios"` // other radios heard in this log
	// Unreadable counts packets the radio heard but could not read (too weak).
	Unreadable int `json:"unreadable"`
	// HelloMinutes is the time between hellos; 0: it only listens.
	HelloMinutes float64   `json:"helloMinutes"`
	Error        string    `json:"error,omitempty"`
	Files        []LogFile `json:"files"`
}

// LogFile is one saved log.
type LogFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// NewLogbook keeps logs in dir; persist saves whether logging is on.
func NewLogbook(dir string, radio *meshtastic.Client, persist func(bool) error) *Logbook {
	b, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return &Logbook{dir: dir, radio: radio, persist: persist, boot: strings.TrimSpace(string(b)), every: HelloEvery, heard: map[string]bool{}}
}

// SetHelloEvery sets the time between hellos; 0 stops them.
func (l *Logbook) SetHelloEvery(d time.Duration) {
	l.mu.Lock()
	l.every = d
	l.mu.Unlock()
}

// Start opens a new log file.
func (l *Logbook) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f != nil {
		return nil
	}
	if err := os.MkdirAll(l.dir, 0o750); err != nil {
		return err
	}
	now := time.Now()
	name := "log-" + now.Format("20060102-150405") + ".jsonl"
	f, err := os.OpenFile(filepath.Join(l.dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return err
	}
	l.f, l.name, l.started, l.lines, l.packets, l.hellos, l.unread, l.heard, l.lastErr = f, name, now, 0, 0, 0, 0, map[string]bool{}, ""
	l.writeLocked(map[string]any{"kind": "start", "boot": l.boot,
		"note": "times are the Pi's clock: without internet or GPS it may be off by the time the Pi was unplugged; 'up' (seconds since boot) is exact"})
	if l.persist != nil {
		return l.persist(true)
	}
	return nil
}

// Stop closes the log.
func (l *Logbook) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	l.writeLocked(map[string]any{"kind": "stop", "lines": l.lines, "packets": l.packets, "radios": len(l.heard), "unreadable": l.unread})
	l.f.Sync()
	l.f.Close()
	l.f = nil
	if l.persist != nil {
		return l.persist(false)
	}
	return nil
}

// Event logs one radio event, if logging is on.
func (l *Logbook) Event(e meshtastic.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	if e.Kind == "packet" {
		l.packets++
		l.heard[e.From] = true
	}
	b, _ := json.Marshal(e)
	var m map[string]any
	json.Unmarshal(b, &m)
	delete(m, "t")
	l.writeLocked(m)
}

// Unreadable logs a packet the radio heard but could not read.
func (l *Logbook) Unreadable(u meshtastic.Unreadable) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	l.unread++
	b, _ := json.Marshal(u)
	var m map[string]any
	json.Unmarshal(b, &m)
	delete(m, "t")
	m["kind"] = "unreadable"
	l.writeLocked(m)
}

// Note logs a line of its own kind.
func (l *Logbook) Note(kind string, fields map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	m := map[string]any{"kind": kind}
	for k, v := range fields {
		m[k] = v
	}
	l.writeLocked(m)
}

func uptime() float64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	var up float64
	fmt.Sscan(string(b), &up)
	return up
}

func (l *Logbook) writeLocked(m map[string]any) {
	m["t"] = time.Now().Format(time.RFC3339Nano)
	m["up"] = uptime()
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	if _, err := l.f.Write(append(b, '\n')); err != nil {
		l.lastErr = err.Error()
		return
	}
	l.lines++
}

// Run samples and says hello while logging is on, until ctx ends.
func (l *Logbook) Run(ctx context.Context, dataDir string) {
	sample := time.NewTicker(sampleEvery)
	defer sample.Stop()
	// The last hello is remembered across restarts: radios answer a hello
	// together and their answers collide, so a restart does not say it again
	// before its time. A time ahead of the clock (B39) does not count.
	helloFile := filepath.Join(l.dir, "last-hello")
	var lastHello time.Time
	if b, err := os.ReadFile(helloFile); err == nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(b))); err == nil && !t.After(time.Now()) {
			lastHello = t
		}
	}
	for {
		select {
		case <-ctx.Done():
			l.mu.Lock()
			if l.f != nil {
				l.writeLocked(map[string]any{"kind": "shutdown"})
				l.f.Sync()
				l.f.Close()
				l.f = nil // still on in hub.json: the next start opens a new file
			}
			l.mu.Unlock()
			return
		case <-sample.C:
		}
		if !l.Status().On {
			continue
		}
		sys := readSystem(dataDir)
		st := l.radio.Status()
		l.Note("sample", map[string]any{"tempC": sys.TempC, "memAvailMB": sys.MemAvailMB, "load1": sys.Load1,
			"radioConnected": st.Connected, "radioReady": st.Ready, "nodes": len(st.Nodes), "packetsIn": st.PacketsIn})
		l.mu.Lock()
		every := l.every
		l.mu.Unlock()
		if every > 0 && time.Since(lastHello) >= every && st.Ready {
			// A hello every few minutes is a range test (a walk, a ride):
			// it asks for answers. An hourly one only says the hub is here,
			// sparing the air a burst of answers that collide (B48).
			err := l.radio.SayHello(every < askBelow)
			f := map[string]any{"ok": err == nil}
			if err != nil {
				f["error"] = err.Error()
			} else {
				lastHello = time.Now()
				os.WriteFile(helloFile, []byte(lastHello.Format(time.RFC3339)+"\n"), 0o644)
				l.mu.Lock()
				l.hellos++
				l.mu.Unlock()
			}
			var heard []map[string]any
			for _, n := range st.Nodes {
				if !n.Self {
					heard = append(heard, map[string]any{"id": n.ID, "name": n.LongName, "snr": n.SNR, "rssi": n.RSSI, "hops": n.Hops, "lastHeard": n.LastHeard})
				}
			}
			f["nodes"] = heard
			l.Note("hello", f)
		}
		l.mu.Lock()
		if l.f != nil {
			l.f.Sync() // a bike ride ends by pulling the power
		}
		l.mu.Unlock()
	}
}

// Status reports the log and the saved files, newest first.
func (l *Logbook) Status() LogStatus {
	l.mu.Lock()
	st := LogStatus{On: l.f != nil, Lines: l.lines, Packets: l.packets, Hellos: l.hellos, Radios: len(l.heard), Unreadable: l.unread, HelloMinutes: l.every.Minutes(), Error: l.lastErr}
	if st.On {
		st.File, st.Started = l.name, l.started
	}
	l.mu.Unlock()
	ents, _ := os.ReadDir(l.dir)
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			if info, err := e.Info(); err == nil {
				st.Files = append(st.Files, LogFile{Name: e.Name(), Size: info.Size()})
			}
		}
	}
	sort.Slice(st.Files, func(i, j int) bool { return st.Files[i].Name > st.Files[j].Name })
	return st
}

// Path returns a saved log's path, or "" if name is not one.
func (l *Logbook) Path(name string) string {
	if name != filepath.Base(name) || !strings.HasPrefix(name, "log-") || !strings.HasSuffix(name, ".jsonl") {
		return ""
	}
	p := filepath.Join(l.dir, name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}
