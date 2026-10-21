package hub

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/gpio"
)

// LightSettings are the LED and the button on the Pi's GPIO pins (BCM
// numbers; -1 for none). No omitempty: see SoundSettings.
type LightSettings struct {
	LED          int  `json:"led"`
	Button       int  `json:"button"`
	KeepBlinking bool `json:"keepBlinking"` // a slow blink until the messages are read
	Public       bool `json:"public"`       // one blink for a message to everyone
}

// DefaultLight: an LED on GPIO 17 (pin 11), a button on GPIO 27 (pin 13).
var DefaultLight = LightSettings{LED: 17, Button: 27, KeepBlinking: true, Public: true}

// physicalPin maps a BCM GPIO number to its pin on the 40-pin header.
var physicalPin = map[int]int{2: 3, 3: 5, 4: 7, 5: 29, 6: 31, 7: 26, 8: 24, 9: 21, 10: 19, 11: 23, 12: 32, 13: 33,
	14: 8, 15: 10, 16: 36, 17: 11, 18: 12, 19: 35, 20: 38, 21: 40, 22: 15, 23: 16, 24: 18, 25: 22, 26: 37, 27: 13}

// LightStatus is what the panel shows.
type LightStatus struct {
	Settings   LightSettings `json:"settings"`
	LEDPin     int           `json:"ledPin,omitempty"` // on the header
	ButtonPin  int           `json:"buttonPin,omitempty"`
	LEDOK      bool          `json:"ledOK"`
	ButtonOK   bool          `json:"buttonOK"`
	Blinking   bool          `json:"blinking"`
	LastPress  time.Time     `json:"lastPress,omitempty"`
	LastPressK string        `json:"lastPressKind,omitempty"` // short, long
	Error      string        `json:"error,omitempty"`
}

type gpioLine interface {
	Set(bool) error
	Get() (bool, error)
	Close() error
}

// Light drives the LED and reads the button.
type Light struct {
	open    func(pin int, out bool) (gpioLine, error)
	save    func(LightSettings) error
	onShort func() // a short press: messages read
	onLong  func() // a press of 3 s: the hub's own WiFi on or off

	kick chan struct{}

	mu        sync.Mutex
	set       LightSettings
	led, btn  gpioLine
	flashes   []string
	blinking  bool
	lastPress time.Time
	lastKind  string
	err       string
}

// LightConfig wires a Light.
type LightConfig struct {
	Settings LightSettings
	Save     func(LightSettings) error
	OnShort  func()
	OnLong   func()
}

// NewLight returns the LED and button; Run drives them.
func NewLight(c LightConfig) *Light {
	return &Light{set: c.Settings, save: c.Save, onShort: c.OnShort, onLong: c.OnLong, kick: make(chan struct{}, 1),
		open: func(pin int, out bool) (gpioLine, error) {
			if out {
				return gpio.Output("/dev/gpiochip0", pin)
			}
			return gpio.Input("/dev/gpiochip0", pin)
		}}
}

func validPin(p int) bool { _, ok := physicalPin[p]; return ok || p == -1 }

// Configure changes the pins and the blinking.
func (l *Light) Configure(s LightSettings) error {
	if !validPin(s.LED) || !validPin(s.Button) {
		return errors.New("A GPIO number from 2 to 27, or none.")
	}
	if s.LED != -1 && s.LED == s.Button {
		return errors.New("The LED and the button need different pins.")
	}
	l.mu.Lock()
	l.set = s
	l.closeLines()
	l.mu.Unlock()
	l.poke()
	if l.save != nil {
		return l.save(s)
	}
	return nil
}

// Settings returns the light settings.
func (l *Light) Settings() LightSettings {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.set
}

func (l *Light) closeLines() { // caller holds mu
	if l.led != nil {
		l.led.Set(false)
		l.led.Close()
		l.led = nil
	}
	if l.btn != nil {
		l.btn.Close()
		l.btn = nil
	}
}

// openLines requests the pins the settings name (caller holds mu).
func (l *Light) openLines() {
	l.err = ""
	if l.led == nil && l.set.LED >= 0 {
		ln, err := l.open(l.set.LED, true)
		if err != nil {
			l.err = "LED: " + err.Error()
		} else {
			l.led = ln
		}
	}
	if l.btn == nil && l.set.Button >= 0 {
		ln, err := l.open(l.set.Button, false)
		if err != nil {
			if l.err != "" {
				l.err += "; "
			}
			l.err += "button: " + err.Error()
		} else {
			l.btn = ln
		}
	}
}

// Flash shows an event: direct (three quick blinks, then a slow blink until
// read), public (one blink), test (five), wifi (two long).
func (l *Light) Flash(kind string) {
	l.mu.Lock()
	switch kind {
	case "public":
		if !l.set.Public {
			l.mu.Unlock()
			return
		}
	case "direct", "reply":
		if l.set.KeepBlinking {
			l.blinking = true
		}
	}
	if len(l.flashes) < 8 {
		l.flashes = append(l.flashes, kind)
	}
	l.mu.Unlock()
	l.poke()
}

// Seen stops the slow blink: the messages were read.
func (l *Light) Seen() {
	l.mu.Lock()
	l.blinking = false
	l.mu.Unlock()
	l.poke()
}

func (l *Light) poke() {
	select {
	case l.kick <- struct{}{}:
	default:
	}
}

var patterns = map[string][]time.Duration{ // on, off, on, off...
	"direct": {120, 120, 120, 120, 120, 400},
	"reply":  {120, 120, 120, 400},
	"public": {200, 300},
	"test":   {150, 150, 150, 150, 150, 150, 150, 150, 150, 400},
	"wifi":   {600, 300, 600, 400},
}

// Run drives the LED and polls the button until ctx ends.
func (l *Light) Run(ctx context.Context) {
	defer func() {
		l.mu.Lock()
		l.closeLines()
		l.mu.Unlock()
	}()
	go l.runButton(ctx)
	for ctx.Err() == nil {
		l.mu.Lock()
		l.openLines()
		var next string
		if len(l.flashes) > 0 {
			next, l.flashes = l.flashes[0], l.flashes[1:]
		}
		led, blinking := l.led, l.blinking
		l.mu.Unlock()
		switch {
		case next != "" && led != nil:
			p := patterns[next]
			for i, d := range p {
				led.Set(i%2 == 0)
				sleepCtx(ctx, d*time.Millisecond)
			}
			led.Set(false)
		case blinking && led != nil:
			led.Set(true)
			sleepCtx(ctx, 150*time.Millisecond)
			led.Set(false)
			select {
			case <-ctx.Done():
			case <-l.kick:
			case <-time.After(2 * time.Second):
			}
		default:
			select {
			case <-ctx.Done():
			case <-l.kick:
			case <-time.After(30 * time.Second): // try a pin that failed again now and then
			}
		}
	}
}

// runButton polls the button every 30 ms: a press under 1.5 s is short, a
// press held for 3 s is long (it fires while still held).
func (l *Light) runButton(ctx context.Context) {
	t := time.NewTicker(30 * time.Millisecond)
	defer t.Stop()
	var down time.Time
	stable, count, fired := false, 0, false
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		l.mu.Lock()
		btn := l.btn
		l.mu.Unlock()
		if btn == nil {
			stable, count, fired = false, 0, false
			continue
		}
		v, err := btn.Get()
		if err != nil {
			continue
		}
		if v == stable {
			count = 0
		} else if count++; count >= 3 { // 90 ms the same: real
			stable, count = v, 0
			now := time.Now()
			if v {
				down, fired = now, false
			} else if !fired && now.Sub(down) < 1500*time.Millisecond {
				l.pressed("short")
			}
		}
		if stable && !fired && time.Since(down) >= 3*time.Second {
			fired = true
			l.pressed("long")
		}
	}
}

func (l *Light) pressed(kind string) {
	l.mu.Lock()
	l.lastPress, l.lastKind = time.Now(), kind
	l.mu.Unlock()
	switch kind {
	case "short":
		l.Seen()
		if l.onShort != nil {
			go l.onShort()
		}
	case "long":
		l.Flash("wifi")
		if l.onLong != nil {
			go l.onLong()
		}
	}
}

// Status returns what the panel shows.
func (l *Light) Status() LightStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return LightStatus{Settings: l.set, LEDPin: physicalPin[l.set.LED], ButtonPin: physicalPin[l.set.Button],
		LEDOK: l.led != nil, ButtonOK: l.btn != nil, Blinking: l.blinking, LastPress: l.lastPress, LastPressK: l.lastKind, Error: l.err}
}
