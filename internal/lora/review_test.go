package lora

import (
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

// Review C1: a node's non-post entries (node.announce) must not stall recovery.
func TestRecoveryPassesNonPosts(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	gws[0].author.Append(ledger.TypeNodeAnnounce, 1, map[string]string{"house": "Miller"})
	var posts []*ledger.Entry
	for i := 0; i < 3; i++ {
		e, _ := gws[0].author.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "missed"})
		posts = append(posts, e)
	}
	run(air, gws, 3*60*60_000_000)
	for _, e := range posts {
		if x, _ := gws[1].store.Get(e.Key); x == nil {
			t.Fatal("a post behind a node.announce was never recovered")
		}
	}
	if gws[0].b.Stats.Recovered > 8 {
		t.Fatalf("%d recovery sends for 4 entries: answers repeat", gws[0].b.Stats.Recovered)
	}
}

// Review I1: an entry from another group, received over WiFi (not heard on air), is not re-sent.
func TestNoEchoAcrossGateways(t *testing.T) {
	air, gws := newGateways(t, 1, 1, 2) // A and A2 in group 1, R in group 2
	run(air, gws, 16*60_000_000)
	r := gws[2]
	e, _ := r.author.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "from group 2"})
	gws[1].store.Put(e)
	gws[1].b.Consider(e) // A2 got it over WiFi from A, without hearing it on air
	sent := gws[1].b.Stats.Sent
	run(air, gws, 30_000_000)
	if gws[1].b.Stats.Sent != sent {
		t.Fatal("a gateway re-sent another group's post it got over WiFi")
	}
}

// Review I2: an ordinary level-4 post (public, or a Normal resend) does not trigger catch-up beacons.
func TestCatchUpOnlyForRecovery(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	run(air, gws, 16*60_000_000)
	before := gws[1].b.nextBeacon
	e, _ := gws[0].author.Append(ledger.TypeMsgPost, uint8(prio.ForPost("normal", "public", false)), map[string]string{"text": "public"})
	raw, _ := e.MarshalBinary()
	frames, _ := Split(KindEntry, prio.Level(e.Prio), ShortID(gws[0].author.ID()), raw)
	for _, f := range frames {
		gws[1].b.Receive(f)
	}
	if gws[1].b.nextBeacon != before {
		t.Fatal("a public post pulled the next beacon forward")
	}
}

// B10: a frame with a valid header and garbage behind it doesn't open the
// 30-min gate for sending over LoRa; a verified entry from another group does.
func TestGateOpensOnlyOnVerifiedFrames(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	garbage, _ := Split(KindEntry, prio.Normal, ShortID(gws[1].author.ID()), []byte("not an entry at all"))
	gws[0].b.Receive(garbage[0])
	if gws[0].b.remoteHeard() {
		t.Fatal("garbage with a valid header opened the gate")
	}
	beaconish, _ := Split(KindBeacon, prio.Beacon, ShortID(gws[1].author.ID()), []byte{255, 255, 255})
	gws[0].b.Receive(beaconish[0])
	if gws[0].b.remoteHeard() {
		t.Fatal("a malformed beacon opened the gate")
	}
	e, _ := gws[1].author.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "real"})
	frames, _ := Split(KindEntry, prio.Normal, ShortID(gws[1].author.ID()), mustMarshal(e))
	for _, f := range frames {
		gws[0].b.Receive(f)
	}
	if !gws[0].b.remoteHeard() {
		t.Fatal("a verified entry from another group did not open the gate")
	}
	_ = air
}

// Decision 0024: a gateway left holding part of a message (the rest collided
// on air) beacons within a couple of minutes, once, instead of waiting for
// its next quarter-hour beacon.
func TestStalledPartialPullsTheBeaconForward(t *testing.T) {
	air, gws := newGateways(t, 1, 2)
	run(air, gws, 16*60_000_000)
	long := ""
	for len(long) < 2*MaxFrame {
		long += "The bridge on the north road is closed; use the ford by the mill. "
	}
	e, _ := gws[1].author.Append(ledger.TypeMsgPost, 5, map[string]string{"audience": "everyone", "text": long})
	frames, _ := Split(KindEntry, prio.Normal, ShortID(gws[1].author.ID()), mustMarshal(e))
	if len(frames) < 2 {
		t.Fatal("the test needs an entry of several fragments")
	}
	gws[0].b.Receive(frames[0]) // the rest never arrives
	before := gws[0].b.Stats.Beacons
	run(air, gws, 4*60_000_000)
	if gws[0].b.Stats.Beacons != before+1 {
		t.Fatalf("%d beacons in 4 min after a stalled message, want 1", gws[0].b.Stats.Beacons-before)
	}
	if got, _ := gws[0].store.Get(e.Key); got == nil {
		t.Fatal("the beacon's answer did not bring the rest of the message")
	}
	r := NewReassembler(reassemblyTTLUs, 4)
	f, _ := Parse(frames[0])
	r.Add(f, 0)
	if r.Stalled(30_000_000, stalledUs) {
		t.Fatal("a partial message counted as stalled before its time")
	}
	if !r.Stalled(stalledUs, stalledUs) || r.Stalled(2*stalledUs, stalledUs) {
		t.Fatal("a stalled partial message must count once, not again")
	}
}

// B12: when the reassembler is full, the stalest partial message makes room
// for a new one, instead of the new one being refused for ten minutes.
func TestReassemblerEvictsTheOldestPartial(t *testing.T) {
	r := NewReassembler(60_000_000, 4)
	for i := uint32(1); i <= 4; i++ {
		fr, _ := Split(KindEntry, prio.Normal, i, make([]byte, 2*maxChunk))
		f, _ := Parse(fr[0])
		r.Add(f, int64(i)*1_000_000) // sender 1 started first
	}
	fresh, _ := Split(KindEntry, prio.Normal, 9, []byte("a new two-part message that must get through, long enough to need a second fragment because it goes past the chunk size of a single frame, which is one hundred and ninety bytes, so a bit more text here"))
	if len(fresh) < 2 {
		t.Fatal("the test needs a fragmented payload")
	}
	for _, fr := range fresh {
		f, _ := Parse(fr)
		if _, done := r.Add(f, 5_000_000); done {
			return // rebuilt: room was made
		}
	}
	t.Fatal("a new message was refused while the table was full of stale partials")
}

// Review I3: a recovered High post keeps High; a recovered Normal one drops one level.
func TestRecoveredKeepsPriority(t *testing.T) {
	cases := []struct{ in, want prio.Level }{{prio.Alert, prio.Alert}, {prio.High, prio.High}, {prio.Normal, prio.Normal - 1}, {prio.Low, prio.Low - 1}, {0, 0}}
	for _, c := range cases {
		if got := resendLevel(&ledger.Entry{Prio: uint8(c.in)}); got != c.want {
			t.Errorf("resendLevel(%d) = %d, want %d", c.in, got, c.want)
		}
	}
	_, gws := newGateways(t, 1, 2)
	gws[0].author.Append(ledger.TypeMsgPost, uint8(prio.High), map[string]string{"text": "Insulin needed"})
	gws[0].b.answerBeacon(map[uint32]uint64{})
	for k, kind := range gws[0].b.waitKind {
		if kind != KindRecovery {
			t.Fatalf("%s waits as kind %d, want a recovery", k, kind)
		}
	}
	if len(gws[0].b.waitKind) != 1 {
		t.Fatalf("%d answers waiting, want 1", len(gws[0].b.waitKind))
	}
}
