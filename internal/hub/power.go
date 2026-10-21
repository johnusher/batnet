package hub

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Power modes (decision 0029).
const (
	PowerHigh   = "high"
	PowerMedium = "medium"
	PowerLow    = "low"
)

// PowerSettings are the power mode and the reduced screen.
type PowerSettings struct {
	Mode string `json:"mode"`
	// Reduced asks for the reduced screen in high and medium too (low always has it).
	Reduced bool `json:"reduced"`
}

// PowerStatus is what the panel and the kiosk see.
type PowerStatus struct {
	Settings     PowerSettings `json:"settings"`
	ScreenOn     bool          `json:"screenOn"`
	Reduced      bool          `json:"reduced"`
	SleepsIn     int           `json:"sleepsIn"` // seconds until the screen sleeps; -1 never
	PollSeconds  int           `json:"pollSeconds"`
	UnderVoltage bool          `json:"underVoltage"` // the supply is too weak right now
	UVCount      int           `json:"uvCount"`      // times it went too weak since the start
	UVLast       time.Time     `json:"uvLast,omitempty"`
	UVKnown      bool          `json:"uvKnown"` // the Pi reports it
	Error        string        `json:"error,omitempty"`
}

// Power runs the power mode and the screen's sleep.
type Power struct {
	run  Runner
	save func(PowerSettings) error
	now  func() time.Time
	uv   string // the under-voltage alarm file

	mu       sync.Mutex
	set      PowerSettings
	screenOn bool
	lastAct  time.Time
	changed  chan struct{} // closed and replaced on every screen change
	uvNow    bool
	uvCount  int
	uvLast   time.Time
	err      string
}

// PowerConfig wires a Power.
type PowerConfig struct {
	Run      Runner
	Settings PowerSettings
	Save     func(PowerSettings) error
}

// NewPower returns the power side; Run applies the mode and watches.
func NewPower(c PowerConfig) *Power {
	if !validPower(c.Settings.Mode) {
		c.Settings.Mode = PowerHigh
	}
	p := &Power{run: c.Run, save: c.Save, set: c.Settings, now: time.Now, screenOn: true, changed: make(chan struct{})}
	p.lastAct = p.now()
	p.uv = findUnderVoltage()
	return p
}

func validPower(m string) bool { return m == PowerHigh || m == PowerMedium || m == PowerLow }

// findUnderVoltage finds the Pi's under-voltage alarm (the rpi_volt hwmon).
func findUnderVoltage() string {
	names, _ := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	for _, n := range names {
		if b, err := os.ReadFile(n); err == nil && strings.TrimSpace(string(b)) == "rpi_volt" {
			return filepath.Join(filepath.Dir(n), "in0_lcrit_alarm")
		}
	}
	return ""
}

func (p *Power) idleFor() time.Duration { // caller holds mu
	switch p.set.Mode {
	case PowerMedium:
		return 10 * time.Minute
	case PowerLow:
		return 2 * time.Minute
	}
	return 0
}

// Run applies the mode, then sleeps the screen when idle and watches the
// supply, until ctx ends.
func (p *Power) Run(ctx context.Context) {
	p.apply()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		p.check()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (p *Power) check() {
	uv := false
	if p.uv != "" {
		if b, err := os.ReadFile(p.uv); err == nil {
			uv = strings.TrimSpace(string(b)) == "1"
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if uv && !p.uvNow {
		p.uvCount++
		p.uvLast = p.now()
	}
	p.uvNow = uv
	if idle := p.idleFor(); idle > 0 && p.screenOn && p.now().Sub(p.lastAct) >= idle {
		p.screenOn = false
		p.signal()
	}
}

func (p *Power) signal() { // caller holds mu
	close(p.changed)
	p.changed = make(chan struct{})
}

// apply runs the root part of the mode: batnet-power@<mode>.service.
func (p *Power) apply() {
	p.mu.Lock()
	mode := p.set.Mode
	p.mu.Unlock()
	if p.run == nil {
		return
	}
	err := StartUnit(p.run, "batnet-power@"+mode+".service", false)
	p.mu.Lock()
	p.err = ""
	if err != nil {
		p.err = "The power mode's system part did not run: " + err.Error()
	}
	p.mu.Unlock()
}

// Activity is a key, the mouse, a touch: the screen wakes and its idle time
// starts again.
func (p *Power) Activity() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastAct = p.now()
	if !p.screenOn {
		p.screenOn = true
		p.signal()
	}
}

// Wake wakes the screen for an event: in low power only a key does.
func (p *Power) Wake(kind string) {
	p.mu.Lock()
	low := p.set.Mode == PowerLow
	p.mu.Unlock()
	if kind == "direct" && !low {
		p.Activity()
	}
}

// screenToken is what the kiosk reads: "on", "on reduced" or "off".
func (p *Power) screenToken() string { // caller holds mu
	if !p.screenOn {
		return "off"
	}
	if p.set.Mode == PowerLow || p.set.Reduced {
		return "on reduced"
	}
	return "on"
}

// WaitScreen returns the screen's state as soon as it differs from known, or
// after wait.
func (p *Power) WaitScreen(ctx context.Context, known string, wait time.Duration) string {
	p.mu.Lock()
	tok, ch := p.screenToken(), p.changed
	p.mu.Unlock()
	if tok != known {
		return tok
	}
	select {
	case <-ctx.Done():
	case <-ch:
	case <-time.After(wait):
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.screenToken()
}

// Configure changes the mode.
func (p *Power) Configure(s PowerSettings) error {
	if !validPower(s.Mode) {
		return errors.New("The power mode is high, medium or low.")
	}
	p.mu.Lock()
	p.set = s
	p.lastAct = p.now()
	p.screenOn = true
	p.signal()
	p.mu.Unlock()
	go p.apply()
	if p.save != nil {
		return p.save(s)
	}
	return nil
}

// Settings returns the power settings.
func (p *Power) Settings() PowerSettings {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.set
}

// IdleStop is how long the translator may sit loaded and unused.
func (p *Power) IdleStop() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.set.Mode == PowerLow {
		return time.Minute
	}
	return 10 * time.Minute
}

// Status returns what the panel shows.
func (p *Power) Status() PowerStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := PowerStatus{Settings: p.set, ScreenOn: p.screenOn, Reduced: p.set.Mode == PowerLow || p.set.Reduced, SleepsIn: -1,
		UnderVoltage: p.uvNow, UVCount: p.uvCount, UVLast: p.uvLast, UVKnown: p.uv != "", Error: p.err, PollSeconds: 2}
	if idle := p.idleFor(); idle > 0 && p.screenOn {
		left := idle - p.now().Sub(p.lastAct)
		if left < 0 {
			left = 0
		}
		st.SleepsIn = int(left / time.Second)
	}
	switch {
	case !p.screenOn:
		st.PollSeconds = 30
	case p.set.Mode == PowerLow:
		st.PollSeconds = 5
	}
	return st
}
