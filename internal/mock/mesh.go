// Package mock is an in-process stand-in for the batman-adv mesh: nodes,
// links between them, power cuts and packet loss, so the sync code runs
// without radios.
package mock

import (
	"context"
	"errors"
	"math/rand"
	"sync"

	"github.com/johnusher/batnet/internal/meshsync"
)

// ErrUnreachable means no mesh path to that node right now.
var ErrUnreachable = errors.New("mock: node unreachable")

const inboxSize = 4096

// Mesh connects Links like batman-adv: a broadcast floods every node that has
// a path to the sender.
type Mesh struct {
	mu    sync.Mutex
	links map[string]*Link
	adj   map[string]map[string]bool
	down  map[string]bool
	loss  float64
	rnd   *rand.Rand
}

// NewMesh returns an empty mesh; seed fixes the loss pattern.
func NewMesh(seed int64) *Mesh {
	return &Mesh{links: map[string]*Link{}, adj: map[string]map[string]bool{}, down: map[string]bool{}, rnd: rand.New(rand.NewSource(seed))}
}

// Add creates a node.
func (m *Mesh) Add(addr string) *Link {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := &Link{mesh: m, addr: addr, inbox: make(chan meshsync.Packet, inboxSize)}
	m.links[addr] = l
	m.adj[addr] = map[string]bool{}
	return l
}

// Connect puts two nodes in radio range of each other.
func (m *Mesh) Connect(a, b string) { m.setEdge(a, b, true) }

// Disconnect takes two nodes out of range.
func (m *Mesh) Disconnect(a, b string) { m.setEdge(a, b, false) }

func (m *Mesh) setEdge(a, b string, on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if on {
		m.adj[a][b], m.adj[b][a] = true, true
	} else {
		delete(m.adj[a], b)
		delete(m.adj[b], a)
	}
}

// SetUp cuts or restores a node's power.
func (m *Mesh) SetUp(addr string, up bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.down[addr] = !up
}

// SetLoss sets the chance that a broadcast misses each receiver.
func (m *Mesh) SetLoss(p float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loss = p
}

// reachable lists the powered nodes with a path from addr, excluding addr. Caller holds mu.
func (m *Mesh) reachable(addr string) []string {
	if m.down[addr] {
		return nil
	}
	seen := map[string]bool{addr: true}
	queue := []string{addr}
	var out []string
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		for y := range m.adj[x] {
			if seen[y] || m.down[y] {
				continue
			}
			seen[y] = true
			out = append(out, y)
			queue = append(queue, y)
		}
	}
	return out
}

// Link is one node's connection to the mock mesh.
type Link struct {
	mesh    *Mesh
	addr    string
	inbox   chan meshsync.Packet
	hmu     sync.Mutex
	handler func(from string, req []byte) []byte
}

func (l *Link) Addr() string                  { return l.addr }
func (l *Link) Inbox() <-chan meshsync.Packet { return l.inbox }

func (l *Link) SetHandler(h func(from string, req []byte) []byte) {
	l.hmu.Lock()
	defer l.hmu.Unlock()
	l.handler = h
}

func (l *Link) Broadcast(pkt []byte) error {
	m := l.mesh
	m.mu.Lock()
	var targets []*Link
	for _, addr := range m.reachable(l.addr) {
		if m.loss > 0 && m.rnd.Float64() < m.loss {
			continue
		}
		targets = append(targets, m.links[addr])
	}
	m.mu.Unlock()
	for _, t := range targets {
		select {
		case t.inbox <- meshsync.Packet{From: l.addr, Data: append([]byte(nil), pkt...)}:
		default: // a full inbox drops the packet, like a busy radio
		}
	}
	return nil
}

func (l *Link) Request(ctx context.Context, to string, req []byte) ([]byte, error) {
	m := l.mesh
	m.mu.Lock()
	ok := false
	for _, addr := range m.reachable(l.addr) {
		if addr == to {
			ok = true
			break
		}
	}
	target := m.links[to]
	m.mu.Unlock()
	if !ok || target == nil {
		return nil, ErrUnreachable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target.hmu.Lock()
	h := target.handler
	target.hmu.Unlock()
	if h == nil {
		return nil, ErrUnreachable
	}
	return h(l.addr, append([]byte(nil), req...)), nil
}
