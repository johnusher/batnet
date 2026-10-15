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

// NewAuthor continues from the highest seq of this node already in store.
func NewAuthor(priv ed25519.PrivateKey, clock *hlc.Clock, store Store) (*Author, error) {
	id := NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
	v, err := store.Vector()
	if err != nil {
		return nil, err
	}
	return &Author{priv: priv, id: id, clock: clock, store: store, seq: v[id]}, nil
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
	if _, err := a.store.Put(e); err != nil {
		return nil, err
	}
	a.seq++
	return e, nil
}
