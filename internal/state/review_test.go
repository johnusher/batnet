package state

import (
	"math/rand"
	"strings"
	"sync"
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
)

// Review C1: reading an account while its groups change must not race.
func TestAccountCopiesDontRace(t *testing.T) {
	all, miller, _, anna, _, _ := village(t)
	s := apply(*all)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			verb := "join"
			if i%2 == 1 {
				verb = "leave"
			}
			s.Apply(miller.write(t, ledger.TypeAccountUpdate, map[string]any{"account": anna, verb: []string{"medical"}}))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			a := s.Account(anna)
			_ = a.Groups["medical"]
			_ = s.Directory()
		}
	}()
	wg.Wait()
}

// Review I4: the latest verdict and the latest join/leave win, whatever the arrival order.
func TestLatestWins(t *testing.T) {
	all, miller, _, _, ben, _ := village(t)
	miller.write(t, ledger.TypeAccountDeny, map[string]string{"account": ben}) // after the approval
	miller.write(t, ledger.TypeAccountUpdate, map[string]any{"account": ben, "join": []string{"medical"}})
	miller.write(t, ledger.TypeAccountUpdate, map[string]any{"account": ben, "leave": []string{"medical"}})
	rnd := rand.New(rand.NewSource(2))
	for i := 0; i < 30; i++ {
		es := append([]*ledger.Entry(nil), *all...)
		rnd.Shuffle(len(es), func(a, b int) { es[a], es[b] = es[b], es[a] })
		s := apply(es)
		a := s.Account(ben)
		if a.Status != "denied" || a.Groups["medical"] {
			t.Fatalf("order %d: status %s, medical %v; want denied and not a member", i, a.Status, a.Groups["medical"])
		}
	}
}

// Review I5: an alert counts only if written by the admin's own house node.
func TestAlertMustComeFromTheAdminsHouse(t *testing.T) {
	all, _, fern, anna, ben, _ := village(t)
	fern.write(t, ledger.TypeAlert, map[string]string{"from": anna, "text": "forged in Anna's name"})
	s := apply(*all)
	for _, m := range s.Messages(ben, "everyone", 50) {
		if m.Alert {
			t.Fatal("an alert written by another house in Anna's name counts")
		}
	}
}

// A message sent twice with the same clientId (a retry) counts once.
func TestClientIDOnce(t *testing.T) {
	all, miller, _, anna, ben, _ := village(t)
	miller.write(t, ledger.TypeMsgPost, map[string]any{"from": anna, "audience": "everyone", "text": "once", "clientId": "abc12345"})
	miller.write(t, ledger.TypeMsgPost, map[string]any{"from": anna, "audience": "everyone", "text": "once", "clientId": "abc12345"})
	s := apply(*all)
	if got := s.Messages(ben, "everyone", 10); len(got) != 1 || got[0].ClientID != "abc12345" {
		t.Fatalf("got %d messages", len(got))
	}
	if s.FindClient(anna, "abc12345") == nil {
		t.Fatal("FindClient missed it")
	}
}

func TestSettingsLatestWins(t *testing.T) {
	all, miller, _, anna, _, _ := village(t)
	miller.write(t, ledger.TypeAccountUpdate, map[string]any{"account": anna, "speaker": true})
	miller.write(t, ledger.TypeAccountUpdate, map[string]any{"account": anna, "speaker": false, "quietHours": false})
	es := append([]*ledger.Entry(nil), *all...)
	es[len(es)-1], es[len(es)-2] = es[len(es)-2], es[len(es)-1] // reversed arrival
	a := apply(es).Account(anna)
	if a.Speaker || a.QuietHours {
		t.Fatalf("speaker %v quiet %v; want both off", a.Speaker, a.QuietHours)
	}
	if b := apply(*all).Account(anna); b.Speaker {
		t.Fatal("in order: speaker should be off")
	}
}

// A post can name its author as "~/<seq>": an account created by the node that wrote the post (saves 64 bytes on LoRa).
func TestCompactFrom(t *testing.T) {
	all, miller, _, anna, ben, _ := village(t)
	seq := anna[strings.Index(anna, "/")+1:]
	miller.write(t, ledger.TypeMsgPost, map[string]any{"from": "~/" + seq, "audience": "everyone", "text": "short form"})
	s := apply(*all)
	got := s.Messages(ben, "everyone", 10)
	if len(got) != 1 || got[0].From != anna {
		t.Fatalf("compact author not resolved: %+v", got)
	}
	if c := CompactAccount(anna, miller.id); c != "~/"+seq {
		t.Fatalf("CompactAccount = %q", c)
	}
}
