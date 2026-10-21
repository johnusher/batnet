package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/johnusher/batnet/internal/hub"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/meshtastic"
	"github.com/johnusher/batnet/internal/translate"
)

// kit is the hub's power-cut kit (plan 5a): its own WiFi, sound, the LED and
// button, power modes, plugged-in devices and the translator.
type kit struct {
	wifi       *hub.WiFi
	sound      *hub.Sound
	light      *hub.Light
	power      *hub.Power
	devices    *hub.Devices
	translator *translate.Client
	notifier   *hub.Notifier
}

// saver saves one part of hub.json.
func (h *hubRun) saver(set func(*hubSettings)) func() error {
	return func() error {
		h.mu.Lock()
		defer h.mu.Unlock()
		set(&h.settings)
		return h.save()
	}
}

func (h *hubRun) startKit(ctx context.Context, radio *meshtastic.Client, id ledger.NodeID, wifiPass string) *kit {
	h.mu.Lock()
	set := h.settings
	h.mu.Unlock()
	k := &kit{}
	k.sound = hub.NewSound(hub.SoundConfig{Run: hub.Exec, Settings: set.Sound,
		Save: func(s hub.SoundSettings) error { return h.saver(func(x *hubSettings) { x.Sound = s })() }})
	k.power = hub.NewPower(hub.PowerConfig{Run: hub.Exec, Settings: set.Power,
		Save: func(s hub.PowerSettings) error { return h.saver(func(x *hubSettings) { x.Power = s })() }})
	k.notifier = &hub.Notifier{Sound: k.sound, Power: k.power,
		MyID: func() string { return fmt.Sprintf("!%08x", radio.Status().MyNum) },
		Mine: func(msg uint32) bool {
			for _, t := range radio.Status().Texts {
				if t.Mine && t.ID == msg {
					return true
				}
			}
			return false
		}}
	var mu sync.Mutex
	lastMode := ""
	k.wifi = hub.NewWiFi(hub.WiFiConfig{Run: hub.Exec, Password: wifiPass, DefaultSSID: fmt.Sprintf("batnet-%x", id[:2]),
		Settings: set.WiFi, StartedFile: filepath.Join(h.data, "wifi-started"),
		Save: func(s hub.WiFiSettings) error { return h.saver(func(x *hubSettings) { x.WiFi = s })() },
		OnChange: func(mode string) {
			mu.Lock()
			prev := lastMode
			if mode != "switching" {
				lastMode = mode
			}
			mu.Unlock()
			switch {
			case mode == "own" && prev != "own":
				log.Printf("wifi: the hub's own WiFi is on")
				k.sound.Notify("wifi-on", "", "")
				k.light.Flash("wifi")
			case mode == "home" && prev == "own":
				log.Printf("wifi: back on the home WiFi")
				k.sound.Notify("wifi-off", "", "")
			}
		}})
	k.light = hub.NewLight(hub.LightConfig{Settings: set.Light,
		Save:    func(s hub.LightSettings) error { return h.saver(func(x *hubSettings) { x.Light = s })() },
		OnShort: k.notifier.Seen,
		OnLong: func() {
			if on, err := k.wifi.Toggle(); err != nil {
				log.Printf("button: wifi: %v", err)
			} else {
				log.Printf("button: the hub's own WiFi %v", map[bool]string{true: "on", false: "off"}[on])
			}
		}})
	k.notifier.Light = k.light
	k.translator = translate.New("", "")
	k.translator.IdleStop = k.power.IdleStop
	// The screen runs only while one is plugged in: with none, the kiosk drew
	// to nothing and kept a CPU core busy (decision 0029).
	k.devices = hub.NewDevices("/", func(added, removed, all []hub.Device) {
		for _, d := range added {
			log.Printf("devices: plugged in: %s", d.Name)
		}
		for _, d := range removed {
			log.Printf("devices: unplugged: %s", d.Name)
		}
		screenNow := hub.HasScreen(all)
		k.power.SetScreen(screenNow)
		changed := len(added)+len(removed) == 0 // the first look
		for _, d := range append(append([]hub.Device{}, added...), removed...) {
			changed = changed || d.Kind == "screen"
		}
		if !changed {
			if len(added) > 0 {
				k.power.Activity() // a keyboard or mouse plugged in wakes the screen
			}
			return
		}
		verb := "stop"
		if screenNow {
			verb = "start"
			k.power.Activity()
		}
		if _, err := os.Stat("/etc/systemd/system/batnet-screen.service"); err == nil {
			if err := hub.Systemctl(hub.Exec, verb, "batnet-screen.service"); err != nil {
				log.Printf("screen: %s: %v", verb, err)
			}
		}
	})
	radio.OnEvent(k.notifier.Event)
	go k.notifier.Run(ctx)
	go k.sound.Run(ctx)
	go k.power.Run(ctx)
	go k.light.Run(ctx)
	go k.devices.Run(ctx)
	go k.translator.Run(ctx)
	go hub.WatchInput(ctx, "/dev/input", k.power.Activity)
	if _, err := os.Stat("/usr/bin/nmcli"); err == nil {
		go k.wifi.Run(ctx)
	}
	return k
}

// loadWiFiPassword reads the hub's own WiFi password, or makes one: three
// groups of four letters and digits that can't be mixed up (no 0/o, 1/l/i).
func loadWiFiPassword(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if p := strings.TrimSpace(string(b)); len(p) >= 8 {
			return p, nil
		}
	}
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	var b strings.Builder
	for i := 0; i < 12; i++ {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	p := b.String()
	return p, writeFileAtomic(path, []byte(p+"\n"), 0o600)
}
