package sim

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/johnusher/batnet/internal/auth"
	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/lora"
	"github.com/johnusher/batnet/internal/meshsync"
	"github.com/johnusher/batnet/internal/prio"
	"github.com/johnusher/batnet/internal/seal"
	"github.com/johnusher/batnet/internal/state"
	"github.com/johnusher/batnet/internal/web"
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
	LoRa bool    `json:"lora"`
}

// Node is one real batnet node in the simulation.
type Node struct {
	Name, Kind string // kind: house or relay
	House      HouseSpec
	Store      *ledger.MemStore
	Author     *ledger.Author
	Sync       *meshsync.Syncer
	Radio      *AirNode
	PoweredAll bool         // never lost power during the run
	Bridge     *lora.Bridge // houses and relays with a LoRa radio
	LoRaRadio  *LoRaRadio
	Web        *web.Server // houses: the real house server, with its people as real accounts
	People     []string    // houses: the account IDs of the people living here
	Admin      string      // houses: the house admin's account ID (posts when no one else lives here)
	wakeAt     int64
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
	Sch        *Scheduler
	Air        *Air
	LoRa       *LoRaAir
	Meshtastic []*LoRaRadio
	Nodes      []*Node
	Posts      []*Post
	EpochMs    int64
	MaxDutyUs  int64    // the most any radio put on air in an hour, as the air saw it, sampled each minute
	Skipped    []string // scripted posts that could not be made (a house without power), as reasons
	preset     lora.Preset

	rnd    *rand.Rand
	who    *rand.Rand // who writes a post: apart from rnd, so it doesn't shift the schedule
	key    seal.VillageKey
	groups []groupSnap // WiFi groups over time, recorded when they can change
	byName map[string]*Node
	byKey  map[ledger.Key]*Post
	seeded bool
}

// NewVillage starts an empty village whose clocks read epochMs at time 0.
func NewVillage(seed, epochMs int64, preset lora.Preset) (*Village, error) {
	rnd := rand.New(rand.NewSource(seed))
	var key seal.VillageKey
	rnd.Read(key[:]) // deterministic per seed; a real village uses crypto/rand
	sch := &Scheduler{}
	return &Village{Sch: sch, Air: NewAir(sch, seed+1), LoRa: NewLoRaAir(sch, preset, seed+2), preset: preset,
		EpochMs: epochMs, rnd: rnd, who: rand.New(rand.NewSource(seed + 3)), key: key, byName: map[string]*Node{}, byKey: map[ledger.Key]*Post{}}, nil
}

func (v *Village) add(name, kind string, x, y float64, pi, wifi string, withLoRa bool) (*Node, error) {
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
		Spawn: func(f func()) { f() },
		OnEntry: func(e *ledger.Entry) {
			v.arrived(n, e)
			if n.Web != nil {
				n.Web.OnEntry(e)
			}
			if n.Bridge != nil {
				n.Bridge.Consider(e)
				v.wakeBridge(n)
			}
		},
	})
	n.Radio.SetDeliver(n.Sync.HandlePacket)
	if kind == "house" {
		n.Web = web.New(web.Config{House: name, Author: n.Author, State: state.New(),
			Now: func() time.Time { return time.UnixMilli(v.EpochMs + v.Sch.Now()/Ms) },
			Publish: func(e *ledger.Entry) {
				v.track(n, e)
				n.Sync.Publish(e)
				if n.Bridge != nil {
					n.Bridge.Consider(e)
					v.wakeBridge(n)
				}
			}})
	}
	if withLoRa {
		n.LoRaRadio = v.LoRa.Add(name, x, y, func(frame []byte) {
			if n.Bridge != nil {
				n.Bridge.Receive(frame)
				v.wakeBridge(n)
			}
		})
		n.Bridge = lora.NewBridge(lora.BridgeConfig{
			Self: n.Author.ID(), Preset: v.preset, Store: n.Store, Radio: n.LoRaRadio, Seed: v.rnd.Int63(),
			Now:       func() int64 { return v.Sch.Now() },
			Inject:    n.Sync.Inject,
			InMyGroup: func(short uint32) bool { return v.sameWiFiGroup(n, short) },
		})
		n.LoRaRadio.Duty = n.Bridge.Duty // sends and rebroadcasts share one hour's budget
	}
	v.Nodes = append(v.Nodes, n)
	v.byName[name] = n
	return n, nil
}

// AddHouse adds a house node.
func (v *Village) AddHouse(h HouseSpec) (*Node, error) {
	n, err := v.add(h.Name, "house", h.X, h.Y, h.Pi, h.WiFi, h.LoRa)
	if n != nil {
		n.House = h
	}
	return n, err
}

// AddRelay adds a relay node.
func (v *Village) AddRelay(r RelaySpec) (*Node, error) {
	return v.add(r.Name, "relay", r.X, r.Y, r.Pi, r.WiFi, r.LoRa)
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
	if err := v.Seed(); err != nil {
		panic(err) // seeding only fails if signing fails
	}
	v.snapGroups()
	v.Sch.After(Minute, v.probeDuty)
	for _, n := range v.Nodes {
		if n.Bridge != nil {
			n := n
			v.Sch.At(v.Sch.Now(), func() { v.wakeBridge(n) })
		}
	}
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
func (v *Village) Post(house, text string, level uint8) (*Post, error) {
	n := v.byName[house]
	if n == nil || n.Kind != "house" {
		return nil, fmt.Errorf("sim: no house %q", house)
	}
	if !n.Radio.On {
		return nil, fmt.Errorf("sim: house %q has no power", house)
	}
	from := n.Admin // a house with no one else living there posts as its admin (B31)
	if len(n.People) > 0 {
		from = n.People[v.who.Intn(len(n.People))] // one of the people living here
	}
	// Written through the house server, as the app would: stored, folded, published, handed to the radio.
	e, err := n.Web.Write(ledger.TypeMsgPost, prio.Level(level), map[string]string{"from": state.CompactAccount(from, n.Author.ID()), "audience": "everyone", "text": text})
	if err != nil {
		return nil, err
	}
	return v.byKey[e.Key], nil // tracked when the server published it
}

// scriptedPost is Post for a scenario's script: a post that can't be made
// (the house has no power) is recorded, not dropped in silence (B3).
func (v *Village) scriptedPost(house, text string, level uint8) {
	if _, err := v.Post(house, text, level); err != nil {
		v.Skipped = append(v.Skipped, err.Error())
	}
}

// track records every message a house writes, from the page's button or from a
// phone, so the page lists it and the checks judge its delivery.
func (v *Village) track(n *Node, e *ledger.Entry) {
	if e.Type != ledger.TypeMsgPost && e.Type != ledger.TypeAlert {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	json.Unmarshal(e.Body, &body)
	p := &Post{Key: e.Key, House: n.Name, Text: body.Text, Prio: e.Prio, Created: v.Sch.Now(), Arrived: map[string]int64{n.Name: v.Sch.Now()}}
	v.Posts = append(v.Posts, p)
	v.byKey[e.Key] = p
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
	if n.LoRaRadio != nil {
		n.LoRaRadio.On = on
		if on {
			n.Bridge.Restart() // a power cut is a reboot: in-memory queues are gone
			v.wakeBridge(n)
		}
	}
	v.snapGroups()
	return nil
}

// groupSnap is which WiFi group each powered node was in from time T on.
type groupSnap struct {
	T     int64
	Group map[string]int
}

// snapGroups records which nodes can exchange messages at all: WiFi links
// plus LoRa links between powered radios in range (Meshtastic users relay).
func (v *Village) snapGroups() {
	names := []string{}
	adj := map[string][]string{}
	link := func(a, b string) { adj[a] = append(adj[a], b); adj[b] = append(adj[b], a) }
	for _, grp := range v.Air.Groups() {
		for i, n := range grp {
			names = append(names, n.Name)
			if i > 0 {
				link(grp[0].Name, n.Name)
			}
		}
	}
	var radios []*LoRaRadio
	for _, n := range v.Nodes {
		if n.LoRaRadio != nil && n.LoRaRadio.On {
			radios = append(radios, n.LoRaRadio)
		}
	}
	for _, r := range v.Meshtastic {
		if r.On {
			radios = append(radios, r)
			names = append(names, r.Name)
		}
	}
	for i, p := range radios {
		for _, q := range radios[i+1:] {
			if v.LoRa.Reach(p, q) {
				link(p.Name, q.Name)
			}
		}
	}
	g := map[string]int{}
	comp := 0
	for _, start := range names {
		if _, done := g[start]; done {
			continue
		}
		queue := []string{start}
		g[start] = comp
		for len(queue) > 0 {
			x := queue[0]
			queue = queue[1:]
			for _, y := range adj[x] {
				if _, done := g[y]; !done {
					g[y] = comp
					queue = append(queue, y)
				}
			}
		}
		comp++
	}
	v.groups = append(v.groups, groupSnap{T: v.Sch.Now(), Group: g})
}

// sameWiFiGroup: is the node with this short ID in n's WiFi group now?
// (batman-adv's originator table, on a Pi.)
func (v *Village) sameWiFiGroup(n *Node, short uint32) bool {
	for _, grp := range v.Air.Groups() {
		in := false
		for _, x := range grp {
			if x == n.Radio {
				in = true
			}
		}
		if !in {
			continue
		}
		for _, x := range grp {
			if m := v.byName[x.Name]; m != nil && lora.ShortID(m.Author.ID()) == short {
				return true
			}
		}
	}
	return false
}

// wakeBridge runs a node's bridge now and schedules its next Tick (one pending wake per node).
func (v *Village) wakeBridge(n *Node) {
	if n.Bridge == nil || !n.Radio.On {
		return
	}
	next := n.Bridge.Tick()
	if next <= v.Sch.Now() {
		next = v.Sch.Now() + 1
	}
	if n.wakeAt != 0 && n.wakeAt <= next && n.wakeAt > v.Sch.Now() {
		return
	}
	n.wakeAt = next
	v.Sch.At(next, func() {
		if n.wakeAt == next {
			n.wakeAt = 0
			v.wakeBridge(n)
		}
	})
}

// AddMeshtastic adds a plain Meshtastic radio: it relays (managed flooding) and nothing else.
func (v *Village) AddMeshtastic(name string, x, y float64) *LoRaRadio {
	r := v.LoRa.Add(name, x, y, nil)
	v.Meshtastic = append(v.Meshtastic, r)
	return r
}

// probeDuty samples every radio's hourly airtime each simulated minute, from
// the air's own tally of what each radio transmitted (B14): a bridge that
// miscounted its airtime would still be caught.
func (v *Village) probeDuty() {
	now := v.Sch.Now()
	for _, n := range v.Nodes {
		if n.LoRaRadio != nil {
			v.MaxDutyUs = max(v.MaxDutyUs, v.LoRa.Used(n.LoRaRadio, now))
		}
	}
	for _, r := range v.Meshtastic {
		v.MaxDutyUs = max(v.MaxDutyUs, v.LoRa.Used(r, now))
	}
	v.Sch.After(Minute, v.probeDuty)
}

// RunFor advances virtual time by d.
func (v *Village) RunFor(d int64) { v.Sch.RunUntil(v.Sch.Now() + d) }

// peopleNames are the seeded accounts' names, the same as the JavaScript model's:
// ten from Hitchcock films, then Bond villains and British prime ministers, alternating.
var peopleNames = []string{"Norman Bates", "Marion Crane", "Scottie Ferguson", "Madeleine Elster", "Roger Thornhill", "Eve Kendall", "L.B. Jefferies", "Lisa Fremont", "Mrs Danvers", "Melanie Daniels", "Auric Goldfinger", "Robert Walpole", "Ernst Blofeld", "William Pitt", "Dr. No", "Robert Peel", "Rosa Klebb", "Benjamin Disraeli", "Emilio Largo", "William Gladstone", "Red Grant", "Lord Salisbury", "Scaramanga", "Arthur Balfour", "Oddjob", "Herbert Asquith", "Karl Stromberg", "Lloyd George", "Jaws", "Stanley Baldwin", "Hugo Drax", "Ramsay MacDonald", "Kamal Khan", "Neville Chamberlain", "Max Zorin", "Winston Churchill", "Franz Sanchez", "Clement Attlee", "Alec Trevelyan", "Anthony Eden", "Xenia Onatopp", "Harold Macmillan", "Elliot Carver", "Harold Wilson", "Elektra King", "Edward Heath", "Renard", "James Callaghan", "Gustav Graves", "Margaret Thatcher", "Le Chiffre", "John Major", "Dominic Greene", "Tony Blair", "Raoul Silva", "Gordon Brown", "Mr. Big", "Theresa May", "Lyutsifer Safin", "Keir Starmer"}

// Seed makes the village an established one: every house announced, with an
// approved admin and its people as approved accounts, and the groups Medical
// and Food. The entries go straight into every node's store and state, as if
// they had been synced long ago: no airtime, nothing to repair.
func (v *Village) Seed() error {
	if v.seeded {
		return nil
	}
	v.seeded = true
	pin, err := auth.HashPIN("000000") // one placeholder PIN for every simulated person, hashed once
	if err != nil {
		return err
	}
	var all []*ledger.Entry
	write := func(n *Node, t ledger.Type, body any) (*ledger.Entry, error) {
		e, err := n.Author.Append(t, uint8(prio.Housekeeping), body)
		if err == nil {
			all = append(all, e)
		}
		return e, err
	}
	first := true
	k := 0 // names run on across the village, so each house has different people
	for _, n := range v.Nodes {
		if n.Kind != "house" {
			continue
		}
		if _, err := write(n, ledger.TypeNodeAnnounce, map[string]string{"house": n.Name}); err != nil {
			return err
		}
		for i := 0; i <= n.House.People; i++ {
			name := peopleNames[k%len(peopleNames)]
			if k >= len(peopleNames) {
				name = fmt.Sprintf("%s %d", name, k/len(peopleNames)+1)
			}
			k++
			r, err := write(n, ledger.TypeAccountRequest, map[string]any{"name": name, "pinHash": pin, "admin": i == 0})
			if err != nil {
				return err
			}
			id := state.AccountID(r.Key)
			if _, err := write(n, ledger.TypeAccountApprove, map[string]string{"account": id}); err != nil {
				return err
			}
			if i > 0 {
				n.People = append(n.People, id)
			} else {
				n.Admin = id
			}
		}
		if first {
			for _, g := range []string{"Medical", "Food"} {
				if _, err := write(n, ledger.TypeGroupCreate, map[string]string{"id": strings.ToLower(g), "name": g}); err != nil {
					return err
				}
			}
			first = false
		}
	}
	for _, n := range v.Nodes {
		for _, e := range all {
			n.Store.Put(e)
			if n.Web != nil {
				n.Web.State().Apply(e)
			}
		}
	}
	return nil
}
