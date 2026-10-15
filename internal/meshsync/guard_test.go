package meshsync

import (
	"context"
	"crypto/ed25519"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/ledger/ledgertest"
	"github.com/johnusher/batnet/internal/seal"
)

// fakeLink records requests and can hold them open, to test the Syncer's guards.
type fakeLink struct {
	mu       sync.Mutex
	asked    []string
	inflight atomic.Int32
	peak     atomic.Int32
	release  chan struct{}
	inbox    chan Packet
}

func (l *fakeLink) Addr() string                           { return "me" }
func (l *fakeLink) Broadcast([]byte) error                 { return nil }
func (l *fakeLink) BroadcastLocal([]byte) error            { return nil }
func (l *fakeLink) Inbox() <-chan Packet                   { return l.inbox }
func (l *fakeLink) SetHandler(func(string, []byte) []byte) {}
func (l *fakeLink) Request(ctx context.Context, to string, req []byte) ([]byte, error) {
	l.mu.Lock()
	l.asked = append(l.asked, to)
	l.mu.Unlock()
	n := l.inflight.Add(1)
	for {
		p := l.peak.Load()
		if n <= p || l.peak.CompareAndSwap(p, n) {
			break
		}
	}
	defer l.inflight.Add(-1)
	select {
	case <-l.release:
	case <-ctx.Done():
	}
	return nil, context.Canceled
}

func newGuardSyncer(t *testing.T, link Link, store ledger.Store) (*Syncer, *seal.Sealer) {
	t.Helper()
	k, _ := seal.NewVillageKey()
	sealer, _ := seal.NewSealer(k, seal.PurposeMesh)
	return New(Config{Link: link, Store: store, Clock: hlc.New(func() int64 { return 1 }), Sealer: sealer, Heartbeat: time.Hour}), sealer
}

func sealed(t *testing.T, s *seal.Sealer, plain []byte) []byte {
	t.Helper()
	box, err := s.Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	return box
}

// Review: a heartbeat whose claimed address isn't its sender's is ignored.
func TestHeartbeatAddressMustMatchSender(t *testing.T) {
	link := &fakeLink{release: make(chan struct{}), inbox: make(chan Packet)}
	defer close(link.release)
	s, sealer := newGuardSyncer(t, link, ledger.NewMemStore())
	hb := encodeHeartbeat(heartbeat{Addr: "victim", Hash: [32]byte{9}})
	s.receive(context.Background(), Packet{From: "attacker", Data: sealed(t, sealer, hb)})
	time.Sleep(50 * time.Millisecond)
	link.mu.Lock()
	defer link.mu.Unlock()
	if len(link.asked) != 0 {
		t.Fatalf("repair started against %v on a spoofed heartbeat", link.asked)
	}
}

// Review: a flood of heartbeats from many peers runs at most maxRepairs repairs at once.
func TestRepairsAreCapped(t *testing.T) {
	link := &fakeLink{release: make(chan struct{}), inbox: make(chan Packet)}
	s, sealer := newGuardSyncer(t, link, ledger.NewMemStore())
	for i := 0; i < 20; i++ {
		addr := string(rune('a' + i))
		hb := encodeHeartbeat(heartbeat{Addr: addr, Hash: [32]byte{byte(i + 1)}})
		s.receive(context.Background(), Packet{From: addr, Data: sealed(t, sealer, hb)})
	}
	time.Sleep(100 * time.Millisecond)
	close(link.release)
	if p := link.peak.Load(); p > maxRepairs {
		t.Fatalf("%d repairs ran at once, want at most %d", p, maxRepairs)
	}
}

// Review: a range request with too many ranges gets no answer, and an answer
// stops at maxEntriesPerResponse without reading the remaining ranges.
func TestRangeRequestBounds(t *testing.T) {
	store := ledger.NewMemStore()
	s, sealer := newGuardSyncer(t, &fakeLink{inbox: make(chan Packet)}, store)
	_, priv, _ := ed25519.GenerateKey(nil)
	origin := ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
	for seq := uint64(1); seq <= maxEntriesPerResponse+20; seq++ {
		store.Put(ledgertest.Entry(t, priv, seq, int64(seq)))
	}
	many := make([]ledger.Range, maxRangesPerRequest+1)
	for i := range many {
		many[i] = ledger.Range{Origin: origin, From: 1, To: 1}
	}
	if resp := s.handle("peer", sealed(t, sealer, ledger.AppendRanges([]byte{kindRangeReq}, many))); resp != nil {
		t.Fatal("answered a request with too many ranges")
	}
	two := []ledger.Range{{Origin: origin, From: 1, To: 1 << 40}, {Origin: origin, From: 1, To: 1 << 40}}
	resp := s.handle("peer", sealed(t, sealer, ledger.AppendRanges([]byte{kindRangeReq}, two)))
	plain, err := sealer.Open(resp)
	if err != nil {
		t.Fatal(err)
	}
	es, err := decodeEntries(plain)
	if err != nil || len(es) != maxEntriesPerResponse {
		t.Fatalf("answer has %d entries (%v), want %d", len(es), err, maxEntriesPerResponse)
	}
}
