package lora

import (
	"bytes"
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

func TestSplitAndParse(t *testing.T) {
	payload := bytes.Repeat([]byte("abcdefghij"), 45) // 450 bytes -> 3 frames
	frames, err := Split(KindEntry, prio.High, 0xdeadbeef, payload)
	if err != nil || len(frames) != 3 {
		t.Fatalf("%d frames, %v", len(frames), err)
	}
	for i, b := range frames {
		if len(b) > MaxFrame {
			t.Fatalf("frame %d is %d bytes", i, len(b))
		}
		f, err := Parse(b)
		if err != nil || f.Kind != KindEntry || f.Prio != prio.High || f.Sender != 0xdeadbeef || int(f.Index) != i || f.Count != 3 {
			t.Fatalf("frame %d: %+v %v", i, f, err)
		}
	}
	if _, err := Parse([]byte{1, 2}); err == nil {
		t.Fatal("short frame parsed")
	}
	if _, err := Parse(append([]byte{0x01}, frames[0][1:]...)); err == nil {
		t.Fatal("wrong version parsed")
	}
}

// Review focus 3.
func TestReassembly(t *testing.T) {
	a, _ := Split(KindEntry, prio.Normal, 1, bytes.Repeat([]byte("A"), 400))
	b, _ := Split(KindEntry, prio.Normal, 2, bytes.Repeat([]byte("B"), 400))
	r := NewReassembler(60_000_000, 16)
	order := [][]byte{a[2], b[0], a[0], a[0], b[1], a[1]} // interleaved, one duplicate, b incomplete
	var got [][]byte
	for _, fr := range order {
		f, _ := Parse(fr)
		if p, done := r.Add(f, 0); done {
			got = append(got, p)
		}
	}
	if len(got) != 1 || !bytes.Equal(got[0], bytes.Repeat([]byte("A"), 400)) {
		t.Fatalf("rebuilt %d payloads", len(got))
	}
	f, _ := Parse(b[2])
	if _, done := r.Add(f, 120_000_000); done {
		t.Fatal("rebuilt from fragments that had expired")
	}
}

func TestCompactVector(t *testing.T) {
	var x, y ledger.NodeID
	x[0], x[1], x[2], x[3] = 1, 2, 3, 4
	y[0] = 9
	v := ledger.Vector{x: 7, y: 1 << 33}
	got, err := ParseCompactVector(AppendCompactVector(nil, v))
	if err != nil || got[ShortID(x)] != 7 || got[ShortID(y)] != 1<<33 || len(got) != 2 {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := ParseCompactVector([]byte{200}); err == nil {
		t.Fatal("truncated vector parsed")
	}
}
