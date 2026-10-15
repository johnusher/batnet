package state

import (
	"crypto/ed25519"
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
)

type node struct {
	id     ledger.NodeID
	author *ledger.Author
	all    *[]*ledger.Entry
}

var wall int64 = 1_790_000_000_000

func newNode(t *testing.T, all *[]*ledger.Entry) *node {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(nil)
	a, _ := ledger.NewAuthor(priv, hlc.New(func() int64 { wall++; return wall }), ledger.NewMemStore())
	return &node{id: a.ID(), author: a, all: all}
}

func (n *node) write(t *testing.T, typ ledger.Type, body any) *ledger.Entry {
	t.Helper()
	e, err := n.author.Append(typ, 5, body)
	if err != nil {
		t.Fatal(err)
	}
	*n.all = append(*n.all, e)
	return e
}

func apply(es []*ledger.Entry) *State {
	s := New()
	for _, e := range es {
		s.Apply(e)
	}
	return s
}

// a village of two houses: Miller (admin Anna, member Ben) and Fern (admin Cleo)
func village(t *testing.T) (all *[]*ledger.Entry, miller, fern *node, anna, ben, cleo string) {
	t.Helper()
	all = &[]*ledger.Entry{}
	miller, fern = newNode(t, all), newNode(t, all)
	miller.write(t, ledger.TypeNodeAnnounce, map[string]string{"house": "Miller"})
	fern.write(t, ledger.TypeNodeAnnounce, map[string]string{"house": "Fern"})
	a := miller.write(t, ledger.TypeAccountRequest, map[string]any{"name": "Anna", "pinHash": "h1", "admin": true})
	anna = AccountID(a.Key)
	miller.write(t, ledger.TypeAccountApprove, map[string]string{"account": anna})
	b := miller.write(t, ledger.TypeAccountRequest, map[string]any{"name": "Ben", "pinHash": "h2"})
	ben = AccountID(b.Key)
	miller.write(t, ledger.TypeAccountApprove, map[string]string{"account": ben})
	c := fern.write(t, ledger.TypeAccountRequest, map[string]any{"name": "Cleo", "pinHash": "h3", "admin": true})
	cleo = AccountID(c.Key)
	fern.write(t, ledger.TypeAccountApprove, map[string]string{"account": cleo})
	miller.write(t, ledger.TypeGroupCreate, map[string]string{"id": "medical", "name": "Medical"})
	return
}

func TestAccountsAndDirectory(t *testing.T) {
	all, miller, _, anna, ben, cleo := village(t)
	s := apply(*all)
	if !s.Approved(anna) || !s.Approved(ben) || !s.Approved(cleo) {
		t.Fatal("approved accounts not approved")
	}
	if !s.Account(anna).Admin || s.Account(ben).Admin {
		t.Fatal("admin flags wrong")
	}
	if s.HouseName(miller.id) != "Miller" {
		t.Fatalf("house name %q", s.HouseName(miller.id))
	}
	dir := s.Directory()
	if len(dir) != 2 || len(dir[0].People)+len(dir[1].People) != 3 {
		t.Fatalf("directory %+v", dir)
	}
	if got := s.AccountsNamed("ANNA"); len(got) != 1 || got[0].ID != anna {
		t.Fatal("name lookup should be case-insensitive and approved-only")
	}
}

func TestMessagesAndVisibility(t *testing.T) {
	all, miller, fern, anna, ben, cleo := village(t)
	miller.write(t, ledger.TypeMsgPost, map[string]any{"from": anna, "audience": "everyone", "text": "Water point open"})
	miller.write(t, ledger.TypeMsgPost, map[string]any{"from": ben, "audience": "dm:" + anna, "text": "for Anna only"})
	miller.write(t, ledger.TypeAccountUpdate, map[string]any{"account": cleo, "join": []string{"medical"}})
	fern.write(t, ledger.TypeMsgPost, map[string]any{"from": cleo, "audience": "group:medical", "text": "Nurse at the school",
		"loc": map[string]any{"lat": 52.1, "lon": 13.2, "acc": 12, "source": "phone"}})
	s := apply(*all)
	if got := s.Messages(cleo, "everyone", 10); len(got) != 1 || got[0].Text != "Water point open" {
		t.Fatalf("everyone: %+v", got)
	}
	if len(s.Messages(cleo, "dm:"+ben, 10)) != 0 || len(s.Messages(anna, "dm:"+ben, 10)) != 1 || len(s.Messages(ben, "dm:"+anna, 10)) != 1 {
		t.Fatal("direct messages visible to the wrong people")
	}
	if len(s.Messages(ben, "group:medical", 10)) != 0 {
		t.Fatal("a group message visible to a non-member")
	}
	g := s.Messages(cleo, "group:medical", 10)
	if len(g) != 1 || g[0].Loc == nil || g[0].Loc.Lat != 52.1 {
		t.Fatalf("group message or its location missing: %+v", g)
	}
	convs := s.Conversations(anna)
	if len(convs) < 2 {
		t.Fatalf("Anna's conversations: %+v", convs)
	}
}

// Review focus 1.
func TestAuthority(t *testing.T) {
	all, miller, fern, _, ben, _ := village(t)
	r := miller.write(t, ledger.TypeAccountRequest, map[string]any{"name": "Dan", "pinHash": "h4"})
	dan := AccountID(r.Key)
	fern.write(t, ledger.TypeAccountApprove, map[string]string{"account": dan})           // not Dan's house
	fern.write(t, ledger.TypeAlert, map[string]string{"from": ben, "text": "fake alert"}) // Ben is no admin
	miller.write(t, ledger.TypeMsgPost, map[string]any{"from": dan, "audience": "everyone", "text": "unapproved"})
	s := apply(*all)
	if s.Approved(dan) {
		t.Fatal("another house approved Dan")
	}
	for _, m := range s.Messages(ben, "everyone", 50) {
		if m.Alert || m.Text == "unapproved" {
			t.Fatalf("message %q should not count", m.Text)
		}
	}
	if len(s.Pending(miller.id)) != 1 {
		t.Fatal("Dan should be pending at Miller")
	}
}

// Review focus 2.
func TestOrderIndependent(t *testing.T) {
	all, miller, _, anna, _, _ := village(t)
	miller.write(t, ledger.TypeMsgPost, map[string]any{"from": anna, "audience": "everyone", "text": "one"})
	miller.write(t, ledger.TypeAlert, map[string]string{"from": anna, "text": "Flood on the low road"})
	want, _ := json.Marshal(apply(*all).Messages(anna, "everyone", 50))
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		shuffled := append([]*ledger.Entry(nil), *all...)
		rnd.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		shuffled = append(shuffled, shuffled[:3]...) // and some twice
		got, _ := json.Marshal(apply(shuffled).Messages(anna, "everyone", 50))
		if string(got) != string(want) {
			t.Fatalf("order changed the result:\n%s\n%s", got, want)
		}
	}
}
