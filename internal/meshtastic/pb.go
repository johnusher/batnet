// Package meshtastic is a small client for meshtasticd's TCP API (port 4403):
// it reads the node list, the radio's settings and text messages, and sends
// text. It decodes the few protobuf fields batnet needs by hand, so batnetd
// stays pure Go with no generated code (decision 0005).
package meshtastic

import (
	"encoding/binary"
	"errors"
	"math"
)

// Protobuf wire types.
const (
	wireVarint = 0
	wireI64    = 1
	wireLen    = 2
	wireI32    = 5
)

var errTruncated = errors.New("meshtastic: truncated protobuf")

// field is one decoded protobuf field: num and type, with the value in v
// (varint, fixed32, fixed64) or b (length-delimited).
type field struct {
	num int
	typ int
	v   uint64
	b   []byte
}

func (f field) u32() uint32  { return uint32(f.v) }
func (f field) f32() float32 { return math.Float32frombits(uint32(f.v)) }
func (f field) i32() int32   { return int32(f.v) }

// fields splits a message into its fields, in order.
func fields(b []byte) ([]field, error) {
	var out []field
	for len(b) > 0 {
		key, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, errTruncated
		}
		b = b[n:]
		f := field{num: int(key >> 3), typ: int(key & 7)}
		switch f.typ {
		case wireVarint:
			f.v, n = binary.Uvarint(b)
			if n <= 0 {
				return nil, errTruncated
			}
			b = b[n:]
		case wireI64:
			if len(b) < 8 {
				return nil, errTruncated
			}
			f.v, b = binary.LittleEndian.Uint64(b), b[8:]
		case wireI32:
			if len(b) < 4 {
				return nil, errTruncated
			}
			f.v, b = uint64(binary.LittleEndian.Uint32(b)), b[4:]
		case wireLen:
			l, n := binary.Uvarint(b)
			if n <= 0 || uint64(len(b)-n) < l {
				return nil, errTruncated
			}
			f.b, b = b[n:n+int(l)], b[n+int(l):]
		default:
			return nil, errors.New("meshtastic: unsupported protobuf wire type")
		}
		out = append(out, f)
	}
	return out, nil
}

// enc builds a protobuf message.
type enc struct{ b []byte }

func (e *enc) key(num, typ int) { e.b = binary.AppendUvarint(e.b, uint64(num)<<3|uint64(typ)) }

func (e *enc) varint(num int, v uint64) {
	e.key(num, wireVarint)
	e.b = binary.AppendUvarint(e.b, v)
}

func (e *enc) fixed32(num int, v uint32) {
	e.key(num, wireI32)
	e.b = binary.LittleEndian.AppendUint32(e.b, v)
}

func (e *enc) bytes(num int, v []byte) {
	e.key(num, wireLen)
	e.b = binary.AppendUvarint(e.b, uint64(len(v)))
	e.b = append(e.b, v...)
}

// put appends f as it was received (to rebuild a message with some fields changed).
func (e *enc) put(f field) {
	switch f.typ {
	case wireVarint:
		e.varint(f.num, f.v)
	case wireI32:
		e.fixed32(f.num, uint32(f.v))
	case wireI64:
		e.key(f.num, wireI64)
		e.b = binary.LittleEndian.AppendUint64(e.b, f.v)
	case wireLen:
		e.bytes(f.num, f.b)
	}
}
