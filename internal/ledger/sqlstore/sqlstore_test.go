package sqlstore

import (
	"crypto/ed25519"
	"path/filepath"
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/ledger/ledgertest"
)

func TestSQLStore(t *testing.T) {
	ledgertest.Run(t, func(t *testing.T) ledger.Store {
		s, err := Open(filepath.Join(t.TempDir(), "batnet.db"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "batnet.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_, priv, _ := ed25519.GenerateKey(nil)
	for seq := uint64(1); seq <= 3; seq++ {
		if _, err := s.Put(ledgertest.Entry(t, priv, seq, int64(seq))); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	v, _ := s2.Vector()
	if got := v[ledger.NodeIDFromPublic(priv.Public().(ed25519.PublicKey))]; got != 3 {
		t.Fatalf("vector after reopen = %d, want 3", got)
	}
}
