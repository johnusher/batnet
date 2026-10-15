package ledger

import (
	"bytes"
	"sort"
	"sync"

	"github.com/johnusher/batnet/internal/hlc"
)

// MemStore is a Store in memory, for tests and the mock village.
type MemStore struct {
	mu       sync.Mutex
	entries  map[Key]*Entry
	byOrigin map[NodeID][]uint64 // seqs stored per origin, unsorted
	contig   Vector
	top      Vector // highest seq stored per origin, gaps or not
	latest   hlc.Timestamp
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{entries: map[Key]*Entry{}, byOrigin: map[NodeID][]uint64{}, contig: Vector{}, top: Vector{}}
}

func (s *MemStore) Put(e *Entry) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[e.Key]; ok {
		return false, nil
	}
	s.entries[e.Key] = e
	s.byOrigin[e.Key.Origin] = append(s.byOrigin[e.Key.Origin], e.Key.Seq)
	if s.latest.Less(e.TS) {
		s.latest = e.TS
	}
	if e.Key.Seq > s.top[e.Key.Origin] {
		s.top[e.Key.Origin] = e.Key.Seq
	}
	for {
		next := Key{Origin: e.Key.Origin, Seq: s.contig[e.Key.Origin] + 1}
		if _, ok := s.entries[next]; !ok {
			break
		}
		s.contig[e.Key.Origin] = next.Seq
	}
	return true, nil
}

func (s *MemStore) Get(k Key) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[k]; ok {
		return e, nil
	}
	return nil, ErrNotFound
}

func (s *MemStore) Vector() (Vector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := make(Vector, len(s.contig))
	for o, n := range s.contig {
		v[o] = n
	}
	return v, nil
}

func (s *MemStore) Range(r Range, limit int) ([]*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var seqs []uint64
	for _, seq := range s.byOrigin[r.Origin] {
		if seq >= r.From && seq <= r.To {
			seqs = append(seqs, seq)
		}
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	if len(seqs) > limit {
		seqs = seqs[:limit]
	}
	out := make([]*Entry, 0, len(seqs))
	for _, seq := range seqs {
		out = append(out, s.entries[Key{Origin: r.Origin, Seq: seq}])
	}
	return out, nil
}

func (s *MemStore) Top(origin NodeID) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.top[origin], nil
}

func (s *MemStore) Latest() (hlc.Timestamp, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latest, nil
}

func (s *MemStore) All() ([]*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	SortEntries(out)
	return out, nil
}

func (s *MemStore) Close() error { return nil }

// SortEntries orders entries by timestamp, then origin, then seq.
func SortEntries(es []*Entry) {
	sort.Slice(es, func(i, j int) bool {
		a, b := es[i], es[j]
		if a.TS != b.TS {
			return a.TS.Less(b.TS)
		}
		if c := bytes.Compare(a.Key.Origin[:], b.Key.Origin[:]); c != 0 {
			return c < 0
		}
		return a.Key.Seq < b.Key.Seq
	})
}
