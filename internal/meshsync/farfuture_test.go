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

// An entry stamped past the year 2100 is refused: stored, it would sort last
// for ever and drag every node's clock after it.
func TestFarFutureEntryRefused(t *testing.T) {
	m := mock.NewMesh(1)
	k, _ := seal.NewVillageKey()
	sealer, _ := seal.NewSealer(k, seal.PurposeMesh)
	st := ledger.NewMemStore()
	s := meshsync.New(meshsync.Config{Link: m.Add("a"), Store: st, Clock: hlc.New(func() int64 { return 1 }), Sealer: sealer,
		Spawn: func(f func()) { f() }})
	_, priv, _ := ed25519.GenerateKey(nil)
	far, _ := ledger.NewAuthor(priv, hlc.New(func() int64 { return hlc.MaxWall + 1000 }), ledger.NewMemStore())
	e, err := far.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "from 2100"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Inject(e) {
		t.Fatal("a far-future entry was accepted")
	}
	if all, _ := st.All(); len(all) != 0 {
		t.Fatalf("stored %d entries, want 0", len(all))
	}
	now, _ := ledger.NewAuthor(priv, hlc.New(func() int64 { return 1_790_000_000_000 }), ledger.NewMemStore())
	ok, _ := now.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "today"})
	if !s.Inject(ok) {
		t.Fatal("an entry from today was refused")
	}
}
