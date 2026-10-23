package hub

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
	"github.com/johnusher/batnet/internal/seal"
)

// Places shares the radios' places a hub's radio has heard with the other
// hubs of its village (B70). A radio counts for the map's zone only once the
// hub knows where it is, and a radio says that every few hours at most, so a
// new hub's zone stayed small for days although the hub next door knew the
// places. Each hub broadcasts what its own radio heard over every network it
// is on (the home network, the WiFi mesh), sealed with the village key so
// only the village's hubs read it; never by radio.
type Places struct {
	cfg PlacesConfig

	mu  sync.Mutex
	got map[string]meshtastic.NodePosition // radio ID -> newest place another hub shared
}

// PlacesConfig wires Places to the hub.
type PlacesConfig struct {
	Hub    string        // this hub's node ID, so it skips its own broadcasts
	Name   func() string // names this hub on the places the others take
	Sealer *seal.Sealer
	Nodes  func() []meshtastic.Node // the radios this hub's radio knows; nil: none
	Now    func() time.Time         // nil: time.Now
	// Port is where hubs listen; To where this one sends (nil: the broadcast
	// address of every network it is on); Every how often (0: 10 minutes).
	Port  int
	To    func() []string
	Every time.Duration
}

const (
	maxPlaces   = 1000
	maxPlacesIn = 60000 // bytes in one broadcast
)

type placesMsg struct {
	Hub    string                             `json:"hub"`
	House  string                             `json:"house"`
	Places map[string]meshtastic.NodePosition `json:"places"`
}

// NewPlaces returns a Places; Run starts the sharing.
func NewPlaces(cfg PlacesConfig) *Places {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Every == 0 {
		cfg.Every = 10 * time.Minute
	}
	if cfg.Name == nil {
		cfg.Name = func() string { return "" }
	}
	if cfg.To == nil {
		cfg.To = func() []string { return broadcasts(cfg.Port) }
	}
	return &Places{cfg: cfg, got: map[string]meshtastic.NodePosition{}}
}

// Count is how many radios' places came from other hubs.
func (p *Places) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.got)
}

// Fill gives each radio with no place, or an older one, the newest place
// another hub shared, naming that hub in From. It changes only the copies.
func (p *Places) Fill(nodes []meshtastic.Node) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range nodes {
		n := &nodes[i]
		if g, ok := p.got[n.ID]; ok && !n.Self && (n.Position == nil || g.At.After(n.Position.At)) {
			n.Position = &g
		}
	}
}

// Run listens for the other hubs' places and sends this hub's until ctx ends.
func (p *Places) Run(ctx context.Context) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: p.cfg.Port})
	if err != nil {
		log.Printf("places: %v", err)
		return
	}
	go func() { <-ctx.Done(); conn.Close() }()
	go func() {
		buf := make([]byte, maxPlacesIn+1)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n <= maxPlacesIn {
				p.receive(buf[:n])
			}
		}
	}()
	wait := min(p.cfg.Every, 30*time.Second) // the radio's node list fills in the first seconds
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = p.cfg.Every
		box, err := p.box()
		if err != nil || box == nil {
			continue
		}
		for _, to := range p.cfg.To() {
			if a, err := net.ResolveUDPAddr("udp4", to); err == nil {
				conn.WriteToUDP(box, a) // a network that is down is tried again next time
			}
		}
	}
}

// box seals the places this hub's own radio heard; nil when there are none.
func (p *Places) box() ([]byte, error) {
	if p.cfg.Nodes == nil {
		return nil, nil
	}
	m := placesMsg{Hub: p.cfg.Hub, House: p.cfg.Name(), Places: map[string]meshtastic.NodePosition{}}
	for _, n := range p.cfg.Nodes() {
		if n.Position != nil && len(m.Places) < maxPlaces {
			m.Places[n.ID] = meshtastic.NodePosition{Lat: n.Position.Lat, Lon: n.Position.Lon, AccuracyM: n.Position.AccuracyM, At: n.Position.At}
		}
	}
	if len(m.Places) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return p.cfg.Sealer.Seal(b)
}

// receive takes the places in another hub's broadcast, keeping the newest of each.
func (p *Places) receive(box []byte) {
	b, err := p.cfg.Sealer.Open(box)
	if err != nil {
		return // another village's, or not a hub's
	}
	var m placesMsg
	if json.Unmarshal(b, &m) != nil || m.Hub == p.cfg.Hub {
		return
	}
	late := p.cfg.Now().Add(time.Hour)
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, g := range m.Places {
		if !strings.HasPrefix(id, "!") || g.Lat < -90 || g.Lat > 90 || g.Lon < -180 || g.Lon > 180 || g.At.After(late) {
			continue
		}
		if old, ok := p.got[id]; (ok && !g.At.After(old.At)) || (!ok && len(p.got) >= maxPlaces) {
			continue
		}
		g.Alt, g.From = nil, m.House
		p.got[id] = g
	}
}

// broadcasts lists the broadcast address of every IPv4 network the hub is on.
func broadcasts(port int) []string {
	var out []string
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil {
				continue
			}
			ip, mask := ipn.IP.To4(), net.IP(ipn.Mask).To4()
			if mask == nil {
				continue
			}
			b := make(net.IP, 4)
			for i := range b {
				b[i] = ip[i] | ^mask[i]
			}
			out = append(out, net.JoinHostPort(b.String(), strconv.Itoa(port)))
		}
	}
	return out
}
