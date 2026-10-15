package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
)

// Vector holds, per origin, the highest seq a node has with no gaps below it.
type Vector map[NodeID]uint64

// Range is a run of one origin's entries, From and To inclusive.
type Range struct {
	Origin   NodeID
	From, To uint64
}

// MaxRanges bounds a range list on the wire.
const MaxRanges = 1024

var errVector = errors.New("ledger: malformed vector or range list")

func (v Vector) sorted() []NodeID {
	ids := make([]NodeID, 0, len(v))
	for o, s := range v {
		if s > 0 {
			ids = append(ids, o)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	return ids
}

// Hash summarises the vector; two nodes with equal hashes hold the same contiguous log.
func (v Vector) Hash() [32]byte {
	h := sha256.New()
	var buf [binary.MaxVarintLen64]byte
	for _, o := range v.sorted() {
		h.Write(o[:])
		h.Write(buf[:binary.PutUvarint(buf[:], v[o])])
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// Missing lists what other has that v lacks, sorted by origin.
func (v Vector) Missing(other Vector) []Range {
	var out []Range
	for _, o := range other.sorted() {
		if other[o] > v[o] {
			out = append(out, Range{Origin: o, From: v[o] + 1, To: other[o]})
		}
	}
	return out
}

// AppendVector encodes v.
func AppendVector(b []byte, v Vector) []byte {
	ids := v.sorted()
	b = binary.AppendUvarint(b, uint64(len(ids)))
	for _, o := range ids {
		b = append(b, o[:]...)
		b = binary.AppendUvarint(b, v[o])
	}
	return b
}

// ParseVector decodes a vector and returns the bytes after it.
func ParseVector(b []byte) (Vector, []byte, error) {
	r := reader{b: b}
	n := r.uvarint()
	if r.err != nil || n > MaxRanges*64 {
		return nil, nil, errVector
	}
	v := make(Vector, n)
	for i := uint64(0); i < n; i++ {
		var o NodeID
		copy(o[:], r.fixed(len(o)))
		v[o] = r.uvarint()
	}
	if r.err != nil {
		return nil, nil, errVector
	}
	return v, r.b, nil
}

// AppendRanges encodes a range list.
func AppendRanges(b []byte, rs []Range) []byte {
	b = binary.AppendUvarint(b, uint64(len(rs)))
	for _, x := range rs {
		b = append(b, x.Origin[:]...)
		b = binary.AppendUvarint(b, x.From)
		b = binary.AppendUvarint(b, x.To)
	}
	return b
}

// ParseRanges decodes a range list and returns the bytes after it.
func ParseRanges(b []byte) ([]Range, []byte, error) {
	r := reader{b: b}
	n := r.uvarint()
	if r.err != nil || n > MaxRanges {
		return nil, nil, errVector
	}
	rs := make([]Range, 0, n)
	for i := uint64(0); i < n; i++ {
		var x Range
		copy(x.Origin[:], r.fixed(len(x.Origin)))
		x.From, x.To = r.uvarint(), r.uvarint()
		rs = append(rs, x)
	}
	if r.err != nil {
		return nil, nil, errVector
	}
	return rs, r.b, nil
}
