package meshsync

import (
	"context"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/seal"
)

// Config wires a Syncer to its node.
type Config struct {
	Link      Link
	Store     ledger.Store
	Clock     *hlc.Clock
	Sealer    *seal.Sealer
	Heartbeat time.Duration       // 5 s on a Pi
	OnEntry   func(*ledger.Entry) // once per entry new to this node, from the network
	Logf      func(format string, args ...any)
	// Spawn runs a repair. Default: a new goroutine. The simulator runs it inline.
	Spawn func(func())
}

// Syncer keeps one node's log in step with the mesh.
type Syncer struct {
	cfg       Config
	ctx       context.Context // for repairs started by HandlePacket
	mu        sync.Mutex
	repairing map[string]bool
	slots     chan struct{} // one per repair allowed to run at once
}

const (
	maxRangesPerRequest = 64
	maxRepairRounds     = 50
	maxRepairs          = 4 // repairs running at once
	requestTimeout      = 10 * time.Second
)

// New returns a Syncer and installs its request handler on the link.
func New(cfg Config) *Syncer {
	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = 5 * time.Second
	}
	if cfg.OnEntry == nil {
		cfg.OnEntry = func(*ledger.Entry) {}
	}
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	if cfg.Spawn == nil {
		cfg.Spawn = func(f func()) { go f() }
	}
	s := &Syncer{cfg: cfg, ctx: context.Background(), repairing: map[string]bool{}, slots: make(chan struct{}, maxRepairs)}
	cfg.Link.SetHandler(s.handle)
	return s
}

// Publish broadcasts an entry this node just appended.
func (s *Syncer) Publish(e *ledger.Entry) error { return s.broadcast(encodeEntry(e)) }

// SendHeartbeat broadcasts this node's address and vector hash.
func (s *Syncer) SendHeartbeat() error {
	v, err := s.cfg.Store.Vector()
	if err != nil {
		return err
	}
	box, err := s.cfg.Sealer.Seal(encodeHeartbeat(heartbeat{Addr: s.cfg.Link.Addr(), Hash: v.Hash()}))
	if err != nil {
		return err
	}
	return s.cfg.Link.BroadcastLocal(box) // one hop: neighbours compare and repair hop by hop
}

func (s *Syncer) broadcast(pkt []byte) error {
	box, err := s.cfg.Sealer.Seal(pkt)
	if err != nil {
		return err
	}
	return s.cfg.Link.Broadcast(box)
}

// Run handles broadcasts and sends heartbeats until ctx ends.
func (s *Syncer) Run(ctx context.Context) {
	tick := time.NewTicker(s.cfg.Heartbeat)
	defer tick.Stop()
	s.SendHeartbeat()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := s.SendHeartbeat(); err != nil {
				s.cfg.Logf("heartbeat: %v", err)
			}
		case p := <-s.cfg.Link.Inbox():
			s.receive(ctx, p)
		}
	}
}

// HandlePacket processes one broadcast from the mesh. Run calls it for every
// packet; the simulator calls it directly in virtual time.
func (s *Syncer) HandlePacket(p Packet) { s.receive(s.ctx, p) }

func (s *Syncer) receive(ctx context.Context, p Packet) {
	plain, err := s.cfg.Sealer.Open(p.Data)
	if err != nil {
		return // another village, or noise
	}
	e, hb, err := decodeBroadcast(plain)
	if err != nil {
		return
	}
	if e != nil {
		s.accept(e)
		return
	}
	if hb.Addr == s.cfg.Link.Addr() || hb.Addr != p.From {
		return // our own, or a heartbeat claiming to be someone else
	}
	v, err := s.cfg.Store.Vector()
	if err != nil || v.Hash() == hb.Hash {
		return
	}
	s.mu.Lock()
	busy := s.repairing[hb.Addr]
	if !busy {
		select {
		case s.slots <- struct{}{}:
			s.repairing[hb.Addr] = true
		default:
			busy = true // enough repairs running; the next heartbeat tries again
		}
	}
	s.mu.Unlock()
	if busy {
		return
	}
	s.cfg.Spawn(func() {
		defer func() { s.mu.Lock(); delete(s.repairing, hb.Addr); <-s.slots; s.mu.Unlock() }()
		if n, err := s.Repair(ctx, hb.Addr); err != nil {
			s.cfg.Logf("repair from %s: %v", hb.Addr, err)
		} else if n > 0 {
			s.cfg.Logf("repair from %s: %d entries", hb.Addr, n)
		}
	})
}

// accept verifies and stores an entry from the network; true if it was new.
func (s *Syncer) accept(e *ledger.Entry) bool {
	if e.Verify() != nil {
		return false
	}
	added, err := s.cfg.Store.Put(e)
	if err != nil || !added {
		return false
	}
	s.cfg.Clock.Observe(e.TS)
	s.cfg.OnEntry(e)
	return true
}

// Repair pulls from peer whatever it has that this node lacks.
func (s *Syncer) Repair(ctx context.Context, peer string) (int, error) {
	total := 0
	for round := 0; round < maxRepairRounds; round++ {
		theirs, err := ask(s, ctx, peer, []byte{kindVectorReq}, func(b []byte) (ledger.Vector, error) {
			v, rest, err := ledger.ParseVector(b)
			if err == nil && len(rest) != 0 {
				err = errWire
			}
			return v, err
		})
		if err != nil {
			return total, err
		}
		mine, err := s.cfg.Store.Vector()
		if err != nil {
			return total, err
		}
		missing := mine.Missing(theirs)
		if len(missing) == 0 {
			return total, nil
		}
		if len(missing) > maxRangesPerRequest {
			missing = missing[:maxRangesPerRequest]
		}
		es, err := ask(s, ctx, peer, ledger.AppendRanges([]byte{kindRangeReq}, missing), decodeEntries)
		if err != nil {
			return total, err
		}
		added := 0
		for _, e := range es {
			if s.accept(e) {
				added++
			}
		}
		total += added
		if added == 0 {
			return total, nil // nothing usable came back; try again at the next heartbeat
		}
	}
	return total, nil
}

// ask sends a sealed request and decodes the sealed answer. (A function, not a
// method: Go methods can't have type parameters.)
func ask[T any](s *Syncer, ctx context.Context, peer string, req []byte, decode func([]byte) (T, error)) (T, error) {
	var zero T
	box, err := s.cfg.Sealer.Seal(req)
	if err != nil {
		return zero, err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	resp, err := s.cfg.Link.Request(ctx, peer, box)
	if err != nil {
		return zero, err
	}
	plain, err := s.cfg.Sealer.Open(resp)
	if err != nil {
		return zero, err
	}
	return decode(plain)
}

// handle answers another node's request; nil means no answer.
func (s *Syncer) handle(from string, req []byte) []byte {
	plain, err := s.cfg.Sealer.Open(req)
	if err != nil || len(plain) == 0 {
		return nil
	}
	var resp []byte
	switch plain[0] {
	case kindVectorReq:
		v, err := s.cfg.Store.Vector()
		if err != nil {
			return nil
		}
		resp = ledger.AppendVector(nil, v)
	case kindRangeReq:
		rs, rest, err := ledger.ParseRanges(plain[1:])
		if err != nil || len(rest) != 0 || len(rs) > maxRangesPerRequest {
			return nil
		}
		var out []*ledger.Entry
		for _, r := range rs {
			if len(out) == maxEntriesPerResponse {
				break
			}
			es, err := s.cfg.Store.Range(r, maxEntriesPerResponse-len(out))
			if err != nil {
				return nil
			}
			out = append(out, es...)
		}
		resp = encodeEntries(out)
	default:
		return nil
	}
	box, err := s.cfg.Sealer.Seal(resp)
	if err != nil {
		return nil
	}
	return box
}

// Inject takes an entry from outside the mesh (the LoRa bridge): it is verified,
// stored, reported to OnEntry and broadcast on the mesh if new.
func (s *Syncer) Inject(e *ledger.Entry) bool {
	if !s.accept(e) {
		return false
	}
	if err := s.Publish(e); err != nil {
		s.cfg.Logf("inject: %v", err)
	}
	return true
}
