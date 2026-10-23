package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	mrand "math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/hub"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/ledger/sqlstore"
	"github.com/johnusher/batnet/internal/meshsync"
	"github.com/johnusher/batnet/internal/meshtastic"
	"github.com/johnusher/batnet/internal/mock"
	"github.com/johnusher/batnet/internal/prio"
	"github.com/johnusher/batnet/internal/seal"
	"github.com/johnusher/batnet/internal/state"
	"github.com/johnusher/batnet/internal/udplink"
	"github.com/johnusher/batnet/internal/web"
)

// hubSettings is data/hub.json: what survives a restart.
type hubSettings struct {
	Mode  string `json:"mode"`
	House string `json:"house"`
	Log   bool   `json:"log"` // logging the radio; stays on across restarts
	// HelloMinutes is the time between the log's hellos: 0 or unset, every 5
	// minutes; negative, none (listen only).
	HelloMinutes int `json:"helloMinutes,omitempty"`
	// Where the hub stands, for the panel's map; never sent by radio.
	Where *hub.Where `json:"where,omitempty"`
	// The power-cut kit (plan 5a).
	WiFi          hub.WiFiSettings  `json:"wifi"`
	Sound         hub.SoundSettings `json:"sound"`
	Light         hub.LightSettings `json:"light"`
	Power         hub.PowerSettings `json:"power"`
	TranslateTo   string            `json:"translateTo,omitempty"`
	TranslateAuto bool              `json:"translateAuto,omitempty"`
}

// The real mesh: bat0 in 172.27.0.0/16, broadcasts on UDP 4200, repairs on HTTP 4201.
const (
	meshUDPPort  = 4200
	meshHTTPPort = 4201
	meshBcast    = "172.27.255.255:4200"
	placesPort   = 4202 // the radios' places hubs share (B70)
)

// pretend houses for mock mode (placeholders, never real people).
var pretend = []struct{ house, person string }{
	{"Miller House", "Anna"}, {"Fern Cottage", "Ben"}, {"Old Mill", "Cleo"}, {"Green Farm", "Dev"},
}

var pretendLines = []string{
	"Water is back on at our end.", "Anyone have a spare 12 V battery?", "Generator running till 8 if you need to charge phones.",
	"Checked on the Bakers, they're fine.", "Bread at the Old Mill tomorrow morning.", "Road by the bridge is blocked by a tree.",
	"Radio says power back by Thursday.", "We have room for two more tonight.", "Soup at ours at 6.", "Who has a chainsaw?",
}

type hubRun struct {
	data, listen, radioAddr, meshBin string
	settings                         hubSettings

	mu      sync.Mutex
	restart bool // switchMode ended this run
	self    string
	mesh    hub.MeshStatus
	houseSt *state.State
	store   ledger.Store
	peers   func() []hub.Peer
}

func runHub(data, listen, radioAddr, mode, house string) error {
	if err := os.MkdirAll(data, 0o700); err != nil {
		return err
	}
	h := &hubRun{data: data, listen: listen, radioAddr: radioAddr, meshBin: "/usr/local/lib/batnet/batnet-mesh"}
	h.settings = hubSettings{Mode: hub.ModeOff, House: "Hub House", Sound: hub.DefaultSound, Light: hub.DefaultLight,
		Power: hub.PowerSettings{Mode: hub.PowerHigh}}
	if b, err := os.ReadFile(filepath.Join(data, "hub.json")); err == nil {
		json.Unmarshal(b, &h.settings)
	}
	if mode != "" {
		h.settings.Mode = mode
	}
	if house != "" {
		h.settings.House = house
	}
	if !hub.ValidMode(h.settings.Mode) {
		h.settings.Mode = hub.ModeOff
	}
	if err := h.save(); err != nil {
		return err
	}
	priv, err := loadNodeKey(filepath.Join(data, "node.key"))
	if err != nil {
		return err
	}
	vkey, err := loadVillageKey(filepath.Join(data, "village.key"))
	if err != nil {
		return err
	}
	hubPIN, err := loadPIN(filepath.Join(data, "hub-pin"))
	if err != nil {
		return err
	}
	adminPIN, err := loadPIN(filepath.Join(data, "admin-pin"))
	if err != nil {
		return err
	}
	wifiPass, err := loadWiFiPassword(filepath.Join(data, "wifi-password"))
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// The house log: on the SD card, except in mock mode, where nothing is kept.
	var store ledger.Store
	if h.settings.Mode == hub.ModeMock {
		store = ledger.NewMemStore()
	} else {
		if store, err = sqlstore.Open(filepath.Join(data, "house.db")); err != nil {
			return err
		}
	}
	defer store.Close()
	clock := hlc.New(func() int64 { return time.Now().UnixMilli() })
	author, err := ledger.NewAuthor(priv, clock, store)
	if err != nil {
		return err
	}
	st := state.New()
	all, err := store.All()
	if err != nil {
		return err
	}
	for _, e := range all {
		st.Apply(e)
	}
	id := author.ID()
	h.store, h.houseSt, h.self = store, st, hex.EncodeToString(id[:])
	var syncer atomic.Pointer[meshsync.Syncer] // set when the mesh is up
	srv := web.New(web.Config{House: h.settings.House, Author: author, State: st,
		Publish: func(e *ledger.Entry) {
			if sy := syncer.Load(); sy != nil {
				sy.Publish(e)
			}
		}})
	if len(all) == 0 {
		if _, err := srv.Bootstrap("Admin", adminPIN, []string{"Medical", "Food"}); err != nil {
			return err
		}
	}

	sealer, err := seal.NewSealer(vkey, seal.PurposeMesh)
	if err != nil {
		return err
	}
	newSyncer := func(link meshsync.Link) *meshsync.Syncer {
		return meshsync.New(meshsync.Config{Link: link, Store: store, Clock: clock, Sealer: sealer, Heartbeat: 5 * time.Second,
			OnEntry: srv.OnEntry, Logf: log.Printf})
	}
	if h.settings.Mode != hub.ModeReal {
		if _, err := os.Stat("/sys/class/net/bat0"); err == nil {
			h.meshCmd("down") // left from a run in real mode
		}
	}
	switch h.settings.Mode {
	case hub.ModeOff:
		h.setMesh(hub.MeshStatus{OK: true, State: "Off", Detail: "This hub is on its own"})
	case hub.ModeMock:
		m := mock.NewMesh(time.Now().UnixNano())
		sy := newSyncer(m.Add("hub"))
		syncer.Store(sy)
		go sy.Run(ctx)
		if err := h.startMock(ctx, m, vkey); err != nil {
			return err
		}
	case hub.ModeReal:
		go h.runReal(ctx, priv, newSyncer, &syncer)
	}

	radio := meshtastic.New(radioAddr)
	var kept sync.WaitGroup // the last saves on the way out
	kept.Add(2)
	go func() {
		defer kept.Done()
		keepFile(ctx, filepath.Join(data, hub.LinksFile), 5*time.Minute, func(b []byte) error {
			var m map[string]meshtastic.SavedLink
			err := json.Unmarshal(b, &m)
			radio.LoadLinks(m)
			return err
		}, func() ([]byte, error) { return json.Marshal(radio.SavedLinks()) })
	}()
	go func() { // the messages, which the panel showed only since the last start (L16)
		defer kept.Done()
		keepFile(ctx, filepath.Join(data, "texts.json"), time.Minute, func(b []byte) error {
			var ts []meshtastic.Text
			err := json.Unmarshal(b, &ts)
			radio.LoadTexts(ts)
			return err
		}, func() ([]byte, error) { return json.Marshal(radio.SavedTexts()) })
	}()
	linksSaved := make(chan struct{})
	go func() { kept.Wait(); close(linksSaved) }()
	logbook := hub.NewLogbook(filepath.Join(data, "logs"), radio, func(on bool) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.settings.Log = on
		return h.save()
	})
	radio.OnEvent(logbook.Event)
	traffic := hub.NewTraffic(func() string { return radio.Status().Preset })
	radio.OnEvent(traffic.Event)
	setHello := func(m int) {
		switch {
		case m < 0:
			logbook.SetHelloEvery(0)
		case m > 0:
			logbook.SetHelloEvery(time.Duration(m) * time.Minute)
		default:
			logbook.SetHelloEvery(hub.HelloEvery)
		}
	}
	setHello(h.settings.HelloMinutes)
	settings := func() hub.HubSettings {
		h.mu.Lock()
		defer h.mu.Unlock()
		return hub.HubSettings{HelloMinutes: h.settings.HelloMinutes, Where: h.settings.Where,
			TranslateTo: h.settings.TranslateTo, TranslateAuto: h.settings.TranslateAuto}
	}
	saveSettings := func(hs hub.HubSettings) error {
		h.mu.Lock()
		h.settings.HelloMinutes, h.settings.Where = hs.HelloMinutes, hs.Where
		h.settings.TranslateTo, h.settings.TranslateAuto = hs.TranslateTo, hs.TranslateAuto
		err := h.save()
		h.mu.Unlock()
		setHello(hs.HelloMinutes)
		return err
	}
	if h.settings.Log {
		if err := logbook.Start(); err != nil {
			log.Printf("log: %v", err)
		}
	}
	go logbook.Run(ctx, data)
	go radio.Run(ctx)
	go meshtastic.WatchJournal(ctx, "meshtasticd.service", meshtastic.JournalHandlers{
		Unreadable: func(u meshtastic.Unreadable) { logbook.Unreadable(u); traffic.Garbled(u) },
		Relayed:    func(r string) { logbook.Note("relayed", map[string]any{"relay": r}); traffic.Relayed(r) },
		Busy:       traffic.Busy,
	})

	kit := h.startKit(ctx, radio, id, wifiPass)
	placesSealer, err := seal.NewSealer(vkey, seal.PurposePlaces)
	if err != nil {
		return err
	}
	places := hub.NewPlaces(hub.PlacesConfig{Hub: h.self, Sealer: placesSealer, Port: placesPort,
		Nodes: func() []meshtastic.Node { return radio.Status().Nodes },
		Name: func() string { // its radio's name, which the map shows; the house name is often the default
			for _, n := range radio.Status().Nodes {
				if n.Self && n.LongName != "" {
					return n.LongName
				}
			}
			return h.settings.House
		}})
	go places.Run(ctx)
	maps := hub.NewMaps(hub.MapsConfig{Dir: filepath.Join(data, "maps"), Where: func() *hub.Where { return settings().Where }})
	go maps.Run(ctx)

	panel := hub.New(hub.Config{House: h.settings.House, DataDir: data, Radio: radio, Mesh: h.meshStatus,
		SetMode: func(m string) error { return h.switchMode(cancel, m) }, HubPIN: hubPIN, AdminPIN: adminPIN, Log: logbook, Traffic: traffic,
		Settings: settings, SaveSettings: saveSettings,
		WiFi: kit.wifi, Sound: kit.sound, Light: kit.light, Power: kit.power, Devices: kit.devices, Translator: kit.translator,
		Notifier: kit.notifier, Maps: maps, Places: places, Run: hub.Exec})
	mux := http.NewServeMux()
	mux.Handle("/hub/", panel.Handler())
	mux.Handle("/", srv.Handler())
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	// No WriteTimeout: the app's live streams stay open for hours. Idle
	// connections go after two minutes so phones that wander off do not hold
	// the Pi's sockets.
	hs := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	go hs.Serve(ln)
	log.Printf("batnet hub %q, mesh %s, panel on http://%s/hub/, radio %s", h.settings.House, h.settings.Mode, listen, radioAddr)
	go watchdog(ctx, ln.Addr().String())

	<-ctx.Done()
	h.mu.Lock()
	restart := h.restart
	h.mu.Unlock()
	if !restart { // a mode switch re-runs batnetd in place: systemd must not see it stop
		sdNotify("STOPPING=1")
	}
	shut, done := context.WithTimeout(context.Background(), 3*time.Second)
	defer done()
	hs.Shutdown(shut)
	select { // the last saves, or up to five minutes of readings and a minute of messages are lost
	case <-linksSaved:
	case <-time.After(2 * time.Second):
	}
	if h.settings.Mode == hub.ModeReal {
		h.meshCmd("down")
	}
	if restart {
		return errRestart
	}
	return nil
}

var errRestart = errors.New("restart in the new mode")

func (h *hubRun) save() error {
	b, _ := json.MarshalIndent(h.settings, "", "  ")
	return writeFileAtomic(filepath.Join(h.data, "hub.json"), append(b, '\n'), 0o600)
}

// switchMode saves the new mode and ends this run; main restarts batnetd in it.
func (h *hubRun) switchMode(stop context.CancelFunc, m string) error {
	h.mu.Lock()
	h.settings.Mode = m
	h.restart = true
	err := h.save()
	h.mu.Unlock()
	if err != nil {
		return err
	}
	time.AfterFunc(300*time.Millisecond, stop) // let the answer reach the page first
	return nil
}

func (h *hubRun) setMesh(m hub.MeshStatus) {
	h.mu.Lock()
	m.Mode = h.settings.Mode
	h.mesh = m
	h.mu.Unlock()
}

func (h *hubRun) meshStatus() hub.MeshStatus {
	h.mu.Lock()
	m, peers := h.mesh, h.peers
	h.mu.Unlock()
	if peers != nil {
		m.Peers = peers()
	}
	if all, err := h.store.All(); err == nil {
		m.Entries = len(all)
	}
	return m
}

// startMock runs the pretend houses: each its own node with its own log on
// the in-process mesh, linked to the hub and to its neighbour, posting now and then.
func (h *hubRun) startMock(ctx context.Context, m *mock.Mesh, vkey seal.VillageKey) error {
	type house struct {
		name, person, account string
		srv                   *web.Server
		store                 *ledger.MemStore
	}
	var houses []*house
	for i, p := range pretend {
		hs := &house{name: p.house, person: p.person, store: ledger.NewMemStore()}
		_, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		clock := hlc.New(func() int64 { return time.Now().UnixMilli() })
		author, err := ledger.NewAuthor(priv, clock, hs.store)
		if err != nil {
			return err
		}
		sealer, err := seal.NewSealer(vkey, seal.PurposeMesh)
		if err != nil {
			return err
		}
		addr := fmt.Sprintf("pretend-%d", i)
		var sy *meshsync.Syncer
		hs.srv = web.New(web.Config{House: p.house, Author: author, State: state.New(),
			Publish: func(e *ledger.Entry) { sy.Publish(e) }})
		sy = meshsync.New(meshsync.Config{Link: m.Add(addr), Store: hs.store, Clock: clock, Sealer: sealer,
			Heartbeat: 5 * time.Second, OnEntry: hs.srv.OnEntry})
		go sy.Run(ctx)
		if hs.account, err = hs.srv.Bootstrap(p.person, "135790", nil); err != nil { // placeholder PIN, pretend house only
			return err
		}
		hs.account = state.CompactAccount(hs.account, author.ID())
		if i == 0 {
			m.Connect("hub", addr)
		} else {
			m.Connect(fmt.Sprintf("pretend-%d", i-1), addr)
		}
		houses = append(houses, hs)
	}
	m.Connect("hub", fmt.Sprintf("pretend-%d", len(pretend)-1)) // a ring: two ways to every house
	h.mu.Lock()
	h.peers = func() []hub.Peer {
		var out []hub.Peer
		for _, hs := range houses {
			all, _ := hs.store.All()
			out = append(out, hub.Peer{Name: hs.name, Detail: fmt.Sprintf("%s, %d entries", hs.person, len(all))})
		}
		return out
	}
	h.mu.Unlock()
	h.setMesh(hub.MeshStatus{OK: true, State: "Mock", Detail: fmt.Sprintf("%d pretend houses in a ring with this hub", len(houses))})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(20+mrand.IntN(40)) * time.Second):
			}
			hs := houses[mrand.IntN(len(houses))]
			hs.srv.Write(ledger.TypeMsgPost, prio.Normal, map[string]any{"from": hs.account, "audience": "everyone",
				"text": pretendLines[mrand.IntN(len(pretendLines))]})
		}
	}()
	return nil
}

// runReal brings up batman-adv on the USB WiFi stick (waiting for it to be
// plugged in) and syncs the log over bat0.
func (h *hubRun) runReal(ctx context.Context, priv ed25519.PrivateKey, newSyncer func(meshsync.Link) *meshsync.Syncer, out *atomic.Pointer[meshsync.Syncer]) {
	id := ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
	a, b := id[0], id[1]
	if b == 0 || b == 255 {
		b = 1
	}
	addr := fmt.Sprintf("172.27.%d.%d", a, b)
	for ctx.Err() == nil {
		stick := hub.MeshStick()
		if stick == "" {
			h.setMesh(hub.MeshStatus{State: "Waiting for the USB WiFi stick", Detail: "Plug it in; the mesh starts by itself"})
			sleep(ctx, 5*time.Second)
			continue
		}
		if outp, err := h.meshCmd("up", addr); err != nil {
			h.setMesh(hub.MeshStatus{State: "The mesh did not start", Detail: outp, Iface: stick})
			sleep(ctx, 15*time.Second)
			continue
		}
		link, err := udplink.Open(udplink.Config{Addr: addr, BroadcastTo: meshBcast, UDPPort: meshUDPPort, HTTPPort: meshHTTPPort})
		if err != nil {
			h.setMesh(hub.MeshStatus{State: "The mesh is up but sync could not start", Detail: err.Error(), Iface: stick})
			sleep(ctx, 15*time.Second)
			continue
		}
		sy := newSyncer(link)
		out.Store(sy)
		h.mu.Lock()
		h.peers = func() []hub.Peer { return h.batmanPeers() }
		h.mu.Unlock()
		h.setMesh(hub.MeshStatus{OK: true, State: "Real", Detail: "batman-adv on " + stick, Iface: stick, Addr: addr})
		sy.Run(ctx)
		out.Store(nil)
		link.Close()
	}
}

// batmanPeers lists batman-adv's neighbours and the houses the log knows.
func (h *hubRun) batmanPeers() []hub.Peer {
	var out []hub.Peer
	if o, err := h.meshCmd("status"); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(o), "\n") {
			f := strings.Fields(line)
			if len(f) >= 3 {
				out = append(out, hub.Peer{Name: "Neighbour " + f[1], Detail: "on " + f[0] + ", last seen " + f[2]})
			}
		}
	}
	for _, d := range h.houseSt.Directory() {
		if d.Node == h.self {
			continue
		}
		out = append(out, hub.Peer{Name: d.Name, Detail: fmt.Sprintf("%d people", len(d.People))})
	}
	return out
}

func (h *hubRun) meshCmd(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, h.meshBin, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// ---- keys and PINs in the data folder ----

// writeFileAtomic writes the file whole or not at all, and makes sure it is on
// the card before it takes the old one's place: a power cut must not leave a
// half-written key or settings file (L23 is what a cut does to open files).
func writeFileAtomic(path string, b []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadHex(path string, n int, fresh func() ([]byte, error)) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		k, err := hex.DecodeString(strings.TrimSpace(string(b)))
		if err != nil || len(k) != n {
			return nil, fmt.Errorf("%s is damaged", path)
		}
		return k, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	k, err := fresh()
	if err != nil {
		return nil, err
	}
	return k, writeFileAtomic(path, []byte(hex.EncodeToString(k)+"\n"), 0o600)
}

func loadNodeKey(path string) (ed25519.PrivateKey, error) {
	seed, err := loadHex(path, ed25519.SeedSize, func() ([]byte, error) {
		s := make([]byte, ed25519.SeedSize)
		_, err := rand.Read(s)
		return s, err
	})
	if err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// loadVillageKey reads the village key; a first start makes a new village.
// Copy this file to another Pi to put it in the same village (until the QR join of plan 5).
func loadVillageKey(path string) (seal.VillageKey, error) {
	var k seal.VillageKey
	b, err := loadHex(path, len(k), func() ([]byte, error) {
		nk, err := seal.NewVillageKey()
		return nk[:], err
	})
	copy(k[:], b)
	return k, err
}

func loadPIN(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(b)), nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	pin := fmt.Sprintf("%06d", n.Int64())
	return pin, writeFileAtomic(path, []byte(pin+"\n"), 0o600)
}

// ---- systemd: ready, watchdog, stopping ----

// sdNotify tells systemd about this service (Type=notify); a no-op outside systemd.
func sdNotify(msg string) bool {
	sock := os.Getenv("NOTIFY_SOCKET")
	if sock == "" {
		return false
	}
	if sock[0] == '@' {
		sock = "\x00" + sock[1:]
	}
	c, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: sock, Net: "unixgram"})
	if err != nil {
		return false
	}
	defer c.Close()
	_, err = c.Write([]byte(msg))
	return err == nil
}

// watchdog says READY once the panel answers, then pets systemd's watchdog
// every 10 s, but only while the panel still answers: a hung server stops
// the petting and systemd restarts batnetd.
func watchdog(ctx context.Context, addr string) {
	host, port, _ := net.SplitHostPort(addr)
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	url := "http://" + net.JoinHostPort(host, port) + "/hub/api/ping"
	client := &http.Client{Timeout: 5 * time.Second}
	alive := func() bool {
		res, err := client.Get(url)
		if err != nil {
			return false
		}
		res.Body.Close()
		return res.StatusCode == http.StatusOK
	}
	for !alive() {
		sleep(ctx, 200*time.Millisecond)
		if ctx.Err() != nil {
			return
		}
	}
	sdNotify("READY=1")
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if alive() {
				sdNotify("WATCHDOG=1")
			}
		}
	}
}

// keepFile keeps something of the radio's across restarts: loaded once,
// saved every so often when it changed, and on the way out.
func keepFile(ctx context.Context, path string, every time.Duration, load func([]byte) error, save func() ([]byte, error)) {
	if b, err := os.ReadFile(path); err == nil {
		if err := load(b); err != nil {
			log.Printf("%s: %v", filepath.Base(path), err)
		}
	}
	var last []byte
	write := func() {
		b, err := save()
		if err != nil || bytes.Equal(b, last) {
			return // unchanged: spare the SD card
		}
		if err := writeFileAtomic(path, append(b, '\n'), 0o600); err != nil {
			log.Printf("%s: %v", filepath.Base(path), err)
			return
		}
		last = b
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			write()
			return
		case <-t.C:
			write()
		}
	}
}
