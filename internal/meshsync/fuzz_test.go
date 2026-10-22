package meshsync

import (
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/seal"
)

// Security review 2026-10-22: broadcasts and requests reach a node from
// anyone on bat0. Garbage is dropped by the seal; what a village-key holder
// sends must still not panic.

func FuzzWire(f *testing.F) {
	f.Add([]byte{kindHeartbeat, 2, 'a', 'b'})
	f.Add(encodeHeartbeat(heartbeat{Addr: "172.27.1.2"}))
	f.Add([]byte{kindEntry, 1})
	f.Add(encodeEntries(nil))
	f.Add([]byte{0xff, 0xff, 0x03})
	f.Fuzz(func(t *testing.T, b []byte) {
		decodeBroadcast(b)
		decodeEntries(b)
	})
}

// FuzzSyncer: the request handler and the broadcast path, with the fuzz
// input sealed so it gets past the AEAD, and once more raw.
func FuzzSyncer(f *testing.F) {
	f.Add([]byte{kindVectorReq})
	f.Add([]byte{kindRangeReq, 1})
	f.Add([]byte{kindHeartbeat, 2, 'm', 'e'})
	f.Add([]byte{kindEntry})
	f.Fuzz(func(t *testing.T, b []byte) {
		k, _ := seal.NewVillageKey()
		sealer, _ := seal.NewSealer(k, seal.PurposeMesh)
		link := &fakeLink{inbox: make(chan Packet, 1), release: make(chan struct{})}
		close(link.release)
		s := New(Config{Link: link, Store: ledger.NewMemStore(), Clock: hlc.New(func() int64 { return 1 }), Sealer: sealer,
			Heartbeat: time.Hour, Spawn: func(fn func()) { fn() }})
		box, _ := sealer.Seal(b)
		if resp := s.handle("1.2.3.4", box); resp != nil {
			if _, err := sealer.Open(resp); err != nil {
				t.Fatal("an answer that does not open")
			}
		}
		s.handle("1.2.3.4", b)
		s.HandlePacket(Packet{From: "1.2.3.4", Data: box})
		s.HandlePacket(Packet{From: "1.2.3.4", Data: b})
	})
}
