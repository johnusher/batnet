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
