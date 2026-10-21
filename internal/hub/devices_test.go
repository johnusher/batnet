package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, v := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(v), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const inputFixture = `I: Bus=0000 Vendor=0000 Product=0000 Version=0000
N: Name="vc4-hdmi-0"
H: Handlers=kbd event0
B: KEY=1

I: Bus=0003 Vendor=046d Product=c31c Version=0110
N: Name="Logitech USB Keyboard"
H: Handlers=sysrq kbd leds event2
B: EV=120013
B: KEY=1000000000007 ff9f207ac14057ff febeffdfffefffff fffffffffffffffe

I: Bus=0003 Vendor=046d Product=c31c Version=0110
N: Name="Logitech USB Keyboard Consumer Control"
H: Handlers=kbd event3
B: KEY=1f 0 0 0 0 0 0 0 0 0 0 3 0 0 0 0

I: Bus=0003 Vendor=093a Product=2510 Version=0111
N: Name="PixArt USB Optical Mouse"
H: Handlers=mouse0 event4
B: EV=17
`

func TestDevicesSnapshotAndEvents(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"sys/class/drm/card1-HDMI-A-1/status": "disconnected\n",
		"sys/class/drm/card1-HDMI-A-2/status": "disconnected\n",
		"proc/bus/input/devices":              "",
		"proc/asound/cards":                   " 0 [Headphones     ]: bcm2835_headpho - bcm2835 Headphones\n                      bcm2835 Headphones\n",
		"sys/bus/usb/devices/1-1.3/idVendor":  "1a86\n",
		"sys/bus/usb/devices/1-1.3/idProduct": "5512\n",
		"sys/bus/usb/devices/usb1/idVendor":   "1d6b\n",
	})
	var events []string
	var lastAll []Device
	d := NewDevices(root, func(added, removed, all []Device) {
		for _, x := range added {
			events = append(events, "+"+x.Kind)
		}
		for _, x := range removed {
			events = append(events, "-"+x.Kind)
		}
		lastAll = all
	})
	d.Scan()
	if len(events) != 0 || HasScreen(lastAll) {
		t.Fatalf("first scan: %v", events)
	}
	kinds := func() string {
		var k []string
		for _, x := range d.Status().Devices {
			k = append(k, x.Kind)
		}
		return strings.Join(k, ",")
	}
	if got := kinds(); got != "sound,lora" {
		t.Fatalf("devices %s", got)
	}
	// A screen and a keyboard and mouse are plugged in.
	writeTree(t, root, map[string]string{
		"sys/class/drm/card1-HDMI-A-1/status": "connected\n",
		"sys/class/drm/card1-HDMI-A-1/modes":  "800x480\n640x480\n",
		"proc/bus/input/devices":              inputFixture,
	})
	d.Scan()
	if got := strings.Join(events, ","); got != "+screen,+keyboard,+mouse" {
		t.Fatalf("events %s", got)
	}
	if !HasScreen(lastAll) {
		t.Fatal("no screen")
	}
	st := d.Status()
	if st.Devices[0].Name != "Screen on HDMI 0" || st.Devices[0].Detail != "800x480" {
		t.Fatalf("%+v", st.Devices[0])
	}
	if len(st.Events) != 3 || !st.Events[0].Plugged {
		t.Fatalf("%+v", st.Events)
	}
	// The screen is unplugged.
	writeTree(t, root, map[string]string{"sys/class/drm/card1-HDMI-A-1/status": "disconnected\n"})
	events = nil
	d.Scan()
	if strings.Join(events, ",") != "-screen" || HasScreen(lastAll) {
		t.Fatalf("events %v", events)
	}
}
