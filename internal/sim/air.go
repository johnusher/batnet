package sim

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"sort"

	"github.com/johnusher/batnet/internal/meshsync"
)

var builtinRange = map[string]float64{"Zero W": 80, "Zero 2 W": 100, "Pi 3": 120, "Pi 4": 120, "Pi 5": 120}

// WiFiRange is how far a node's mesh radio reaches, in metres (the JavaScript
// model's numbers; field tests will correct them).
func WiFiRange(pi, radio string) float64 {
	switch radio {
	case "dongle":
		return 350
	case "pole":
		return 450
	case "none":
		return 0
	}
	if r, ok := builtinRange[pi]; ok {
		return r
	}
	return 100
}

// errUnreachable means no powered path to that node right now.
var errUnreachable = errors.New("sim: node unreachable")

// Air is the WiFi mesh in virtual time. batman-adv floods each broadcast
// through every node of the sender's group; frames go at the 1 Mbit/s
// broadcast rate over one channel per group; a busy channel loses more frames.
type Air struct {
	BaseLoss    float64
	OnBroadcast func(from string, to []string, at int64)
	Stats       AirStats

	sch      *Scheduler
	rnd      *rand.Rand
	nodes    []*AirNode
	byAddr   map[string]*AirNode
	channels map[string]*channel // by the group's first address
}

// AirStats counts frames since the start.
type AirStats struct {
	Frames, Delivered, Lost int
	Bytes                   int64
	RequestBytes            int64 // repair requests and answers
	MaxUtil                 float64
}

type channel struct {
	busyUntil int64
	spans     [][2]int64 // recent [start, end) occupancy
}

// NewAir returns an empty world; seed fixes which frames are lost.
func NewAir(s *Scheduler, seed int64) *Air {
	return &Air{BaseLoss: 0.02, sch: s, rnd: rand.New(rand.NewSource(seed)), byAddr: map[string]*AirNode{}, channels: map[string]*channel{}}
}

// Add puts a node at (x, y) metres with a mesh range in metres, powered on.
func (a *Air) Add(addr string, x, y, rng float64) *AirNode {
	n := &AirNode{air: a, Name: addr, X: x, Y: y, Range: rng, On: true}
	a.nodes = append(a.nodes, n)
	a.byAddr[addr] = n
	return n
}

// Linked reports whether two nodes hear each other directly.
func (a *Air) Linked(p, q *AirNode) bool {
	r := math.Min(p.Range, q.Range)
	return r > 0 && math.Hypot(p.X-q.X, p.Y-q.Y) <= r
}

// hops returns the powered nodes reachable from n and their hop counts, in
// the order nodes were added (deterministic).
func (a *Air) hops(n *AirNode) ([]*AirNode, map[*AirNode]int) {
	if !n.On {
		return nil, nil
	}
	h := map[*AirNode]int{n: 0}
	queue := []*AirNode{n}
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		for _, y := range a.nodes {
			if _, seen := h[y]; seen || !y.On || !a.Linked(x, y) {
				continue
			}
			h[y] = h[x] + 1
			queue = append(queue, y)
		}
	}
	var out []*AirNode
	for _, y := range a.nodes {
		if _, ok := h[y]; ok && y != n {
			out = append(out, y)
		}
	}
	return out, h
}

// Groups lists the powered nodes by WiFi group, each in the order added,
// groups ordered by their first node.
func (a *Air) Groups() [][]*AirNode {
	seen := map[*AirNode]bool{}
	var out [][]*AirNode
	for _, n := range a.nodes {
		if seen[n] || !n.On {
			continue
		}
		others, _ := a.hops(n)
		g := append([]*AirNode{n}, others...)
		sort.SliceStable(g, func(i, j int) bool { return a.index(g[i]) < a.index(g[j]) })
		for _, x := range g {
			seen[x] = true
		}
		out = append(out, g)
	}
	return out
}

func (a *Air) index(n *AirNode) int {
	for i, x := range a.nodes {
		if x == n {
			return i
		}
	}
	return -1
}

func (a *Air) channelFor(n *AirNode, others []*AirNode) *channel {
	first := n
	for _, x := range others {
		if a.index(x) < a.index(first) {
			first = x
		}
	}
	c := a.channels[first.Name]
	if c == nil {
		c = &channel{}
		a.channels[first.Name] = c
	}
	return c
}

// utilisation is the busy share of the last second, ending at now.
func (c *channel) utilisation(now int64) float64 {
	from := now - Second
	var busy int64
	keep := c.spans[:0]
	for _, s := range c.spans {
		if s[1] <= from {
			continue
		}
		keep = append(keep, s)
		lo, hi := max(s[0], from), min(s[1], now)
		if hi > lo {
			busy += hi - lo
		}
	}
	c.spans = keep
	return float64(busy) / float64(Second)
}

func (a *Air) broadcast(n *AirNode, pkt []byte) {
	others, hops := a.hops(n)
	if len(others) == 0 {
		return
	}
	now := a.sch.Now()
	frame := 200 + 8*int64(len(pkt)) // µs at 1 Mbit/s
	c := a.channelFor(n, others)
	start := max(now, c.busyUntil)
	occupancy := frame * int64(1+len(others)) // every node floods it once
	util := c.utilisation(start)              // load when the frame goes on air, backlog included
	c.busyUntil = start + occupancy
	c.spans = append(c.spans, [2]int64{start, start + occupancy})
	if u := c.utilisation(start + occupancy); u > a.Stats.MaxUtil {
		a.Stats.MaxUtil = u
	}
	loss := a.BaseLoss + math.Min(0.6, util*util)
	a.Stats.Frames++
	a.Stats.Bytes += int64(len(pkt))
	var to []string
	for _, r := range others {
		if a.rnd.Float64() < loss {
			a.Stats.Lost++
			continue
		}
		to = append(to, r.Name)
		r := r
		data := append([]byte(nil), pkt...)
		a.sch.At(start+frame*int64(hops[r]), func() {
			if r.On && r.deliver != nil {
				a.Stats.Delivered++
				r.deliver(meshsync.Packet{From: n.Name, Data: data})
			}
		})
	}
	if a.OnBroadcast != nil {
		a.OnBroadcast(n.Name, to, start)
	}
}

// AirNode is one node's radio in the Air; it implements meshsync.Link.
type AirNode struct {
	Name        string
	X, Y, Range float64
	On          bool
	air         *Air
	deliver     func(meshsync.Packet)
	handler     func(from string, req []byte) []byte
}

// SetDeliver sets what happens when a broadcast arrives (the Village calls the Syncer).
func (n *AirNode) SetDeliver(f func(meshsync.Packet)) { n.deliver = f }

func (n *AirNode) Addr() string { return n.Name }

func (n *AirNode) Broadcast(pkt []byte) error {
	if n.On {
		n.air.broadcast(n, pkt)
	}
	return nil
}

// Request is a unicast exchange over the mesh. It takes airtime on the group's
// channel for the request and the answer across every hop, and either leg can be
// lost when the channel is busy. (It is answered at once in virtual time; only
// the channel's load records its cost.)
func (n *AirNode) Request(ctx context.Context, to string, req []byte) ([]byte, error) {
	a := n.air
	t := a.byAddr[to]
	if t == nil || !n.On || !t.On || t.handler == nil {
		return nil, errUnreachable
	}
	others, h := a.hops(n)
	hopsTo, ok := h[t]
	if !ok {
		return nil, errUnreachable
	}
	c := a.channelFor(n, others)
	if !a.useChannel(c, len(req), hopsTo) {
		return nil, errLost
	}
	resp := t.handler(n.Name, append([]byte(nil), req...))
	if !a.useChannel(c, len(resp), hopsTo) {
		return nil, errLost
	}
	return resp, nil
}

// errLost means a unicast leg was lost on a busy channel.
var errLost = errors.New("sim: request lost on a busy channel")

// useChannel books airtime for one unicast leg of size bytes over hops hops
// and reports whether it got through.
func (a *Air) useChannel(c *channel, size, hops int) bool {
	now := a.sch.Now()
	start := max(now, c.busyUntil)
	occupancy := (200 + 8*int64(size)) * int64(max(1, hops))
	util := c.utilisation(start)
	c.busyUntil = start + occupancy
	c.spans = append(c.spans, [2]int64{start, start + occupancy})
	if u := c.utilisation(start + occupancy); u > a.Stats.MaxUtil {
		a.Stats.MaxUtil = u
	}
	a.Stats.RequestBytes += int64(size)
	return a.rnd.Float64() >= a.BaseLoss+math.Min(0.6, util*util)
}

func (n *AirNode) Inbox() <-chan meshsync.Packet { return nil }

func (n *AirNode) SetHandler(h func(from string, req []byte) []byte) { n.handler = h }
