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

// After receiving recovered posts, a gateway beacons again soon, so a long
// backlog catches up in minutes, not hours.
func TestCatchUpIsFast(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	var es []*ledger.Entry
	for i := 0; i < 30; i++ { // written while nobody was listening
		e, _ := gws[0].author.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "missed"})
		es = append(es, e)
	}
	run(air, gws, 35*60_000_000)
	got := 0
	for _, e := range es {
		if x, _ := gws[1].store.Get(e.Key); x != nil {
			got++
		}
	}
	if got != len(es) {
		t.Fatalf("%d of %d missed posts recovered in 35 min", got, len(es))
	}
}

// A gateway that restarts (a power cut) beacons within a minute.
func TestRestartBeaconsSoon(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	run(air, gws, 20*60_000_000)
	before := gws[0].b.Stats.Beacons
	gws[0].b.Restart()
	run(air, gws, 61_000_000)
	if gws[0].b.Stats.Beacons == before {
		t.Fatal("no beacon within a minute of a restart")
	}
}
