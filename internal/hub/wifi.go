package hub

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The hub's own WiFi (decision 0028): a NetworkManager connection that is
// never started automatically, so every boot joins the home WiFi.
const (
	OwnWiFiName = "batnet-own-wifi"
	OwnWiFiAddr = "10.42.0.1"
	ownWiFiNet  = "10.42.0.0/24"
)

// WiFiSettings are the WiFi settings kept in hub.json.
type WiFiSettings struct {
	// NoFallback turns off the automatic start of the hub's own WiFi.
	NoFallback bool `json:"noFallback,omitempty"`
	// AfterMinutes without home WiFi start the hub's own (0: 3 minutes).
	AfterMinutes int `json:"afterMinutes,omitempty"`
	// Neighbours lets anyone join the hub's own WiFi, without a password.
	Neighbours bool `json:"neighbours,omitempty"`
	// SSID is the hub's own WiFi name ("": batnet- and four letters of the node).
	SSID string `json:"ssid,omitempty"`
}

func (s WiFiSettings) after() time.Duration {
	if s.AfterMinutes <= 0 {
		return 3 * time.Minute
	}
	return time.Duration(s.AfterMinutes) * time.Minute
}

// SavedWiFi is a WiFi network the hub knows.
type SavedWiFi struct {
	Name     string `json:"name"`
	SSID     string `json:"ssid"`
	Priority int    `json:"priority"`
	Default  bool   `json:"default"`
	Active   bool   `json:"active"`
}

// NearbyWiFi is a network in range.
type NearbyWiFi struct {
	SSID     string `json:"ssid"`
	Signal   int    `json:"signal"`
	Security string `json:"security"`
	Saved    bool   `json:"saved"`
	InUse    bool   `json:"inUse"`
}

// WiFiStatus is what the panel shows.
type WiFiStatus struct {
	Available bool   `json:"available"` // NetworkManager and the built-in WiFi are there
	Iface     string `json:"iface"`
	// Mode is home (on a saved network), own (the hub's own WiFi), none, or
	// switching.
	Mode       string `json:"mode"`
	Connection string `json:"connection,omitempty"`
	SSID       string `json:"ssid,omitempty"`
	Signal     int    `json:"signal,omitempty"`
	// The hub's own WiFi.
	OwnSSID   string `json:"ownSSID"`
	Password  string `json:"password,omitempty"` // only for the hub's own screen
	Open      bool   `json:"open"`
	Addr      string `json:"addr"`
	Clients   int    `json:"clients"`
	StartedBy string `json:"startedBy,omitempty"` // you, auto, button
	// The fallback: on, after how long, and seconds left before it starts the
	// hub's own WiFi (-1: not counting).
	Auto         bool        `json:"auto"`
	AfterMinutes int         `json:"afterMinutes"`
	FallbackIn   int         `json:"fallbackIn"`
	Waiting      bool        `json:"waiting"` // the owner chose home WiFi; the fallback waits until it is seen
	Saved        []SavedWiFi `json:"saved"`
	Error        string      `json:"error,omitempty"`
	Note         string      `json:"note,omitempty"`
}

// WiFi runs the built-in WiFi: home networks, the hub's own, the fallback.
type WiFi struct {
	run      Runner
	iface    string
	password string
	save     func(WiFiSettings) error
	now      func() time.Time
	// tryHomeEvery and homeWait time the return to the home WiFi after an
	// automatic start.
	tryHomeEvery, homeWait, step time.Duration

	op sync.Mutex // one change at a time

	mu        sync.Mutex
	set       WiFiSettings
	defSSID   string
	avail     bool
	mode      string
	conn      string
	ssid      string
	signal    int
	clients   int
	startedBy string
	ownSince  time.Time
	lastHome  time.Time // when the home WiFi was last up (or the start)
	lastBusy  time.Time // when a device was last on the hub's own WiFi
	waiting   bool
	retrying  bool // tryHome is under way
	saved     []SavedWiFi
	savedAt   time.Time
	err, note string
	onChange  func(mode string)

	startedFile string
}

func (w *WiFi) readStarted() string { // caller holds mu
	if w.startedFile != "" {
		if b, err := os.ReadFile(w.startedFile); err == nil {
			if s := strings.TrimSpace(string(b)); s == "auto" || s == "you" || s == "button" {
				return s
			}
		}
	}
	return "you"
}

func (w *WiFi) writeStarted(by string) { // caller holds mu
	if w.startedFile == "" {
		return
	}
	if by == "" {
		os.Remove(w.startedFile)
		return
	}
	os.WriteFile(w.startedFile, []byte(by+"\n"), 0o600)
}

// WiFiConfig wires a WiFi.
type WiFiConfig struct {
	Run         Runner
	Iface       string // "": wlan0
	Password    string // the hub's own WiFi password (8 to 63 characters)
	DefaultSSID string
	Settings    WiFiSettings
	Save        func(WiFiSettings) error
	// OnChange is called when the mode changes (for sounds and the LED).
	OnChange func(mode string)
	// StartedFile keeps who started the hub's own WiFi (you, auto, button)
	// across a restart of batnetd ("": not kept).
	StartedFile string
}

// NewWiFi returns the WiFi manager; Run starts it.
func NewWiFi(c WiFiConfig) *WiFi {
	if c.Iface == "" {
		c.Iface = "wlan0"
	}
	w := &WiFi{run: c.Run, iface: c.Iface, password: c.Password, save: c.Save, set: c.Settings, defSSID: c.DefaultSSID, startedFile: c.StartedFile,
		now: time.Now, tryHomeEvery: 10 * time.Minute, homeWait: 45 * time.Second, step: 3 * time.Second, onChange: c.OnChange, mode: "none"}
	w.lastHome = w.now()
	return w
}

func (w *WiFi) ownSSID() string {
	if w.set.SSID != "" {
		return w.set.SSID
	}
	return w.defSSID
}

// nm runs nmcli in terse mode.
func (w *WiFi) nm(d time.Duration, args ...string) (string, error) {
	return runFor(w.run, d, "nmcli", append([]string{"-t"}, args...)...)
}

// Run keeps the state fresh and runs the fallback until ctx ends.
func (w *WiFi) Run(ctx context.Context) {
	if err := w.ensureOwn(); err != nil {
		w.setErr("The hub's own WiFi could not be set up: " + err.Error())
	}
	for ctx.Err() == nil {
		w.Tick()
		// On the home WiFi a look every 30 s is enough; without it, every 10 s
		// keeps the fallback on time.
		every := 10 * time.Second
		w.mu.Lock()
		if w.mode == "home" {
			every = 30 * time.Second
		}
		w.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-time.After(every):
		}
	}
}

// Tick refreshes the state and takes the fallback's next step.
func (w *WiFi) Tick() {
	if !w.op.TryLock() {
		return // a change is under way
	}
	defer w.op.Unlock()
	w.refresh()
	now := w.now()
	w.mu.Lock()
	mode, auto, waiting, started := w.mode, !w.set.NoFallback, w.waiting, w.startedBy
	due := mode == "none" && auto && !waiting && w.avail && now.Sub(w.lastHome) >= w.set.after()
	tryHome := mode == "own" && started == "auto" && w.clients == 0 && len(w.homeNets()) > 0 &&
		now.Sub(w.lastBusy) >= w.tryHomeEvery && now.Sub(w.ownSince) >= w.tryHomeEvery
	w.mu.Unlock()
	switch {
	case due:
		if err := w.up("auto"); err != nil {
			w.setErr("The hub's own WiFi did not start: " + err.Error())
		}
	case tryHome:
		w.tryHome()
	}
}

// homeNets is the saved networks other than the hub's own (caller holds mu).
func (w *WiFi) homeNets() []SavedWiFi {
	var out []SavedWiFi
	for _, s := range w.saved {
		if s.Name != OwnWiFiName {
			out = append(out, s)
		}
	}
	return out
}

// tryHome stops the hub's own WiFi for a short while; if no home WiFi comes,
// it starts again.
func (w *WiFi) tryHome() {
	w.setNote("Looking for the home WiFi; the hub's own WiFi is back in a minute if it isn't there.")
	// A try is not news: no chime or blink for the WiFi going down and up again.
	w.mu.Lock()
	w.retrying = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.retrying = false
		w.mu.Unlock()
	}()
	w.nm(30*time.Second, "connection", "down", OwnWiFiName)
	tries := 1
	if w.step > 0 {
		tries = int(w.homeWait / w.step)
	}
	for i := 0; i < tries; i++ {
		time.Sleep(w.step)
		w.refresh()
		w.mu.Lock()
		home := w.mode == "home"
		w.mu.Unlock()
		if home {
			w.setNote("Back on the home WiFi by itself.")
			if w.onChange != nil {
				go w.onChange("home")
			}
			return
		}
	}
	if err := w.up("auto"); err != nil {
		w.setErr("The hub's own WiFi did not start again: " + err.Error())
	}
	w.mu.Lock()
	w.lastBusy = w.now()
	w.mu.Unlock()
}

// refresh reads the device's state from NetworkManager.
func (w *WiFi) refresh() {
	out, err := w.nm(10*time.Second, "-f", "GENERAL.STATE,GENERAL.CONNECTION", "device", "show", w.iface)
	w.mu.Lock()
	defer w.mu.Unlock()
	prev := w.mode
	if err != nil {
		w.avail = false
		w.mode = "none"
		return
	}
	w.avail = true
	state, conn := "", ""
	for _, l := range strings.Split(out, "\n") {
		k, v, _ := strings.Cut(l, ":")
		switch k {
		case "GENERAL.STATE":
			state = v
		case "GENERAL.CONNECTION":
			conn = unescape(v)
		}
	}
	connected := strings.HasPrefix(state, "100")
	switch {
	case connected && conn == OwnWiFiName:
		w.mode = "own"
	case connected && conn != "":
		w.mode = "home"
		w.lastHome = w.now()
		w.waiting = false
		if w.startedBy != "" {
			w.writeStarted("")
		}
		w.startedBy = ""
	default:
		w.mode = "none"
	}
	w.conn = conn
	mode := w.mode
	if w.mode == "own" && w.ownSince.IsZero() {
		w.ownSince = w.now()
		if w.startedBy == "" {
			w.startedBy = w.readStarted() // running already when batnetd started
		}
	}
	if w.mode != "own" {
		w.ownSince = time.Time{}
		w.clients = 0
	}
	w.mu.Unlock()
	// what goes with the mode, outside the lock
	ssid, sig, clients := "", 0, 0
	switch mode {
	case "home":
		ssid, sig = w.activeNet()
	case "own":
		clients = w.countClients()
	}
	saved, fresh := []SavedWiFi(nil), false
	if w.now().Sub(w.savedAt) > time.Minute || prev != mode {
		saved, fresh = w.readSaved(), true
	}
	w.mu.Lock()
	w.ssid, w.signal, w.clients = ssid, sig, clients
	if clients > 0 {
		w.lastBusy = w.now()
	}
	if fresh {
		w.saved, w.savedAt = saved, w.now()
	}
	if prev != w.mode && w.onChange != nil && !w.retrying {
		go w.onChange(w.mode)
	}
}

func (w *WiFi) activeNet() (string, int) {
	out, err := w.nm(15*time.Second, "-f", "IN-USE,SSID,SIGNAL", "device", "wifi", "list", "ifname", w.iface, "--rescan", "no")
	if err != nil {
		return "", 0
	}
	for _, l := range strings.Split(out, "\n") {
		f := splitTerse(l)
		if len(f) >= 3 && f[0] == "*" {
			n, _ := strconv.Atoi(f[2])
			return f[1], n
		}
	}
	return "", 0
}

func (w *WiFi) countClients() int {
	out, err := runFor(w.run, 5*time.Second, "/usr/sbin/iw", "dev", w.iface, "station", "dump")
	if err != nil {
		return 0
	}
	n := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "Station ") {
			n++
		}
	}
	return n
}

func (w *WiFi) readSaved() []SavedWiFi {
	out, err := w.nm(10*time.Second, "-f", "NAME,TYPE,AUTOCONNECT-PRIORITY,ACTIVE", "connection", "show")
	if err != nil {
		return nil
	}
	var list []SavedWiFi
	for _, l := range strings.Split(out, "\n") {
		f := splitTerse(l)
		if len(f) < 4 || f[1] != "802-11-wireless" || f[0] == OwnWiFiName {
			continue
		}
		p, _ := strconv.Atoi(f[2])
		s := SavedWiFi{Name: f[0], Priority: p, Active: f[3] == "yes"}
		if o, err := w.nm(10*time.Second, "-g", "802-11-wireless.ssid", "connection", "show", "id", f[0]); err == nil {
			s.SSID = unescape(strings.TrimSpace(o))
		}
		list = append(list, s)
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Priority > list[j].Priority })
	if len(list) > 0 && (len(list) == 1 || list[0].Priority > list[1].Priority) {
		list[0].Default = true
	}
	return list
}

// ensureOwn creates the hub's own WiFi connection, or brings an existing one
// in line with the settings by changing it in place: a running one stays up
// (a restart of batnetd must not drop the devices on it).
func (w *WiFi) ensureOwn() error {
	w.mu.Lock()
	ssid, open := w.ownSSID(), w.set.Neighbours
	w.mu.Unlock()
	if ssid == "" {
		return errors.New("no name for it")
	}
	if cur, err := w.nm(10*time.Second, "-g", "802-11-wireless.ssid", "connection", "show", "id", OwnWiFiName); err == nil {
		key, _ := w.nm(10*time.Second, "-g", "802-11-wireless-security.key-mgmt", "connection", "show", "id", OwnWiFiName)
		if unescape(strings.TrimSpace(cur)) == ssid && (strings.TrimSpace(key) == "") == open {
			return nil
		}
		if _, err := w.nm(15*time.Second, "connection", "modify", "id", OwnWiFiName, "802-11-wireless.ssid", ssid); err != nil {
			return err
		}
		if open {
			_, err = w.nm(15*time.Second, "connection", "modify", "id", OwnWiFiName, "remove", "802-11-wireless-security")
		} else {
			_, err = w.nm(15*time.Second, append([]string{"connection", "modify", "id", OwnWiFiName}, w.security()...)...)
		}
		return err
	}
	args := []string{"connection", "add", "type", "wifi", "ifname", w.iface, "con-name", OwnWiFiName,
		"autoconnect", "no", "ssid", ssid,
		"802-11-wireless.mode", "ap", "802-11-wireless.band", "bg", "802-11-wireless.channel", "6",
		"802-11-wireless.ap-isolation", "yes", "802-11-wireless.powersave", "2",
		"ipv4.method", "shared", "ipv4.addresses", OwnWiFiAddr + "/24", "ipv6.method", "disabled"}
	if !open {
		if len(w.password) < 8 {
			return errors.New("the password is too short")
		}
		args = append(args, w.security()...)
	}
	_, err := w.nm(20*time.Second, args...)
	return err
}

// security is WPA2 with the hub's password.
func (w *WiFi) security() []string {
	return []string{"wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", w.password, "wifi-sec.proto", "rsn",
		"wifi-sec.pairwise", "ccmp", "wifi-sec.group", "ccmp", "wifi-sec.pmf", "1"}
}

// up starts the hub's own WiFi; by is you, auto or button.
func (w *WiFi) up(by string) error {
	w.setMode("switching")
	_, err := w.nm(45*time.Second, "--wait", "40", "connection", "up", OwnWiFiName)
	if err != nil && strings.Contains(err.Error(), "unknown connection") {
		if e := w.ensureOwn(); e == nil {
			_, err = w.nm(45*time.Second, "--wait", "40", "connection", "up", OwnWiFiName)
		}
	}
	w.mu.Lock()
	if err == nil {
		w.startedBy, w.ownSince, w.lastBusy, w.err = by, w.now(), w.now(), ""
		w.note = ""
		w.writeStarted(by)
	}
	w.mu.Unlock()
	w.refresh()
	return err
}

// StartOwn starts the hub's own WiFi (by: you or button).
func (w *WiFi) StartOwn(by string) error {
	w.op.Lock()
	defer w.op.Unlock()
	w.mu.Lock()
	w.waiting = false
	w.mu.Unlock()
	return w.up(by)
}

// BackHome stops the hub's own WiFi and lets the home WiFi connect. The
// fallback then waits until the home WiFi has been seen again.
func (w *WiFi) BackHome() error {
	w.op.Lock()
	defer w.op.Unlock()
	w.setMode("switching")
	_, err := w.nm(30*time.Second, "--wait", "25", "connection", "down", OwnWiFiName)
	if err != nil && !strings.Contains(err.Error(), "not an active connection") {
		w.refresh()
		return err
	}
	w.mu.Lock()
	w.waiting, w.startedBy, w.err = true, "", ""
	w.note = "Back to the home WiFi. If it isn't there, start the hub's own WiFi again here."
	w.mu.Unlock()
	if nets := w.homeNetsLocked(); len(nets) > 0 {
		// Ask for the default one now rather than waiting for NetworkManager.
		w.nm(40*time.Second, "--wait", "30", "connection", "up", "id", nets[0].Name)
	}
	w.refresh()
	return nil
}

func (w *WiFi) homeNetsLocked() []SavedWiFi {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.homeNets()
}

// Toggle is the button's long press: the hub's own WiFi on or off.
func (w *WiFi) Toggle() (on bool, err error) {
	w.mu.Lock()
	own := w.mode == "own"
	w.mu.Unlock()
	if own {
		return false, w.BackHome()
	}
	return true, w.StartOwn("button")
}

// Configure changes the WiFi settings; a change of name or of the password
// rule makes the hub's own WiFi again (restarting it if it runs).
func (w *WiFi) Configure(s WiFiSettings) error {
	if s.SSID != "" && (len(s.SSID) > 32 || strings.TrimSpace(s.SSID) != s.SSID) {
		return errors.New("A WiFi name has at most 32 bytes and no spaces at the ends.")
	}
	if s.AfterMinutes < 0 || s.AfterMinutes > 120 {
		return errors.New("Start it after 1 to 120 minutes.")
	}
	if s.SSID == w.defSSID {
		s.SSID = "" // the default, shown in the form: not a choice
	}
	w.op.Lock()
	defer w.op.Unlock()
	w.mu.Lock()
	old := w.set
	w.set = s
	running := w.mode == "own"
	w.mu.Unlock()
	if w.save != nil {
		if err := w.save(s); err != nil {
			return err
		}
	}
	if old.SSID != s.SSID || old.Neighbours != s.Neighbours {
		if err := w.ensureOwn(); err != nil {
			return err
		}
		if running {
			w.mu.Lock()
			by := w.startedBy
			w.mu.Unlock()
			return w.up(by)
		}
	}
	return nil
}

// Settings returns the WiFi settings.
func (w *WiFi) Settings() WiFiSettings {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.set
}

// Scan lists the networks in range (not while the hub runs its own WiFi).
func (w *WiFi) Scan() ([]NearbyWiFi, error) {
	w.mu.Lock()
	own := w.mode == "own"
	saved := map[string]bool{}
	for _, s := range w.saved {
		saved[s.SSID] = true
	}
	w.mu.Unlock()
	if own {
		return nil, errors.New("The hub can't look for networks while it runs its own WiFi. Go back to home WiFi first.")
	}
	out, err := w.nm(30*time.Second, "-f", "IN-USE,SSID,SIGNAL,SECURITY", "device", "wifi", "list", "ifname", w.iface, "--rescan", "yes")
	if err != nil {
		return nil, err
	}
	best := map[string]NearbyWiFi{}
	for _, l := range strings.Split(out, "\n") {
		f := splitTerse(l)
		if len(f) < 4 || f[1] == "" {
			continue
		}
		n, _ := strconv.Atoi(f[2])
		x := NearbyWiFi{SSID: f[1], Signal: n, Security: f[3], Saved: saved[f[1]], InUse: f[0] == "*"}
		if b, ok := best[x.SSID]; !ok || x.Signal > b.Signal || x.InUse {
			x.InUse = x.InUse || b.InUse
			best[x.SSID] = x
		}
	}
	list := make([]NearbyWiFi, 0, len(best))
	for _, x := range best {
		list = append(list, x)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Signal > list[j].Signal })
	return list, nil
}

// Join connects to a network (saving it); hidden for one that doesn't
// announce its name. A failed join leaves nothing saved.
func (w *WiFi) Join(ssid, password string, hidden bool) error {
	if ssid == "" || len(ssid) > 32 {
		return errors.New("Which network? A WiFi name has 1 to 32 bytes.")
	}
	if password != "" && (len(password) < 8 || len(password) > 63) {
		return errors.New("A WiFi password has 8 to 63 characters.")
	}
	w.op.Lock()
	defer w.op.Unlock()
	before := map[string]bool{}
	for _, s := range w.readSaved() {
		before[s.Name] = true
	}
	w.setMode("switching")
	args := []string{"--wait", "40", "device", "wifi", "connect", ssid}
	if password != "" {
		args = append(args, "password", password)
	}
	args = append(args, "ifname", w.iface)
	if hidden {
		args = append(args, "hidden", "yes")
	}
	_, err := w.nm(50*time.Second, args...)
	if err != nil {
		for _, s := range w.readSaved() {
			if !before[s.Name] {
				w.nm(15*time.Second, "connection", "delete", "id", s.Name)
			}
		}
	}
	w.mu.Lock()
	w.savedAt = time.Time{}
	w.waiting = false
	w.mu.Unlock()
	w.refresh()
	if err != nil {
		return fmt.Errorf("Could not join %s: %s", ssid, strings.TrimPrefix(err.Error(), "Error: "))
	}
	return nil
}

// Forget deletes a saved network.
func (w *WiFi) Forget(name string) error {
	if name == "" || name == OwnWiFiName {
		return errors.New("That network can't be forgotten here.")
	}
	w.op.Lock()
	defer w.op.Unlock()
	if !w.isSaved(name) {
		return errors.New("The hub doesn't know that network.")
	}
	_, err := w.nm(15*time.Second, "connection", "delete", "id", name)
	w.mu.Lock()
	w.savedAt = time.Time{}
	w.mu.Unlock()
	w.refresh()
	return err
}

// MakeDefault gives a saved network the highest priority, so it is joined
// first at boot.
func (w *WiFi) MakeDefault(name string) error {
	w.op.Lock()
	defer w.op.Unlock()
	if !w.isSaved(name) {
		return errors.New("The hub doesn't know that network.")
	}
	for _, s := range w.readSaved() {
		if s.Name != name && s.Priority >= 10 {
			w.nm(15*time.Second, "connection", "modify", "id", s.Name, "connection.autoconnect-priority", "0")
		}
	}
	_, err := w.nm(15*time.Second, "connection", "modify", "id", name, "connection.autoconnect-priority", "10", "connection.autoconnect", "yes")
	w.mu.Lock()
	w.savedAt = time.Time{}
	w.mu.Unlock()
	w.refresh()
	return err
}

func (w *WiFi) isSaved(name string) bool {
	for _, s := range w.readSaved() {
		if s.Name == name {
			return true
		}
	}
	return false
}

func (w *WiFi) setMode(m string) {
	w.mu.Lock()
	w.mode = m
	w.mu.Unlock()
}

func (w *WiFi) setErr(e string) {
	w.mu.Lock()
	w.err = e
	w.mu.Unlock()
}

func (w *WiFi) setNote(n string) {
	w.mu.Lock()
	w.note = n
	w.mu.Unlock()
}

// Status returns the WiFi's state; withSecret adds the hub's own password.
func (w *WiFi) Status(withSecret bool) WiFiStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := WiFiStatus{Available: w.avail, Iface: w.iface, Mode: w.mode, Connection: w.conn, SSID: w.ssid, Signal: w.signal,
		OwnSSID: w.ownSSID(), Open: w.set.Neighbours, Addr: OwnWiFiAddr, Clients: w.clients, StartedBy: w.startedBy,
		Auto: !w.set.NoFallback, AfterMinutes: int(w.set.after() / time.Minute), FallbackIn: -1, Waiting: w.waiting,
		Saved: append([]SavedWiFi{}, w.saved...), Error: w.err, Note: w.note}
	if st.Mode != "home" {
		st.Connection = ""
	}
	if withSecret && !w.set.Neighbours {
		st.Password = w.password
	}
	if w.mode == "none" && st.Auto && !w.waiting && w.avail {
		left := w.set.after() - w.now().Sub(w.lastHome)
		if left < 0 {
			left = 0
		}
		st.FallbackIn = int(left / time.Second)
	}
	return st
}

// OpenToAll reports whether neighbours are let in: the hub's own WiFi, when
// it runs, has no password. Whatever the mode (it may be switching), so a
// request from the hub's own WiFi is never let through unchecked.
func (w *WiFi) OpenToAll() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.set.Neighbours
}

// OnOwnWiFi reports whether a remote address is on the hub's own WiFi.
func OnOwnWiFi(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(host)
	_, n, _ := net.ParseCIDR(ownWiFiNet)
	return ip != nil && n.Contains(ip)
}

// splitTerse splits a line of nmcli -t output: fields by ':', with '\:' and
// '\\' escaped.
func splitTerse(l string) []string {
	var out []string
	var b strings.Builder
	for i := 0; i < len(l); i++ {
		switch c := l[i]; {
		case c == '\\' && i+1 < len(l):
			i++
			b.WriteByte(l[i])
		case c == ':':
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	return append(out, b.String())
}

// unescape undoes nmcli's escapes in a single field.
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
