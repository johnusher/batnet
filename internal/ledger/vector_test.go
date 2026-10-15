package ledger

import (
	"reflect"
	"testing"
)

func id(b byte) NodeID { var n NodeID; n[0] = b; return n }

func TestHashIgnoresMapOrder(t *testing.T) {
	a := Vector{id(1): 3, id(2): 5}
	b := Vector{id(2): 5, id(1): 3}
	if a.Hash() != b.Hash() {
		t.Fatal("equal vectors hash differently")
	}
	if a.Hash() == (Vector{id(1): 3, id(2): 6}).Hash() {
		t.Fatal("different vectors hash the same")
	}
}

func TestHashTreatsZeroAsAbsent(t *testing.T) {
	if (Vector{id(1): 0}).Hash() != (Vector{}).Hash() {
		t.Fatal("a zero entry should hash like an absent one")
	}
}

func TestMissing(t *testing.T) {
	mine := Vector{id(1): 3, id(2): 5}
	theirs := Vector{id(1): 7, id(2): 2, id(3): 4}
	want := []Range{{Origin: id(1), From: 4, To: 7}, {Origin: id(3), From: 1, To: 4}}
	if got := mine.Missing(theirs); !reflect.DeepEqual(got, want) {
		t.Fatalf("Missing = %+v, want %+v", got, want)
	}
	if got := theirs.Missing(theirs); len(got) != 0 {
		t.Fatalf("nothing should be missing from an equal vector, got %+v", got)
	}
}

func TestVectorEncoding(t *testing.T) {
	v := Vector{id(1): 3, id(9): 1 << 40}
	got, rest, err := ParseVector(AppendVector(nil, v))
	if err != nil || len(rest) != 0 || !reflect.DeepEqual(got, v) {
		t.Fatalf("got %v rest %d err %v", got, len(rest), err)
	}
	if _, _, err := ParseVector([]byte{5}); err == nil {
		t.Fatal("truncated vector parsed")
	}
}

func TestRangesEncoding(t *testing.T) {
	rs := []Range{{Origin: id(1), From: 4, To: 7}, {Origin: id(2), From: 1, To: 1}}
	got, rest, err := ParseRanges(AppendRanges(nil, rs))
	if err != nil || len(rest) != 0 || !reflect.DeepEqual(got, rs) {
		t.Fatalf("got %v rest %d err %v", got, len(rest), err)
	}
	if _, _, err := ParseRanges([]byte{0xff, 0xff, 0x03}); err == nil {
		t.Fatal("over-long range list parsed")
	}
}
