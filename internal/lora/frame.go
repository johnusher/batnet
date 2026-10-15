package lora

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
	"sort"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

// Kind says what a frame's payload is.
type Kind uint8

const (
	KindEntry    Kind = 1 // a signed log entry
	KindBeacon   Kind = 2 // a gateway's compact version vector
	KindRecovery Kind = 3 // a signed log entry sent to answer a beacon
)

const (
	frameVersion = 1
	HeaderLen    = 10
	maxChunk     = MaxFrame - HeaderLen
)

var errFrame = errors.New("lora: malformed frame")

// Frame is one LoRa packet's worth of a message.
type Frame struct {
	Kind         Kind
	Prio         prio.Level
	Sender       uint32 // the sending node's short ID
	MsgID        uint32 // 24 bits
	Index, Count uint8
	Chunk        []byte
}

func msgID(payload []byte) uint32 {
	h := fnv.New32a()
	h.Write(payload)
	return h.Sum32() & 0xffffff
}

// Split cuts a payload into frames of at most MaxFrame bytes.
func Split(kind Kind, p prio.Level, sender uint32, payload []byte) ([][]byte, error) {
	n := (len(payload) + maxChunk - 1) / maxChunk
	if n == 0 {
		n = 1
	}
	if n > 255 {
		return nil, errors.New("lora: payload too big for 255 fragments")
	}
	id := msgID(payload)
	out := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		lo, hi := i*maxChunk, min((i+1)*maxChunk, len(payload))
		b := make([]byte, 0, HeaderLen+hi-lo)
		b = append(b, frameVersion<<6|byte(kind&3)<<4|byte(p&15))
		b = binary.BigEndian.AppendUint32(b, sender)
		b = append(b, byte(id>>16), byte(id>>8), byte(id), byte(i), byte(n))
		out = append(out, append(b, payload[lo:hi]...))
	}
	return out, nil
}

// Parse reads a frame's header.
func Parse(b []byte) (Frame, error) {
	if len(b) < HeaderLen || b[0]>>6 != frameVersion {
		return Frame{}, errFrame
	}
	f := Frame{Kind: Kind(b[0] >> 4 & 3), Prio: prio.Level(b[0] & 15), Sender: binary.BigEndian.Uint32(b[1:5]),
		MsgID: uint32(b[5])<<16 | uint32(b[6])<<8 | uint32(b[7]), Index: b[8], Count: b[9], Chunk: b[HeaderLen:]}
	if f.Count == 0 || f.Index >= f.Count || f.Prio > prio.Alert || (f.Kind != KindEntry && f.Kind != KindBeacon && f.Kind != KindRecovery) {
		return Frame{}, errFrame
	}
	return f, nil
}

// Reassembler rebuilds payloads from fragments, dropping incomplete ones after a TTL.
type Reassembler struct {
	ttl        int64
	maxPending int
	pending    map[[2]uint32]*partial
}

type partial struct {
	first  int64
	chunks [][]byte
	have   int
	nudged bool // a beacon was already pulled forward for this one
}

// Stalled reports whether a partial message has waited longer than age
// without completing, once per partial: its sender's frames were lost, so a
// beacon soon would ask for the rest.
func (r *Reassembler) Stalled(nowUs, ageUs int64) bool {
	stalled := false
	for _, p := range r.pending {
		if !p.nudged && p.have < len(p.chunks) && nowUs-p.first >= ageUs {
			p.nudged, stalled = true, true
		}
	}
	return stalled
}

// NewReassembler keeps partial messages for ttlUs, at most maxPending at once.
func NewReassembler(ttlUs int64, maxPending int) *Reassembler {
	return &Reassembler{ttl: ttlUs, maxPending: maxPending, pending: map[[2]uint32]*partial{}}
}

// Add takes a fragment; it returns the payload when the last missing one arrives.
func (r *Reassembler) Add(f Frame, nowUs int64) ([]byte, bool) {
	for k, p := range r.pending {
		if nowUs-p.first > r.ttl {
			delete(r.pending, k)
		}
	}
	if f.Count == 1 {
		return append([]byte(nil), f.Chunk...), true
	}
	k := [2]uint32{f.Sender, f.MsgID}
	p := r.pending[k]
	if p == nil {
		if len(r.pending) >= r.maxPending {
			r.evictOldest() // a new message counts for more than the stalest partial one (B12)
		}
		p = &partial{first: nowUs, chunks: make([][]byte, f.Count)}
		r.pending[k] = p
	}
	if int(f.Count) != len(p.chunks) || p.chunks[f.Index] != nil {
		return nil, false // a different message with a clashing ID, or a duplicate
	}
	p.chunks[f.Index] = append([]byte(nil), f.Chunk...)
	p.have++
	if p.have < len(p.chunks) {
		return nil, false
	}
	delete(r.pending, k)
	var out []byte
	for _, c := range p.chunks {
		out = append(out, c...)
	}
	if msgID(out) != f.MsgID {
		return nil, false
	}
	return out, true
}

// evictOldest drops the partial message that started longest ago.
func (r *Reassembler) evictOldest() {
	var oldest [2]uint32
	var at int64
	found := false
	for k, p := range r.pending {
		if !found || p.first < at || (p.first == at && (k[0] < oldest[0] || (k[0] == oldest[0] && k[1] < oldest[1]))) {
			oldest, at, found = k, p.first, true
		}
	}
	if found {
		delete(r.pending, oldest)
	}
}

// ShortID is a node's 4-byte short form, used on air.
func ShortID(id ledger.NodeID) uint32 { return binary.BigEndian.Uint32(id[:4]) }

// AppendCompactVector encodes a vector with short IDs, for beacons.
func AppendCompactVector(b []byte, v ledger.Vector) []byte {
	ids := make([]ledger.NodeID, 0, len(v))
	for id, s := range v {
		if s > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ShortID(ids[i]) < ShortID(ids[j]) })
	b = binary.AppendUvarint(b, uint64(len(ids)))
	for _, id := range ids {
		b = binary.BigEndian.AppendUint32(b, ShortID(id))
		b = binary.AppendUvarint(b, v[id])
	}
	return b
}

// ParseCompactVector decodes a beacon's vector.
func ParseCompactVector(b []byte) (map[uint32]uint64, error) {
	n, k := binary.Uvarint(b)
	if k <= 0 || n > uint64(len(b)) {
		return nil, errFrame
	}
	b = b[k:]
	out := make(map[uint32]uint64, n)
	for i := uint64(0); i < n; i++ {
		if len(b) < 4 {
			return nil, errFrame
		}
		id := binary.BigEndian.Uint32(b)
		s, k := binary.Uvarint(b[4:])
		if k <= 0 {
			return nil, errFrame
		}
		out[id] = s
		b = b[4+k:]
	}
	return out, nil
}
