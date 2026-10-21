package hub

import (
	"bufio"
	"context"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Device is one thing plugged into the hub.
type Device struct {
	Kind   string `json:"kind"` // screen, keyboard, mouse, touch, sound, lora, wifi, gps, storage, usb
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
}

func (d Device) key() string { return d.Kind + "|" + d.Name }

// DeviceEvent is a device arriving or leaving.
type DeviceEvent struct {
	At      time.Time `json:"at"`
	Plugged bool      `json:"plugged"`
	Device  Device    `json:"device"`
}

// DevicesStatus is what the panel shows.
type DevicesStatus struct {
	Devices []Device      `json:"devices"`
	Events  []DeviceEvent `json:"events"`
}

// Devices watches what is plugged in.
type Devices struct {
	root     string // "/" on the Pi; a test folder in tests
	now      func() time.Time
	onChange func(added, removed []Device, all []Device)

	mu     sync.Mutex
	list   []Device
	events []DeviceEvent
	primed bool
}

// NewDevices returns the watcher. onChange is called from Run's goroutine
// with what changed, and once after the first look with nothing added or
// removed.
func NewDevices(root string, onChange func(added, removed, all []Device)) *Devices {
	if root == "" {
		root = "/"
	}
	return &Devices{root: root, now: time.Now, onChange: onChange}
}

// Run looks every 3 s until ctx ends.
func (d *Devices) Run(ctx context.Context) {
	for {
		d.Scan()
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// Scan looks once and reports changes. The first scan reports nothing as
// added or removed, only the list.
func (d *Devices) Scan() {
	now := Snapshot(d.root)
	d.mu.Lock()
	prev := map[string]Device{}
	for _, x := range d.list {
		prev[x.key()] = x
	}
	cur := map[string]bool{}
	var added, removed []Device
	for _, x := range now {
		cur[x.key()] = true
		if _, ok := prev[x.key()]; !ok && d.primed {
			added = append(added, x)
		}
	}
	for k, x := range prev {
		if !cur[k] {
			removed = append(removed, x)
		}
	}
	first := !d.primed
	d.primed = true
	d.list = now
	t := d.now()
	for _, x := range added {
		d.events = append(d.events, DeviceEvent{At: t, Plugged: true, Device: x})
	}
	for _, x := range removed {
		d.events = append(d.events, DeviceEvent{At: t, Plugged: false, Device: x})
	}
	if len(d.events) > 20 {
		d.events = d.events[len(d.events)-20:]
	}
	d.mu.Unlock()
	if d.onChange != nil && (first || len(added) > 0 || len(removed) > 0) {
		d.onChange(added, removed, now)
	}
}

// Status returns the devices and the latest plug events, newest first.
func (d *Devices) Status() DevicesStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	st := DevicesStatus{Devices: append([]Device{}, d.list...)}
	for i := len(d.events) - 1; i >= 0; i-- {
		st.Events = append(st.Events, d.events[i])
	}
	return st
}

// HasScreen reports whether a screen is in the list.
func HasScreen(all []Device) bool {
	for _, x := range all {
		if x.Kind == "screen" {
			return true
		}
	}
	return false
}

// Snapshot lists what is plugged in, reading /sys and /proc under root.
func Snapshot(root string) []Device {
	p := func(s string) string { return filepath.Join(root, s) }
	var out []Device
	// Screens: the HDMI connectors that see a screen.
	conns, _ := filepath.Glob(p("sys/class/drm/card*-HDMI-A-*"))
	sort.Strings(conns)
	for _, c := range conns {
		st, err := os.ReadFile(filepath.Join(c, "status"))
		if err != nil || strings.TrimSpace(string(st)) != "connected" {
			continue
		}
		n := c[strings.LastIndex(c, "-")+1:]
		num, _ := strconv.Atoi(n)
		d := Device{Kind: "screen", Name: fmt.Sprintf("Screen on HDMI %d", num-1)}
		if m, err := os.ReadFile(filepath.Join(c, "modes")); err == nil {
			d.Detail = strings.TrimSpace(firstLine(string(m)))
		}
		out = append(out, d)
	}
	out = append(out, inputDevices(p("proc/bus/input/devices"))...)
	// Sound: the jack is always there; USB speakers come and go.
	if f, err := os.Open(p("proc/asound/cards")); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			c, ok := parseCardLine(sc.Text())
			switch {
			case !ok:
			case c.Kind == "jack":
				out = append(out, Device{Kind: "sound", Name: "Audio jack", Detail: "built in; the Pi can't tell whether a speaker is plugged into it"})
			case c.Kind == "usb":
				out = append(out, Device{Kind: "sound", Name: "USB speaker: " + c.Name})
			}
		}
		f.Close()
	}
	// WiFi sticks: a wireless interface that is not the built-in one.
	ifs, _ := filepath.Glob(p("sys/class/net/*"))
	for _, i := range ifs {
		if _, err := os.Stat(filepath.Join(i, "wireless")); err != nil {
			continue
		}
		drv := ""
		if l, err := os.Readlink(filepath.Join(i, "device/driver")); err == nil {
			drv = filepath.Base(l)
		}
		if drv == "brcmfmac" {
			continue
		}
		out = append(out, Device{Kind: "wifi", Name: "USB WiFi stick (" + filepath.Base(i) + ")", Detail: drv})
	}
	out = append(out, usbDevices(p("sys/bus/usb/devices"))...)
	return out
}

// inputDevices reads /proc/bus/input/devices: keyboards, mice, touch screens.
func inputDevices(path string) []Device {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Device
	for _, block := range strings.Split(string(b), "\n\n") {
		var name, handlers, keyBits, abs string
		for _, l := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(l, "N: Name="):
				name = strings.Trim(strings.TrimPrefix(l, "N: Name="), `"`)
			case strings.HasPrefix(l, "H: Handlers="):
				handlers = " " + strings.TrimPrefix(l, "H: Handlers=") + " "
			case strings.HasPrefix(l, "B: KEY="):
				keyBits = strings.TrimPrefix(l, "B: KEY=")
			case strings.HasPrefix(l, "B: ABS="):
				abs = strings.TrimPrefix(l, "B: ABS=")
			}
		}
		if name == "" || strings.HasPrefix(name, "vc4-hdmi") {
			continue
		}
		kind := ""
		switch {
		case strings.Contains(handlers, " mouse") && abs != "" && strings.Contains(strings.ToLower(name), "touch"):
			kind = "touch"
		case strings.Contains(handlers, " mouse"):
			kind = "mouse"
		case strings.Contains(handlers, " kbd ") && countBits(keyBits) >= 30:
			kind = "keyboard"
		}
		if kind == "" {
			continue
		}
		// One keyboard often shows as several devices (its media keys): list it once.
		base := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, " Keyboard"), " Mouse"), " Consumer Control"))
		if seen[kind+base] {
			continue
		}
		seen[kind+base] = true
		label := map[string]string{"keyboard": "Keyboard", "mouse": "Mouse", "touch": "Touch screen"}[kind]
		out = append(out, Device{Kind: kind, Name: label + ": " + name})
	}
	return out
}

func countBits(hexWords string) int {
	n := 0
	for _, w := range strings.Fields(hexWords) {
		v, err := strconv.ParseUint(w, 16, 64)
		if err == nil {
			n += bits.OnesCount64(v)
		}
	}
	return n
}

// usbDevices lists USB devices that are not hubs and not already listed as
// keyboards, mice, sound or network.
func usbDevices(dir string) []Device {
	ds, _ := filepath.Glob(filepath.Join(dir, "*"))
	sort.Strings(ds)
	var out []Device
	rd := func(d, f string) string {
		b, _ := os.ReadFile(filepath.Join(d, f))
		return strings.TrimSpace(string(b))
	}
	for _, d := range ds {
		vid, pid := rd(d, "idVendor"), rd(d, "idProduct")
		if vid == "" || rd(d, "bDeviceClass") == "09" || vid == "1d6b" {
			continue
		}
		classes := map[string]bool{}
		hasNet := false
		ifs, _ := filepath.Glob(d + ":*")
		for _, i := range ifs {
			classes[rd(i, "bInterfaceClass")] = true
			if _, err := os.Stat(filepath.Join(i, "net")); err == nil {
				hasNet = true
			}
		}
		name := rd(d, "product")
		if m := rd(d, "manufacturer"); m != "" && name != "" && !strings.Contains(name, m) {
			name = m + " " + name
		}
		if name == "" {
			name = "USB device " + vid + ":" + pid
		}
		switch {
		case vid == "1a86" && pid == "5512":
			out = append(out, Device{Kind: "lora", Name: "LoRa radio stick", Detail: "Meshstick (CH341)"})
		case vid == "1546":
			out = append(out, Device{Kind: "gps", Name: "GPS: " + name})
		case hasNet || classes["03"] || classes["01"]:
			// listed above as WiFi, keyboard, mouse or sound
		case classes["08"]:
			out = append(out, Device{Kind: "storage", Name: "USB drive: " + name})
		default:
			out = append(out, Device{Kind: "usb", Name: name, Detail: vid + ":" + pid})
		}
	}
	return out
}
