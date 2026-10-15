package ledger

import (
	"errors"

	"github.com/johnusher/batnet/internal/hlc"
)

// ErrNotFound is returned by Get for an entry the store doesn't hold.
var ErrNotFound = errors.New("ledger: entry not found")

// Store keeps entries. Callers verify signatures before Put.
type Store interface {
	// Put stores e; added is false if the store already had it.
	Put(e *Entry) (added bool, err error)
	Get(k Key) (*Entry, error)
	// Vector is the highest contiguous seq per origin.
	Vector() (Vector, error)
	// Range returns up to limit stored entries in r, ascending by seq; gaps are
	// skipped. Its cost follows what is stored and limit, never the span of r.
	Range(r Range, limit int) ([]*Entry, error)
	// Top is the highest seq stored for origin, gaps or not (0 if none).
	Top(origin NodeID) (uint64, error)
	// Latest is the highest timestamp stored (zero if empty).
	Latest() (hlc.Timestamp, error)
	// All returns every entry ordered by timestamp, then origin, then seq.
	All() ([]*Entry, error)
	Close() error
}
