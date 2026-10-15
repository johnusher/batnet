package ledger

import (
	"testing"

	"github.com/johnusher/batnet/internal/hlc"
)

type post struct {
	Audience string `json:"audience"`
	Text     string `json:"text"`
}

func TestAppendSignsAndStores(t *testing.T) {
	s := NewMemStore()
	a, err := NewAuthor(newKey(t), hlc.New(func() int64 { return 1000 }), s)
	if err != nil {
		t.Fatal(err)
	}
	e1, err := a.Append(TypeMsgPost, 5, post{"everyone", "Bread at the bakery"})
	if err != nil {
		t.Fatal(err)
	}
	e2, _ := a.Append(TypeMsgPost, 5, post{"everyone", "Sold out"})
	if e1.Key.Seq != 1 || e2.Key.Seq != 2 || !e1.TS.Less(e2.TS) {
		t.Fatalf("seqs %d,%d ts %v,%v", e1.Key.Seq, e2.Key.Seq, e1.TS, e2.TS)
	}
	if err := e2.Verify(); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(e2.Key); got == nil {
		t.Fatal("entry not stored")
	}
}

// Review focus 5: a restart on an existing store continues the sequence.
func TestAuthorResumes(t *testing.T) {
	s, priv := NewMemStore(), newKey(t)
	clock := hlc.New(func() int64 { return 1000 })
	a, _ := NewAuthor(priv, clock, s)
	a.Append(TypeMsgPost, 5, post{"everyone", "one"})
	a.Append(TypeMsgPost, 5, post{"everyone", "two"})
	b, err := NewAuthor(priv, clock, s)
	if err != nil {
		t.Fatal(err)
	}
	e, err := b.Append(TypeMsgPost, 5, post{"everyone", "three"})
	if err != nil || e.Key.Seq != 3 {
		t.Fatalf("after restart seq = %d, err %v; want 3", e.Key.Seq, err)
	}
}

func TestAppendRejectsBadPriority(t *testing.T) {
	a, _ := NewAuthor(newKey(t), hlc.New(func() int64 { return 1 }), NewMemStore())
	if _, err := a.Append(TypeMsgPost, 10, post{"everyone", "x"}); err == nil {
		t.Fatal("priority 10 accepted")
	}
}
