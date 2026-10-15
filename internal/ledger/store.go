package ledger

import "errors"

// ErrNotFound is returned by Get for an entry the store doesn't hold.
var ErrNotFound = errors.New("ledger: entry not found")

// Store keeps entries. Callers verify signatures before Put.
type Store interface {
	// Put stores e; added is false if the store already had it.
	Put(e *Entry) (added bool, err error)
	Get(k Key) (*Entry, error)
	// Vector is the highest contiguous seq per origin.
	Vector() (Vector, error)
	// Range returns the stored entries in r, ascending by seq; gaps are skipped.
	Range(r Range) ([]*Entry, error)
	// All returns every entry ordered by timestamp, then origin, then seq.
	All() ([]*Entry, error)
	Close() error
}
