package meshsync

import (
	"crypto/ed25519"
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/ledger/ledgertest"
)

func TestEntryPacket(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	e := ledgertest.Entry(t, priv, 1, 1000)
	got, hb, err := decodeBroadcast(encodeEntry(e))
	if err != nil || hb != nil || got == nil || got.Key != e.Key {
		t.Fatalf("got %+v %+v %v", got, hb, err)
	}
}

func TestHeartbeatPacket(t *testing.T) {
	h := heartbeat{Addr: "house-2", Hash: [32]byte{1, 2, 3}}
	e, got, err := decodeBroadcast(encodeHeartbeat(h))
	if err != nil || e != nil || got == nil || *got != h {
		t.Fatalf("got %+v %+v %v", e, got, err)
	}
}

func TestEntriesList(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	es := []*ledger.Entry{ledgertest.Entry(t, priv, 1, 1), ledgertest.Entry(t, priv, 2, 2)}
	got, err := decodeEntries(encodeEntries(es))
	if err != nil || len(got) != 2 || got[1].Key.Seq != 2 {
		t.Fatalf("got %d entries, %v", len(got), err)
	}
}

// Review focus 1: garbage never panics and never decodes.
func TestGarbage(t *testing.T) {
	for _, b := range [][]byte{nil, {}, {'E'}, {'H', 200}, {'Z', 1, 2}, {'E', 1, 2, 3}} {
		if e, h, err := decodeBroadcast(b); err == nil {
			t.Errorf("%v decoded to %v %v", b, e, h)
		}
	}
	if _, err := decodeEntries([]byte{0xff, 0xff, 0xff, 0x0f}); err == nil {
		t.Error("over-long entry list decoded")
	}
}
