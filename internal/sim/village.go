package sim

import (
	"crypto/ed25519"
	"fmt"
	"math/rand"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/meshsync"
	"github.com/johnusher/batnet/internal/seal"
)

// HeartbeatEvery is the real nodes' heartbeat interval (spec §5).
const HeartbeatEvery = 5 * Second

// HouseSpec describes a house node.
type HouseSpec struct {
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Pi     string  `json:"pi"`
	WiFi   string  `json:"wifi"`
	LoRa   bool    `json:"lora"`
	People int     `json:"people"`
}

// RelaySpec describes a relay (mesh only, no people).
type RelaySpec struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Pi   string  `json:"pi"`
	WiFi string  `json:"wifi"`
}

// Node is one real batnet node in the simulation.
type Node struct {
	Name, Kind string // kind: house or relay
	House      HouseSpec
	Store      *ledger.MemStore
	Author     *ledger.Author
	Sync       *meshsync.Syncer
	Radio      *AirNode
	PoweredAll bool // never lost power during the run
}

// Post is one message and where and when it arrived.
type Post struct {
	Key     ledger.Key
	House   string
	Text    string
	Prio    uint8
	Created int64
	Arrived map[string]int64 // house name → virtual time; includes the sender
}

// Village is real nodes on the Air, in virtual time.
type Village struct {
	Sch     *Scheduler
	Air     *Air
	Nodes   []*Node
	Posts   []*Post
	EpochMs int64

	rnd    *rand.Rand
	key    seal.VillageKey
	byName map[string]*Node
	byKey  map[ledger.Key]*Post
}

// NewVillage starts an empty village whose clocks read epochMs at time 0.
func NewVillage(seed, epochMs int64) (*Village, error) {
	rnd := rand.New(rand.NewSource(seed))
	var key seal.VillageKey
	rnd.Read(key[:]) // deterministic per seed; a real village uses crypto/rand
	sch := &Scheduler{}
	return &Village{Sch: sch, Air: NewAir(sch, seed+1), EpochMs: epochMs, rnd: rnd, key: key,
		byName: map[string]*Node{}, byKey: map[ledger.Key]*Post{}}, nil
}

func (v *Village) add(name, kind string, x, y float64, pi, wifi string) (*Node, error) {
	if name == "" || v.byName[name] != nil {
		return nil, fmt.Errorf("sim: node name %q empty or used twice", name)
	}
	n := &Node{Name: name, Kind: kind, Store: ledger.NewMemStore(), PoweredAll: true}
	n.Radio = v.Air.Add(name, x, y, WiFiRange(pi, wifi))
	seed := make([]byte, ed25519.SeedSize)
	v.rnd.Read(seed)
	priv := ed25519.NewKeyFromSeed(seed)
	clock := hlc.New(func() int64 { return v.EpochMs + v.Sch.Now()/Ms })
	var err error
	if n.Author, err = ledger.NewAuthor(priv, clock, n.Store); err != nil {
		return nil, err
	}
	sealer, err := seal.NewSealer(v.key, seal.PurposeMesh)
	if err != nil {
		return nil, err
	}
	n.Sync = meshsync.New(meshsync.Config{
		Link: n.Radio, Store: n.Store, Clock: clock, Sealer: sealer,
		Spawn:   func(f func()) { f() },
		OnEntry: func(e *ledger.Entry) { v.arrived(n, e) },
	})
	n.Radio.SetDeliver(n.Sync.HandlePacket)
	v.Nodes = append(v.Nodes, n)
	v.byName[name] = n
	return n, nil
}

// AddHouse adds a house node.
func (v *Village) AddHouse(h HouseSpec) (*Node, error) {
	n, err := v.add(h.Name, "house", h.X, h.Y, h.Pi, h.WiFi)
	if n != nil {
		n.House = h
	}
	return n, err
}

// AddRelay adds a relay node.
func (v *Village) AddRelay(r RelaySpec) (*Node, error) {
	return v.add(r.Name, "relay", r.X, r.Y, r.Pi, r.WiFi)
}

// Node finds a node by name.
func (v *Village) Node(name string) *Node { return v.byName[name] }

func (v *Village) arrived(n *Node, e *ledger.Entry) {
	if p := v.byKey[e.Key]; p != nil && n.Kind == "house" {
		if _, ok := p.Arrived[n.Name]; !ok {
			p.Arrived[n.Name] = v.Sch.Now()
		}
	}
}

// Start schedules every node's heartbeat every 5 s, each with its own phase.
func (v *Village) Start() {
	for _, n := range v.Nodes {
		n := n
		var beat func()
		beat = func() {
			if n.Radio.On {
				n.Sync.SendHeartbeat()
			}
			v.Sch.After(HeartbeatEvery, beat)
		}
		v.Sch.After(v.rnd.Int63n(HeartbeatEvery), beat)
	}
}

// Post makes house append a message and broadcast it now.
func (v *Village) Post(house, text string, prio uint8) (*Post, error) {
	n := v.byName[house]
	if n == nil || n.Kind != "house" {
		return nil, fmt.Errorf("sim: no house %q", house)
	}
	if !n.Radio.On {
		return nil, fmt.Errorf("sim: house %q has no power", house)
	}
	e, err := n.Author.Append(ledger.TypeMsgPost, prio, map[string]string{"audience": "everyone", "text": text})
	if err != nil {
		return nil, err
	}
	p := &Post{Key: e.Key, House: house, Text: text, Prio: prio, Created: v.Sch.Now(), Arrived: map[string]int64{house: v.Sch.Now()}}
	v.Posts = append(v.Posts, p)
	v.byKey[e.Key] = p
	return p, n.Sync.Publish(e)
}

// SetPower cuts or restores a node's power. Its store survives (the SD card).
func (v *Village) SetPower(name string, on bool) error {
	n := v.byName[name]
	if n == nil {
		return fmt.Errorf("sim: no node %q", name)
	}
	if !on {
		n.PoweredAll = false
	}
	n.Radio.On = on
	return nil
}

// RunFor advances virtual time by d.
func (v *Village) RunFor(d int64) { v.Sch.RunUntil(v.Sch.Now() + d) }
