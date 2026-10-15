package meshsync

import (
	"encoding/binary"
	"errors"

	"github.com/johnusher/batnet/internal/ledger"
)

// Packet kinds: the first byte of every (unsealed) packet.
const (
	kindEntry     byte = 'E' // broadcast: one new entry
	kindHeartbeat byte = 'H' // broadcast: sender address and vector hash
	kindVectorReq byte = 'V' // request: send me your vector
	kindRangeReq  byte = 'R' // request: send me these ranges
)

const (
	maxEntriesPerResponse = 500
	maxAddr               = 64
)

var errWire = errors.New("meshsync: malformed packet")

type heartbeat struct {
	Addr string
	Hash [32]byte
}

func encodeEntry(e *ledger.Entry) []byte {
	raw, _ := e.MarshalBinary()
	return append([]byte{kindEntry}, raw...)
}

func encodeHeartbeat(h heartbeat) []byte {
	b := []byte{kindHeartbeat}
	b = binary.AppendUvarint(b, uint64(len(h.Addr)))
	b = append(b, h.Addr...)
	return append(b, h.Hash[:]...)
}

// decodeBroadcast returns exactly one of an entry or a heartbeat.
func decodeBroadcast(b []byte) (*ledger.Entry, *heartbeat, error) {
	if len(b) < 2 {
		return nil, nil, errWire
	}
	switch b[0] {
	case kindEntry:
		var e ledger.Entry
		if err := e.UnmarshalBinary(b[1:]); err != nil {
			return nil, nil, errWire
		}
		return &e, nil, nil
	case kindHeartbeat:
		n, k := binary.Uvarint(b[1:])
		rest := b[1:]
		if k <= 0 || n > maxAddr || uint64(len(rest)-k) != n+32 {
			return nil, nil, errWire
		}
		rest = rest[k:]
		h := heartbeat{Addr: string(rest[:n])}
		copy(h.Hash[:], rest[n:])
		return nil, &h, nil
	}
	return nil, nil, errWire
}

func encodeEntries(es []*ledger.Entry) []byte {
	b := binary.AppendUvarint(nil, uint64(len(es)))
	for _, e := range es {
		raw, _ := e.MarshalBinary()
		b = binary.AppendUvarint(b, uint64(len(raw)))
		b = append(b, raw...)
	}
	return b
}

func decodeEntries(b []byte) ([]*ledger.Entry, error) {
	n, k := binary.Uvarint(b)
	if k <= 0 || n > maxEntriesPerResponse {
		return nil, errWire
	}
	b = b[k:]
	out := make([]*ledger.Entry, 0, n)
	for i := uint64(0); i < n; i++ {
		l, k := binary.Uvarint(b)
		if k <= 0 || uint64(len(b)-k) < l {
			return nil, errWire
		}
		var e ledger.Entry
		if err := e.UnmarshalBinary(b[k : k+int(l)]); err != nil {
			return nil, errWire
		}
		out = append(out, &e)
		b = b[k+int(l):]
	}
	if len(b) != 0 {
		return nil, errWire
	}
	return out, nil
}
