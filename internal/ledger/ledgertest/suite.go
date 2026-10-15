// Package ledgertest is the behaviour every ledger.Store must have.
package ledgertest

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
)

// Entry makes a signed msg.post entry for tests.
func Entry(t *testing.T, priv ed25519.PrivateKey, seq uint64, wall int64) *ledger.Entry {
	t.Helper()
	e := &ledger.Entry{
		Key:  ledger.Key{Origin: ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey)), Seq: seq},
		TS:   hlc.Timestamp{Wall: wall},
		Type: ledger.TypeMsgPost,
		Prio: 5,
		Body: []byte(fmt.Sprintf(`{"audience":"everyone","text":"message %d"}`, seq)),
	}
	e.Sign(priv)
	return e
}

func key(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

// Run checks a Store implementation; open must return an empty store.
func Run(t *testing.T, open func(t *testing.T) ledger.Store) {
	t.Run("put get", func(t *testing.T) {
		s := open(t)
		defer s.Close()
		e := Entry(t, key(t), 1, 100)
		added, err := s.Put(e)
		if err != nil || !added {
			t.Fatalf("Put = %v, %v", added, err)
		}
		got, err := s.Get(e.Key)
		if err != nil || got.Key != e.Key || string(got.Body) != string(e.Body) {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		if _, err := s.Get(ledger.Key{Origin: e.Key.Origin, Seq: 2}); !errors.Is(err, ledger.ErrNotFound) {
			t.Fatalf("Get missing = %v, want ErrNotFound", err)
		}
	})
	t.Run("put twice", func(t *testing.T) {
		s := open(t)
		defer s.Close()
		e := Entry(t, key(t), 1, 100)
		s.Put(e)
		added, err := s.Put(e)
		if err != nil || added {
			t.Fatalf("second Put = %v, %v; want false, nil", added, err)
		}
	})
	t.Run("gap", func(t *testing.T) {
		s := open(t)
		defer s.Close()
		priv := key(t)
		origin := ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
		for _, seq := range []uint64{1, 2, 4, 5} {
			s.Put(Entry(t, priv, seq, int64(seq)))
		}
		v, _ := s.Vector()
		if v[origin] != 2 {
			t.Fatalf("vector with a gap at 3 = %d, want 2", v[origin])
		}
		s.Put(Entry(t, priv, 3, 3))
		v, _ = s.Vector()
		if v[origin] != 5 {
			t.Fatalf("vector after filling the gap = %d, want 5", v[origin])
		}
	})
	t.Run("range", func(t *testing.T) {
		s := open(t)
		defer s.Close()
		priv := key(t)
		origin := ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
		for seq := uint64(1); seq <= 6; seq++ {
			if seq != 4 {
				s.Put(Entry(t, priv, seq, int64(seq)))
			}
		}
		got, err := s.Range(ledger.Range{Origin: origin, From: 2, To: 5}, 100)
		if err != nil {
			t.Fatal(err)
		}
		var seqs []uint64
		for _, e := range got {
			seqs = append(seqs, e.Key.Seq)
		}
		if fmt.Sprint(seqs) != "[2 3 5]" {
			t.Fatalf("Range = %v, want [2 3 5]", seqs)
		}
	})
	t.Run("huge range returns quickly", func(t *testing.T) {
		// a range comes from another node's request: its size must not set the cost
		s := open(t)
		defer s.Close()
		priv := key(t)
		origin := ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
		s.Put(Entry(t, priv, 1, 1))
		s.Put(Entry(t, priv, 1<<62, 2)) // an author may pick a huge seq
		done := make(chan int, 1)
		go func() {
			es, _ := s.Range(ledger.Range{Origin: origin, From: 1, To: 1 << 62}, 10)
			done <- len(es)
		}()
		select {
		case n := <-done:
			if n != 2 {
				t.Fatalf("Range = %d entries, want 2", n)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Range over a huge span did not return within 2 s")
		}
	})
	t.Run("range limit", func(t *testing.T) {
		s := open(t)
		defer s.Close()
		priv := key(t)
		origin := ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
		for seq := uint64(1); seq <= 5; seq++ {
			s.Put(Entry(t, priv, seq, int64(seq)))
		}
		got, _ := s.Range(ledger.Range{Origin: origin, From: 1, To: 5}, 2)
		if len(got) != 2 || got[0].Key.Seq != 1 || got[1].Key.Seq != 2 {
			t.Fatalf("Range with limit 2 = %d entries", len(got))
		}
	})
	t.Run("top and latest", func(t *testing.T) {
		s := open(t)
		defer s.Close()
		priv := key(t)
		origin := ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey))
		if top, err := s.Top(origin); err != nil || top != 0 {
			t.Fatalf("empty Top = %d, %v", top, err)
		}
		s.Put(Entry(t, priv, 1, 500))
		s.Put(Entry(t, priv, 4, 300)) // gap at 2-3, older stamp
		if top, _ := s.Top(origin); top != 4 {
			t.Fatalf("Top = %d, want 4", top)
		}
		if ts, err := s.Latest(); err != nil || ts.Wall != 500 {
			t.Fatalf("Latest = %v, %v; want wall 500", ts, err)
		}
	})
	t.Run("all ordered by clock", func(t *testing.T) {
		s := open(t)
		defer s.Close()
		a, b := key(t), key(t)
		s.Put(Entry(t, a, 1, 300))
		s.Put(Entry(t, b, 1, 100))
		s.Put(Entry(t, a, 2, 200)) // a wrong clock: still stored, ordered by its stamp
		all, err := s.All()
		if err != nil || len(all) != 3 {
			t.Fatalf("All = %d entries, %v", len(all), err)
		}
		for i := 1; i < len(all); i++ {
			if all[i].TS.Less(all[i-1].TS) {
				t.Fatalf("All not ordered at %d: %v before %v", i, all[i-1].TS, all[i].TS)
			}
		}
	})
}
