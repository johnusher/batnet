# Milestone 1, plan 2: deterministic village runtime, scenarios, and the real code in the simulator

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run batnet's real Go code for a whole village in virtual time — headless with pass/fail checks (`batnetd -sim scenarios/bang.json`, in CI) and live in the GitHub Pages simulator through WebAssembly.

**Architecture:** `meshsync` becomes drivable by events (`HandlePacket`, an injectable `Spawn`) so no goroutine or ticker is needed. `internal/sim` adds a discrete-event `Scheduler` (microseconds), an `Air` medium modelling WiFi range per Pi and radio, batman-adv flooding over a shared channel whose loss grows with load, a `Village` of real nodes (MemStore, Author, Syncer) wired to it, a JSON `Scenario` format with scripted events and checks, a `Report`, and a JSON `API` for the page. `cmd/batnetd -sim` runs scenarios; `cmd/batnetwasm` exposes the `API` to JavaScript; `prototypes/village-sim/real.js` draws it.

**Tech Stack:** Go 1.26 (`GOOS=js GOARCH=wasm` for the browser), standard library only in new code; plain JavaScript for the page.

**Spec:** [`docs/specs/2026-10-15-m1-design.md`](../specs/2026-10-15-m1-design.md), sections 5, 13, 14 and **14a**.

**Milestone 1 plans:** 1 core (done) · **2 village runtime (this)** · 3 LoRa bridge and priority · 4 web app and accounts · 5 Pi set-up, radios, kiosk, speaker · 6 image, installer, CI hardening.

## Global Constraints

- Same seed, same scenario → identical results (no wall clock, no map-order dependence, no goroutines in the simulated path).
- Virtual time unit: microseconds (`int64`). The HLC wall clock of a simulated node is `epochMs + now/1000`.
- WiFi ranges (metres), as the JavaScript model: built-in Zero W 80, Zero 2 W 100, Pi 3/4/5 120; USB dongle 350; dongle on a pole 450; none 0. Two nodes link when their distance ≤ the smaller of their ranges.
- Broadcast frames at 1 Mbit/s: airtime = 200 µs + 8 µs per byte; batman-adv floods each broadcast once per node of the component.
- Heartbeat every 5 s per node (spec §5).
- No real names, places or IDs in scenarios: placeholder house names only.
- `internal/sim` and `cmd/batnetwasm` must build with `GOOS=js GOARCH=wasm`; nothing they import may need cgo or SQLite.

## Review Focus

1. **Determinism breaks quietly** (iterating a Go map while scheduling, `time.Now` inside the simulated path): two runs of one scenario differ. → Task 4 `TestDeterministic`, Task 6 `TestRunTwiceSameReport`.
2. **A broadcast storm** ("everyone hears a bang"): the channel saturates; frames are lost and delayed, but the village still converges through repair. → Task 7 `scenarios/bang.json` checks.
3. **A powered-off node** must neither send, receive, answer requests nor run heartbeats, and must catch up after power returns. → Task 3 `TestPowerOff`, Task 4 `TestPowerCutCatchUp`.
4. **Separate WiFi groups** must not share a channel or be expected to exchange messages (LoRa arrives in plan 3); checks judge delivery per final WiFi group. → Task 3 `TestSeparateGroupsSeparateChannels`, Task 6 checks.
5. **Bad scenario files** (unknown house in an event, unparsable duration, empty village): a clear error, never a panic. → Task 5 `TestScenarioErrors`.

---

## File structure

```
internal/meshsync/syncer.go        (modify) HandlePacket, Config.Spawn
internal/meshsync/inline_test.go   driving a Syncer with no goroutines
internal/sim/scheduler.go, _test   virtual time
internal/sim/air.go, _test         WiFi model: ranges, flooding, shared channel, loss
internal/sim/village.go, _test     real nodes on the Air; posts, power, arrivals
internal/sim/scenario.go, _test    JSON scenarios: houses or a generator, traffic, events, checks
internal/sim/report.go, _test      running a scenario; latency stats; check results
internal/sim/api.go, _test         JSON API for the page
cmd/batnetd/main.go                (modify) -sim
cmd/batnetwasm/main.go             js/wasm bridge to the API
scenarios/*.json, scenarios/index.json
prototypes/village-sim/real.js     the "real batnet code" view
prototypes/village-sim/index.html  (modify) engine switch
.github/workflows/pages.yml        (modify) build batnet.wasm
.github/workflows/ci.yml           (modify) run the scenarios
```

---

### Task 1: A Syncer driven by events

**Files:**
- Modify: `internal/meshsync/syncer.go`
- Create: `internal/meshsync/inline_test.go`

**Interfaces:**
- Produces: `(*Syncer).HandlePacket(p Packet)`; `Config.Spawn func(func())` (default `func(f func()) { go f() }`). `Run` is unchanged for real time.

- [ ] **Step 1: Write the failing test** — `internal/meshsync/inline_test.go`

```go
package meshsync_test

import (
	"crypto/ed25519"
	"testing"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/meshsync"
	"github.com/johnusher/batnet/internal/mock"
	"github.com/johnusher/batnet/internal/seal"
)

// The simulator drives a Syncer by hand: no Run, no goroutines, repairs inline.
func TestInlineDriving(t *testing.T) {
	m := mock.NewMesh(1)
	k, _ := seal.NewVillageKey()
	type node struct {
		link   *mock.Link
		store  *ledger.MemStore
		author *ledger.Author
		sync   *meshsync.Syncer
	}
	mk := func(addr string) *node {
		n := &node{link: m.Add(addr), store: ledger.NewMemStore()}
		_, priv, _ := ed25519.GenerateKey(nil)
		clock := hlc.New(func() int64 { return 1_790_000_000_000 })
		n.author, _ = ledger.NewAuthor(priv, clock, n.store)
		sealer, _ := seal.NewSealer(k, seal.PurposeMesh)
		n.sync = meshsync.New(meshsync.Config{Link: n.link, Store: n.store, Clock: clock, Sealer: sealer,
			Spawn: func(f func()) { f() }})
		return n
	}
	a, b := mk("a"), mk("b")
	m.Connect("a", "b")
	for i := 0; i < 3; i++ {
		a.author.Append(ledger.TypeMsgPost, 5, map[string]int{"n": i}) // stored, never broadcast
	}
	if err := a.sync.SendHeartbeat(); err != nil {
		t.Fatal(err)
	}
	p := <-b.link.Inbox()
	b.sync.HandlePacket(p) // b sees a's hash differ and repairs, inline
	if all, _ := b.store.All(); len(all) != 3 {
		t.Fatalf("b holds %d entries right after HandlePacket, want 3", len(all))
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/meshsync/ -run Inline`
Expected: FAIL, `unknown field Spawn` / `HandlePacket undefined`.

- [ ] **Step 3: Implement** — in `internal/meshsync/syncer.go`:

Add to `Config`, after `Logf`:

```go
	// Spawn runs a repair. Default: a new goroutine. The simulator runs it inline.
	Spawn func(func())
```

Add a `ctx` field to `Syncer`:

```go
type Syncer struct {
	cfg       Config
	ctx       context.Context // for repairs started by HandlePacket
	mu        sync.Mutex
	repairing map[string]bool
	slots     chan struct{} // one per repair allowed to run at once
}
```

In `New`, before building `s`:

```go
	if cfg.Spawn == nil {
		cfg.Spawn = func(f func()) { go f() }
	}
```

and build `s` with `ctx: context.Background()`.

Add:

```go
// HandlePacket processes one broadcast from the mesh. Run calls it for every
// packet; the simulator calls it directly in virtual time.
func (s *Syncer) HandlePacket(p Packet) { s.receive(s.ctx, p) }
```

In `receive`, replace `go func() {` (the repair goroutine) with `s.cfg.Spawn(func() {`, keeping the closing `}()` as `})`.

- [ ] **Step 4: Run all meshsync tests**

Run: `go test -race ./internal/meshsync/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/meshsync
git commit -m "meshsync: HandlePacket and an injectable Spawn, so a Syncer can run in virtual time"
```

---

### Task 2: Virtual time

**Files:**
- Create: `internal/sim/scheduler.go`, `internal/sim/scheduler_test.go`

**Interfaces:**
- Produces: `sim.Scheduler` with `Now() int64` (µs), `At(t int64, fn func())`, `After(d int64, fn func())`, `RunUntil(t int64) int`, `Pending() int`; constants `Ms = 1000`, `Second = 1_000_000`, `Minute = 60 * Second`.

- [ ] **Step 1: Write the failing tests** — `internal/sim/scheduler_test.go`

```go
package sim

import (
	"reflect"
	"testing"
)

func TestOrderAndTies(t *testing.T) {
	var s Scheduler
	var got []string
	s.At(30, func() { got = append(got, "c") })
	s.At(10, func() { got = append(got, "a") })
	s.At(10, func() { got = append(got, "b") }) // same time: scheduling order
	s.RunUntil(100)
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("order %v", got)
	}
	if s.Now() != 100 {
		t.Fatalf("Now = %d, want 100", s.Now())
	}
}

func TestNestedAndPast(t *testing.T) {
	var s Scheduler
	var got []int64
	s.At(5, func() {
		s.After(5, func() { got = append(got, s.Now()) })
		s.At(1, func() { got = append(got, s.Now()) }) // in the past: runs now
	})
	s.RunUntil(4)
	if len(got) != 0 || s.Pending() != 1 {
		t.Fatalf("ran early: %v, pending %d", got, s.Pending())
	}
	s.RunUntil(20)
	if !reflect.DeepEqual(got, []int64{5, 10}) {
		t.Fatalf("times %v, want [5 10]", got)
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/sim/`
Expected: FAIL, `undefined: Scheduler`.

- [ ] **Step 3: Implement** — `internal/sim/scheduler.go`

```go
// Package sim runs a village of real batnet nodes in virtual time, over a
// model of the physical world, for scenario tests and the simulator page.
package sim

import "container/heap"

// Units of virtual time.
const (
	Ms     int64 = 1000
	Second int64 = 1000 * Ms
	Minute int64 = 60 * Second
)

// Scheduler runs callbacks in virtual time (microseconds), in time order and,
// for equal times, in the order they were scheduled.
type Scheduler struct {
	now int64
	seq uint64
	q   events
}

type event struct {
	at  int64
	seq uint64
	fn  func()
}

type events []event

func (e events) Len() int { return len(e) }
func (e events) Less(i, j int) bool {
	if e[i].at != e[j].at {
		return e[i].at < e[j].at
	}
	return e[i].seq < e[j].seq
}
func (e events) Swap(i, j int) { e[i], e[j] = e[j], e[i] }
func (e *events) Push(x any)   { *e = append(*e, x.(event)) }
func (e *events) Pop() any {
	old := *e
	x := old[len(old)-1]
	*e = old[:len(old)-1]
	return x
}

// Now is the current virtual time.
func (s *Scheduler) Now() int64 { return s.now }

// At runs fn at time t (now, if t is in the past).
func (s *Scheduler) At(t int64, fn func()) {
	if t < s.now {
		t = s.now
	}
	s.seq++
	heap.Push(&s.q, event{at: t, seq: s.seq, fn: fn})
}

// After runs fn d from now.
func (s *Scheduler) After(d int64, fn func()) { s.At(s.now+d, fn) }

// RunUntil runs every event due by t, then sets the clock to t. It returns how many ran.
func (s *Scheduler) RunUntil(t int64) int {
	n := 0
	for len(s.q) > 0 && s.q[0].at <= t {
		e := heap.Pop(&s.q).(event)
		s.now = e.at
		e.fn()
		n++
	}
	if t > s.now {
		s.now = t
	}
	return n
}

// Pending is the number of events not yet run.
func (s *Scheduler) Pending() int { return len(s.q) }
```

- [ ] **Step 4: Run to see it pass**

Run: `go test ./internal/sim/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/sim
git commit -m "sim: a discrete-event scheduler in virtual microseconds"
```

---

### Task 3: The WiFi world — ranges, flooding, a shared channel

**Files:**
- Create: `internal/sim/air.go`, `internal/sim/air_test.go`

**Interfaces:**
- Consumes: `Scheduler`, `meshsync.Link`, `meshsync.Packet`.
- Produces:
  ```go
  func WiFiRange(pi, radio string) float64
  type Air struct {
  	BaseLoss    float64 // chance a frame misses a receiver on a quiet channel (default 0.02)
  	OnBroadcast func(from string, to []string, at int64)
  	Stats       AirStats
  }
  type AirStats struct{ Frames, Delivered, Lost int; Bytes int64; MaxUtil float64 }
  func NewAir(s *Scheduler, seed int64) *Air
  func (a *Air) Add(addr string, x, y, rng float64) *AirNode
  func (a *Air) Linked(p, q *AirNode) bool
  func (a *Air) Groups() [][]*AirNode   // powered nodes, by WiFi group, deterministic order
  type AirNode struct{ Name string; X, Y, Range float64; On bool; /* … */ }
  func (n *AirNode) SetDeliver(f func(meshsync.Packet))
  // *AirNode implements meshsync.Link
  ```

- [ ] **Step 1: Write the failing tests** — `internal/sim/air_test.go`

```go
package sim

import (
	"context"
	"testing"

	"github.com/johnusher/batnet/internal/meshsync"
)

var _ meshsync.Link = (*AirNode)(nil)

func TestWiFiRange(t *testing.T) {
	cases := []struct {
		pi, radio string
		want      float64
	}{{"Pi 4", "builtin", 120}, {"Zero 2 W", "builtin", 100}, {"Zero W", "builtin", 80}, {"Pi 3", "dongle", 350}, {"Pi 5", "pole", 450}, {"Pi 4", "none", 0}}
	for _, c := range cases {
		if got := WiFiRange(c.pi, c.radio); got != c.want {
			t.Errorf("WiFiRange(%q, %q) = %v, want %v", c.pi, c.radio, got, c.want)
		}
	}
}

func listen(n *AirNode) *[]meshsync.Packet {
	var got []meshsync.Packet
	n.SetDeliver(func(p meshsync.Packet) { got = append(got, p) })
	return &got
}

func TestFloodingThroughARelay(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 1)
	air.BaseLoss = 0
	a := air.Add("a", 0, 0, 350)
	r := air.Add("relay", 300, 0, 350)
	b := air.Add("b", 600, 0, 350)
	if air.Linked(a, b) || !air.Linked(a, r) {
		t.Fatal("wrong links")
	}
	gotR, gotB := listen(r), listen(b)
	a.Broadcast([]byte("hello"))
	if len(*gotB) != 0 {
		t.Fatal("delivered before any virtual time passed")
	}
	s.RunUntil(Second)
	if len(*gotR) != 1 || len(*gotB) != 1 || (*gotB)[0].From != "a" {
		t.Fatalf("relay got %d, b got %d", len(*gotR), len(*gotB))
	}
}

func TestPowerOff(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 1)
	air.BaseLoss = 0
	a, b := air.Add("a", 0, 0, 350), air.Add("b", 100, 0, 350)
	got := listen(b)
	b.SetHandler(func(string, []byte) []byte { return []byte("ok") })
	b.On = false
	a.Broadcast([]byte("x"))
	s.RunUntil(Second)
	if len(*got) != 0 {
		t.Fatal("a node without power received a frame")
	}
	if _, err := a.Request(context.Background(), "b", nil); err == nil {
		t.Fatal("a node without power answered a request")
	}
	b.On = true
	if resp, err := a.Request(context.Background(), "b", nil); err != nil || string(resp) != "ok" {
		t.Fatalf("after power returned: %q, %v", resp, err)
	}
}

func TestBusyChannelLosesAndDelays(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 3)
	air.BaseLoss = 0
	nodes := []*AirNode{}
	for i := 0; i < 10; i++ {
		nodes = append(nodes, air.Add(string(rune('a'+i)), float64(i*30), 0, 350))
	}
	var last int64
	nodes[9].SetDeliver(func(meshsync.Packet) { last = s.Now() })
	for i := 0; i < 400; i++ { // a burst: 400 frames of 300 bytes at the same instant
		nodes[i%9].Broadcast(make([]byte, 300))
	}
	s.RunUntil(Minute)
	if air.Stats.Lost == 0 {
		t.Fatal("no frames lost on a saturated channel")
	}
	if last < Second {
		t.Fatalf("last delivery at %d µs: a saturated channel should delay frames past 1 s", last)
	}
	if air.Stats.MaxUtil < 0.9 {
		t.Fatalf("max utilisation %.2f, want near 1", air.Stats.MaxUtil)
	}
}

func TestSeparateGroupsSeparateChannels(t *testing.T) {
	var s Scheduler
	air := NewAir(&s, 1)
	air.BaseLoss = 0
	a, b := air.Add("a", 0, 0, 120), air.Add("b", 100, 0, 120)
	c, d := air.Add("c", 5000, 0, 120), air.Add("d", 5100, 0, 120)
	for i := 0; i < 300; i++ {
		a.Broadcast(make([]byte, 300)) // keep group 1 busy
	}
	var at int64 = -1
	d.SetDeliver(func(meshsync.Packet) { at = s.Now() })
	c.Broadcast([]byte("quiet over here"))
	s.RunUntil(Minute)
	if at < 0 || at > 10*Ms {
		t.Fatalf("group 2's frame arrived at %d µs; group 1's traffic must not delay it", at)
	}
	if g := air.Groups(); len(g) != 2 || len(g[0]) != 2 || len(g[1]) != 2 {
		t.Fatalf("groups %v", g)
	}
	_ = b
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/sim/`
Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/sim/air.go`

```go
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
	util := c.utilisation(now)
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

func (n *AirNode) Addr_() string { return n.Name }

func (n *AirNode) Broadcast(pkt []byte) error {
	if n.On {
		n.air.broadcast(n, pkt)
	}
	return nil
}

func (n *AirNode) Request(ctx context.Context, to string, req []byte) ([]byte, error) {
	t := n.air.byAddr[to]
	if t == nil || !n.On || !t.On || t.handler == nil {
		return nil, errUnreachable
	}
	if _, h := n.air.hops(n); h == nil {
		return nil, errUnreachable
	} else if _, ok := h[t]; !ok {
		return nil, errUnreachable
	}
	return t.handler(n.Name, append([]byte(nil), req...)), nil
}

func (n *AirNode) Inbox() <-chan meshsync.Packet { return nil }

func (n *AirNode) SetHandler(h func(from string, req []byte) []byte) { n.handler = h }
```


- [ ] **Step 4: Run to see it pass**

Run: `go test -race ./internal/sim/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/sim
git commit -m "sim: the WiFi world - ranges per Pi and radio, batman-adv flooding, a shared channel per group that loses frames under load"
```

---

### Task 4: A village of real nodes

**Files:**
- Create: `internal/sim/village.go`, `internal/sim/village_test.go`

**Interfaces:**
- Consumes: `Scheduler`, `Air`, `AirNode`, `ledger`, `meshsync`, `seal`, `hlc`.
- Produces:
  ```go
  type HouseSpec struct {
  	Name   string  `json:"name"`
  	X      float64 `json:"x"`
  	Y      float64 `json:"y"`
  	Pi     string  `json:"pi"`
  	WiFi   string  `json:"wifi"`
  	LoRa   bool    `json:"lora"`
  	People int     `json:"people"`
  }
  type RelaySpec struct{ Name string `json:"name"`; X, Y float64; Pi string `json:"pi"`; WiFi string `json:"wifi"` }
  type Node struct{ Name, Kind string; House HouseSpec; Store *ledger.MemStore; Author *ledger.Author; Sync *meshsync.Syncer; Radio *AirNode; PoweredAll bool }
  type Post struct{ Key ledger.Key; House, Text string; Prio uint8; Created int64; Arrived map[string]int64 }
  type Village struct{ Sch *Scheduler; Air *Air; Nodes []*Node; Posts []*Post; EpochMs int64 /* … */ }
  func NewVillage(seed, epochMs int64) (*Village, error)
  func (v *Village) AddHouse(h HouseSpec) (*Node, error)
  func (v *Village) AddRelay(r RelaySpec) (*Node, error)
  func (v *Village) Node(name string) *Node
  func (v *Village) Start()                      // heartbeats every 5 s, random phase
  func (v *Village) Post(house, text string, prio uint8) (*Post, error)
  func (v *Village) SetPower(name string, on bool) error
  func (v *Village) RunFor(d int64)
  ```

- [ ] **Step 1: Write the failing tests** — `internal/sim/village_test.go`

```go
package sim

import (
	"reflect"
	"testing"
)

const testEpoch = 1_790_000_000_000

func smallVillage(t *testing.T, seed int64) *Village {
	t.Helper()
	v, err := NewVillage(seed, testEpoch)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []HouseSpec{
		{Name: "Miller", X: 0, Y: 0, Pi: "Pi 4", WiFi: "dongle", People: 3},
		{Name: "Fern", X: 300, Y: 0, Pi: "Pi 4", WiFi: "dongle", People: 2},
		{Name: "Okafor", X: 600, Y: 0, Pi: "Pi 5", WiFi: "dongle", People: 4},
		{Name: "Far", X: 5000, Y: 0, Pi: "Pi 4", WiFi: "dongle", People: 1},
	} {
		if _, err := v.AddHouse(h); err != nil {
			t.Fatal(err)
		}
	}
	v.Start()
	return v
}

func TestPostReachesItsGroup(t *testing.T) {
	v := smallVillage(t, 1)
	p, err := v.Post("Miller", "Water point open until 18:00", 5)
	if err != nil {
		t.Fatal(err)
	}
	v.RunFor(5 * Second)
	for _, h := range []string{"Fern", "Okafor"} {
		at, ok := p.Arrived[h]
		if !ok {
			t.Fatalf("%s never got the post", h)
		}
		if at-p.Created > 500*Ms {
			t.Fatalf("%s got it after %d µs", h, at-p.Created)
		}
	}
	if _, ok := p.Arrived["Far"]; ok {
		t.Fatal("a house out of WiFi range got the post")
	}
}

func TestPowerCutCatchUp(t *testing.T) {
	v := smallVillage(t, 2)
	if err := v.SetPower("Okafor", false); err != nil {
		t.Fatal(err)
	}
	p, _ := v.Post("Miller", "while Okafor was off", 5)
	v.RunFor(30 * Second)
	if _, ok := p.Arrived["Okafor"]; ok {
		t.Fatal("a house without power got the post")
	}
	v.SetPower("Okafor", true)
	v.RunFor(15 * Second) // a few heartbeats
	if _, ok := p.Arrived["Okafor"]; !ok {
		t.Fatal("Okafor did not catch up after power returned")
	}
}

// Review focus 1.
func TestDeterministic(t *testing.T) {
	run := func() []map[string]int64 {
		v := smallVillage(t, 7)
		for i := 0; i < 20; i++ {
			v.Post([]string{"Miller", "Fern", "Okafor"}[i%3], "hello", 5)
			v.RunFor(700 * Ms)
		}
		v.RunFor(20 * Second)
		var out []map[string]int64
		for _, p := range v.Posts {
			out = append(out, p.Arrived)
		}
		return out
	}
	if a, b := run(), run(); !reflect.DeepEqual(a, b) {
		t.Fatal("two runs with the same seed differ")
	}
}

func TestErrors(t *testing.T) {
	v := smallVillage(t, 1)
	if _, err := v.AddHouse(HouseSpec{Name: "Miller", Pi: "Pi 4", WiFi: "dongle"}); err == nil {
		t.Error("duplicate house name accepted")
	}
	if _, err := v.Post("Nobody", "x", 5); err == nil {
		t.Error("post from an unknown house accepted")
	}
	if err := v.SetPower("Nobody", false); err == nil {
		t.Error("power change for an unknown node accepted")
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/sim/ -run 'Post|Power|Determ|Errors'`
Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/sim/village.go`

```go
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
```

- [ ] **Step 4: Run to see it pass, then repeat for determinism**

```bash
go test -race ./internal/sim/
go test -count=3 ./internal/sim/ -run Determ
```

Expected: `ok` both times.

- [ ] **Step 5: Commit**

```bash
git add internal/sim
git commit -m "sim: a village of real nodes (ledger, author, syncer) on the WiFi world, with posts, power cuts and arrival times"
```

---

### Task 5: Scenarios

**Files:**
- Create: `internal/sim/scenario.go`, `internal/sim/scenario_test.go`

**Interfaces:**
- Produces:
  ```go
  type Duration int64 // µs; JSON as "90s", "5m"
  type Scenario struct {
  	Name     string      `json:"name"`
  	Seed     int64       `json:"seed"`
  	Duration Duration    `json:"duration"`
  	Start    string      `json:"start"`   // "20:00", the clock at time 0
  	Houses   []HouseSpec `json:"houses"`
  	Relays   []RelaySpec `json:"relays"`
  	Generate *Generate   `json:"generate"`
  	Traffic  Traffic     `json:"traffic"`
  	Events   []Event     `json:"events"`
  	Checks   []Check     `json:"checks"`
  }
  type Generate struct{ Houses, Hamlets int; Width, Height, Spread, DongleShare float64; MaxPeople int } // json lower-case names
  type Traffic struct{ PerPersonPerHour float64 `json:"perPersonPerHour"` }
  type Event struct{ At Duration; Kind, House, Text string; On *bool; For Duration; Near *Circle; Within Duration; Prio uint8 }
  type Circle struct{ X, Y, R float64 }
  type Check struct{ Kind string; Within Duration }
  func LoadScenario(b []byte) (*Scenario, error)
  func (sc *Scenario) Build() (*Village, error) // houses, relays, traffic and events scheduled; not yet run
  ```

- [ ] **Step 1: Write the failing tests** — `internal/sim/scenario_test.go`

```go
package sim

import (
	"strings"
	"testing"
)

const street = `{
  "name": "street", "seed": 3, "duration": "10m", "start": "20:00",
  "houses": [
    {"name": "Miller", "x": 0,   "y": 0, "pi": "Pi 4", "wifi": "dongle", "people": 3},
    {"name": "Fern",   "x": 250, "y": 0, "pi": "Pi 4", "wifi": "dongle", "people": 2},
    {"name": "Okafor", "x": 500, "y": 0, "pi": "Pi 5", "wifi": "dongle", "people": 4}
  ],
  "traffic": {"perPersonPerHour": 6},
  "events": [
    {"at": "1m", "kind": "post", "house": "Fern", "text": "Bread at the bakery", "prio": 8},
    {"at": "2m", "kind": "power", "house": "Okafor", "on": false, "for": "3m"},
    {"at": "6m", "kind": "burst", "near": {"x": 250, "y": 0, "r": 400}, "within": "30s", "text": "Did you hear that bang?"}
  ],
  "checks": [{"kind": "converged"}, {"kind": "delivered", "within": "2m"}]
}`

func TestLoadAndBuild(t *testing.T) {
	sc, err := LoadScenario([]byte(street))
	if err != nil {
		t.Fatal(err)
	}
	if sc.Duration != Duration(10*Minute) || len(sc.Events) != 3 || sc.Events[1].For != Duration(3*Minute) {
		t.Fatalf("parsed %+v", sc)
	}
	v, err := sc.Build()
	if err != nil {
		t.Fatal(err)
	}
	v.RunFor(int64(sc.Duration))
	burst := 0
	for _, p := range v.Posts {
		if p.Text == "Did you hear that bang?" {
			burst++
		}
	}
	if burst != 9 { // everyone in the three houses
		t.Fatalf("burst posts = %d, want 9", burst)
	}
	if v.Node("Okafor").PoweredAll {
		t.Fatal("power event did not run")
	}
}

func TestGenerator(t *testing.T) {
	sc, err := LoadScenario([]byte(`{"name":"gen","seed":5,"duration":"1m",
	  "generate":{"houses":30,"hamlets":3,"width":1400,"height":900,"spread":150,"dongleShare":0.5,"maxPeople":6}}`))
	if err != nil {
		t.Fatal(err)
	}
	v, err := sc.Build()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(v.Nodes); n != 30 {
		t.Fatalf("generated %d houses, want 30", n)
	}
	again, _ := sc.Build()
	if v.Nodes[17].House != again.Nodes[17].House {
		t.Fatal("the generator is not deterministic")
	}
}

// Review focus 5.
func TestScenarioErrors(t *testing.T) {
	for name, js := range map[string]string{
		"bad json":       `{`,
		"bad duration":   `{"name":"x","duration":"soon","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}]}`,
		"no houses":      `{"name":"x","duration":"1m"}`,
		"unknown house":  `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"events":[{"at":"1s","kind":"post","house":"B","text":"x"}]}`,
		"unknown kind":   `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"events":[{"at":"1s","kind":"party"}]}`,
		"unknown check":  `{"name":"x","duration":"1m","houses":[{"name":"A","pi":"Pi 4","wifi":"dongle"}],"checks":[{"kind":"vibes"}]}`,
	} {
		sc, err := LoadScenario([]byte(js))
		if err == nil {
			_, err = sc.Build()
		}
		if err == nil {
			t.Errorf("%s: no error", name)
		} else if strings.Contains(err.Error(), "panic") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/sim/ -run 'Load|Generator|ScenarioErrors'`
Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/sim/scenario.go`

```go
package sim

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Duration is virtual time in µs, written in JSON like "90s" or "5m".
type Duration int64

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"5m\": %s", b)
	}
	x, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("duration %q: %v", s, err)
	}
	*d = Duration(x.Microseconds())
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal((time.Duration(d) * time.Microsecond).String())
}

// Scenario is a village, what its people do, and what must hold.
type Scenario struct {
	Name     string      `json:"name"`
	Seed     int64       `json:"seed"`
	Duration Duration    `json:"duration"`
	Start    string      `json:"start"`
	Houses   []HouseSpec `json:"houses"`
	Relays   []RelaySpec `json:"relays"`
	Generate *Generate   `json:"generate"`
	Traffic  Traffic     `json:"traffic"`
	Events   []Event     `json:"events"`
	Checks   []Check     `json:"checks"`
}

// Generate places houses in hamlets, like the JavaScript model.
type Generate struct {
	Houses      int     `json:"houses"`
	Hamlets     int     `json:"hamlets"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Spread      float64 `json:"spread"`
	DongleShare float64 `json:"dongleShare"`
	MaxPeople   int     `json:"maxPeople"`
}

// Traffic is background chatter: everyone posts at this rate, at random.
type Traffic struct {
	PerPersonPerHour float64 `json:"perPersonPerHour"`
}

// Event is something scripted: a post, a power change, or a burst.
type Event struct {
	At     Duration `json:"at"`
	Kind   string   `json:"kind"` // post, power, burst
	House  string   `json:"house"`
	Text   string   `json:"text"`
	Prio   uint8    `json:"prio"`
	On     *bool    `json:"on"`
	For    Duration `json:"for"`
	Near   *Circle  `json:"near"`
	Within Duration `json:"within"`
}

// Circle is an area in metres.
type Circle struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	R float64 `json:"r"`
}

// Check is something that must hold at the end of the run.
type Check struct {
	Kind   string   `json:"kind"` // converged, delivered
	Within Duration `json:"within"`
}

var houseNames = []string{"Miller", "Fern", "Okafor", "Novak", "Haddad", "Brandt", "Kowalski", "Moreau", "Lindqvist", "Santos", "Petrov", "Weber", "Osei", "Tanaka", "Rossi", "Byrne", "Hoffmann", "Dubois", "Varga", "Nakamura"}

var chatter = []string{"Water point open until 18:00", "Anyone have spare batteries?", "Road to the bridge is blocked", "Community kitchen at the school from 12:00", "Generator at the fire station charges phones 9-11", "Please boil tap water"}

// LoadScenario parses and checks a scenario file.
func LoadScenario(b []byte) (*Scenario, error) {
	var sc Scenario
	if err := json.Unmarshal(b, &sc); err != nil {
		return nil, fmt.Errorf("scenario: %v", err)
	}
	if sc.Duration <= 0 {
		return nil, fmt.Errorf("scenario %q: duration missing", sc.Name)
	}
	if len(sc.Houses) == 0 && (sc.Generate == nil || sc.Generate.Houses <= 0) {
		return nil, fmt.Errorf("scenario %q: no houses and no generator", sc.Name)
	}
	for _, c := range sc.Checks {
		if c.Kind != "converged" && c.Kind != "delivered" {
			return nil, fmt.Errorf("scenario %q: unknown check %q", sc.Name, c.Kind)
		}
	}
	return &sc, nil
}

// epochMs turns Start ("20:00") into a wall clock on a fixed day.
func (sc *Scenario) epochMs() int64 {
	base := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	if h, m, ok := strings.Cut(sc.Start, ":"); ok {
		hh, _ := strconv.Atoi(h)
		mm, _ := strconv.Atoi(m)
		base = base.Add(time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute)
	}
	return base.UnixMilli()
}

func (sc *Scenario) houses() []HouseSpec {
	if sc.Generate == nil {
		return sc.Houses
	}
	g := sc.Generate
	rnd := rand.New(rand.NewSource(sc.Seed*7919 + 1))
	type pt struct{ x, y float64 }
	centres := make([]pt, max(1, g.Hamlets))
	for i := range centres {
		centres[i] = pt{g.Width * (0.15 + 0.7*rnd.Float64()), g.Height * (0.15 + 0.7*rnd.Float64())}
	}
	pis := []string{"Zero 2 W", "Pi 3", "Pi 4", "Pi 4", "Pi 5"}
	out := make([]HouseSpec, 0, g.Houses)
	for i := 0; i < g.Houses; i++ {
		c := centres[i%len(centres)]
		ang, dist := rnd.Float64()*2*math.Pi, g.Spread*math.Sqrt(rnd.Float64())
		name := houseNames[i%len(houseNames)]
		if i >= len(houseNames) {
			name = fmt.Sprintf("%s %d", name, i/len(houseNames)+1)
		}
		wifi := "builtin"
		if rnd.Float64() < g.DongleShare {
			wifi = "dongle"
		}
		out = append(out, HouseSpec{Name: name, X: math.Round(c.x + dist*math.Cos(ang)), Y: math.Round(c.y + dist*math.Sin(ang)),
			Pi: pis[rnd.Intn(len(pis))], WiFi: wifi, People: 1 + rnd.Intn(max(1, g.MaxPeople))})
	}
	return out
}

// Build makes the village and schedules traffic and events; nothing has run yet.
func (sc *Scenario) Build() (*Village, error) {
	v, err := NewVillage(sc.Seed, sc.epochMs())
	if err != nil {
		return nil, err
	}
	for _, h := range sc.houses() {
		if h.People == 0 {
			h.People = 1
		}
		if _, err := v.AddHouse(h); err != nil {
			return nil, err
		}
	}
	for _, r := range sc.Relays {
		if _, err := v.AddRelay(r); err != nil {
			return nil, err
		}
	}
	for i, e := range sc.Events {
		if err := sc.schedule(v, e); err != nil {
			return nil, fmt.Errorf("scenario %q, event %d: %v", sc.Name, i+1, err)
		}
	}
	sc.scheduleTraffic(v)
	v.Start()
	return v, nil
}

func (sc *Scenario) schedule(v *Village, e Event) error {
	at := int64(e.At)
	prio := e.Prio
	if prio == 0 {
		prio = 5
	}
	switch e.Kind {
	case "post":
		if v.Node(e.House) == nil {
			return fmt.Errorf("no house %q", e.House)
		}
		v.Sch.At(at, func() { v.Post(e.House, e.Text, prio) })
	case "power":
		if v.Node(e.House) == nil || e.On == nil {
			return fmt.Errorf("power needs a known house and \"on\"")
		}
		on := *e.On
		v.Sch.At(at, func() { v.SetPower(e.House, on) })
		if e.For > 0 {
			v.Sch.At(at+int64(e.For), func() { v.SetPower(e.House, !on) })
		}
	case "burst":
		if e.Near == nil || e.Within <= 0 {
			return fmt.Errorf("burst needs \"near\" and \"within\"")
		}
		for _, n := range v.Nodes {
			if n.Kind != "house" || math.Hypot(n.House.X-e.Near.X, n.House.Y-e.Near.Y) > e.Near.R {
				continue
			}
			for p := 0; p < n.House.People; p++ {
				name := n.Name
				v.Sch.At(at+v.rnd.Int63n(int64(e.Within)), func() { v.Post(name, e.Text, prio) })
			}
		}
	default:
		return fmt.Errorf("unknown event kind %q", e.Kind)
	}
	return nil
}

// scheduleTraffic gives every person a Poisson stream of posts.
func (sc *Scenario) scheduleTraffic(v *Village) {
	rate := sc.Traffic.PerPersonPerHour
	if rate <= 0 {
		return
	}
	mean := float64(time.Hour.Microseconds()) / rate
	for _, n := range v.Nodes {
		if n.Kind != "house" {
			continue
		}
		for p := 0; p < n.House.People; p++ {
			name := n.Name
			var next func()
			next = func() {
				if v.Node(name).Radio.On {
					v.Post(name, chatter[v.rnd.Intn(len(chatter))], 5)
				}
				v.Sch.After(int64(v.rnd.ExpFloat64()*mean), next)
			}
			v.Sch.After(int64(v.rnd.ExpFloat64()*mean), next)
		}
	}
}
```

- [ ] **Step 4: Run to see it pass**

Run: `go test -race ./internal/sim/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/sim
git commit -m "sim: scenario files - houses or a generator, background traffic, scripted posts, power cuts and bursts"
```

---

### Task 6: Running a scenario and judging it

**Files:**
- Create: `internal/sim/report.go`, `internal/sim/report_test.go`

**Interfaces:**
- Produces:
  ```go
  type CheckResult struct{ Kind string; Pass bool; Detail string }
  type Report struct {
  	Scenario string; Seed int64; Houses, Relays, Posts, Deliveries int
  	LatencyP50Ms, LatencyP95Ms, LatencyMaxMs float64
  	Frames, FramesLost int; MaxChannelUse float64
  	Checks []CheckResult; Pass bool
  }
  func Judge(sc *Scenario, v *Village) *Report
  func Run(sc *Scenario) (*Report, *Village, error)
  func (r *Report) String() string
  ```

  Check semantics: **converged** — at the end, every powered node in each WiFi group holds the same vector hash. **delivered within W** — every post reaches every house in its sender's final WiFi group by the end; and for houses that never lost power, within W of being sent. Deliveries count arrivals at houses other than the sender.

- [ ] **Step 1: Write the failing tests** — `internal/sim/report_test.go`

```go
package sim

import (
	"strings"
	"testing"
)

func TestStreetPasses(t *testing.T) {
	sc, _ := LoadScenario([]byte(street))
	r, _, err := Run(sc)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Pass {
		t.Fatalf("street failed:\n%s", r)
	}
	if r.Posts == 0 || r.Deliveries == 0 || r.LatencyP50Ms <= 0 {
		t.Fatalf("empty report: %+v", r)
	}
	if !strings.Contains(r.String(), "converged") {
		t.Fatalf("report text lacks the checks:\n%s", r)
	}
}

func TestCheckCatchesAMiss(t *testing.T) {
	sc, _ := LoadScenario([]byte(street))
	v, _ := sc.Build()
	v.RunFor(int64(sc.Duration))
	// tamper: pretend Okafor never got the first post
	delete(v.Posts[0].Arrived, "Okafor")
	if r := Judge(sc, v); r.Pass {
		t.Fatal("a missing delivery passed the checks")
	}
}

// Review focus 1.
func TestRunTwiceSameReport(t *testing.T) {
	sc, _ := LoadScenario([]byte(street))
	a, _, _ := Run(sc)
	b, _, _ := Run(sc)
	if a.String() != b.String() {
		t.Fatalf("two runs differ:\n%s\n---\n%s", a, b)
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/sim/ -run 'Street|Miss|Twice'`
Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/sim/report.go`

```go
package sim

import (
	"fmt"
	"sort"
	"strings"
)

// CheckResult is one check's verdict.
type CheckResult struct {
	Kind   string
	Pass   bool
	Detail string
}

// Report summarises a run.
type Report struct {
	Scenario                                  string
	Seed                                      int64
	Houses, Relays, Posts, Deliveries         int
	LatencyP50Ms, LatencyP95Ms, LatencyMaxMs  float64
	Frames, FramesLost                        int
	MaxChannelUse                             float64
	Checks                                    []CheckResult
	Pass                                      bool
}

// Run builds and runs a scenario to its end and judges it.
func Run(sc *Scenario) (*Report, *Village, error) {
	v, err := sc.Build()
	if err != nil {
		return nil, nil, err
	}
	v.RunFor(int64(sc.Duration))
	return Judge(sc, v), v, nil
}

// Judge measures a finished (or paused) village against the scenario's checks.
func Judge(sc *Scenario, v *Village) *Report {
	r := &Report{Scenario: sc.Name, Seed: sc.Seed, Posts: len(v.Posts), Frames: v.Air.Stats.Frames,
		FramesLost: v.Air.Stats.Lost, MaxChannelUse: v.Air.Stats.MaxUtil, Pass: true}
	for _, n := range v.Nodes {
		if n.Kind == "house" {
			r.Houses++
		} else {
			r.Relays++
		}
	}
	var lat []float64
	for _, p := range v.Posts {
		for h, at := range p.Arrived {
			if h != p.House {
				r.Deliveries++
				lat = append(lat, float64(at-p.Created)/float64(Ms))
			}
		}
	}
	sort.Float64s(lat)
	if len(lat) > 0 {
		r.LatencyP50Ms = lat[len(lat)/2]
		r.LatencyP95Ms = lat[min(len(lat)-1, len(lat)*95/100)]
		r.LatencyMaxMs = lat[len(lat)-1]
	}
	group := map[string]int{} // node name → final WiFi group
	for i, g := range v.Air.Groups() {
		for _, n := range g {
			group[n.Name] = i
		}
	}
	for _, c := range sc.Checks {
		var res CheckResult
		switch c.Kind {
		case "converged":
			res = checkConverged(v, group)
		case "delivered":
			res = checkDelivered(v, group, int64(c.Within))
		}
		r.Checks = append(r.Checks, res)
		r.Pass = r.Pass && res.Pass
	}
	return r
}

func checkConverged(v *Village, group map[string]int) CheckResult {
	hashes := map[int][32]byte{}
	for _, n := range v.Nodes {
		g, ok := group[n.Name]
		if !ok {
			continue // no power at the end
		}
		vec, _ := n.Store.Vector()
		h := vec.Hash()
		if first, seen := hashes[g]; !seen {
			hashes[g] = h
		} else if first != h {
			return CheckResult{Kind: "converged", Detail: fmt.Sprintf("%s differs from the rest of its WiFi group", n.Name)}
		}
	}
	return CheckResult{Kind: "converged", Pass: true, Detail: fmt.Sprintf("%d WiFi groups each agree", len(hashes))}
}

func checkDelivered(v *Village, group map[string]int, within int64) CheckResult {
	missing, late := 0, 0
	example := ""
	for _, p := range v.Posts {
		g, ok := group[p.House]
		if !ok {
			continue // sender off at the end: its group is unknown
		}
		for _, n := range v.Nodes {
			if n.Kind != "house" || n.Name == p.House {
				continue
			}
			if gg, ok := group[n.Name]; !ok || gg != g {
				continue
			}
			at, got := p.Arrived[n.Name]
			switch {
			case !got:
				missing++
				if example == "" {
					example = fmt.Sprintf("%q from %s never reached %s", p.Text, p.House, n.Name)
				}
			case within > 0 && n.PoweredAll && at-p.Created > within:
				late++
				if example == "" {
					example = fmt.Sprintf("%q from %s reached %s after %.1f s", p.Text, p.House, n.Name, float64(at-p.Created)/float64(Second))
				}
			}
		}
	}
	res := CheckResult{Kind: "delivered", Pass: missing == 0 && late == 0}
	res.Detail = fmt.Sprintf("%d missing, %d late (limit %s)", missing, late, fmtDur(within))
	if example != "" {
		res.Detail += "; e.g. " + example
	}
	return res
}

func fmtDur(us int64) string {
	if us <= 0 {
		return "none"
	}
	return fmt.Sprintf("%gs", float64(us)/float64(Second))
}

// String is the report as people read it.
func (r *Report) String() string {
	var b strings.Builder
	verdict := "PASS"
	if !r.Pass {
		verdict = "FAIL"
	}
	fmt.Fprintf(&b, "%s  %s (seed %d): %d houses, %d relays, %d posts, %d deliveries\n", verdict, r.Scenario, r.Seed, r.Houses, r.Relays, r.Posts, r.Deliveries)
	fmt.Fprintf(&b, "      delivery time: median %.0f ms, slowest 5%% %.0f ms, worst %.0f ms\n", r.LatencyP50Ms, r.LatencyP95Ms, r.LatencyMaxMs)
	fmt.Fprintf(&b, "      WiFi frames: %d, lost to receivers: %d, busiest channel %.0f%%\n", r.Frames, r.FramesLost, r.MaxChannelUse*100)
	for _, c := range r.Checks {
		mark := "ok  "
		if !c.Pass {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "      %s %s: %s\n", mark, c.Kind, c.Detail)
	}
	return b.String()
}
```

- [ ] **Step 4: Run to see it pass**

Run: `go test -race ./internal/sim/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/sim
git commit -m "sim: run a scenario to the end and judge it - convergence, delivery within a limit, latency and channel use"
```

---

### Task 7: `batnetd -sim`, scenario files, CI

**Files:**
- Modify: `cmd/batnetd/main.go`, `.github/workflows/ci.yml`
- Create: `scenarios/street.json`, `scenarios/bang.json`, `scenarios/storm.json`, `scenarios/index.json`

**Interfaces:**
- Consumes: `sim.LoadScenario`, `sim.Run`, `(*Report).String`.
- Produces: `go run ./cmd/batnetd -sim scenarios/bang.json [more.json …]` → prints each report; exit status 1 if any fails.

- [ ] **Step 1: Write the scenarios**

`scenarios/street.json` — the scenario from Task 5's test (`street` constant), as a file.

`scenarios/bang.json`:

```json
{
  "name": "bang",
  "about": "40 houses in 4 hamlets. At 20:05 everyone within 700 m of the school hears a bang and posts within 30 seconds, on top of normal chatter.",
  "seed": 11,
  "duration": "20m",
  "start": "20:00",
  "generate": {"houses": 40, "hamlets": 4, "width": 1800, "height": 1200, "spread": 160, "dongleShare": 0.7, "maxPeople": 6},
  "relays": [
    {"name": "Relay north", "x": 900, "y": 350, "pi": "Zero 2 W", "wifi": "pole"},
    {"name": "Relay south", "x": 900, "y": 850, "pi": "Zero 2 W", "wifi": "pole"}
  ],
  "traffic": {"perPersonPerHour": 3},
  "events": [
    {"at": "5m", "kind": "burst", "near": {"x": 900, "y": 600, "r": 700}, "within": "30s", "text": "Did you hear that bang?"}
  ],
  "checks": [{"kind": "converged"}, {"kind": "delivered", "within": "2m"}]
}
```

`scenarios/storm.json`:

```json
{
  "name": "storm",
  "about": "A street of 8 houses loses power house by house and comes back; everyone must catch up.",
  "seed": 5,
  "duration": "30m",
  "start": "02:00",
  "houses": [
    {"name": "Miller", "x": 0, "y": 0, "pi": "Pi 4", "wifi": "dongle", "people": 3},
    {"name": "Fern", "x": 200, "y": 40, "pi": "Pi 4", "wifi": "dongle", "people": 2},
    {"name": "Okafor", "x": 400, "y": 0, "pi": "Pi 5", "wifi": "dongle", "people": 5},
    {"name": "Novak", "x": 600, "y": 60, "pi": "Pi 3", "wifi": "dongle", "people": 2},
    {"name": "Haddad", "x": 800, "y": 0, "pi": "Pi 4", "wifi": "dongle", "people": 4},
    {"name": "Brandt", "x": 1000, "y": 30, "pi": "Pi 4", "wifi": "dongle", "people": 3},
    {"name": "Kowalski", "x": 1200, "y": 0, "pi": "Pi 5", "wifi": "dongle", "people": 2},
    {"name": "Moreau", "x": 1400, "y": 50, "pi": "Pi 4", "wifi": "dongle", "people": 3}
  ],
  "traffic": {"perPersonPerHour": 4},
  "events": [
    {"at": "3m", "kind": "power", "house": "Okafor", "on": false, "for": "8m"},
    {"at": "6m", "kind": "power", "house": "Haddad", "on": false, "for": "10m"},
    {"at": "7m", "kind": "post", "house": "Miller", "text": "Tree down on the low road", "prio": 8},
    {"at": "12m", "kind": "power", "house": "Fern", "on": false, "for": "5m"}
  ],
  "checks": [{"kind": "converged"}, {"kind": "delivered", "within": "1m"}]
}
```

`scenarios/index.json`:

```json
["street.json", "bang.json", "storm.json"]
```

- [ ] **Step 2: Add `-sim` to `cmd/batnetd/main.go`**

Add the flag in `main`:

```go
	simFiles := flag.Bool("sim", false, "run the scenario files given as arguments and judge them")
```

and before the `-mock` usage check:

```go
	if *simFiles {
		os.Exit(runScenarios(flag.Args()))
	}
```

Add, with `"github.com/johnusher/batnet/internal/sim"` imported:

```go
// runScenarios runs each scenario file and prints its report; 1 if any fails.
func runScenarios(paths []string) int {
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: batnetd -sim scenarios/bang.json [more.json ...]")
		return 2
	}
	status := 0
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		sc, err := sim.LoadScenario(b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			return 2
		}
		t0 := time.Now()
		r, _, err := sim.Run(sc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			return 2
		}
		fmt.Print(r)
		fmt.Printf("      (%s simulated in %s)\n\n", time.Duration(sc.Duration)*time.Microsecond, time.Since(t0).Round(time.Millisecond))
		if !r.Pass {
			status = 1
		}
	}
	return status
}
```

- [ ] **Step 3: Run the scenarios**

Run: `go run ./cmd/batnetd -sim scenarios/street.json scenarios/storm.json scenarios/bang.json`
Expected: three reports, each `PASS`, and the bang report showing a busy channel (well above the others) and lost frames that repair made up for. If `bang` fails a check, that is a finding about batnet, not the scenario: record it (Ruling or a ledger note) and decide with the spec; do not loosen the check silently.

- [ ] **Step 4: Run scenarios in CI** — in `.github/workflows/ci.yml`, after the Test step:

```yaml
      - name: Village scenarios (the real code, simulated)
        run: go run ./cmd/batnetd -sim scenarios/*.json
```

- [ ] **Step 5: Commit**

```bash
git add cmd/batnetd scenarios .github/workflows/ci.yml
git commit -m "batnetd -sim: run village scenarios with the real code; street, storm and bang; scenarios in CI"
```

---

### Task 8: The API for the page, and the WebAssembly build

**Files:**
- Create: `internal/sim/api.go`, `internal/sim/api_test.go`, `cmd/batnetwasm/main.go`
- Modify: `.github/workflows/pages.yml`

**Interfaces:**
- Produces:
  ```go
  type API struct{ /* scenario, village, report cursor */ }
  func (a *API) Load(scenarioJSON []byte) error
  func (a *API) Step(ms int64)
  func (a *API) State() []byte        // JSON, see below
  func (a *API) Post(house, text string, prio int) error
  func (a *API) Power(name string, on bool) error
  func (a *API) Report() []byte       // JSON of *Report
  ```
  State JSON: `{"now": µs, "clock": "20:05:12", "end": µs, "nodes": [{"name","kind","x","y","on","range","pi","wifi","people","entries","group"}], "links": [["a","b"]], "frames": [{"from","to":[…],"at"}] (since the last State, at most 300), "posts": [{"house","text","prio","created","arrived"}] (the last 30), "stats": {"posts","deliveries","p50","p95","frames","lost","channel"}}`.
  JavaScript (from `cmd/batnetwasm`): `globalThis.batnetSim = {load(json) → "" | error, step(ms), state() → string, post(house, text, prio) → "" | error, power(name, on) → "" | error, report() → string}`.

- [ ] **Step 1: Write the failing test** — `internal/sim/api_test.go`

```go
package sim

import (
	"encoding/json"
	"testing"
)

func TestAPI(t *testing.T) {
	var a API
	if err := a.Load([]byte(street)); err != nil {
		t.Fatal(err)
	}
	a.Step(90_000) // 90 s: the 1-minute post has happened
	var st struct {
		Now   int64 `json:"now"`
		Clock string `json:"clock"`
		Nodes []struct {
			Name    string `json:"name"`
			Entries int    `json:"entries"`
		} `json:"nodes"`
		Links  [][2]string `json:"links"`
		Frames []any       `json:"frames"`
		Posts  []struct {
			House   string `json:"house"`
			Arrived int    `json:"arrived"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(a.State(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Now != 90*Second || st.Clock != "20:01:30" || len(st.Nodes) != 3 || len(st.Links) == 0 || len(st.Frames) == 0 || len(st.Posts) == 0 {
		t.Fatalf("state %+v", st)
	}
	if err := a.Post("Miller", "from the page", 8); err != nil {
		t.Fatal(err)
	}
	if err := a.Power("Fern", false); err != nil {
		t.Fatal(err)
	}
	if err := a.Post("Nobody", "x", 5); err == nil {
		t.Fatal("post from an unknown house accepted")
	}
	var r Report
	if err := json.Unmarshal(a.Report(), &r); err != nil || r.Scenario != "street" {
		t.Fatalf("report %+v, %v", r, err)
	}
	if err := a.Load([]byte(`{`)); err == nil {
		t.Fatal("bad scenario accepted")
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/sim/ -run API`
Expected: FAIL, `undefined: API`.

- [ ] **Step 3: Implement** — `internal/sim/api.go`

```go
package sim

import (
	"encoding/json"
	"errors"
	"time"
)

// API is what the simulator page calls through WebAssembly.
type API struct {
	sc     *Scenario
	v      *Village
	frames []frameJSON
}

type frameJSON struct {
	From string   `json:"from"`
	To   []string `json:"to"`
	At   int64    `json:"at"`
}

// Load builds a scenario's village at time 0.
func (a *API) Load(scenarioJSON []byte) error {
	sc, err := LoadScenario(scenarioJSON)
	if err != nil {
		return err
	}
	v, err := sc.Build()
	if err != nil {
		return err
	}
	a.sc, a.v, a.frames = sc, v, nil
	v.Air.OnBroadcast = func(from string, to []string, at int64) {
		if len(a.frames) < 300 {
			a.frames = append(a.frames, frameJSON{From: from, To: to, At: at})
		}
	}
	return nil
}

var errNotLoaded = errors.New("sim: no scenario loaded")

// Step advances virtual time by ms milliseconds.
func (a *API) Step(ms int64) {
	if a.v != nil {
		a.v.RunFor(ms * Ms)
	}
}

// State is everything the page draws, as JSON. Frames are those since the last call.
func (a *API) State() []byte {
	if a.v == nil {
		return []byte(`{"error":"no scenario loaded"}`)
	}
	v := a.v
	type nodeJSON struct {
		Name    string  `json:"name"`
		Kind    string  `json:"kind"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		On      bool    `json:"on"`
		Range   float64 `json:"range"`
		Pi      string  `json:"pi"`
		WiFi    string  `json:"wifi"`
		People  int     `json:"people"`
		Entries int     `json:"entries"`
		Group   int     `json:"group"`
	}
	group := map[string]int{}
	for i, g := range v.Air.Groups() {
		for _, n := range g {
			group[n.Name] = i
		}
	}
	st := struct {
		Now    int64       `json:"now"`
		Clock  string      `json:"clock"`
		End    int64       `json:"end"`
		Nodes  []nodeJSON  `json:"nodes"`
		Links  [][2]string `json:"links"`
		Frames []frameJSON `json:"frames"`
		Posts  []any       `json:"posts"`
		Stats  any         `json:"stats"`
	}{Now: v.Sch.Now(), End: int64(a.sc.Duration), Frames: a.frames}
	st.Clock = time.UnixMilli(v.EpochMs + v.Sch.Now()/Ms).UTC().Format("15:04:05")
	for _, n := range v.Nodes {
		all, _ := n.Store.All()
		g, ok := group[n.Name]
		if !ok {
			g = -1
		}
		nj := nodeJSON{Name: n.Name, Kind: n.Kind, X: n.Radio.X, Y: n.Radio.Y, On: n.Radio.On, Range: n.Radio.Range, Entries: len(all), Group: g}
		if n.Kind == "house" {
			nj.Pi, nj.WiFi, nj.People = n.House.Pi, n.House.WiFi, n.House.People
		}
		st.Nodes = append(st.Nodes, nj)
	}
	for i, p := range v.Air.nodes {
		for _, q := range v.Air.nodes[i+1:] {
			if v.Air.Linked(p, q) {
				st.Links = append(st.Links, [2]string{p.Name, q.Name})
			}
		}
	}
	from := max(0, len(v.Posts)-30)
	for _, p := range v.Posts[from:] {
		st.Posts = append(st.Posts, map[string]any{"house": p.House, "text": p.Text, "prio": p.Prio, "created": p.Created, "arrived": len(p.Arrived) - 1})
	}
	r := Judge(a.sc, v)
	st.Stats = map[string]any{"posts": r.Posts, "deliveries": r.Deliveries, "p50": r.LatencyP50Ms, "p95": r.LatencyP95Ms,
		"frames": r.Frames, "lost": r.FramesLost, "channel": r.MaxChannelUse}
	a.frames = nil
	b, _ := json.Marshal(st)
	return b
}

// Post makes a house post now.
func (a *API) Post(house, text string, prio int) error {
	if a.v == nil {
		return errNotLoaded
	}
	if prio < 0 || prio > 9 {
		return errors.New("sim: priority must be 0-9")
	}
	_, err := a.v.Post(house, text, uint8(prio))
	return err
}

// Power cuts or restores a node's power.
func (a *API) Power(name string, on bool) error {
	if a.v == nil {
		return errNotLoaded
	}
	return a.v.SetPower(name, on)
}

// Report judges the village as it stands now.
func (a *API) Report() []byte {
	if a.v == nil {
		return []byte(`{"error":"no scenario loaded"}`)
	}
	b, _ := json.Marshal(Judge(a.sc, a.v))
	return b
}
```

`cmd/batnetwasm/main.go`:

```go
//go:build js && wasm

// Command batnetwasm runs batnet's real code for a simulated village inside
// the simulator page. Build: GOOS=js GOARCH=wasm go build -o batnet.wasm ./cmd/batnetwasm
package main

import (
	"syscall/js"

	"github.com/johnusher/batnet/internal/sim"
)

func main() {
	var api sim.API
	errString := func(err error) any {
		if err != nil {
			return err.Error()
		}
		return ""
	}
	js.Global().Set("batnetSim", js.ValueOf(map[string]any{
		"load": js.FuncOf(func(_ js.Value, args []js.Value) any { return errString(api.Load([]byte(args[0].String()))) }),
		"step": js.FuncOf(func(_ js.Value, args []js.Value) any { api.Step(int64(args[0].Int())); return nil }),
		"state": js.FuncOf(func(_ js.Value, _ []js.Value) any { return string(api.State()) }),
		"post": js.FuncOf(func(_ js.Value, args []js.Value) any {
			return errString(api.Post(args[0].String(), args[1].String(), args[2].Int()))
		}),
		"power":  js.FuncOf(func(_ js.Value, args []js.Value) any { return errString(api.Power(args[0].String(), args[1].Bool())) }),
		"report": js.FuncOf(func(_ js.Value, _ []js.Value) any { return string(api.Report()) }),
	}))
	select {} // keep the functions alive
}
```

- [ ] **Step 4: Test, and check the WebAssembly build**

```bash
go test -race ./internal/sim/
GOOS=js GOARCH=wasm go build -o /tmp/batnet.wasm ./cmd/batnetwasm && ls -l /tmp/batnet.wasm
```

Expected: `ok`; a `batnet.wasm` of a few MB.

- [ ] **Step 5: Build it on Pages** — in `.github/workflows/pages.yml`, change the trigger paths to include Go code and scenarios:

```yaml
    paths: ["prototypes/village-sim/**", "internal/**", "cmd/batnetwasm/**", "scenarios/**", "go.mod", ".github/workflows/pages.yml"]
```

and replace the "Build the site" step with:

```yaml
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Build the site
        # index.html is written without a document skeleton (the artifact host adds one), so add it here.
        run: |
          mkdir -p _site/scenarios
          cp prototypes/village-sim/sim.js prototypes/village-sim/real.js _site/
          cp scenarios/*.json _site/scenarios/
          GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o _site/batnet.wasm ./cmd/batnetwasm
          cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" _site/
          {
            echo '<!doctype html>'
            echo '<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">'
            cat prototypes/village-sim/index.html
          } > _site/index.html
```

- [ ] **Step 6: Commit**

```bash
git add internal/sim cmd/batnetwasm .github/workflows/pages.yml
git commit -m "sim: a JSON API for the page and a WebAssembly build of the real code; Pages builds batnet.wasm"
```

---

### Task 9: "Real batnet code" in the simulator page

**Files:**
- Create: `prototypes/village-sim/real.js`
- Modify: `prototypes/village-sim/index.html` (an engine switch in the header; a `<section id="realview">`; script tags)

**Interfaces:**
- Consumes: `globalThis.batnetSim` (Task 8), `wasm_exec.js` (`new Go()`), `scenarios/index.json` and the scenario files.
- Produces: an "Engine" select with **Model (JavaScript)** and **Real batnet code (Go)**. In real mode the model's sections are hidden and `#realview` shows: scenario picker and its `about` text, Run/Pause and speeds (1×, 10×, 60×), the virtual clock; a map (houses sized by people, relays as triangles, WiFi links, powered-off nodes dashed with a cross, each frame drawn from sender to receivers for 1.2 s, WiFi group colours); tiles (posts, deliveries, median and slowest-5% delivery, frames lost, busiest channel); the checks with pass/fail; a house panel (click a house: its entries count, a post box with priority, a power button); the last posts with "arrived at N houses".

- [ ] **Step 1: Write `real.js`**

```js
// The "real batnet code" view: batnet's Go code, compiled to WebAssembly,
// runs a scenario's village in virtual time; this file draws it.
(() => {
  'use strict';
  const $ = (s) => document.querySelector(s);
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const css = (v) => getComputedStyle(document.documentElement).getPropertyValue(v).trim();
  let ready = null, running = false, speed = 10, last = 0, state = null, selected = null, scenarios = [], scenarioJSON = null;
  const beams = [];

  function boot() {
    if (ready) return ready;
    if (location.protocol === 'file:') {
      ready = Promise.reject(new Error('The real engine loads batnet.wasm, which browsers block on file:// pages. Run: python3 -m http.server, then open http://localhost:8000.'));
      return ready;
    }
    ready = (async () => {
      const go = new Go();
      const res = await WebAssembly.instantiateStreaming(fetch('batnet.wasm'), go.importObject);
      go.run(res.instance);
      scenarios = await (await fetch('scenarios/index.json')).json();
    })();
    return ready;
  }

  async function loadScenario(file) {
    scenarioJSON = await (await fetch('scenarios/' + file)).text();
    const err = batnetSim.load(scenarioJSON);
    if (err) throw new Error(err);
    let about = '';
    try { about = JSON.parse(scenarioJSON).about || ''; } catch (e) {}
    $('#rv-about').textContent = about;
    selected = null; beams.length = 0;
    pull();
  }

  function pull() {
    state = JSON.parse(batnetSim.state());
    const now = performance.now();
    for (const f of state.frames || []) beams.push({ from: f.from, to: f.to, t0: now });
    if (beams.length > 400) beams.splice(0, beams.length - 400);
    renderPanels();
  }

  // ---------- map ----------
  const groupColours = ['--wifi', '--lora', '--mesh', '--warn', '--ink'];
  function drawMap() {
    const cv = $('#rv-map'), ctx = cv.getContext('2d');
    const dpr = window.devicePixelRatio || 1, W = cv.clientWidth, H = cv.clientHeight;
    if (cv.width !== W * dpr) { cv.width = W * dpr; cv.height = H * dpr; }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, W, H);
    if (!state || !state.nodes) return;
    const xs = state.nodes.map((n) => n.x), ys = state.nodes.map((n) => n.y);
    const minX = Math.min(...xs) - 60, maxX = Math.max(...xs) + 60, minY = Math.min(...ys) - 60, maxY = Math.max(...ys) + 60;
    const s = Math.min(W / (maxX - minX), H / (maxY - minY));
    const ox = (W - (maxX - minX) * s) / 2, oy = (H - (maxY - minY) * s) / 2;
    const px = (n) => [ox + (n.x - minX) * s, oy + (n.y - minY) * s];
    const byName = new Map(state.nodes.map((n) => [n.name, n]));
    ctx.lineWidth = 1.5; ctx.strokeStyle = css('--wifi'); ctx.globalAlpha = 0.45;
    for (const [a, b] of state.links || []) {
      const A = byName.get(a), B = byName.get(b);
      if (!A.on || !B.on) continue;
      const [x1, y1] = px(A), [x2, y2] = px(B);
      ctx.beginPath(); ctx.moveTo(x1, y1); ctx.lineTo(x2, y2); ctx.stroke();
    }
    ctx.globalAlpha = 1;
    const now = performance.now();
    ctx.strokeStyle = css('--wifi'); ctx.lineWidth = 2;
    for (let i = beams.length - 1; i >= 0; i--) {
      const b = beams[i], age = (now - b.t0) / 1200;
      if (age > 1) { beams.splice(i, 1); continue; }
      const A = byName.get(b.from); if (!A) continue;
      const [x1, y1] = px(A);
      ctx.globalAlpha = 0.8 * (1 - age);
      for (const t of b.to) {
        const B = byName.get(t); if (!B) continue;
        const [x2, y2] = px(B), k = Math.min(1, age * 3);
        ctx.beginPath(); ctx.moveTo(x1, y1); ctx.lineTo(x1 + (x2 - x1) * k, y1 + (y2 - y1) * k); ctx.stroke();
      }
    }
    ctx.globalAlpha = 1;
    ctx.font = '600 11px ' + css('--f-mono'); ctx.textBaseline = 'middle';
    for (const n of state.nodes) {
      const [x, y] = px(n);
      const col = n.group >= 0 ? css(groupColours[n.group % groupColours.length]) : css('--muted');
      if (n.kind === 'relay') {
        ctx.beginPath(); ctx.moveTo(x, y - 8); ctx.lineTo(x + 7, y + 5); ctx.lineTo(x - 7, y + 5); ctx.closePath();
        ctx.fillStyle = n.on ? css('--muted') : css('--panel'); ctx.fill();
        continue;
      }
      const r = 6 + Math.min(n.people, 10);
      ctx.fillStyle = css('--panel'); ctx.strokeStyle = n.on ? col : css('--muted'); ctx.lineWidth = n.name === selected ? 3.5 : 2.2;
      if (!n.on) ctx.setLineDash([3, 3]);
      ctx.beginPath(); ctx.roundRect(x - r, y - r, 2 * r, 2 * r, 3); ctx.fill(); ctx.stroke(); ctx.setLineDash([]);
      if (!n.on) { ctx.strokeStyle = css('--critical'); ctx.beginPath(); ctx.moveTo(x - 4, y - 4); ctx.lineTo(x + 4, y + 4); ctx.moveTo(x + 4, y - 4); ctx.lineTo(x - 4, y + 4); ctx.stroke(); }
      ctx.fillStyle = n.on ? css('--ink') : css('--muted');
      ctx.fillText(n.name, x + r + 5, y);
    }
    cv.onclick = (e) => {
      const rect = cv.getBoundingClientRect(), mx = e.clientX - rect.left, my = e.clientY - rect.top;
      let best = null, bd = 18;
      for (const n of state.nodes) { const [x, y] = px(n), d = Math.hypot(x - mx, y - my); if (d < bd) { bd = d; best = n; } }
      selected = best ? best.name : null; renderPanels();
    };
  }

  // ---------- panels ----------
  function renderPanels() {
    if (!state || !state.stats) return;
    const st = state.stats;
    $('#rv-clock').textContent = state.clock;
    $('#rv-progress').value = state.end ? state.now / state.end : 0;
    const ms = (v) => v >= 1000 ? (v / 1000).toFixed(1) + ' s' : Math.round(v) + ' ms';
    const tiles = [['Posts', st.posts, 'real signed entries'], ['Deliveries', st.deliveries, 'arrivals at other houses'],
      ['Delivery time', ms(st.p50), 'median · slowest 5% ' + ms(st.p95)], ['WiFi frames', st.frames, st.lost + ' lost to receivers'],
      ['Busiest channel', Math.round(st.channel * 100) + '%', 'share of a second on air']];
    $('#rv-tiles').innerHTML = tiles.map(([k, v, s]) => `<div class="tile"><div class="k">${k}</div><div class="v">${v}</div><div class="s">${s}</div></div>`).join('');
    const rep = JSON.parse(batnetSim.report());
    $('#rv-checks').innerHTML = (rep.Checks || []).map((c) => `<li class="${c.Pass ? 'ok' : 'bad'}"><b>${c.Pass ? '✓' : '✗'} ${esc(c.Kind)}</b> ${esc(c.Detail)}</li>`).join('') || '<li>No checks in this scenario.</li>';
    $('#rv-posts').innerHTML = (state.posts || []).slice().reverse().slice(0, 12).map((p) => `<li><b>${esc(p.house)}</b> ${p.prio >= 8 ? '<span class="chip warn">High</span> ' : ''}${esc(p.text)} <span class="note">· at ${p.arrived} other house${p.arrived === 1 ? '' : 's'}</span></li>`).join('');
    const n = selected && state.nodes.find((x) => x.name === selected);
    $('#rv-house').innerHTML = n ? `<h3>${esc(n.name)}</h3><p class="meta">${n.kind === 'relay' ? 'Relay' : esc(n.pi) + ' · ' + esc(n.wifi) + ' · ' + n.people + ' people'} · holds ${n.entries} entries · ${n.on ? (n.group >= 0 ? 'WiFi group ' + (n.group + 1) : '') : 'no power'}</p>
      ${n.kind === 'house' ? `<form id="rv-postform" style="display:flex;gap:6px;margin:8px 0"><input class="field" id="rv-text" placeholder="Post from ${esc(n.name)}" style="flex:1"><select class="field" id="rv-prio"><option value="5">Normal</option><option value="8">High</option><option value="2">Low</option></select><button class="btn primary">Post</button></form>` : ''}
      <button class="btn" id="rv-power">${n.on ? 'Cut the power' : 'Restore power'}</button>` : '<p class="note">Click a house or relay on the map.</p>';
    if (n) {
      $('#rv-power').onclick = () => { batnetSim.power(n.name, !n.on); pull(); };
      const f = $('#rv-postform');
      if (f) f.onsubmit = (e) => { e.preventDefault(); const t = $('#rv-text').value.trim(); if (t) { const err = batnetSim.post(n.name, t, +$('#rv-prio').value); if (err) alert(err); pull(); } };
    }
  }

  // ---------- loop ----------
  function frame(t) {
    if ($('#realview').hidden) { requestAnimationFrame(frame); return; }
    const dt = Math.min(t - (last || t), 100); last = t;
    if (running && state && state.now < state.end) { batnetSim.step(Math.round(dt * speed)); pull(); }
    drawMap();
    requestAnimationFrame(frame);
  }

  async function enter() {
    $('#rv-status').textContent = 'Loading batnet.wasm…';
    try {
      await boot();
      $('#rv-scenario').innerHTML = scenarios.map((f) => `<option>${esc(f)}</option>`).join('');
      await loadScenario(scenarios[0]);
      $('#rv-status').textContent = 'batnet\'s real Go code is running in your browser (WebAssembly).';
    } catch (e) { $('#rv-status').textContent = e.message; }
  }

  window.realEngine = { enter };
  document.addEventListener('DOMContentLoaded', () => {
    $('#rv-scenario').onchange = (e) => loadScenario(e.target.value).catch((err) => { $('#rv-status').textContent = err.message; });
    $('#rv-run').onclick = () => { running = !running; $('#rv-run').textContent = running ? 'Pause' : 'Run'; };
    $('#rv-restart').onclick = () => loadScenario($('#rv-scenario').value);
    for (const b of document.querySelectorAll('#rv-speeds button')) b.onclick = () => { speed = +b.dataset.speed; for (const x of document.querySelectorAll('#rv-speeds button')) x.setAttribute('aria-pressed', x === b); };
    requestAnimationFrame(frame);
  });
})();
```

- [ ] **Step 2: Wire it into `index.html`**

1. In the header's transport area, before the scenario select, add:

```html
<select class="field" id="engine" aria-label="Engine">
  <option value="model">Model (JavaScript)</option>
  <option value="real">Real batnet code (Go)</option>
</select>
```

2. After the closing tag of `<main>` (the model's layout), add:

```html
<section id="realview" hidden aria-label="Real batnet code">
  <p class="note" id="rv-status"></p>
  <div class="rv-bar">
    <select class="field" id="rv-scenario" aria-label="Scenario"></select>
    <button class="btn primary" id="rv-run">Run</button>
    <button class="btn" id="rv-restart">Restart</button>
    <span class="seg" id="rv-speeds"><button data-speed="1">1×</button><button data-speed="10" aria-pressed="true">10×</button><button data-speed="60">60×</button></span>
    <b id="rv-clock" class="rv-clock">--:--:--</b>
    <progress id="rv-progress" max="1" value="0"></progress>
  </div>
  <p class="note" id="rv-about"></p>
  <div class="rv-grid">
    <canvas id="rv-map" aria-label="Map of the simulated village"></canvas>
    <div>
      <section class="card inspector" id="rv-house"></section>
      <section class="card"><p class="eyebrow">Checks</p><ul class="rv-checks" id="rv-checks"></ul></section>
    </div>
  </div>
  <div class="tiles" id="rv-tiles"></div>
  <section class="card"><p class="eyebrow">Latest posts</p><ul class="rv-posts" id="rv-posts"></ul></section>
</section>
```

3. Before `</style>`, add:

```css
#realview { padding: 0 24px 24px; display: flex; flex-direction: column; gap: 12px; }
.rv-bar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.rv-clock { font-family: var(--f-mono); font-size: 22px; }
.rv-grid { display: grid; grid-template-columns: minmax(0, 2fr) minmax(0, 1fr); gap: 12px; }
#rv-map { width: 100%; height: 520px; background: var(--panel); border: 1px solid var(--rule); border-radius: 10px; }
.rv-checks, .rv-posts { list-style: none; margin: 0; padding: 0; font-size: 14px; display: flex; flex-direction: column; gap: 6px; }
.rv-checks .ok b { color: var(--wifi); } .rv-checks .bad b { color: var(--critical); }
@media (max-width: 900px) { .rv-grid { grid-template-columns: 1fr; } #rv-map { height: 360px; } }
```

4. Replace `<script src="sim.js"></script>` with:

```html
<script src="sim.js"></script>
<script src="wasm_exec.js"></script>
<script src="real.js"></script>
```

5. In the page script, after the speed buttons are wired, add the switch:

```js
$('#engine').addEventListener('change', (e) => {
  const real = e.target.value === 'real';
  document.querySelector('main').hidden = real;
  $('#realview').hidden = !real;
  running = real ? false : running;
  if (real) window.realEngine.enter();
});
```

(`main` is the element wrapping the model's map, panels and phone; if the model's layout uses another wrapper, hide that one: find it with `grep -n "<main\|class=\"layout\"" index.html`.)

- [ ] **Step 3: Check it locally**

```bash
mkdir -p /tmp/batnet-site/scenarios
cp prototypes/village-sim/*.js /tmp/batnet-site/ && cp scenarios/*.json /tmp/batnet-site/scenarios/
GOOS=js GOARCH=wasm go build -o /tmp/batnet-site/batnet.wasm ./cmd/batnetwasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" /tmp/batnet-site/
{ echo '<!doctype html><html lang="en"><head><meta charset="utf-8">'; cat prototypes/village-sim/index.html; } > /tmp/batnet-site/index.html
(cd /tmp/batnet-site && python3 -m http.server 8765 &) ; sleep 1
```

Then screenshot it with headless Chrome after selecting the real engine and running 60 simulated seconds (use `--virtual-time-budget`), and look at it: the map shows houses and links, the clock has moved, tiles have numbers, checks are listed. Stop the server afterwards.

Expected: the real view renders with the `street` scenario; no console errors.

- [ ] **Step 4: Commit**

```bash
git add prototypes/village-sim/real.js prototypes/village-sim/index.html
git commit -m "Simulator: a 'Real batnet code' engine - the Go code as WebAssembly, scenarios, live map, checks and posts"
```

---

### Task 10: Docs and knowledge

**Files:**
- Modify: `README.md` (simulator section), `AGENTS.md` (path table), `knowledge/skills/catalog.md` (running-a-mock-village), `knowledge/log.md`, `knowledge/research/village-sim-findings.md`

- [ ] **Step 1:** README, in "Try the simulator", add after the bullet list:

```markdown
**Two engines.** *Model* is a quick JavaScript model of the whole system.
*Real batnet code* runs batnet's actual Go code for every house, compiled for
the browser, through prepared scenarios such as *bang* (everyone hears a bang
and posts at once) and *storm* (houses lose power and catch up). Today it covers
the WiFi mesh; radio, accounts and the app join as they are built.
```

and in "For developers", add to the code block:

```sh
go run ./cmd/batnetd -sim scenarios/*.json    # run the village scenarios with the real code
```

- [ ] **Step 2:** AGENTS.md path table: add `| \`scenarios/\` | Village scenarios (JSON) run by \`batnetd -sim\`, CI and the simulator's real engine |` and `| \`internal/sim/\` | Virtual time, the WiFi world model, villages of real nodes, scenarios, reports, the page API |`.

- [ ] **Step 3:** In `knowledge/skills/catalog.md`, change the `running-a-mock-village` row's use-when to name `batnetd -sim scenarios/*.json` and the scenario format in `internal/sim/scenario.go`.

- [ ] **Step 4:** In `knowledge/research/village-sim-findings.md`, add a dated section with the three scenario reports' key numbers (copied from Task 7's run), labelled "real code, modelled WiFi", and in `knowledge/log.md` a bullet under 2026-10-15.

- [ ] **Step 5:** Run everything and commit

```bash
go vet ./... && go test -race ./... && go run ./cmd/batnetd -sim scenarios/*.json && (cd tools/okflint && go run . ../../knowledge)
git add README.md AGENTS.md knowledge
git commit -m "Docs: the real-code engine, scenarios and how to run them; first scenario findings"
```

---

## Self-review

**Spec coverage (§14a):** virtual time → Tasks 1–2; Go world model → Task 3 (WiFi; LoRa joins in plan 3); scenarios with events and checks → Tasks 5–6; headless runner and CI → Task 7; WebAssembly in the page with both engines → Tasks 8–9; docs → Task 10. §13 power cuts and splits are exercised by `storm` and the village tests.

**Placeholders:** none.

**Type consistency:** `HouseSpec`, `RelaySpec`, `Node`, `Post`, `Village`, `Scenario`, `Duration`, `Report`, `CheckResult`, `API` are used with the fields and signatures defined in their tasks; `AirNode.Name` is the node's name and `Addr()` returns it (the `Link` interface).
