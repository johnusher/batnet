package meshsync_test

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/meshsync"
	"github.com/johnusher/batnet/internal/mock"
	"github.com/johnusher/batnet/internal/seal"
)

type house struct {
	addr   string
	link   *mock.Link
	store  *ledger.MemStore
	author *ledger.Author
	sync   *meshsync.Syncer
	got    atomic.Int64 // OnEntry calls
}

func (h *house) post(t *testing.T, text string) {
	t.Helper()
	e, err := h.author.Append(ledger.TypeMsgPost, 5, map[string]string{"audience": "everyone", "text": text})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.sync.Publish(e); err != nil {
		t.Fatal(err)
	}
}

func (h *house) count() int {
	all, _ := h.store.All()
	return len(all)
}

// newVillage makes n houses named house-0.. on mesh m, all sharing key, running until the test ends.
func newVillage(t *testing.T, m *mock.Mesh, key seal.VillageKey, n int) []*house {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var hs []*house
	for i := 0; i < n; i++ {
		h := &house{addr: fmt.Sprintf("house-%d", i), store: ledger.NewMemStore()}
		h.link = m.Add(h.addr)
		_, priv, _ := ed25519.GenerateKey(nil)
		clock := hlc.New(func() int64 { return time.Now().UnixMilli() })
		h.author, _ = ledger.NewAuthor(priv, clock, h.store)
		sealer, _ := seal.NewSealer(key, seal.PurposeMesh)
		h.sync = meshsync.New(meshsync.Config{
			Link: h.link, Store: h.store, Clock: clock, Sealer: sealer,
			Heartbeat: 20 * time.Millisecond,
			OnEntry:   func(*ledger.Entry) { h.got.Add(1) },
		})
		go h.sync.Run(ctx)
		hs = append(hs, h)
	}
	return hs
}

func line(m *mock.Mesh, hs []*house) {
	for i := 1; i < len(hs); i++ {
		m.Connect(hs[i-1].addr, hs[i].addr)
	}
}

// waitFor polls until every house in hs holds want entries.
func waitFor(t *testing.T, hs []*house, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, h := range hs {
			if h.count() != want {
				ok = false
			}
		}
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, h := range hs {
		t.Errorf("%s holds %d entries, want %d", h.addr, h.count(), want)
	}
	t.FailNow()
}

func key(t *testing.T) seal.VillageKey {
	k, err := seal.NewVillageKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestLineConverges(t *testing.T) {
	m := mock.NewMesh(1)
	hs := newVillage(t, m, key(t), 5)
	line(m, hs)
	for i, h := range hs {
		for j := 0; j < 3; j++ {
			h.post(t, fmt.Sprintf("post %d from house %d", j, i))
		}
	}
	waitFor(t, hs, 15)
}

func TestPartitionThenMerge(t *testing.T) {
	m := mock.NewMesh(1)
	hs := newVillage(t, m, key(t), 4)
	line(m, hs)
	m.Disconnect("house-1", "house-2")
	hs[0].post(t, "west side")
	hs[3].post(t, "east side")
	waitFor(t, hs[:2], 1)
	waitFor(t, hs[2:], 1)
	m.Connect("house-1", "house-2")
	waitFor(t, hs, 2)
}

func TestPowerCutCatchUp(t *testing.T) {
	m := mock.NewMesh(1)
	hs := newVillage(t, m, key(t), 3)
	line(m, hs)
	m.SetUp("house-2", false)
	hs[0].post(t, "while house 2 was off")
	hs[1].post(t, "also while off")
	waitFor(t, hs[:2], 2)
	if hs[2].count() != 0 {
		t.Fatal("a house without power received entries")
	}
	m.SetUp("house-2", true)
	waitFor(t, hs, 2)
}

func TestLossyMeshStillConverges(t *testing.T) {
	m := mock.NewMesh(3)
	m.SetLoss(0.6)
	hs := newVillage(t, m, key(t), 5)
	line(m, hs)
	for i, h := range hs {
		for j := 0; j < 4; j++ {
			h.post(t, fmt.Sprintf("%d-%d", i, j))
		}
	}
	waitFor(t, hs, 20)
}

// Review focus 2.
func TestOnEntryOncePerEntry(t *testing.T) {
	m := mock.NewMesh(1)
	hs := newVillage(t, m, key(t), 3)
	line(m, hs)
	e, _ := hs[0].author.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "once"})
	for i := 0; i < 5; i++ {
		hs[0].sync.Publish(e) // the same entry, broadcast again and again
	}
	waitFor(t, hs, 1)
	time.Sleep(100 * time.Millisecond) // let heartbeats and repairs run
	for _, h := range hs[1:] {
		if n := h.got.Load(); n != 1 {
			t.Fatalf("%s: OnEntry called %d times, want 1", h.addr, n)
		}
	}
	if n := hs[0].got.Load(); n != 0 {
		t.Fatalf("author's own entry reached its OnEntry %d times", n)
	}
}

// Review focus 1: forged entries and other villages' packets are dropped.
func TestRejectsForgedAndForeign(t *testing.T) {
	m := mock.NewMesh(1)
	ours := key(t)
	hs := newVillage(t, m, ours, 2)
	m.Connect("house-0", "house-1")
	other := m.Add("stranger") // another village's node, or an attacker, in radio range
	m.Connect("stranger", "house-1")

	// 1. a packet sealed with another village's key
	foreign, _ := seal.NewSealer(key(t), seal.PurposeMesh)
	_, priv, _ := ed25519.GenerateKey(nil)
	e := &ledger.Entry{Key: ledger.Key{Origin: ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey)), Seq: 1}, Type: ledger.TypeMsgPost, Prio: 5, Body: []byte(`{}`)}
	e.Sign(priv)
	raw, _ := e.MarshalBinary()
	box, _ := foreign.Seal(append([]byte{'E'}, raw...))
	other.Broadcast(box)

	// 2. our key, but the entry is signed by someone other than its origin
	ourSealer, _ := seal.NewSealer(ours, seal.PurposeMesh)
	_, mallory, _ := ed25519.GenerateKey(nil)
	forged := &ledger.Entry{Key: ledger.Key{Origin: hs[0].author.ID(), Seq: 1}, Type: ledger.TypeMsgPost, Prio: 5, Body: []byte(`{"text":"forged"}`)}
	forged.Sign(mallory)
	raw, _ = forged.MarshalBinary()
	box, _ = ourSealer.Seal(append([]byte{'E'}, raw...))
	other.Broadcast(box)

	time.Sleep(150 * time.Millisecond)
	if n := hs[1].count(); n != 0 {
		t.Fatalf("house-1 stored %d bad entries", n)
	}
	hs[0].post(t, "the real one")
	waitFor(t, hs, 1)
}

func TestGarbageIgnored(t *testing.T) {
	m := mock.NewMesh(1)
	hs := newVillage(t, m, key(t), 2)
	m.Connect("house-0", "house-1")
	junk := m.Add("junk")
	m.Connect("junk", "house-1")
	for _, b := range [][]byte{nil, {1}, make([]byte, 40), []byte("not a packet at all, just words")} {
		junk.Broadcast(b)
	}
	hs[0].post(t, "still works")
	waitFor(t, hs, 1)
}
