// Package ledger is batnet's replicated log: signed, append-only entries keyed
// by (origin node, seq), ordered by hybrid logical clock.
package ledger

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/johnusher/batnet/internal/hlc"
)

// NodeID is a node's ed25519 public key.
type NodeID [ed25519.PublicKeySize]byte

// NodeIDFromPublic turns a public key into a NodeID.
func NodeIDFromPublic(pub ed25519.PublicKey) NodeID {
	var id NodeID
	copy(id[:], pub)
	return id
}

// String is a short form for logs: the first 4 bytes in hex.
func (n NodeID) String() string { return hex.EncodeToString(n[:4]) }

// Key identifies an entry: who wrote it and its place in their sequence (from 1).
type Key struct {
	Origin NodeID
	Seq    uint64
}

// Type says what an entry means; its Body is JSON for that type.
type Type string

const (
	TypeNodeAnnounce   Type = "node.announce"
	TypeHouseJoin      Type = "house.join"
	TypeHouseApprove   Type = "house.approve"
	TypeAccountRequest Type = "account.request"
	TypeAccountApprove Type = "account.approve"
	TypeAccountDeny    Type = "account.deny"
	TypeAccountUpdate  Type = "account.update"
	TypeGroupCreate    Type = "group.create"
	TypeMsgPost        Type = "msg.post"
	TypeLoRaSent       Type = "lora.sent"
	TypeAlert          Type = "alert"
)

const (
	formatVersion = 1
	MaxBody       = 64 << 10 // bytes
	MaxPrio       = 9
	maxType       = 64
)

var (
	ErrBadSignature = errors.New("ledger: bad signature")
	ErrMalformed    = errors.New("ledger: malformed entry")
)

// Entry is one immutable record in the log.
type Entry struct {
	Key  Key
	TS   hlc.Timestamp
	Type Type
	Prio uint8 // 0-9, see decision 0015
	Body []byte
	Sig  []byte // ed25519 by Key.Origin over everything above
}

func (e *Entry) appendSigned(b []byte) []byte {
	b = append(b, formatVersion)
	b = append(b, e.Key.Origin[:]...)
	b = binary.AppendUvarint(b, e.Key.Seq)
	b = binary.AppendVarint(b, e.TS.Wall)
	b = binary.AppendUvarint(b, uint64(e.TS.Logical))
	b = appendBytes(b, []byte(e.Type))
	b = append(b, e.Prio)
	return appendBytes(b, e.Body)
}

// Sign sets Sig using the origin node's private key.
func (e *Entry) Sign(priv ed25519.PrivateKey) {
	e.Sig = ed25519.Sign(priv, e.appendSigned(nil))
}

// Verify checks that Sig was made by Key.Origin over the entry's contents.
func (e *Entry) Verify() error {
	if len(e.Sig) != ed25519.SignatureSize ||
		!ed25519.Verify(ed25519.PublicKey(e.Key.Origin[:]), e.appendSigned(nil), e.Sig) {
		return ErrBadSignature
	}
	return nil
}

// MarshalBinary encodes the entry for the wire and the database.
func (e *Entry) MarshalBinary() ([]byte, error) {
	return appendBytes(e.appendSigned(nil), e.Sig), nil
}

// UnmarshalBinary decodes an entry and checks its fields' limits (not its signature).
func (e *Entry) UnmarshalBinary(b []byte) error {
	r := reader{b: b}
	if r.byte() != formatVersion {
		return fmt.Errorf("%w: unknown version", ErrMalformed)
	}
	var out Entry
	copy(out.Key.Origin[:], r.fixed(len(out.Key.Origin)))
	out.Key.Seq = r.uvarint()
	out.TS.Wall = r.varint()
	logical := r.uvarint()
	out.Type = Type(r.bytes(maxType))
	out.Prio = r.byte()
	out.Body = r.bytes(MaxBody)
	out.Sig = r.bytes(ed25519.SignatureSize)
	switch {
	case r.err != nil:
		return fmt.Errorf("%w: %v", ErrMalformed, r.err)
	case len(r.b) != 0:
		return fmt.Errorf("%w: %d trailing bytes", ErrMalformed, len(r.b))
	case out.Key.Seq == 0:
		return fmt.Errorf("%w: seq 0", ErrMalformed)
	case out.Prio > MaxPrio:
		return fmt.Errorf("%w: priority %d", ErrMalformed, out.Prio)
	case logical > 1<<32-1:
		return fmt.Errorf("%w: logical clock overflow", ErrMalformed)
	}
	out.TS.Logical = uint32(logical)
	*e = out
	return nil
}

func appendBytes(b, v []byte) []byte {
	b = binary.AppendUvarint(b, uint64(len(v)))
	return append(b, v...)
}

// reader walks a byte slice; the first error sticks and later reads return zero values.
type reader struct {
	b   []byte
	err error
}

func (r *reader) fail(msg string) {
	if r.err == nil {
		r.err = errors.New(msg)
	}
}

func (r *reader) byte() byte {
	if r.err != nil || len(r.b) < 1 {
		r.fail("short read")
		return 0
	}
	v := r.b[0]
	r.b = r.b[1:]
	return v
}

func (r *reader) fixed(n int) []byte {
	if r.err != nil || len(r.b) < n {
		r.fail("short read")
		return make([]byte, n)
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v
}

func (r *reader) uvarint() uint64 {
	if r.err != nil {
		return 0
	}
	v, n := binary.Uvarint(r.b)
	if n <= 0 {
		r.fail("bad uvarint")
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *reader) varint() int64 {
	if r.err != nil {
		return 0
	}
	v, n := binary.Varint(r.b)
	if n <= 0 {
		r.fail("bad varint")
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *reader) bytes(max int) []byte {
	n := r.uvarint()
	if r.err != nil {
		return nil
	}
	if n > uint64(max) {
		r.fail("field too long")
		return nil
	}
	return append([]byte(nil), r.fixed(int(n))...)
}
