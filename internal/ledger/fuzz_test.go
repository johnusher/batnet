package ledger

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/johnusher/batnet/internal/hlc"
)

// Security review 2026-10-22: entries, vectors and range lists arrive from
// other nodes (sealed on the mesh, in the clear over LoRa). The decoders must
// not panic, and what decodes must re-encode to the same bytes.

func FuzzEntry(f *testing.F) {
	_, priv, _ := ed25519.GenerateKey(nil)
	e := &Entry{Key: Key{Origin: NodeIDFromPublic(priv.Public().(ed25519.PublicKey)), Seq: 7},
		TS: hlc.Timestamp{Wall: 1_790_000_000_000, Logical: 3}, Type: TypeMsgPost, Prio: 5, Body: []byte(`{"text":"hi"}`)}
	e.Sign(priv)
	raw, _ := e.MarshalBinary()
	f.Add(raw)
	f.Add(raw[:len(raw)-1])
	f.Add([]byte{formatVersion})
	f.Add([]byte{})
	f.Add(append([]byte{formatVersion}, bytes.Repeat([]byte{0xff}, 60)...))
	f.Fuzz(func(t *testing.T, b []byte) {
		var e Entry
		if err := e.UnmarshalBinary(b); err != nil {
			return
		}
		e.Verify()
		// A padded varint decodes too (the signature covers the canonical form,
		// so the same entry has several wire forms); the canonical form is stable.
		again, _ := e.MarshalBinary()
		var e2 Entry
		if err := e2.UnmarshalBinary(again); err != nil {
			t.Fatalf("canonical form does not decode: %v", err)
		}
		if twice, _ := e2.MarshalBinary(); !bytes.Equal(again, twice) {
			t.Fatalf("canonical form is not stable:\n%x\n%x", again, twice)
		}
	})
}

func FuzzVectorAndRanges(f *testing.F) {
	v := Vector{}
	var o NodeID
	o[0] = 1
	v[o] = 5
	f.Add(AppendVector(nil, v))
	f.Add(AppendRanges(nil, []Range{{Origin: o, From: 1, To: 5}}))
	f.Add([]byte{0xff, 0xff, 0x03}) // 65535 claimed, nothing behind (B6)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		if v, _, err := ParseVector(b); err == nil {
			v.Hash()
			v.Missing(v)
			AppendVector(nil, v)
		}
		if rs, _, err := ParseRanges(b); err == nil {
			AppendRanges(nil, rs)
		}
	})
}
