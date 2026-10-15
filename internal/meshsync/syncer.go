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
}

// Syncer keeps one node's log in step with the mesh.
type Syncer struct {
	cfg       Config
	mu        sync.Mutex
	repairing map[string]bool
}

const (
	maxRangesPerRequest = 64
	maxRepairRounds     = 50
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
	s := &Syncer{cfg: cfg, repairing: map[string]bool{}}
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
	return s.broadcast(encodeHeartbeat(heartbeat{Addr: s.cfg.Link.Addr(), Hash: v.Hash()}))
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
	if hb.Addr == s.cfg.Link.Addr() {
		return
	}
	v, err := s.cfg.Store.Vector()
	if err != nil || v.Hash() == hb.Hash {
		return
	}
	s.mu.Lock()
	busy := s.repairing[hb.Addr]
	s.repairing[hb.Addr] = true
	s.mu.Unlock()
	if busy {
		return
	}
	go func() {
		defer func() { s.mu.Lock(); delete(s.repairing, hb.Addr); s.mu.Unlock() }()
		if n, err := s.Repair(ctx, hb.Addr); err != nil {
			s.cfg.Logf("repair from %s: %v", hb.Addr, err)
		} else if n > 0 {
			s.cfg.Logf("repair from %s: %d entries", hb.Addr, n)
		}
	}()
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
		if err != nil || len(rest) != 0 {
			return nil
		}
		var out []*ledger.Entry
		for _, r := range rs {
			es, err := s.cfg.Store.Range(r)
			if err != nil {
				return nil
			}
			for _, e := range es {
				if len(out) == maxEntriesPerResponse {
					break
				}
				out = append(out, e)
			}
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
