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
