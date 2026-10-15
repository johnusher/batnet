# Milestone 1, plan 3: the LoRa bridge and message priority, in the simulator

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Real batnet code carries messages between WiFi groups over LoRa — priority queues, the 10% duty limit, fragments, gateways sharing the sending, recovery beacons — exercised in virtual time over a Go model of the LoRa air, judged by scenarios, and shown in the simulator's real engine.

**Architecture:** `internal/prio` turns a person's choice into a 0–9 level with ageing and a High rate limit (decision 0015). `internal/lora` holds the airtime formula and presets, the frame format (10-byte header, fragments, entry and beacon payloads), a per-gateway priority queue with a duty meter, and the event-driven `Bridge` (no goroutines): it decides what goes on air, suppresses duplicates it hears, injects what it receives into the log, and answers beacons with up to 8 missing entries. `internal/sim` gains a LoRa medium (range per preset, airtime, half-duplex channel, collisions, listen-before-talk, managed flooding by every radio), gateways and Meshtastic users in the Village, LoRa-aware connectivity for the checks, a `duty` check, and new scenarios. The real meshtasticd client (two Meshsticks on the bench) is plan 3b.

**Tech Stack:** Go 1.26, standard library.

**Spec:** [`docs/specs/2026-10-15-m1-design.md`](../specs/2026-10-15-m1-design.md) §6, §7, §13, §14a; decisions 0007, 0008, 0015, 0017, 0021.

**Milestone 1 plans:** 1 core (done) · 2 village runtime (done) · **3 LoRa bridge and priority (this)** · 3b meshtasticd client on real radios · 4 web app and accounts · 5 Pi set-up, radios, kiosk, speaker · 6 image, installer, CI hardening.

## Global Constraints

- LoRa presets (as the JavaScript model): ShortFast SF7, MediumFast SF9, LongFast SF11; all 250 kHz, CR 4/5, preamble 16; ranges 1.2 / 2 / 3 km. Airtime by the Semtech formula with low-data-rate optimisation when a symbol exceeds 16 ms; 26 bytes of Meshtastic overhead per packet.
- Duty: each radio transmits at most 360 s in any sliding hour (EU 869.4–869.65 MHz, 10%), counting its rebroadcasts.
- A LoRa packet carries at most 200 bytes of batnet frame (header included).
- Priority levels per decision 0015: Alert 9, High 8 (never below 8), beacons 6, Normal 5 (+1 direct message, −1 public post or resend), Low 2, housekeeping 1; +1 per 20 min waiting, capped at 7; High limited to 3 per person per hour.
- A gateway puts a post on air only if, within the last 30 min, it heard a batnet radio outside its own WiFi group (decision 0017 and batman-adv's originator table); a gateway that hears an entry on air cancels its own pending send of it.
- Beacons every 15 min per gateway; a beacon answer queues at most 8 missing entries (priority 4).
- Determinism as in plan 2: same seed, same run.

## Review Focus

1. **Two gateways in one WiFi group** both decide to send the same post: the second must cancel on hearing the first; neither may loop an entry it received over LoRa back onto the air (except to answer a beacon). → Task 6 `TestSuppression`, `TestNoEcho`.
2. **A gateway at its duty limit** must stop transmitting until airtime ages out, while other gateways keep going; High waits for the limit too. → Task 5 `TestDutyMeter`, Task 6 `TestDutyHolds`.
3. **Fragments lost, duplicated or interleaved** with another message's: an entry is rebuilt only from all its own fragments; partial ones expire. → Task 3 `TestReassembly`.
4. **A forged or foreign LoRa frame** (bad signature, garbage, another village's beacon): dropped, never stored. → Task 6 `TestRejectsBadFrames`.
5. **The air under load**: overlapping transmissions collide; listen-before-talk defers; managed flooding does not storm. → Task 7 tests.

---

## File structure

```
internal/prio/prio.go, prio_test.go          levels, ageing, High rate limit
internal/lora/airtime.go, airtime_test.go    presets and the airtime formula
internal/lora/frame.go, frame_test.go        frame header, fragments, reassembly, compact vector
internal/lora/queue.go, queue_test.go        priority queue and duty meter
internal/lora/bridge.go, bridge_test.go      the gateway: what to send, suppression, injection, beacons
internal/meshsync/syncer.go                  (modify) Inject
internal/sim/lora.go, lora_test.go           the LoRa air: range, airtime, collisions, LBT, managed flooding
internal/sim/village.go                      (modify) gateways, Meshtastic users, LoRa-aware groups
internal/sim/scenario.go, report.go          (modify) preset, meshtastic users, prio choices, the duty check
internal/sim/api.go                          (modify) LoRa state for the page
scenarios/village.json, farms.json           (new) mixed WiFi/LoRa village; LoRa-only farms
prototypes/village-sim/real.js               (modify) LoRa radios, packets, airtime bars
```

---

### Task 1: Priority

**Files:** Create `internal/prio/prio.go`, `internal/prio/prio_test.go`

**Interfaces — Produces:**
```go
type Level uint8
const (Housekeeping Level = 1; Low Level = 2; Normal Level = 5; Beacon Level = 6; High Level = 8; Alert Level = 9; AgedCap Level = 7)
type Choice string // "low", "normal", "high", "alert"
func ForPost(c Choice, audience string, resend bool) Level // audience: "dm", "group", "everyone", "public"
func Effective(base Level, waitedUs int64) Level
type Limiter struct{ PerHour int /* … */ }
func NewLimiter(perHour int) *Limiter
func (l *Limiter) Allow(person string, nowUs int64) bool
```

- [ ] **Step 1: Write the failing tests** — `internal/prio/prio_test.go`

```go
package prio

import "testing"

func TestForPost(t *testing.T) {
	cases := []struct {
		c        Choice
		audience string
		resend   bool
		want     Level
	}{
		{"normal", "everyone", false, 5},
		{"normal", "dm", false, 6},
		{"normal", "public", false, 4},
		{"normal", "everyone", true, 4},
		{"low", "everyone", false, 2},
		{"low", "dm", false, 3},
		{"high", "everyone", false, 8},
		{"high", "public", true, 8}, // High never drops below 8
		{"high", "dm", false, 9},
		{"alert", "everyone", false, 9},
		{"", "everyone", false, 5}, // empty means normal
	}
	for _, c := range cases {
		if got := ForPost(c.c, c.audience, c.resend); got != c.want {
			t.Errorf("ForPost(%q, %q, %v) = %d, want %d", c.c, c.audience, c.resend, got, c.want)
		}
	}
}

func TestEffective(t *testing.T) {
	min := int64(60_000_000)
	cases := []struct {
		base   Level
		waited int64
		want   Level
	}{
		{Normal, 0, 5}, {Normal, 19 * min, 5}, {Normal, 20 * min, 6}, {Normal, 41 * min, 7},
		{Normal, 300 * min, 7}, // ageing stops at 7
		{Low, 60 * min, 5},
		{High, 300 * min, 8}, {Alert, 0, 9},
		{Housekeeping, 0, 1},
	}
	for _, c := range cases {
		if got := Effective(c.base, c.waited); got != c.want {
			t.Errorf("Effective(%d, %d min) = %d, want %d", c.base, c.waited/min, got, c.want)
		}
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(3)
	hour := int64(3_600_000_000)
	for i := 0; i < 3; i++ {
		if !l.Allow("anna", int64(i)) {
			t.Fatalf("High %d refused", i+1)
		}
	}
	if l.Allow("anna", 10) {
		t.Fatal("a fourth High in the hour allowed")
	}
	if !l.Allow("ben", 10) {
		t.Fatal("another person's High refused")
	}
	if !l.Allow("anna", hour+1) {
		t.Fatal("High refused after the hour passed")
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/prio/` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/prio/prio.go`

```go
// Package prio turns a person's Low / Normal / High (or an admin's Alert) into
// the 0-9 level the radio queue uses (decision 0015).
package prio

// Level is 0-9; higher goes first.
type Level uint8

const (
	Housekeeping Level = 1 // directory cards and housekeeping
	Low          Level = 2
	Normal       Level = 5
	Beacon       Level = 6 // sync beacons keep recovery working
	AgedCap      Level = 7 // waiting never lifts a message past this
	High         Level = 8
	Alert        Level = 9
)

// Choice is what a person picked.
type Choice string

const hourUs = int64(3_600_000_000)

// ForPost is a post's level: the choice, +1 for a direct message, -1 for a
// public post or a resend; High never below 8, Alert always 9.
func ForPost(c Choice, audience string, resend bool) Level {
	if c == "alert" {
		return Alert
	}
	base := map[Choice]int{"low": int(Low), "normal": int(Normal), "high": int(High), "": int(Normal)}[c]
	if base == 0 {
		base = int(Normal)
	}
	if audience == "dm" {
		base++
	}
	if audience == "public" {
		base--
	}
	if resend {
		base--
	}
	switch {
	case c == "high" && base < int(High):
		base = int(High)
	case base > int(Alert):
		base = int(Alert)
	case base < 0:
		base = 0
	}
	return Level(base)
}

// Effective raises a waiting message one level per 20 minutes, up to 7;
// High and Alert stay as they are.
func Effective(base Level, waitedUs int64) Level {
	if base >= High {
		return base
	}
	l := int(base) + int(waitedUs/(20*60_000_000))
	if l > int(AgedCap) {
		if base > AgedCap {
			return base
		}
		return AgedCap
	}
	return Level(l)
}

// Limiter allows each person PerHour High messages in any sliding hour.
type Limiter struct {
	PerHour int
	sent    map[string][]int64
}

// NewLimiter returns a limiter of perHour per person.
func NewLimiter(perHour int) *Limiter { return &Limiter{PerHour: perHour, sent: map[string][]int64{}} }

// Allow records and allows a High message, or refuses it.
func (l *Limiter) Allow(person string, nowUs int64) bool {
	keep := l.sent[person][:0]
	for _, t := range l.sent[person] {
		if nowUs-t < hourUs {
			keep = append(keep, t)
		}
	}
	if len(keep) >= l.PerHour {
		l.sent[person] = keep
		return false
	}
	l.sent[person] = append(keep, nowUs)
	return true
}
```

- [ ] **Step 4: Run to see it pass.** Run: `go test ./internal/prio/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/prio
git commit -m "prio: Low/Normal/High/Alert to 0-9, ageing up to 7, three High per person per hour"
```

---

### Task 2: Airtime and presets

**Files:** Create `internal/lora/airtime.go`, `internal/lora/airtime_test.go`

**Interfaces — Produces:**
```go
type Preset struct{ Name string; SF int; BW float64; CR int; RangeM float64 }
var Presets map[string]Preset // ShortFast, MediumFast, LongFast
const Overhead = 26          // Meshtastic bytes per packet
const MaxFrame = 200         // batnet bytes per packet
const DutyLimitUs = 360_000_000
func AirtimeUs(frameBytes int, p Preset) int64 // includes Overhead
```

- [ ] **Step 1: Write the failing test** — `internal/lora/airtime_test.go`

```go
package lora

import "testing"

func TestAirtimeMatchesTheJavaScriptModel(t *testing.T) {
	// the JavaScript model: LongFast, 130 bytes including overhead -> 1214 ms; ShortFast -> 112 ms
	lf, sf := Presets["LongFast"], Presets["ShortFast"]
	if got := AirtimeUs(130-Overhead, lf) / 1000; got < 1210 || got > 1218 {
		t.Fatalf("LongFast 130 B = %d ms, want about 1214", got)
	}
	if got := AirtimeUs(130-Overhead, sf) / 1000; got < 110 || got > 114 {
		t.Fatalf("ShortFast 130 B = %d ms, want about 112", got)
	}
	if AirtimeUs(MaxFrame, lf) <= AirtimeUs(10, lf) {
		t.Fatal("bigger frames must take longer")
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/lora/` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/lora/airtime.go`

```go
// Package lora is batnet's bridge to the LoRa radio: frames and fragments,
// the priority queue under the duty limit, and the gateway logic that decides
// what goes on air (spec §6, decisions 0008, 0015, 0017).
package lora

import "math"

// Preset is a Meshtastic modem preset.
type Preset struct {
	Name   string
	SF     int
	BW     float64 // Hz
	CR     int     // 1 = 4/5
	RangeM float64 // the model's typical reach
}

// Presets are the ones the simulator offers (the JavaScript model's numbers).
var Presets = map[string]Preset{
	"ShortFast":  {Name: "ShortFast", SF: 7, BW: 250e3, CR: 1, RangeM: 1200},
	"MediumFast": {Name: "MediumFast", SF: 9, BW: 250e3, CR: 1, RangeM: 2000},
	"LongFast":   {Name: "LongFast", SF: 11, BW: 250e3, CR: 1, RangeM: 3000},
}

const (
	Overhead    = 26          // Meshtastic header, protobuf and encryption, bytes per packet
	MaxFrame    = 200         // batnet bytes per packet, header included
	DutyLimitUs = 360_000_000 // 10% of an hour (EU 869.4-869.65 MHz)
	HourUs      = 3_600_000_000
)

// AirtimeUs is how long one packet with frameBytes of batnet data is on air
// (Semtech's formula, preamble 16, CRC on, explicit header).
func AirtimeUs(frameBytes int, p Preset) int64 {
	payload := frameBytes + Overhead
	tsym := math.Pow(2, float64(p.SF)) / p.BW * 1e6 // µs
	de := 0
	if tsym > 16000 {
		de = 1
	}
	num := float64(8*payload - 4*p.SF + 28 + 16)
	sym := 8 + math.Max(math.Ceil(num/float64(4*(p.SF-2*de)))*float64(p.CR+4), 0)
	return int64((16+4.25)*tsym + sym*tsym)
}
```

- [ ] **Step 4: Run to see it pass.** Run: `go test ./internal/lora/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/lora
git commit -m "lora: presets and the airtime formula, matching the JavaScript model"
```

---

### Task 3: Frames, fragments, reassembly, compact vectors

**Files:** Create `internal/lora/frame.go`, `internal/lora/frame_test.go`

**Interfaces — Produces:**
```go
type Kind uint8 // KindEntry = 1, KindBeacon = 2
const HeaderLen = 10
type Frame struct{ Kind Kind; Prio prio.Level; Sender uint32; MsgID uint32; Index, Count uint8; Chunk []byte }
func Split(kind Kind, p prio.Level, sender uint32, payload []byte) ([][]byte, error) // frames <= MaxFrame; at most 255
func Parse(b []byte) (Frame, error)
type Reassembler struct{ /* … */ }
func NewReassembler(ttlUs int64, maxPending int) *Reassembler
func (r *Reassembler) Add(f Frame, nowUs int64) (payload []byte, done bool)
func ShortID(id ledger.NodeID) uint32 // first 4 bytes
func AppendCompactVector(b []byte, v ledger.Vector) []byte
func ParseCompactVector(b []byte) (map[uint32]uint64, error)
```

Header (10 bytes): `version<<6 | kind<<4 | prio` (version 1), sender short ID (4), message ID (3: the low 24 bits of FNV-1a of the payload), index (1), count (1). Chunk: up to `MaxFrame - HeaderLen` = 190 bytes.

- [ ] **Step 1: Write the failing tests** — `internal/lora/frame_test.go`

```go
package lora

import (
	"bytes"
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

func TestSplitAndParse(t *testing.T) {
	payload := bytes.Repeat([]byte("abcdefghij"), 45) // 450 bytes -> 3 frames
	frames, err := Split(KindEntry, prio.High, 0xdeadbeef, payload)
	if err != nil || len(frames) != 3 {
		t.Fatalf("%d frames, %v", len(frames), err)
	}
	for i, b := range frames {
		if len(b) > MaxFrame {
			t.Fatalf("frame %d is %d bytes", i, len(b))
		}
		f, err := Parse(b)
		if err != nil || f.Kind != KindEntry || f.Prio != prio.High || f.Sender != 0xdeadbeef || int(f.Index) != i || f.Count != 3 {
			t.Fatalf("frame %d: %+v %v", i, f, err)
		}
	}
	if _, err := Parse([]byte{1, 2}); err == nil {
		t.Fatal("short frame parsed")
	}
	if _, err := Parse(append([]byte{0x01}, frames[0][1:]...)); err == nil {
		t.Fatal("wrong version parsed")
	}
}

// Review focus 3.
func TestReassembly(t *testing.T) {
	a, _ := Split(KindEntry, prio.Normal, 1, bytes.Repeat([]byte("A"), 400))
	b, _ := Split(KindEntry, prio.Normal, 2, bytes.Repeat([]byte("B"), 400))
	r := NewReassembler(60_000_000, 16)
	order := [][]byte{a[2], b[0], a[0], a[0], b[1], a[1]} // interleaved, one duplicate, b incomplete
	var got [][]byte
	for _, fr := range order {
		f, _ := Parse(fr)
		if p, done := r.Add(f, 0); done {
			got = append(got, p)
		}
	}
	if len(got) != 1 || !bytes.Equal(got[0], bytes.Repeat([]byte("A"), 400)) {
		t.Fatalf("rebuilt %d payloads", len(got))
	}
	f, _ := Parse(b[2])
	if _, done := r.Add(f, 120_000_000); done {
		t.Fatal("rebuilt from fragments that had expired")
	}
}

func TestCompactVector(t *testing.T) {
	var x, y ledger.NodeID
	x[0], x[1], x[2], x[3] = 1, 2, 3, 4
	y[0] = 9
	v := ledger.Vector{x: 7, y: 1 << 33}
	got, err := ParseCompactVector(AppendCompactVector(nil, v))
	if err != nil || got[ShortID(x)] != 7 || got[ShortID(y)] != 1<<33 || len(got) != 2 {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := ParseCompactVector([]byte{200}); err == nil {
		t.Fatal("truncated vector parsed")
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/lora/` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/lora/frame.go`

```go
package lora

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
	"sort"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

// Kind says what a frame's payload is.
type Kind uint8

const (
	KindEntry  Kind = 1 // a signed log entry
	KindBeacon Kind = 2 // a gateway's compact version vector
)

const (
	frameVersion = 1
	HeaderLen    = 10
	maxChunk     = MaxFrame - HeaderLen
)

var errFrame = errors.New("lora: malformed frame")

// Frame is one LoRa packet's worth of a message.
type Frame struct {
	Kind         Kind
	Prio         prio.Level
	Sender       uint32 // the sending node's short ID
	MsgID        uint32 // 24 bits
	Index, Count uint8
	Chunk        []byte
}

func msgID(payload []byte) uint32 {
	h := fnv.New32a()
	h.Write(payload)
	return h.Sum32() & 0xffffff
}

// Split cuts a payload into frames of at most MaxFrame bytes.
func Split(kind Kind, p prio.Level, sender uint32, payload []byte) ([][]byte, error) {
	n := (len(payload) + maxChunk - 1) / maxChunk
	if n == 0 {
		n = 1
	}
	if n > 255 {
		return nil, errors.New("lora: payload too big for 255 fragments")
	}
	id := msgID(payload)
	out := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		lo, hi := i*maxChunk, min((i+1)*maxChunk, len(payload))
		b := make([]byte, 0, HeaderLen+hi-lo)
		b = append(b, frameVersion<<6|byte(kind&3)<<4|byte(p&15))
		b = binary.BigEndian.AppendUint32(b, sender)
		b = append(b, byte(id>>16), byte(id>>8), byte(id), byte(i), byte(n))
		out = append(out, append(b, payload[lo:hi]...))
	}
	return out, nil
}

// Parse reads a frame's header.
func Parse(b []byte) (Frame, error) {
	if len(b) < HeaderLen || b[0]>>6 != frameVersion {
		return Frame{}, errFrame
	}
	f := Frame{Kind: Kind(b[0] >> 4 & 3), Prio: prio.Level(b[0] & 15), Sender: binary.BigEndian.Uint32(b[1:5]),
		MsgID: uint32(b[5])<<16 | uint32(b[6])<<8 | uint32(b[7]), Index: b[8], Count: b[9], Chunk: b[HeaderLen:]}
	if f.Count == 0 || f.Index >= f.Count || f.Prio > prio.Alert || (f.Kind != KindEntry && f.Kind != KindBeacon) {
		return Frame{}, errFrame
	}
	return f, nil
}

// Reassembler rebuilds payloads from fragments, dropping incomplete ones after a TTL.
type Reassembler struct {
	ttl        int64
	maxPending int
	pending    map[[2]uint32]*partial
}

type partial struct {
	first  int64
	chunks [][]byte
	have   int
}

// NewReassembler keeps partial messages for ttlUs, at most maxPending at once.
func NewReassembler(ttlUs int64, maxPending int) *Reassembler {
	return &Reassembler{ttl: ttlUs, maxPending: maxPending, pending: map[[2]uint32]*partial{}}
}

// Add takes a fragment; it returns the payload when the last missing one arrives.
func (r *Reassembler) Add(f Frame, nowUs int64) ([]byte, bool) {
	for k, p := range r.pending {
		if nowUs-p.first > r.ttl {
			delete(r.pending, k)
		}
	}
	if f.Count == 1 {
		return append([]byte(nil), f.Chunk...), true
	}
	k := [2]uint32{f.Sender, f.MsgID}
	p := r.pending[k]
	if p == nil {
		if len(r.pending) >= r.maxPending {
			return nil, false
		}
		p = &partial{first: nowUs, chunks: make([][]byte, f.Count)}
		r.pending[k] = p
	}
	if int(f.Count) != len(p.chunks) || p.chunks[f.Index] != nil {
		return nil, false // a different message with a clashing ID, or a duplicate
	}
	p.chunks[f.Index] = append([]byte(nil), f.Chunk...)
	p.have++
	if p.have < len(p.chunks) {
		return nil, false
	}
	delete(r.pending, k)
	var out []byte
	for _, c := range p.chunks {
		out = append(out, c...)
	}
	if msgID(out) != f.MsgID {
		return nil, false
	}
	return out, true
}

// ShortID is a node's 4-byte short form, used on air.
func ShortID(id ledger.NodeID) uint32 { return binary.BigEndian.Uint32(id[:4]) }

// AppendCompactVector encodes a vector with short IDs, for beacons.
func AppendCompactVector(b []byte, v ledger.Vector) []byte {
	ids := make([]ledger.NodeID, 0, len(v))
	for id, s := range v {
		if s > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ShortID(ids[i]) < ShortID(ids[j]) })
	b = binary.AppendUvarint(b, uint64(len(ids)))
	for _, id := range ids {
		b = binary.BigEndian.AppendUint32(b, ShortID(id))
		b = binary.AppendUvarint(b, v[id])
	}
	return b
}

// ParseCompactVector decodes a beacon's vector.
func ParseCompactVector(b []byte) (map[uint32]uint64, error) {
	n, k := binary.Uvarint(b)
	if k <= 0 || n > uint64(len(b)) {
		return nil, errFrame
	}
	b = b[k:]
	out := make(map[uint32]uint64, n)
	for i := uint64(0); i < n; i++ {
		if len(b) < 4 {
			return nil, errFrame
		}
		id := binary.BigEndian.Uint32(b)
		s, k := binary.Uvarint(b[4:])
		if k <= 0 {
			return nil, errFrame
		}
		out[id] = s
		b = b[4+k:]
	}
	return out, nil
}
```

- [ ] **Step 4: Run to see it pass.** Run: `go test ./internal/lora/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/lora
git commit -m "lora: frames with a 10-byte header, fragments, reassembly with expiry, compact vectors for beacons"
```

---

### Task 4: The queue and the duty meter

**Files:** Create `internal/lora/queue.go`, `internal/lora/queue_test.go`

**Interfaces — Produces:**
```go
type Item struct{ Key string; Kind Kind; Base prio.Level; Enqueued int64; Frames [][]byte; Next int /* frames sent */ }
type Queue struct{ /* … */ }
func (q *Queue) Push(it *Item) bool          // false if Key already queued
func (q *Queue) Has(key string) bool
func (q *Queue) Remove(key string) bool
func (q *Queue) Peek(nowUs int64) *Item     // highest effective level, oldest first; nil if empty
func (q *Queue) Len() int
type DutyMeter struct{ LimitUs, WindowUs int64 /* … */ }
func NewDutyMeter() *DutyMeter              // 360 s per hour
func (d *DutyMeter) Record(startUs, durUs int64)
func (d *DutyMeter) Used(nowUs int64) int64
func (d *DutyMeter) AllowedAt(nowUs, durUs int64) int64
```

- [ ] **Step 1: Write the failing tests** — `internal/lora/queue_test.go`

```go
package lora

import (
	"testing"

	"github.com/johnusher/batnet/internal/prio"
)

func TestQueueOrder(t *testing.T) {
	var q Queue
	min := int64(60_000_000)
	q.Push(&Item{Key: "low-old", Base: prio.Low, Enqueued: 0})
	q.Push(&Item{Key: "normal", Base: prio.Normal, Enqueued: 50 * min})
	q.Push(&Item{Key: "high", Base: prio.High, Enqueued: 55 * min})
	q.Push(&Item{Key: "normal-older", Base: prio.Normal, Enqueued: 45 * min})
	if q.Push(&Item{Key: "high"}) {
		t.Fatal("duplicate key queued")
	}
	now := 60 * min
	var got []string
	for q.Len() > 0 {
		it := q.Peek(now)
		got = append(got, it.Key)
		q.Remove(it.Key)
	}
	// low-old has waited an hour: 2 + 3 = 5, ties with Normal but is oldest
	want := []string{"high", "low-old", "normal-older", "normal"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order %v, want %v", got, want)
		}
	}
}

// Review focus 2.
func TestDutyMeter(t *testing.T) {
	d := NewDutyMeter()
	sec := int64(1_000_000)
	for i := int64(0); i < 300; i++ {
		d.Record(i*10*sec, 1200*1000) // 300 x 1.2 s = 360 s in the first 50 min
	}
	now := 50 * 60 * sec
	if d.Used(now) != 360*sec {
		t.Fatalf("used %d", d.Used(now))
	}
	at := d.AllowedAt(now, 1200*1000)
	if at <= now {
		t.Fatal("a radio at its limit may transmit")
	}
	if at != 60*60*sec+1200*1000 { // the first transmission (0 to 1.2 s) ages out an hour later
		t.Fatalf("allowed at %d", at)
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/lora/ -run 'Queue|Duty'` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/lora/queue.go`

```go
package lora

import "github.com/johnusher/batnet/internal/prio"

// Item is something waiting to go on air: one message, as one or more frames.
type Item struct {
	Key      string
	Kind     Kind
	Base     prio.Level
	Enqueued int64
	Frames   [][]byte
	Next     int // frames already sent
}

// Queue orders items by effective level (with ageing), then by age.
type Queue struct {
	items []*Item
	keys  map[string]bool
}

// Push adds an item unless one with the same key is waiting.
func (q *Queue) Push(it *Item) bool {
	if q.keys == nil {
		q.keys = map[string]bool{}
	}
	if q.keys[it.Key] {
		return false
	}
	q.keys[it.Key] = true
	q.items = append(q.items, it)
	return true
}

// Has reports whether key is waiting.
func (q *Queue) Has(key string) bool { return q.keys[key] }

// Remove drops key; false if it wasn't there.
func (q *Queue) Remove(key string) bool {
	if !q.keys[key] {
		return false
	}
	delete(q.keys, key)
	for i, it := range q.items {
		if it.Key == key {
			q.items = append(q.items[:i], q.items[i+1:]...)
			break
		}
	}
	return true
}

// Peek is the item to send next.
func (q *Queue) Peek(nowUs int64) *Item {
	var best *Item
	var bestLevel prio.Level
	for _, it := range q.items {
		l := prio.Effective(it.Base, nowUs-it.Enqueued)
		if best == nil || l > bestLevel || (l == bestLevel && it.Enqueued < best.Enqueued) {
			best, bestLevel = it, l
		}
	}
	return best
}

// Len is how many items wait.
func (q *Queue) Len() int { return len(q.items) }

// DutyMeter tracks a radio's airtime over a sliding hour.
type DutyMeter struct {
	LimitUs, WindowUs int64
	spans             [][2]int64
}

// NewDutyMeter allows 360 s in any hour.
func NewDutyMeter() *DutyMeter { return &DutyMeter{LimitUs: DutyLimitUs, WindowUs: HourUs} }

// Record notes a transmission.
func (d *DutyMeter) Record(startUs, durUs int64) { d.spans = append(d.spans, [2]int64{startUs, durUs}) }

// Used is the airtime in the hour before now.
func (d *DutyMeter) Used(nowUs int64) int64 {
	keep := d.spans[:0]
	var used int64
	for _, s := range d.spans {
		if s[0]+s[1] <= nowUs-d.WindowUs {
			continue
		}
		keep = append(keep, s)
		used += s[1]
	}
	d.spans = keep
	return used
}

// AllowedAt is the earliest time a transmission of durUs fits under the limit.
func (d *DutyMeter) AllowedAt(nowUs, durUs int64) int64 {
	used := d.Used(nowUs)
	if used+durUs <= d.LimitUs {
		return nowUs
	}
	for _, s := range d.spans {
		used -= s[1]
		if used+durUs <= d.LimitUs {
			return s[0] + s[1] + d.WindowUs
		}
	}
	return nowUs + d.WindowUs
}
```

- [ ] **Step 4: Run to see it pass.** Run: `go test ./internal/lora/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/lora
git commit -m "lora: a priority queue with ageing and a sliding-hour duty meter"
```

---

### Task 5: `Inject` in meshsync

**Files:** Modify `internal/meshsync/syncer.go`; add a test to `internal/meshsync/inline_test.go`

**Interfaces — Produces:** `(*Syncer).Inject(e *ledger.Entry) bool` — verifies and stores an entry that came from elsewhere (LoRa), calls `OnEntry`, and broadcasts it on the mesh if it was new.

- [ ] **Step 1: Write the failing test** — append to `internal/meshsync/inline_test.go`

```go
// An entry from the radio is stored, announced to OnEntry and broadcast on the mesh, once.
func TestInject(t *testing.T) {
	m := mock.NewMesh(1)
	k, _ := seal.NewVillageKey()
	mk := func(addr string, onEntry func(*ledger.Entry)) (*meshsync.Syncer, *ledger.MemStore, *mock.Link) {
		st := ledger.NewMemStore()
		link := m.Add(addr)
		sealer, _ := seal.NewSealer(k, seal.PurposeMesh)
		return meshsync.New(meshsync.Config{Link: link, Store: st, Clock: hlc.New(func() int64 { return 1 }), Sealer: sealer,
			Spawn: func(f func()) { f() }, OnEntry: onEntry}), st, link
	}
	seen := 0
	gw, gwStore, _ := mk("gw", func(*ledger.Entry) { seen++ })
	_, _, peer := mk("peer", nil)
	m.Connect("gw", "peer")
	_, priv, _ := ed25519.GenerateKey(nil)
	remote, _ := ledger.NewAuthor(priv, hlc.New(func() int64 { return 1 }), ledger.NewMemStore())
	e, _ := remote.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "from far away"})
	if !gw.Inject(e) || gw.Inject(e) {
		t.Fatal("Inject should accept a new entry once")
	}
	if got, _ := gwStore.Get(e.Key); got == nil || seen != 1 {
		t.Fatalf("stored %v, OnEntry %d times", got, seen)
	}
	select {
	case <-peer.Inbox():
	default:
		t.Fatal("injected entry not broadcast on the mesh")
	}
	bad := *e
	bad.Body = []byte(`{"text":"changed"}`)
	bad.Key.Seq = 2
	if gw.Inject(&bad) {
		t.Fatal("an entry with a bad signature was injected")
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/meshsync/ -run Inject` — Expected: FAIL, `gw.Inject undefined`.

- [ ] **Step 3: Implement** — add to `internal/meshsync/syncer.go`:

```go
// Inject takes an entry from outside the mesh (the LoRa bridge): it is verified,
// stored, reported to OnEntry and broadcast on the mesh if new.
func (s *Syncer) Inject(e *ledger.Entry) bool {
	if !s.accept(e) {
		return false
	}
	if err := s.Publish(e); err != nil {
		s.cfg.Logf("inject: %v", err)
	}
	return true
}
```

- [ ] **Step 4: Run all meshsync tests.** Run: `go test -race ./internal/meshsync/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/meshsync
git commit -m "meshsync: Inject, for entries that arrive over LoRa"
```

---

### Task 6: The Bridge

**Files:** Create `internal/lora/bridge.go`, `internal/lora/bridge_test.go`

**Interfaces:**
- Consumes: `Queue`, `DutyMeter`, `Split`, `Parse`, `Reassembler`, `AppendCompactVector`, `ParseCompactVector`, `ShortID`, `AirtimeUs`, `prio`, `ledger`.
- Produces:
  ```go
  type Radio interface {
  	Transmit(frame []byte, durUs int64) // start sending now
  	BusyUntil() int64                    // listen-before-talk: when the channel it hears is free
  }
  type BridgeConfig struct {
  	Self        ledger.NodeID
  	Preset      Preset
  	Store       ledger.Store
  	Inject      func(*ledger.Entry) bool // into the log and onto the WiFi mesh
  	InMyGroup   func(short uint32) bool  // batman-adv's originator table
  	Radio       Radio
  	Now         func() int64             // µs
  	Seed        int64
  }
  type Bridge struct{ Queue Queue; Duty *DutyMeter; Stats BridgeStats /* … */ }
  type BridgeStats struct{ Sent, Received, Suppressed, Injected, Bad, Beacons, Recovered int; AirUs int64 }
  func NewBridge(cfg BridgeConfig) *Bridge
  func (b *Bridge) Consider(e *ledger.Entry)  // a new entry seen on WiFi or written here
  func (b *Bridge) Receive(frame []byte)      // a frame heard on air
  func (b *Bridge) Tick() (nextUs int64)      // send what may be sent now; when to call again
  ```
  Rules: a post (`msg.post`, `alert`) seen on WiFi or written here is queued after a suppression delay of `hash(key, self) mod 3 s` (plus 3 s when this radio has used over half its duty), but only if a batnet radio outside this WiFi group was heard in the last 30 min. A frame heard carrying an entry that is queued here cancels it (`Suppressed`). Entries heard on air are injected, never re-queued (no echo). Every 15 min (first at a random phase) the bridge queues a beacon (`prio.Beacon`). On hearing a beacon from outside its group, it queues up to 8 entries the beacon lacks (`prio.Normal - 1`), oldest first. `Tick` sends the queue head if the channel is free and the duty meter allows its next frame; it returns when to call again.

- [ ] **Step 1: Write the failing tests** — `internal/lora/bridge_test.go`

```go
package lora

import (
	"crypto/ed25519"
	"testing"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

// fakeAir connects bridges directly: what one transmits, every other hears.
type fakeAir struct {
	now     int64
	bridges []*Bridge
	sent    [][]byte
}

type fakeRadio struct {
	air  *fakeAir
	self *Bridge
}

func (r *fakeRadio) Transmit(frame []byte, durUs int64) {
	r.air.sent = append(r.air.sent, frame)
	for _, b := range r.air.bridges {
		if b != r.self {
			b.Receive(frame)
		}
	}
}
func (r *fakeRadio) BusyUntil() int64 { return 0 }

type gw struct {
	b      *Bridge
	store  *ledger.MemStore
	author *ledger.Author
	group  int
}

func newGateways(t *testing.T, groups ...int) (*fakeAir, []*gw) {
	t.Helper()
	air := &fakeAir{}
	var gws []*gw
	for i, g := range groups {
		_, priv, _ := ed25519.GenerateKey(nil)
		st := ledger.NewMemStore()
		au, _ := ledger.NewAuthor(priv, hlc.New(func() int64 { return 1_790_000_000_000 + air.now/1000 }), st)
		x := &gw{store: st, author: au, group: g}
		gws = append(gws, x)
		_ = i
	}
	for _, x := range gws {
		x := x
		radio := &fakeRadio{air: air}
		x.b = NewBridge(BridgeConfig{
			Self: x.author.ID(), Preset: Presets["LongFast"], Store: x.store, Radio: radio, Seed: int64(x.group),
			Now:    func() int64 { return air.now },
			Inject: func(e *ledger.Entry) bool { added, _ := x.store.Put(e); return added && e.Verify() == nil },
			InMyGroup: func(short uint32) bool {
				for _, y := range gws {
					if ShortID(y.author.ID()) == short {
						return y.group == x.group
					}
				}
				return false
			},
		})
		radio.self = x.b
		air.bridges = append(air.bridges, x.b)
	}
	return air, gws
}

// run calls Tick on every bridge for d µs in 100 ms steps.
func run(air *fakeAir, gws []*gw, d int64) {
	for end := air.now + d; air.now < end; air.now += 100_000 {
		for _, x := range gws {
			x.b.Tick()
		}
	}
}

func post(t *testing.T, x *gw, text string, p prio.Level) *ledger.Entry {
	t.Helper()
	e, err := x.author.Append(ledger.TypeMsgPost, uint8(p), map[string]string{"audience": "everyone", "text": text})
	if err != nil {
		t.Fatal(err)
	}
	x.b.Consider(e)
	return e
}

func TestCrossesToAnotherGroup(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	run(air, gws, 16*60_000_000) // beacons: each learns the other is outside its group
	e := post(t, gws[0], "Water point open until 18:00", prio.Normal)
	run(air, gws, 30_000_000)
	if got, _ := gws[1].store.Get(e.Key); got == nil {
		t.Fatal("the post did not cross to the other group")
	}
}

func TestStaysHomeWhenNobodyElse(t *testing.T) {
	air, gws := newGateways(t, 1, 1) // two gateways, one WiFi group
	run(air, gws, 16*60_000_000)
	before := len(air.sent)
	post(t, gws[0], "only for us", prio.Normal)
	run(air, gws, 30_000_000)
	for _, f := range air.sent[before:] {
		if fr, _ := Parse(f); fr.Kind == KindEntry {
			t.Fatal("a post went on air though no other group was heard")
		}
	}
}

// Review focus 1.
func TestSuppression(t *testing.T) {
	air, gws := newGateways(t, 1, 1, 2)
	run(air, gws, 16*60_000_000)
	e := post(t, gws[0], "both gateways see this", prio.Normal)
	gws[1].store.Put(e)
	gws[1].b.Consider(e) // the second gateway of group 1 also has it
	before := len(air.sent)
	run(air, gws, 30_000_000)
	n := 0
	for _, f := range air.sent[before:] {
		if fr, _ := Parse(f); fr.Kind == KindEntry {
			n++
		}
	}
	frames, _ := Split(KindEntry, prio.Normal, 0, mustMarshal(e))
	if n != len(frames) {
		t.Fatalf("%d entry frames on air, want %d (sent once)", n, len(frames))
	}
	if gws[0].b.Stats.Suppressed+gws[1].b.Stats.Suppressed != 1 {
		t.Fatal("one of the two gateways should have cancelled")
	}
}

func TestNoEcho(t *testing.T) {
	air, gws := newGateways(t, 1, 2, 2)
	run(air, gws, 16*60_000_000)
	post(t, gws[0], "hello group 2", prio.Normal)
	run(air, gws, 30_000_000)
	if gws[1].b.Stats.Sent+gws[2].b.Stats.Sent > gws[1].b.Stats.Beacons+gws[2].b.Stats.Beacons {
		t.Fatal("a gateway re-sent an entry it received over LoRa")
	}
}

// Review focus 2.
func TestDutyHolds(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	run(air, gws, 16*60_000_000)
	for i := 0; i < 400; i++ {
		post(t, gws[0], "a long message that takes a while on air, again and again and again", prio.High)
	}
	run(air, gws, 60*60_000_000)
	if used := gws[0].b.Duty.Used(air.now); used > DutyLimitUs {
		t.Fatalf("used %d µs in an hour, over the limit", used)
	}
	if gws[0].b.Queue.Len() == 0 {
		t.Fatal("400 High posts fitted in one hour's duty; the test needs more")
	}
}

// Review focus 4.
func TestRejectsBadFrames(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	run(air, gws, 16*60_000_000)
	_, priv, _ := ed25519.GenerateKey(nil)
	forged := &ledger.Entry{Key: ledger.Key{Origin: gws[0].author.ID(), Seq: 99}, Type: ledger.TypeMsgPost, Prio: 5, Body: []byte(`{}`)}
	forged.Sign(priv)
	frames, _ := Split(KindEntry, prio.Normal, 7, mustMarshal(forged))
	for _, f := range frames {
		gws[1].b.Receive(f)
	}
	gws[1].b.Receive([]byte("garbage"))
	if got, _ := gws[1].store.Get(forged.Key); got != nil {
		t.Fatal("a forged entry was stored")
	}
	if gws[1].b.Stats.Bad == 0 {
		t.Fatal("bad frames not counted")
	}
}

func TestBeaconRecovers(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	// group 2's gateway misses three posts (written before it was listening)
	var es []*ledger.Entry
	for i := 0; i < 3; i++ {
		e, _ := gws[0].author.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "missed"})
		es = append(es, e)
	}
	run(air, gws, 35*60_000_000) // two beacon rounds
	for _, e := range es {
		if got, _ := gws[1].store.Get(e.Key); got == nil {
			t.Fatal("a beacon did not recover a missed post")
		}
	}
}

func mustMarshal(e *ledger.Entry) []byte { b, _ := e.MarshalBinary(); return b }
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/lora/ -run 'Crosses|Stays|Suppression|Echo|DutyHolds|BadFrames|Beacon'` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/lora/bridge.go`

```go
package lora

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

const (
	beaconEveryUs   = 15 * 60_000_000
	remoteHeardUs   = 30 * 60_000_000
	suppressSpanUs  = 3_000_000
	maxRecoverEach  = 8
	reassemblyTTLUs = 10 * 60_000_000
)

// Radio is the bridge's view of the LoRa radio (meshtasticd on a Pi; the
// simulated air in tests and the simulator).
type Radio interface {
	Transmit(frame []byte, durUs int64)
	BusyUntil() int64
}

// BridgeConfig wires a bridge to its node.
type BridgeConfig struct {
	Self      ledger.NodeID
	Preset    Preset
	Store     ledger.Store
	Inject    func(*ledger.Entry) bool
	InMyGroup func(short uint32) bool
	Radio     Radio
	Now       func() int64
	Seed      int64
}

// BridgeStats counts what a bridge did.
type BridgeStats struct {
	Sent, Received, Suppressed, Injected, Bad, Beacons, Recovered int
	AirUs                                                          int64
}

// Bridge is one gateway's LoRa logic. It is driven by events and Tick; it
// starts no goroutines.
type Bridge struct {
	Queue Queue
	Duty  *DutyMeter
	Stats BridgeStats

	cfg         BridgeConfig
	self        uint32
	rnd         *rand.Rand
	reasm       *Reassembler
	heard       map[ledger.Key]bool
	waiting     map[string]int64 // key → time it may be queued (suppression delay)
	waitEntry   map[string]*ledger.Entry
	remoteAt    int64 // last time a batnet radio outside this group was heard; -1 never
	nextBeacon  int64
	busyUntil   int64 // our own transmission in progress
}

// NewBridge returns a bridge; its first beacon goes at a random point in the first 15 min.
func NewBridge(cfg BridgeConfig) *Bridge {
	b := &Bridge{cfg: cfg, Duty: NewDutyMeter(), self: ShortID(cfg.Self), rnd: rand.New(rand.NewSource(cfg.Seed)),
		reasm: NewReassembler(reassemblyTTLUs, 64), heard: map[ledger.Key]bool{}, waiting: map[string]int64{},
		waitEntry: map[string]*ledger.Entry{}, remoteAt: -1}
	b.nextBeacon = cfg.Now() + b.rnd.Int63n(beaconEveryUs)
	return b
}

func keyString(k ledger.Key) string { return fmt.Sprintf("%x/%d", k.Origin[:], k.Seq) }

func (b *Bridge) remoteHeard() bool {
	return b.remoteAt >= 0 && b.cfg.Now()-b.remoteAt <= remoteHeardUs
}

// Consider looks at an entry new to this node, from the WiFi mesh or written here.
func (b *Bridge) Consider(e *ledger.Entry) {
	if b.heard[e.Key] || (e.Type != ledger.TypeMsgPost && e.Type != ledger.TypeAlert) || !b.remoteHeard() {
		return
	}
	k := keyString(e.Key)
	if b.Queue.Has(k) || b.waiting[k] != 0 {
		return
	}
	h := fnv.New32a()
	h.Write([]byte(k))
	h.Write([]byte{byte(b.self >> 24), byte(b.self >> 16), byte(b.self >> 8), byte(b.self)})
	delay := int64(h.Sum32()) % suppressSpanUs
	if b.Duty.Used(b.cfg.Now()) > DutyLimitUs/2 {
		delay += suppressSpanUs // a busy radio lets the others go first
	}
	b.waiting[k] = b.cfg.Now() + delay + 1
	b.waitEntry[k] = e
}

func (b *Bridge) queueEntry(e *ledger.Entry, level prio.Level) {
	raw, _ := e.MarshalBinary()
	frames, err := Split(KindEntry, level, b.self, raw)
	if err != nil {
		return
	}
	b.Queue.Push(&Item{Key: keyString(e.Key), Kind: KindEntry, Base: level, Enqueued: b.cfg.Now(), Frames: frames})
}

func (b *Bridge) queueBeacon() {
	v, err := b.cfg.Store.Vector()
	if err != nil {
		return
	}
	frames, err := Split(KindBeacon, prio.Beacon, b.self, AppendCompactVector(nil, v))
	if err != nil {
		return
	}
	b.Queue.Push(&Item{Key: fmt.Sprintf("beacon/%d", b.cfg.Now()), Kind: KindBeacon, Base: prio.Beacon, Enqueued: b.cfg.Now(), Frames: frames})
}

// Receive handles a frame heard on air.
func (b *Bridge) Receive(frame []byte) {
	f, err := Parse(frame)
	if err != nil {
		b.Stats.Bad++
		return
	}
	if f.Sender == b.self {
		return
	}
	if !b.cfg.InMyGroup(f.Sender) {
		b.remoteAt = b.cfg.Now()
	}
	payload, done := b.reasm.Add(f, b.cfg.Now())
	if !done {
		return
	}
	b.Stats.Received++
	switch f.Kind {
	case KindEntry:
		var e ledger.Entry
		if e.UnmarshalBinary(payload) != nil || e.Verify() != nil {
			b.Stats.Bad++
			return
		}
		b.heard[e.Key] = true
		k := keyString(e.Key)
		if b.Queue.Remove(k) || b.waiting[k] != 0 {
			delete(b.waiting, k)
			delete(b.waitEntry, k)
			b.Stats.Suppressed++
		}
		if b.cfg.Inject(&e) {
			b.Stats.Injected++
		}
	case KindBeacon:
		if b.cfg.InMyGroup(f.Sender) {
			return
		}
		theirs, err := ParseCompactVector(payload)
		if err != nil {
			b.Stats.Bad++
			return
		}
		b.answerBeacon(theirs)
	}
}

// answerBeacon queues up to 8 entries the beacon's sender lacks, oldest first.
func (b *Bridge) answerBeacon(theirs map[uint32]uint64) {
	mine, err := b.cfg.Store.Vector()
	if err != nil {
		return
	}
	origins := make([]ledger.NodeID, 0, len(mine))
	for o := range mine {
		origins = append(origins, o)
	}
	sort.Slice(origins, func(i, j int) bool { return bytes.Compare(origins[i][:], origins[j][:]) < 0 })
	var missing []*ledger.Entry
	for _, o := range origins {
		have := theirs[ShortID(o)]
		if mine[o] <= have {
			continue
		}
		es, err := b.cfg.Store.Range(ledger.Range{Origin: o, From: have + 1, To: mine[o]}, maxRecoverEach)
		if err != nil {
			continue
		}
		for _, e := range es {
			if e.Type == ledger.TypeMsgPost || e.Type == ledger.TypeAlert {
				missing = append(missing, e)
			}
		}
	}
	sort.SliceStable(missing, func(i, j int) bool { return missing[i].TS.Less(missing[j].TS) })
	for i, e := range missing {
		if i == maxRecoverEach {
			break
		}
		if !b.Queue.Has(keyString(e.Key)) {
			b.queueEntry(e, prio.Normal-1)
			b.Stats.Recovered++
		}
	}
}

// Tick sends what may go on air now and returns when to call again.
func (b *Bridge) Tick() int64 {
	now := b.cfg.Now()
	for k, at := range b.waiting {
		if at <= now {
			if e := b.waitEntry[k]; e != nil {
				b.queueEntry(e, prio.Level(e.Prio))
			}
			delete(b.waiting, k)
			delete(b.waitEntry, k)
		}
	}
	if now >= b.nextBeacon {
		b.queueBeacon()
		b.Stats.Beacons++
		b.nextBeacon = now + beaconEveryUs
	}
	next := b.nextBeacon
	for _, at := range b.waiting {
		next = min(next, at)
	}
	it := b.Queue.Peek(now)
	if it == nil {
		return next
	}
	if now < b.busyUntil {
		return min(next, b.busyUntil)
	}
	if busy := b.cfg.Radio.BusyUntil(); busy > now {
		return min(next, busy+int64(b.rnd.Intn(200_000))) // listen before talk, with a little jitter
	}
	frame := it.Frames[it.Next]
	dur := AirtimeUs(len(frame), b.cfg.Preset)
	if at := b.Duty.AllowedAt(now, dur); at > now {
		return min(next, at)
	}
	b.cfg.Radio.Transmit(frame, dur)
	b.Duty.Record(now, dur)
	b.busyUntil = now + dur
	b.Stats.AirUs += dur
	it.Next++
	if it.Next == len(it.Frames) {
		b.Queue.Remove(it.Key)
		if it.Kind == KindEntry {
			b.Stats.Sent++
		}
	}
	return min(next, b.busyUntil)
}
```

- [ ] **Step 4: Run to see it pass.** Run: `go test -race ./internal/lora/` — Expected: `ok`. If a test fails, it is a finding about the bridge or the test: debug it (superpowers:systematic-debugging), fix the cause, and record a ruling if the plan's rule changes.

- [ ] **Step 5: Commit**

```bash
git add internal/lora
git commit -m "lora: the gateway bridge - posts cross to other WiFi groups, gateways suppress duplicates, beacons recover, the duty limit holds"
```

---

### Task 7: The LoRa air in the simulator

**Files:** Create `internal/sim/lora.go`, `internal/sim/lora_test.go`

**Interfaces — Produces:**
```go
type LoRaAir struct{ Preset lora.Preset; BaseLoss float64; Stats LoRaStats; OnTransmit func(from string, at, durUs int64, heard []string) }
type LoRaStats struct{ Packets, Delivered, Collided, Lost, Rebroadcasts int }
func NewLoRaAir(s *Scheduler, p lora.Preset, seed int64) *LoRaAir
func (a *LoRaAir) Add(name string, x, y float64, onFrame func([]byte)) *LoRaRadio
type LoRaRadio struct{ Name string; X, Y float64; On bool; Duty *lora.DutyMeter }
func (r *LoRaRadio) Transmit(frame []byte, durUs int64)   // implements lora.Radio
func (r *LoRaRadio) BusyUntil() int64
func (a *LoRaAir) Reach(p, q *LoRaRadio) bool
```
Model: a packet is heard by every powered radio within `Preset.RangeM`, after its airtime, unless another packet that radio could hear overlapped it in time (collision, both lost there) or the distance loss draw fails (`BaseLoss + 0.3·(d/range)³`). Every radio that hears a packet for the first time rebroadcasts it once after a random 0.2–1.2 s delay, unless it heard the same packet rebroadcast meanwhile (Meshtastic's managed flooding); rebroadcasts count against that radio's own duty meter and are skipped if the meter refuses. `BusyUntil` is the end of the latest transmission the radio can hear.

- [ ] **Step 1: Write the failing tests** — `internal/sim/lora_test.go`

```go
package sim

import (
	"testing"

	"github.com/johnusher/batnet/internal/lora"
)

func TestLoRaReachAndAirtime(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	air.BaseLoss = 0
	var gotNear, gotFar []int64
	a := air.Add("a", 0, 0, nil)
	air.Add("near", 2000, 0, func([]byte) { gotNear = append(gotNear, s.Now()) })
	air.Add("far", 9000, 0, func([]byte) { gotFar = append(gotFar, s.Now()) })
	dur := lora.AirtimeUs(100, air.Preset)
	a.Transmit(make([]byte, 100), dur)
	s.RunUntil(10 * Second)
	if len(gotNear) != 1 || gotNear[0] != dur {
		t.Fatalf("near heard %v, want once at %d", gotNear, dur)
	}
	if len(gotFar) != 0 {
		t.Fatal("a radio 9 km away heard a LongFast packet")
	}
}

func TestCollision(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	air.BaseLoss = 0
	got := 0
	a := air.Add("a", 0, 0, nil)
	b := air.Add("b", 2000, 0, nil)
	air.Add("mid", 1000, 0, func([]byte) { got++ })
	dur := lora.AirtimeUs(100, air.Preset)
	a.Transmit(make([]byte, 100), dur)
	b.Transmit(make([]byte, 100), dur) // same time: they collide at mid
	s.RunUntil(10 * Second)
	if got != 0 || air.Stats.Collided == 0 {
		t.Fatalf("mid heard %d packets, collided %d", got, air.Stats.Collided)
	}
}

func TestManagedFloodingExtendsReach(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 2)
	air.BaseLoss = 0
	heard := 0
	a := air.Add("a", 0, 0, nil)
	air.Add("relay", 2800, 0, nil) // a Meshtastic user in between
	air.Add("b", 5600, 0, func([]byte) { heard++ })
	a.Transmit(make([]byte, 60), lora.AirtimeUs(60, air.Preset))
	s.RunUntil(30 * Second)
	if heard != 1 {
		t.Fatalf("b heard the packet %d times, want once (through the relay)", heard)
	}
	if air.Stats.Rebroadcasts > 2 {
		t.Fatalf("%d rebroadcasts for one packet: flooding is storming", air.Stats.Rebroadcasts)
	}
}

func TestBusyUntil(t *testing.T) {
	var s Scheduler
	air := NewLoRaAir(&s, lora.Presets["LongFast"], 1)
	a, b := air.Add("a", 0, 0, nil), air.Add("b", 1000, 0, nil)
	dur := lora.AirtimeUs(100, air.Preset)
	a.Transmit(make([]byte, 100), dur)
	if b.BusyUntil() != dur {
		t.Fatalf("b sees the channel busy until %d, want %d", b.BusyUntil(), dur)
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/sim/ -run 'LoRa|Collision|Flooding|BusyUntil'` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/sim/lora.go`

```go
package sim

import (
	"hash/fnv"
	"math"
	"math/rand"

	"github.com/johnusher/batnet/internal/lora"
)

// LoRaAir is the 868 MHz channel in virtual time: one preset for everyone,
// range and distance loss, collisions, and Meshtastic's managed flooding.
type LoRaAir struct {
	Preset     lora.Preset
	BaseLoss   float64
	Stats      LoRaStats
	OnTransmit func(from string, at, durUs int64, heard []string)

	sch    *Scheduler
	rnd    *rand.Rand
	radios []*LoRaRadio
	live   []transmission // recent and current packets, for collisions
}

// LoRaStats counts packets since the start.
type LoRaStats struct{ Packets, Delivered, Collided, Lost, Rebroadcasts int }

type transmission struct {
	from       *LoRaRadio
	start, end int64
}

// LoRaRadio is one node's LoRa radio (a batnet gateway or a Meshtastic user).
type LoRaRadio struct {
	Name    string
	X, Y    float64
	On      bool
	Duty    *lora.DutyMeter
	air     *LoRaAir
	onFrame func([]byte)
	seen    map[uint32]bool // packets heard (for managed flooding)
	pending map[uint32]bool // rebroadcasts waiting
}

// NewLoRaAir returns an empty channel for one preset.
func NewLoRaAir(s *Scheduler, p lora.Preset, seed int64) *LoRaAir {
	return &LoRaAir{Preset: p, BaseLoss: 0.02, sch: s, rnd: rand.New(rand.NewSource(seed))}
}

// Add puts a radio at (x, y); onFrame receives what it hears (nil for none).
func (a *LoRaAir) Add(name string, x, y float64, onFrame func([]byte)) *LoRaRadio {
	r := &LoRaRadio{Name: name, X: x, Y: y, On: true, Duty: lora.NewDutyMeter(), air: a, onFrame: onFrame, seen: map[uint32]bool{}, pending: map[uint32]bool{}}
	a.radios = append(a.radios, r)
	return r
}

// Reach reports whether two radios hear each other directly.
func (a *LoRaAir) Reach(p, q *LoRaRadio) bool {
	return math.Hypot(p.X-q.X, p.Y-q.Y) <= a.Preset.RangeM
}

func packetID(frame []byte) uint32 {
	h := fnv.New32a()
	h.Write(frame)
	return h.Sum32()
}

// Transmit starts sending a frame now (lora.Radio).
func (r *LoRaRadio) Transmit(frame []byte, durUs int64) {
	if !r.On {
		return
	}
	r.seen[packetID(frame)] = true
	r.air.send(r, append([]byte(nil), frame...), durUs)
}

// BusyUntil is when the last packet this radio can hear ends (lora.Radio).
func (r *LoRaRadio) BusyUntil() int64 {
	var until int64
	for _, t := range r.air.live {
		if (t.from == r || r.air.Reach(t.from, r)) && t.end > until {
			until = t.end
		}
	}
	return until
}

func (a *LoRaAir) send(from *LoRaRadio, frame []byte, dur int64) {
	now := a.sch.Now()
	keep := a.live[:0]
	for _, t := range a.live {
		if t.end > now-10*Second {
			keep = append(keep, t)
		}
	}
	a.live = append(keep, transmission{from: from, start: now, end: now + dur})
	a.Stats.Packets++
	var heard []string
	id := packetID(frame)
	for _, r := range a.radios {
		if r == from || !r.On || !a.Reach(from, r) {
			continue
		}
		r, d := r, math.Hypot(from.X-r.X, from.Y-r.Y)
		heard = append(heard, r.Name)
		a.sch.At(now+dur, func() { a.arrive(from, r, frame, id, now, now+dur, d) })
	}
	if a.OnTransmit != nil {
		a.OnTransmit(from.Name, now, dur, heard)
	}
}

func (a *LoRaAir) arrive(from, r *LoRaRadio, frame []byte, id uint32, start, end int64, d float64) {
	if !r.On {
		return
	}
	for _, t := range a.live {
		if t.from != from && t.start < end && t.end > start && (t.from == r || a.Reach(t.from, r)) {
			a.Stats.Collided++
			return
		}
	}
	if a.rnd.Float64() < a.BaseLoss+0.3*math.Pow(d/a.Preset.RangeM, 3) {
		a.Stats.Lost++
		return
	}
	if r.seen[id] {
		delete(r.pending, id) // someone else rebroadcast it first: managed flooding stands down
		return
	}
	r.seen[id] = true
	a.Stats.Delivered++
	if r.onFrame != nil {
		r.onFrame(frame)
	}
	r.pending[id] = true
	delay := 200*Ms + a.rnd.Int63n(1000*Ms)
	a.sch.After(delay, func() {
		if !r.pending[id] || !r.On {
			return
		}
		delete(r.pending, id)
		dur := lora.AirtimeUs(len(frame), a.Preset)
		now := a.sch.Now()
		if r.Duty.AllowedAt(now, dur) > now || r.BusyUntil() > now {
			return
		}
		r.Duty.Record(now, dur)
		a.Stats.Rebroadcasts++
		a.send(r, frame, dur)
	})
}
```

The gateway's `lora.Bridge` records its own transmissions on its own `DutyMeter`; give each gateway's `LoRaRadio.Duty` the same meter as its bridge (Task 8 wires `radio.Duty = bridge.Duty`) so rebroadcasts and sends share one hour's budget.

- [ ] **Step 4: Run to see it pass.** Run: `go test -race ./internal/sim/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/sim
git commit -m "sim: the LoRa air - range per preset, airtime, collisions, listen-before-talk, managed flooding"
```

---

### Task 8: Gateways and Meshtastic users in the village; scenarios and checks

**Files:** Modify `internal/sim/village.go`, `internal/sim/scenario.go`, `internal/sim/report.go`, `internal/sim/api.go`; add tests to `internal/sim/village_test.go` and `internal/sim/report_test.go`; create `scenarios/village.json`, `scenarios/farms.json`; modify `scenarios/index.json`.

**Interfaces — Produces:**
- `Village.LoRa *LoRaAir`; `Node.Bridge *lora.Bridge`, `Node.LoRaRadio *LoRaRadio` (houses with `"lora": true`, and relays with `"lora": true`).
- `NewVillage(seed, epochMs int64, preset lora.Preset)` (signature change; callers pass `lora.Presets["LongFast"]` by default).
- `(*Village).AddMeshtastic(name string, x, y float64) *LoRaRadio` — a plain Meshtastic node: it rebroadcasts (managed flooding) and does nothing else.
- `Village.Post(house, text string, prio uint8)` now also hands the entry to the house's bridge; every node's `OnEntry` hands entries to its bridge.
- Group snapshots (`snapGroups`) use **combined connectivity**: WiFi links plus LoRa links between powered LoRa radios within range (Meshtastic users included as relays), so `delivered` judges cross-group delivery.
- `Scenario`: `"preset": "LongFast"` (default), `"meshtastic": [{"name","x","y"}]`, `HouseSpec.LoRa` and `RelaySpec.LoRa` honoured; `Event.Prio` keeps 0–9.
- New check `{"kind": "duty"}`: every radio used at most 360 s in every hour (checked from the bridges' and radios' meters at the end and by the LoRa air's record of transmissions).
- `Report`: `LoRaPackets, LoRaCollided, LoRaRebroadcasts int; LoRaMaxDutyPct float64`.
- `API.State()` adds `"lora": {"packets": [{"from","at","dur","heard":[…]}] (since the last call), "radios": [{"name","x","y","on","dutyPct","queue","kind"}]}`.

- [ ] **Step 1: Write the failing tests** — append to `internal/sim/village_test.go`:

```go
func TestPostCrossesByLoRa(t *testing.T) {
	v, err := NewVillage(3, testEpoch, lora.Presets["LongFast"])
	if err != nil {
		t.Fatal(err)
	}
	v.AddHouse(HouseSpec{Name: "Miller", X: 0, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2})
	v.AddHouse(HouseSpec{Name: "Fern", X: 2500, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2}) // far apart: no WiFi link
	v.Start()
	v.RunFor(16 * Minute) // beacons tell each gateway the other group exists
	p, err := v.Post("Miller", "Bread at the bakery", 5)
	if err != nil {
		t.Fatal(err)
	}
	v.RunFor(2 * Minute)
	if _, ok := p.Arrived["Fern"]; !ok {
		t.Fatal("the post did not reach the other house by LoRa")
	}
}
```

(with `"github.com/johnusher/batnet/internal/lora"` imported; update `smallVillage` and other `NewVillage` calls to pass `lora.Presets["LongFast"]`).

Append to `internal/sim/report_test.go`:

```go
func TestDutyCheck(t *testing.T) {
	sc, err := LoadScenario([]byte(`{"name":"duty","seed":2,"duration":"70m",
	  "houses":[{"name":"A","x":0,"y":0,"pi":"Pi 4","wifi":"dongle","lora":true,"people":30},
	            {"name":"B","x":2500,"y":0,"pi":"Pi 4","wifi":"dongle","lora":true,"people":1}],
	  "traffic":{"perPersonPerHour":40},
	  "checks":[{"kind":"duty"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	r, _, _ := Run(sc)
	if !r.Pass || r.LoRaMaxDutyPct > 100 || r.LoRaPackets == 0 {
		t.Fatalf("duty: %+v", r)
	}
	if r.LoRaMaxDutyPct < 90 {
		t.Fatalf("1,200 posts an hour should push A's radio near its limit, got %.0f%%", r.LoRaMaxDutyPct)
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/sim/ -run 'CrossesByLoRa|DutyCheck'` — Expected: FAIL (compile errors on the new signature and fields).

- [ ] **Step 3: Implement.** In `village.go`:

1. `Village` gains `LoRa *LoRaAir` and `preset lora.Preset`; `NewVillage(seed, epochMs int64, preset lora.Preset)` creates `LoRa: NewLoRaAir(sch, preset, seed+2)`.
2. `Node` gains `Bridge *lora.Bridge` and `LoRaRadio *LoRaRadio`.
3. In `add(...)`, take a `withLoRa bool`; when true, after the Syncer is built:

```go
	if withLoRa {
		var br *lora.Bridge
		n.LoRaRadio = v.LoRa.Add(name, x, y, func(frame []byte) {
			if br != nil {
				br.Receive(frame)
				v.wakeBridge(n)
			}
		})
		br = lora.NewBridge(lora.BridgeConfig{
			Self: n.Author.ID(), Preset: v.preset, Store: n.Store, Radio: n.LoRaRadio, Seed: v.rnd.Int63(),
			Now:       func() int64 { return v.Sch.Now() },
			Inject:    n.Sync.Inject,
			InMyGroup: func(short uint32) bool { return v.sameWiFiGroup(n, short) },
		})
		n.Bridge = br
		n.LoRaRadio.Duty = br.Duty
	}
```

and wrap the Syncer's `OnEntry` so it also calls `n.Bridge.Consider(e)` and `v.wakeBridge(n)` when `n.Bridge != nil`.

4. Bridge scheduling (one pending wake per node, no duplicates):

```go
// wakeBridge runs a node's bridge now and schedules its next Tick.
func (v *Village) wakeBridge(n *Node) {
	if n.Bridge == nil || !n.Radio.On {
		return
	}
	next := n.Bridge.Tick()
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
```

(`Node` gains `wakeAt int64`.) `Start()` calls `v.wakeBridge(n)` for every node with a bridge.

5. `sameWiFiGroup(n, short)`: true if some powered node in `n`'s WiFi group (from `v.Air.Groups()`) has `lora.ShortID(node.Author.ID()) == short`.

6. `SetPower` also sets `n.LoRaRadio.On` and, when power returns, calls `v.wakeBridge(n)`.

7. `AddMeshtastic(name, x, y)` returns `v.LoRa.Add(name, x, y, nil)` and records it in `v.Meshtastic` for the page and groups.

8. `Post` hands the entry to the bridge: after `n.Sync.Publish(e)`, `if n.Bridge != nil { n.Bridge.Consider(e); v.wakeBridge(n) }`.

9. `snapGroups` uses combined connectivity: build a graph over all nodes (WiFi `Air.Linked` for powered pairs) plus LoRa reach between powered LoRa radios, including Meshtastic radios as intermediates; record each house's component index.

In `scenario.go`: `Scenario` gains `Preset string \`json:"preset"\`` and `Meshtastic []struct{ Name string; X, Y float64 } \`json:"meshtastic"\``; validate the preset name; `Build` passes the preset and adds Meshtastic radios; `RelaySpec` gains `LoRa bool \`json:"lora"\``; `add` receives `h.LoRa` / `r.LoRa`. Checks accept `"duty"`.

In `report.go`: fill the LoRa fields from `v.LoRa.Stats`; `LoRaMaxDutyPct` is the largest `Duty.Used(now)` over all radios times 100 / `lora.DutyLimitUs`, sampled every simulated minute by a scheduled probe that `Build` installs (store the max in `Village.MaxDutyUs`); the `duty` check passes when `MaxDutyUs ≤ lora.DutyLimitUs`. Add LoRa lines to `String()`.

In `api.go`: collect `OnTransmit` events like frames, and add the `"lora"` object to `State()`.

- [ ] **Step 4: Scenarios.** `scenarios/village.json`:

```json
{
  "name": "village",
  "about": "12 houses in 3 hamlets 1-2 km apart: WiFi inside each hamlet, LoRa between them, two Meshtastic neighbours. Normal chatter with some High posts.",
  "seed": 7,
  "duration": "60m",
  "start": "18:00",
  "preset": "LongFast",
  "houses": [
    {"name": "Miller", "x": 100, "y": 100, "pi": "Pi 4", "wifi": "dongle", "lora": true, "people": 4},
    {"name": "Fern", "x": 300, "y": 120, "pi": "Pi 5", "wifi": "dongle", "people": 3},
    {"name": "Okafor", "x": 200, "y": 300, "pi": "Pi 4", "wifi": "dongle", "people": 5},
    {"name": "Novak", "x": 380, "y": 280, "pi": "Pi 3", "wifi": "dongle", "lora": true, "people": 2},
    {"name": "Haddad", "x": 1500, "y": 200, "pi": "Pi 4", "wifi": "dongle", "lora": true, "people": 4},
    {"name": "Brandt", "x": 1700, "y": 150, "pi": "Pi 4", "wifi": "dongle", "people": 3},
    {"name": "Kowalski", "x": 1600, "y": 380, "pi": "Pi 5", "wifi": "dongle", "people": 2},
    {"name": "Moreau", "x": 1800, "y": 330, "pi": "Pi 4", "wifi": "builtin", "people": 3},
    {"name": "Lindqvist", "x": 900, "y": 1500, "pi": "Pi 4", "wifi": "dongle", "lora": true, "people": 5},
    {"name": "Santos", "x": 1100, "y": 1450, "pi": "Pi 4", "wifi": "dongle", "people": 2},
    {"name": "Petrov", "x": 1000, "y": 1650, "pi": "Zero 2 W", "wifi": "builtin", "people": 1},
    {"name": "Weber", "x": 1150, "y": 1700, "pi": "Pi 4", "wifi": "dongle", "people": 4}
  ],
  "meshtastic": [{"name": "Hiker", "x": 700, "y": 700}, {"name": "Shepherd", "x": 1400, "y": 900}],
  "traffic": {"perPersonPerHour": 3},
  "events": [
    {"at": "20m", "kind": "post", "house": "Okafor", "text": "Insulin needed, fridge down", "prio": 8},
    {"at": "35m", "kind": "power", "house": "Haddad", "on": false, "for": "10m"}
  ],
  "checks": [{"kind": "converged"}, {"kind": "delivered", "within": "15m"}, {"kind": "duty"}]
}
```

`scenarios/farms.json`: 8 houses 1.5–2.5 km apart, every one `"lora": true`, `"wifi": "builtin"`, 2 people each, LongFast, 90 min, `perPersonPerHour` 2, checks `delivered` within 20 min and `duty`. Choose coordinates so every farm reaches at least one other within 2.8 km.

`scenarios/index.json`: `["village.json", "street.json", "storm.json", "bang.json", "farms.json"]`.

- [ ] **Step 5: Run everything**

```bash
gofmt -l internal cmd; go vet ./... && go test -race ./...
go run ./cmd/batnetd -sim scenarios/*.json
GOOS=js GOARCH=wasm go build -o /tmp/batnet.wasm ./cmd/batnetwasm
```

Expected: tests `ok`; every scenario PASS. A scenario that fails is a finding about batnet: debug it, then either fix the cause (with a test) or mark the scenario `"expect": "fail"` with a reason **and** add the problem to `knowledge/known-problems.md`. Never loosen a check silently.

- [ ] **Step 6: Commit**

```bash
git add internal/sim scenarios
git commit -m "sim: gateways with the real LoRa bridge, Meshtastic users, LoRa-aware delivery checks, a duty check; village and farms scenarios"
```

---

### Task 9: LoRa in the real engine view

**Files:** Modify `prototypes/village-sim/real.js`, `prototypes/village-sim/index.html` (a card for radios)

**Produces:** in the real view, LoRa radios drawn with an amber dot (gateways) or purple diamond (Meshtastic users), LoRa packets as amber rings from the sender with lines to the radios that heard it (1.4 s), a "LoRa radios" card with an airtime bar per radio (`dutyPct` of 360 s) and its queue length, and LoRa tiles (packets, collisions, rebroadcasts, busiest radio's duty).

- [ ] **Step 1: Implement** — in `real.js`:

1. In `pull()`, push `state.lora.packets` into a `rings` array `{from, heard, t0}` like `beams`.
2. In `drawMap()`, include `state.lora.radios` in the bounds; draw Meshtastic radios (`kind === "meshtastic"`) as purple diamonds; for houses that appear in `state.lora.radios`, draw a small amber dot at the top-right of the house box; for each ring, draw an expanding amber circle at the sender (radius `8 + age * 70`, fading) and amber lines to each heard radio.
3. Add tiles: `['LoRa packets', lora.packets, collided + ' collided · ' + rebroadcasts + ' rebroadcasts']`, `['Busiest radio', maxDuty + '%', 'of its 360 s an hour']` from `state.stats` (extend `State()`'s stats with `loraPackets, loraCollided, loraRebroadcasts, loraMaxDutyPct`).
4. Render `#rv-radios`: one bar per radio, `name · queue N`, width `dutyPct%` capped at 100, amber (purple for Meshtastic).

In `index.html`, inside `#realview` after the tiles: `<section class="card"><p class="eyebrow">LoRa radios · airtime used in the last hour (limit 360 s)</p><div class="bars" id="rv-radios"></div></section>`.

- [ ] **Step 2: Check it** with the local build and server from plan 2 Task 9 Step 3, loading `village.json`, running 20 simulated minutes, and screenshotting: amber dots on gateways, Meshtastic diamonds, rings during transmissions, bars filling.

- [ ] **Step 3: Commit**

```bash
git add prototypes/village-sim
git commit -m "Real engine: LoRa radios, packets, airtime bars and LoRa tiles"
```

---

### Task 10: Knowledge and known problems

- [ ] **Step 1:** In `knowledge/research/village-sim-findings.md`, add a dated section with the five scenarios' reports (real code; modelled WiFi and LoRa): delivery times across groups, LoRa packets, collisions, rebroadcasts, the busiest radio's duty. Compare with the JavaScript model's numbers for the 12-house village where they overlap (the model's "81% within 10 min").
- [ ] **Step 2:** In `knowledge/known-problems.md`, add anything found while building this plan, and move "L2 Posts can't cross between WiFi groups yet" to Resolved.
- [ ] **Step 3:** `knowledge/log.md` bullet; README's "Two engines" paragraph: the real engine now covers WiFi and LoRa.
- [ ] **Step 4:** Run everything and commit

```bash
go vet ./... && go test -race ./... && go run ./cmd/batnetd -sim scenarios/*.json && (cd tools/okflint && go run . ../../knowledge)
git add knowledge README.md
git commit -m "Knowledge: LoRa bridge findings from the real code; known problems updated"
```

---

## Self-review

**Spec coverage:** §6 what goes over LoRa (posts to other groups, beacons with recovery ≤ 8), formats (binary envelope with fragments ≤ 200 B; 4-bit priority in the header), who sends (all gateways, by suppression; supersedes the hash assignment in 0017's wording, recorded as a ruling when built), airtime and per-radio queues → Tasks 2–6; §7 priority → Tasks 1, 4, 6; §13 radio at duty limit, group without radio → Task 6 tests and the scenarios; §14a LoRa in the world model → Tasks 7–8; the page → Task 9. **Not here (plan 3b):** the meshtasticd client, everyone-channel posts as plain Meshtastic text for users holding the key (done-when check 3), public-channel interop text, Meshtastic queue priorities on the real radio. Recorded in `known-problems.md` in Task 10.

**Placeholders:** none in code steps; Task 8 Step 3 describes edits to existing functions with the new code shown for the new parts.

**Type consistency:** `prio.Level` in frames, queue items and `ForPost`; `lora.Radio` implemented by `sim.LoRaRadio`; `Bridge.Duty` shared with `LoRaRadio.Duty`; `meshsync.Syncer.Inject` used as `BridgeConfig.Inject`.
