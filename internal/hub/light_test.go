package hub

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeLine struct {
	mu      sync.Mutex
	val     bool
	changes int
}

func (f *fakeLine) Set(v bool) error {
	f.mu.Lock()
	if v != f.val {
		f.changes++
	}
	f.val = v
	f.mu.Unlock()
	return nil
}
func (f *fakeLine) Get() (bool, error) { f.mu.Lock(); defer f.mu.Unlock(); return f.val, nil }
func (f *fakeLine) Close() error       { return nil }

func TestLightFlashAndButton(t *testing.T) {
	led, btn := &fakeLine{}, &fakeLine{}
	var mu sync.Mutex
	short, long := 0, 0
	l := NewLight(LightConfig{Settings: DefaultLight,
		OnShort: func() { mu.Lock(); short++; mu.Unlock() }, OnLong: func() { mu.Lock(); long++; mu.Unlock() }})
	l.open = func(pin int, out bool) (gpioLine, error) {
		if out {
			return led, nil
		}
		return btn, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.Run(ctx)
	l.Flash("public")
	time.Sleep(700 * time.Millisecond)
	led.mu.Lock()
	if led.changes < 2 || led.val {
		t.Fatalf("public: %d changes, on %v", led.changes, led.val)
	}
	led.mu.Unlock()
	l.Flash("direct")
	time.Sleep(50 * time.Millisecond)
	if !l.Status().Blinking {
		t.Fatal("a direct message should keep the LED blinking")
	}
	// A short press: read.
	btn.Set(true)
	time.Sleep(200 * time.Millisecond)
	btn.Set(false)
	time.Sleep(200 * time.Millisecond)
	if l.Status().Blinking {
		t.Fatal("a short press should stop the blinking")
	}
	// Held for 3 s: long, once.
	btn.Set(true)
	time.Sleep(3300 * time.Millisecond)
	btn.Set(false)
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if short != 1 || long != 1 {
		t.Fatalf("short %d, long %d", short, long)
	}
	if err := l.Configure(LightSettings{LED: 17, Button: 17}); err == nil {
		t.Fatal("the same pin twice was accepted")
	}
	if err := l.Configure(LightSettings{LED: 0, Button: -1}); err == nil {
		t.Fatal("GPIO 0 (the HAT's EEPROM) was accepted")
	}
}
