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
	firstBeaconUs   = 60_000_000     // after a start or restart, beacon within a minute
	catchUpBeaconUs = 2 * 60_000_000 // after receiving recovered posts, beacon again soon
	lbtJitterUs     = 2_000_000      // random wait after a busy channel clears
	lbtMinUs        = 150_000        // at least this, longer than a radio needs to notice a transmission, so a sender's next fragment is heard first
	remoteHeardUs   = 30 * 60_000_000
	suppressSpanUs  = 3_000_000
	maxRecoverEach  = 8
	reassemblyTTLUs = 10 * 60_000_000
	stalledUs       = 60_000_000 // a partial message older than this lost frames on air: beacon soon
	// holdFactor: how long to wait for the rest of an entry another gateway
	// started, in fragment times. A busy channel stretches a sender's fragments
	// apart; 6 gave the fewest missing and late posts on farms over six seeds.
	holdFactor = 6
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
	AirUs                                                         int64
}

// Bridge is one gateway's LoRa logic. It is driven by events and Tick; it
// starts no goroutines.
type Bridge struct {
	Queue Queue
	Duty  *DutyMeter
	Stats BridgeStats

	cfg        BridgeConfig
	self       uint32
	rnd        *rand.Rand
	reasm      *Reassembler
	heard      map[ledger.Key]int64 // entries heard on air lately, with when: not to be sent again (bounded, B16)
	waiting    map[string]int64     // key → time it may be queued (suppression delay)
	waitEntry  map[string]*ledger.Entry
	waitKind   map[string]Kind      // KindEntry, or KindRecovery for beacon answers
	held       map[string]heldEntry // another gateway started sending these: wait for the rest
	backoff    int64                // don't transmit before this: a random wait after the channel was busy (CSMA)
	remoteAt   int64                // last time a batnet radio outside this group was heard; -1 never
	nextBeacon int64
	busyUntil  int64 // our own transmission in progress
}

// heldEntry is one of ours that another gateway started sending. If the rest
// of it isn't heard by until (its frames collided, or the sender lost power),
// it goes back in line.
type heldEntry struct {
	e     *ledger.Entry
	kind  Kind
	until int64
}

// NewBridge returns a bridge; its first beacon goes at a random point in the first 15 min.
func NewBridge(cfg BridgeConfig) *Bridge {
	b := &Bridge{cfg: cfg, Duty: NewDutyMeter(), self: ShortID(cfg.Self), rnd: rand.New(rand.NewSource(cfg.Seed)),
		reasm: NewReassembler(reassemblyTTLUs, 64), heard: map[ledger.Key]int64{}, waiting: map[string]int64{},
		waitEntry: map[string]*ledger.Entry{}, waitKind: map[string]Kind{}, held: map[string]heldEntry{}, remoteAt: -1}
	b.nextBeacon = cfg.Now() + b.rnd.Int63n(firstBeaconUs)
	return b
}

// Restart is what a reboot does: the queue, pending sends and partial
// fragments are gone (they lived in memory); a beacon goes out within a minute
// so the backlog is recovered. The duty meter is kept (batnetd stores it).
func (b *Bridge) Restart() {
	b.Queue = Queue{}
	b.waiting = map[string]int64{}
	b.waitEntry = map[string]*ledger.Entry{}
	b.waitKind = map[string]Kind{}
	b.held = map[string]heldEntry{}
	b.reasm = NewReassembler(reassemblyTTLUs, 64)
	b.busyUntil = 0
	b.backoff = 0
	b.nextBeacon = b.cfg.Now() + b.rnd.Int63n(firstBeaconUs)
}

func keyString(k ledger.Key) string { return fmt.Sprintf("%x/%d", k.Origin[:], k.Seq) }

func (b *Bridge) remoteHeard() bool {
	return b.remoteAt >= 0 && b.cfg.Now()-b.remoteAt <= remoteHeardUs
}

// relayable: every shared entry type crosses by radio (posts, accounts,
// approvals, directory cards: spec §6); only LoRa bookkeeping stays home.
// Sending them all also keeps vectors' gap-free counts moving, which recovery needs.
func relayable(e *ledger.Entry) bool { return e.Type != ledger.TypeLoRaSent }

// Consider looks at an entry new to this node, from the WiFi mesh or written
// here. Only entries written inside this WiFi group go on air: one from
// another group came over LoRa already (through another gateway, maybe).
func (b *Bridge) Consider(e *ledger.Entry) {
	if at, ok := b.heard[e.Key]; (ok && b.cfg.Now()-at <= heardTTLUs) || !relayable(e) || !b.remoteHeard() {
		return
	}
	if o := ShortID(e.Key.Origin); o != b.self && !b.cfg.InMyGroup(o) {
		return
	}
	b.wait(e, KindEntry)
}

// wait schedules an entry to be queued after a delay that differs per entry
// and gateway, so gateways holding the same entry don't all send it: the first
// heard on air cancels the rest (decision 0022).
func (b *Bridge) wait(e *ledger.Entry, kind Kind) bool {
	k := keyString(e.Key)
	if b.Queue.Has(k) || b.waiting[k] != 0 || b.held[k].e != nil {
		return false
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
	b.waitKind[k] = kind
	return true
}

func (b *Bridge) queueEntry(e *ledger.Entry, kind Kind, level prio.Level) {
	raw, _ := e.MarshalBinary()
	frames, err := Split(kind, level, b.self, raw)
	if err != nil {
		return
	}
	b.Queue.Push(&Item{Key: keyString(e.Key), Kind: kind, Base: level, Enqueued: b.cfg.Now(), Frames: frames, entry: e})
}

// resendLevel is a resend's level: one below its own, but High and Alert keep theirs (decision 0015).
func resendLevel(e *ledger.Entry) prio.Level {
	l := prio.Level(e.Prio)
	if l >= prio.High || l == 0 {
		return l
	}
	return l - 1
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
	// The channel just became free: contend with a random wait, so radios that
	// were all waiting for this frame to end don't start together.
	b.backoff = max(b.backoff, b.cfg.Now()+lbtMinUs+b.rnd.Int63n(lbtJitterUs))
	if f.Kind != KindBeacon {
		b.holdByMsgID(f.MsgID, int(f.Count)) // someone is already sending this entry: don't start it too
	}
	payload, done := b.reasm.Add(f, b.cfg.Now())
	if !done {
		return
	}
	b.Stats.Received++
	// The 30-min gate for sending over LoRa opens only on a frame whose payload
	// checks out (a signed entry or a well-formed beacon) from a radio outside
	// this WiFi group: a valid-looking header with garbage behind it is noise (B10).
	remote := !b.cfg.InMyGroup(f.Sender)
	switch f.Kind {
	case KindEntry, KindRecovery:
		var e ledger.Entry
		if e.UnmarshalBinary(payload) != nil || e.Verify() != nil {
			b.Stats.Bad++
			return
		}
		if remote {
			b.remoteAt = b.cfg.Now()
		}
		b.noteHeard(e.Key)
		k := keyString(e.Key)
		if b.Queue.Remove(k) || b.waiting[k] != 0 || b.held[k].e != nil {
			delete(b.waiting, k)
			delete(b.waitEntry, k)
			delete(b.waitKind, k)
			delete(b.held, k)
			b.Stats.Suppressed++
		}
		if b.cfg.Inject(&e) {
			b.Stats.Injected++
			if f.Kind == KindRecovery { // recovered: there may be more, so ask again soon
				b.nextBeacon = min(b.nextBeacon, b.cfg.Now()+catchUpBeaconUs)
			}
		}
	case KindBeacon:
		theirs, err := ParseCompactVector(payload)
		if err != nil {
			b.Stats.Bad++
			return
		}
		if !remote {
			return
		}
		b.remoteAt = b.cfg.Now()
		b.answerBeacon(theirs)
	}
}

// heardTTLUs is how long an entry heard on air is remembered as heard. It
// only has to outlive the moment it is injected and reaches Consider; an hour
// leaves room for the same entry arriving late over the mesh.
const heardTTLUs = 60 * 60_000_000

// noteHeard records an entry heard on air, dropping stale notes once the
// table is large, so it never grows with the life of the gateway (B16).
func (b *Bridge) noteHeard(k ledger.Key) {
	now := b.cfg.Now()
	if len(b.heard) >= 4096 {
		for key, at := range b.heard {
			if now-at > heardTTLUs {
				delete(b.heard, key)
			}
		}
	}
	b.heard[k] = now
}

// answerBeacon queues up to 8 entries the beacon's sender lacks, oldest first.
// A beacon also shows what its sender has that this gateway lacks: then this
// gateway's own next beacon comes soon, so the gap is asked for in minutes,
// not at the next quarter-hour (decision 0024).
func (b *Bridge) answerBeacon(theirs map[uint32]uint64) {
	mine, err := b.cfg.Store.Vector()
	if err != nil {
		return
	}
	short := make(map[uint32]uint64, len(mine))
	for o, n := range mine {
		short[ShortID(o)] = max(short[ShortID(o)], n)
	}
	for id, n := range theirs {
		if n > short[id] {
			b.nextBeacon = min(b.nextBeacon, b.cfg.Now()+catchUpBeaconUs)
			break
		}
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
			if relayable(e) {
				missing = append(missing, e)
			}
		}
	}
	sort.SliceStable(missing, func(i, j int) bool { return missing[i].TS.Less(missing[j].TS) })
	for i, e := range missing {
		if i == maxRecoverEach {
			break
		}
		if b.wait(e, KindRecovery) {
			b.Stats.Recovered++
		}
	}
}

// Tick sends what may go on air now and returns when to call again.
func (b *Bridge) Tick() int64 {
	now := b.cfg.Now()
	due := make([]string, 0, len(b.waiting))
	for k := range b.waiting {
		due = append(due, k)
	}
	sort.Strings(due) // map order is random; queue order must not be
	for _, k := range due {
		if at := b.waiting[k]; at <= now {
			if e := b.waitEntry[k]; e != nil {
				if b.waitKind[k] == KindRecovery {
					b.queueEntry(e, KindRecovery, resendLevel(e))
				} else {
					b.queueEntry(e, KindEntry, prio.Level(e.Prio))
				}
			}
			delete(b.waiting, k)
			delete(b.waitEntry, k)
			delete(b.waitKind, k)
		}
	}
	held := make([]string, 0, len(b.held))
	for k := range b.held {
		held = append(held, k)
	}
	sort.Strings(held)
	for _, k := range held {
		if h := b.held[k]; h.until <= now {
			delete(b.held, k) // the other sender stopped: back in line
			b.wait(h.e, h.kind)
		}
	}
	if b.reasm.Stalled(now, stalledUs) { // part of a message was lost on air: ask for it soon, not next quarter-hour
		b.nextBeacon = min(b.nextBeacon, now+catchUpBeaconUs)
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
	for _, h := range b.held {
		next = min(next, h.until)
	}
	it := b.Queue.Peek(now)
	if it == nil {
		return next
	}
	if now < b.busyUntil {
		return min(next, b.busyUntil)
	}
	if busy := b.cfg.Radio.BusyUntil(); busy > now {
		b.backoff = busy + lbtMinUs + b.rnd.Int63n(lbtJitterUs) // listen before talk, then a random wait
		return min(next, b.backoff)
	}
	if now < b.backoff {
		return min(next, b.backoff)
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
		if it.Kind != KindBeacon {
			b.Stats.Sent++
		}
	}
	return min(next, b.busyUntil)
}

// holdByMsgID holds back a waiting or not-yet-started queued entry whose
// payload has this message ID: another gateway has started sending it. It is
// dropped when the whole entry is heard, and goes back in line if that doesn't
// happen within the time its remaining fragments take.
func (b *Bridge) holdByMsgID(id uint32, count int) {
	until := b.cfg.Now() + int64(max(count, 1))*holdFactor*(AirtimeUs(MaxFrame, b.cfg.Preset)+lbtMinUs+lbtJitterUs)
	for k, e := range b.waitEntry {
		if raw, err := e.MarshalBinary(); err == nil && msgID(raw) == id {
			b.held[k] = heldEntry{e: e, kind: b.waitKind[k], until: until}
			delete(b.waiting, k)
			delete(b.waitEntry, k)
			delete(b.waitKind, k)
		}
	}
	for _, it := range b.Queue.items {
		if it.Next == 0 && it.Kind != KindBeacon && it.entry != nil && len(it.Frames) > 0 {
			if f, err := Parse(it.Frames[0]); err == nil && f.MsgID == id {
				b.held[it.Key] = heldEntry{e: it.entry, kind: it.Kind, until: until}
				b.Queue.Remove(it.Key)
				return
			}
		}
	}
}
