package hub

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// System is the hub machine's health, read from /proc and /sys (zero values
// where the platform does not have them, e.g. on a laptop).
type System struct {
	Hostname   string    `json:"hostname"`
	Model      string    `json:"model"`
	Uptime     float64   `json:"uptime"` // seconds
	Load1      float64   `json:"load1"`
	TempC      float64   `json:"tempC"`
	MemTotalMB int       `json:"memTotalMB"`
	MemAvailMB int       `json:"memAvailMB"`
	DiskFreeGB float64   `json:"diskFreeGB"`
	DiskSizeGB float64   `json:"diskSizeGB"`
	Interfaces []Iface   `json:"interfaces"`
	Meshstick  bool      `json:"meshstick"` // the CH341 of an Elecrow Meshstick is on USB
	Time       time.Time `json:"time"`
}

// Iface is one network interface.
type Iface struct {
	Name   string   `json:"name"`
	Up     bool     `json:"up"`
	WiFi   bool     `json:"wifi"`
	Driver string   `json:"driver,omitempty"`
	Addrs  []string `json:"addrs"`
}

func readSystem(dataDir string) System {
	s := System{Time: time.Now()}
	s.Hostname, _ = os.Hostname()
	if b, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		s.Model = strings.TrimRight(string(b), "\x00\n")
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		if f := strings.Fields(string(b)); len(f) > 0 {
			s.Uptime, _ = strconv.ParseFloat(f[0], 64)
		}
	}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		if f := strings.Fields(string(b)); len(f) > 0 {
			s.Load1, _ = strconv.ParseFloat(f[0], 64)
		}
	}
	if b, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			s.TempC = float64(v) / 1000
		}
	}
	if f, err := os.Open("/proc/meminfo"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fs := strings.Fields(sc.Text())
			if len(fs) < 2 {
				continue
			}
			kb, _ := strconv.Atoi(fs[1])
			switch fs[0] {
			case "MemTotal:":
				s.MemTotalMB = kb / 1024
			case "MemAvailable:":
				s.MemAvailMB = kb / 1024
			}
		}
		f.Close()
	}
	var fs syscall.Statfs_t
	if dataDir == "" {
		dataDir = "/"
	}
	if syscall.Statfs(dataDir, &fs) == nil {
		s.DiskFreeGB = float64(fs.Bavail) * float64(fs.Bsize) / 1e9
		s.DiskSizeGB = float64(fs.Blocks) * float64(fs.Bsize) / 1e9
	}
	s.Interfaces = interfaces()
	s.Meshstick = usbPresent("1a86", "5512")
	return s
}

func interfaces() []Iface {
	ifs, _ := net.Interfaces()
	var out []Iface
	for _, i := range ifs {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		it := Iface{Name: i.Name, Up: i.Flags&net.FlagUp != 0}
		if _, err := os.Stat(filepath.Join("/sys/class/net", i.Name, "wireless")); err == nil {
			it.WiFi = true
		}
		if l, err := os.Readlink(filepath.Join("/sys/class/net", i.Name, "device/driver")); err == nil {
			it.Driver = filepath.Base(l)
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
				it.Addrs = append(it.Addrs, ipn.String())
			}
		}
		out = append(out, it)
	}
	return out
}

func usbPresent(vendor, product string) bool {
	ds, _ := filepath.Glob("/sys/bus/usb/devices/*/idVendor")
	for _, d := range ds {
		v, _ := os.ReadFile(d)
		p, _ := os.ReadFile(filepath.Join(filepath.Dir(d), "idProduct"))
		if strings.TrimSpace(string(v)) == vendor && strings.TrimSpace(string(p)) == product {
			return true
		}
	}
	return false
}

// MeshStick finds a USB WiFi adapter for batman-adv: a wireless interface
// that is not the Pi's built-in one (brcmfmac).
func MeshStick() string {
	for _, i := range interfaces() {
		if i.WiFi && i.Driver != "" && i.Driver != "brcmfmac" {
			return i.Name
		}
	}
	return ""
}
