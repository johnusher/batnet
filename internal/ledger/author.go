package ledger

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/johnusher/batnet/internal/hlc"
)

// Author appends this node's own entries to the log.
type Author struct {
	mu    sync.Mutex
	priv  ed25519.PrivateKey
	id    NodeID
	clock *hlc.Clock
	store Store
	seq   uint64
}

// NewAuthor continues after the highest seq of this node already in store
// (gaps or not, so a seq is never reused), and moves the clock past every
// stored timestamp, so a restart with a wrong clock still orders correctly.
func NewAuthor(priv ed25519.PrivateKey, clock *hlc.Clock, store Store) (*Author, error) {
	id := NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
	top, err := store.Top(id)
	if err != nil {
		return nil, err
	}
	latest, err := store.Latest()
	if err != nil {
		return nil, err
	}
	clock.Observe(latest)
	return &Author{priv: priv, id: id, clock: clock, store: store, seq: top}, nil
}

// ID is this node's identity.
func (a *Author) ID() NodeID { return a.id }

// Append signs and stores a new entry with body encoded as JSON.
func (a *Author) Append(t Type, prio uint8, body any) (*Entry, error) {
	if prio > MaxPrio {
		return nil, fmt.Errorf("ledger: priority %d above %d", prio, MaxPrio)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxBody {
		return nil, fmt.Errorf("ledger: body of %d bytes above %d", len(raw), MaxBody)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	e := &Entry{Key: Key{Origin: a.id, Seq: a.seq + 1}, TS: a.clock.Now(), Type: t, Prio: prio, Body: raw}
	e.Sign(a.priv)
	added, err := a.store.Put(e)
	if err != nil {
		return nil, err
	}
	if !added {
		return nil, fmt.Errorf("ledger: seq %d of this node is already stored", e.Key.Seq)
	}
	a.seq++
	return e, nil
}
