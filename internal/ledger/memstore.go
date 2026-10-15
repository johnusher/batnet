package ledger

import (
	"bytes"
	"sort"
	"sync"
)

// MemStore is a Store in memory, for tests and the mock village.
type MemStore struct {
	mu      sync.Mutex
	entries map[Key]*Entry
	contig  Vector
	top     Vector // highest seq stored per origin, gaps or not
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{entries: map[Key]*Entry{}, contig: Vector{}, top: Vector{}}
}

func (s *MemStore) Put(e *Entry) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[e.Key]; ok {
		return false, nil
	}
	s.entries[e.Key] = e
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

func (s *MemStore) Range(r Range) ([]*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Entry
	to := r.To
	if top := s.top[r.Origin]; to > top {
		to = top // the request's size must not set the cost
	}
	for seq := r.From; seq <= to && seq >= r.From; seq++ {
		if e, ok := s.entries[Key{Origin: r.Origin, Seq: seq}]; ok {
			out = append(out, e)
		}
	}
	return out, nil
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
