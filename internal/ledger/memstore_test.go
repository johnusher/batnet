package ledger_test

import (
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/ledger/ledgertest"
)

func TestMemStore(t *testing.T) {
	ledgertest.Run(t, func(t *testing.T) ledger.Store { return ledger.NewMemStore() })
}
