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
