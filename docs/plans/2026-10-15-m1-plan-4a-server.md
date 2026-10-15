# Milestone 1, plan 4a: the house server - state, accounts, the app's API, the captive portal

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Each house Pi serves the batnet app over HTTP: phones join through the captive portal, sign up with a name and 6-digit PIN, wait for the house admin's approval, then read and post messages (with priority and optional location) that travel through the real log - runnable on a laptop as `go run ./cmd/batnetd -mock 3`, one app per house on localhost.

**Architecture:** `internal/state` folds log entries into accounts, houses, groups and messages, tolerant of any arrival order; authority comes from the log itself (a house's own node approves its accounts; only admins' alerts count). `internal/auth` hashes PINs with argon2id and keeps sessions and lockouts in memory. `internal/web` is the HTTP server: captive-portal probes, a welcome page, a JSON API, a server-sent-events stream, and the embedded app (plain HTML/JS, no build step). `cmd/batnetd -mock` wires a server to every mock house.

**Tech Stack:** Go 1.26 standard library (`net/http`, `embed`, `encoding/json`), `golang.org/x/crypto/argon2`; plain HTML, CSS and JavaScript for the app.

**Spec:** [`docs/specs/2026-10-15-m1-design.md`](../specs/2026-10-15-m1-design.md) §4 (entry types), §7 (priority), §8 (web app), §9 (accounts at set-up), §12 (security); decisions 0003, 0004, 0015, 0019.

**Milestone 1 plans:** 1, 2, 3 done · **4a house server (this)** · 4b the full app screens and the real app in the simulator · 3b real radios · 5 Pi set-up · 6 image and installer.

## Global Constraints

- PIN: exactly 6 digits; stored only as argon2id (19 MiB, t=2, p=1, 16-byte salt); 5 wrong tries lock that account at this house for 1 min, doubling each time; success resets.
- Sessions: random 32-byte tokens in an `HttpOnly; Secure; SameSite=Strict` cookie `batnet`, valid 1 year (`Secure` off only for `http://localhost` in mock mode).
- Authority from the log: an `account.approve`/`account.deny` counts only if written by the account's home node; an `alert` counts only if its author account is an approved admin; a post counts only if its author is approved.
- Priority choices `low|normal|high|alert`, levels from `prio.ForPost`; High limited to 3 per account per hour (decision 0015); alerts only by admins.
- Location optional: `lat` −90..90, `lon` −180..180, `acc` 0..10000 m, `source` `phone|node`; off unless the request carries it (decision 0019).
- Message text 1–2000 characters; names 1–40 characters.
- The app is plain HTML/CSS/JS embedded with `go:embed`; no npm, no build step; under 200 KB.
- No real names, places or keys in code or tests; mock PINs are placeholders.

## Review Focus

1. **Approvals and alerts forged by another node** (any village node can write entries): state must ignore an approval not written by the account's home node and an alert whose author isn't an approved admin. → Task 1 `TestAuthority`.
2. **Entries out of order** (an approval arriving over LoRa before its request; a post before its author's approval): the end state must be the same as in order. → Task 1 `TestOrderIndependent`.
3. **PIN guessing**: lockout per account per house, doubling; the same error for a wrong name and a wrong PIN. → Task 2 `TestLockout`, Task 4 `TestLoginErrorsAlike`.
4. **A pending or signed-out phone** must not read or post messages, approve, or receive events. → Task 4 and 5 tests.
5. **Malformed input** (huge bodies, bad JSON, out-of-range location, empty text, unknown conversation): a 4xx with a short reason, never a panic or a stored entry. → Task 5 `TestPostValidation`.

---

## File structure

```
internal/state/state.go, state_test.go     accounts, houses, groups, messages folded from the log
internal/auth/auth.go, auth_test.go        PIN hashing, lockout, sessions
internal/web/server.go                     Server, Config, wiring to the log, SSE hub
internal/web/captive.go                    connectivity-check probes and the welcome page
internal/web/api.go                        JSON API handlers
internal/web/web_test.go                   httptest tests for all of it
internal/web/app/index.html, app.js, app.css   the first real app (embedded)
cmd/batnetd/main.go                        (modify) -mock serves the app per house
```

---

### Task 1: State folded from the log

**Files:** Create `internal/state/state.go`, `internal/state/state_test.go`

**Interfaces — Produces:**
```go
type Location struct{ Lat, Lon, Acc float64; Source string }  // json: lat, lon, acc, source
type Account struct{ ID, Name, PinHash string; Home ledger.NodeID; Admin bool; Status string; Groups map[string]bool; Since hlc.Timestamp }
type Group struct{ ID, Name string }
type Message struct{ ID string; Key ledger.Key; From, Audience, Text string; Prio uint8; Alert bool; Loc *Location; TS hlc.Timestamp }
type Conversation struct{ Key, Name string; Last *Message; Count int }
type State struct{ /* … */ }
func New() *State
func (s *State) Apply(e *ledger.Entry)                       // idempotent, any order
func AccountID(k ledger.Key) string                         // "<origin hex>/<seq>"
func (s *State) Account(id string) *Account
func (s *State) Approved(id string) bool
func (s *State) AccountsNamed(name string) []*Account        // approved, case-insensitive
func (s *State) Pending(home ledger.NodeID) []*Account
func (s *State) HouseName(node ledger.NodeID) string
func (s *State) Directory() []HouseDir                       // HouseDir{Node, Name string; People []*Account}
func (s *State) Groups() []*Group
func (s *State) Visible(m *Message, acct string) bool
func (s *State) Messages(acct, conv string, limit int) []*Message
func (s *State) Conversations(acct string) []Conversation
```
Entry bodies (JSON): `node.announce {house}`; `account.request {name, pinHash, admin}`; `account.approve|deny {account}`; `account.update {account, join, leave}`; `group.create {id, name}`; `msg.post {from, audience, text, loc}`; `alert {from, text}`. Audiences: `everyone`, `group:<id>`, `dm:<account>`. A message is counted when its author is approved (and, for alerts, an admin).

- [ ] **Step 1: Write the failing tests** — `internal/state/state_test.go`

```go
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
func village(t *testing.T) (all []*ledger.Entry, miller, fern *node, anna, ben, cleo string) {
	t.Helper()
	miller, fern = newNode(t, &all), newNode(t, &all)
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
	s := apply(all)
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
	s := apply(all)
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
	fern.write(t, ledger.TypeAccountApprove, map[string]string{"account": dan}) // not Dan's house
	fern.write(t, ledger.TypeAlert, map[string]string{"from": ben, "text": "fake alert"}) // Ben is no admin
	miller.write(t, ledger.TypeMsgPost, map[string]any{"from": dan, "audience": "everyone", "text": "unapproved"})
	s := apply(all)
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
	want, _ := json.Marshal(apply(all).Messages(anna, "everyone", 50))
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		shuffled := append([]*ledger.Entry(nil), all...)
		rnd.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		shuffled = append(shuffled, shuffled[:3]...) // and some twice
		got, _ := json.Marshal(apply(shuffled).Messages(anna, "everyone", 50))
		if string(got) != string(want) {
			t.Fatalf("order changed the result:\n%s\n%s", got, want)
		}
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/state/` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/state/state.go`

```go
// Package state folds the log into what the app shows: houses, accounts,
// groups and messages. It accepts entries in any order and any number of
// times, and takes authority from the log itself (spec §4, §12).
package state

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
)

// Location is where a message was sent from (decision 0019).
type Location struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Acc    float64 `json:"acc"`
	Source string  `json:"source"`
}

// Account is a person.
type Account struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	PinHash string          `json:"-"`
	Home    ledger.NodeID   `json:"-"`
	Admin   bool            `json:"admin"`
	Status  string          `json:"status"` // pending, approved, denied
	Groups  map[string]bool `json:"-"`
	Since   hlc.Timestamp   `json:"-"`
}

// Group is a named conversation people join.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Message is a post or an alert.
type Message struct {
	ID       string        `json:"id"`
	Key      ledger.Key    `json:"-"`
	From     string        `json:"from"`
	Audience string        `json:"audience"`
	Text     string        `json:"text"`
	Prio     uint8         `json:"prio"`
	Alert    bool          `json:"alert"`
	Loc      *Location     `json:"loc,omitempty"`
	TS       hlc.Timestamp `json:"ts"`
}

// Conversation is one chat as a person sees it.
type Conversation struct {
	Key   string   `json:"key"`
	Name  string   `json:"name"`
	Last  *Message `json:"last"`
	Count int      `json:"count"`
}

// HouseDir is one house and its approved people.
type HouseDir struct {
	Node   string     `json:"node"`
	Name   string     `json:"name"`
	People []*Account `json:"people"`
}

// State is safe for concurrent use.
type State struct {
	mu       sync.RWMutex
	seen     map[ledger.Key]bool
	accounts map[string]*Account
	verdicts map[string][]verdict // decisions that arrived before their request
	houses   map[ledger.NodeID]string
	groups   map[string]*Group
	updates  map[string][]membership // group changes that arrived before their account
	messages []*Message
}

type verdict struct {
	by      ledger.NodeID
	approve bool
	ts      hlc.Timestamp
}

type membership struct {
	join, leave []string
	ts          hlc.Timestamp
}

// New returns an empty state.
func New() *State {
	return &State{seen: map[ledger.Key]bool{}, accounts: map[string]*Account{}, verdicts: map[string][]verdict{},
		houses: map[ledger.NodeID]string{}, groups: map[string]*Group{}, updates: map[string][]membership{}}
}

// AccountID names an account by its request entry.
func AccountID(k ledger.Key) string { return fmt.Sprintf("%s/%d", hex.EncodeToString(k.Origin[:]), k.Seq) }

// Apply folds one entry in. Unknown types and malformed bodies are ignored.
func (s *State) Apply(e *ledger.Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[e.Key] {
		return
	}
	s.seen[e.Key] = true
	switch e.Type {
	case ledger.TypeNodeAnnounce:
		var b struct{ House string }
		if json.Unmarshal(e.Body, &b) == nil && b.House != "" {
			s.houses[e.Key.Origin] = b.House
		}
	case ledger.TypeAccountRequest:
		var b struct {
			Name    string `json:"name"`
			PinHash string `json:"pinHash"`
			Admin   bool   `json:"admin"`
		}
		if json.Unmarshal(e.Body, &b) != nil || b.Name == "" {
			return
		}
		id := AccountID(e.Key)
		a := &Account{ID: id, Name: b.Name, PinHash: b.PinHash, Home: e.Key.Origin, Admin: b.Admin, Status: "pending", Groups: map[string]bool{}, Since: e.TS}
		s.accounts[id] = a
		for _, v := range s.verdicts[id] {
			s.decide(a, v)
		}
		delete(s.verdicts, id)
		for _, u := range s.updates[id] {
			s.member(a, u)
		}
		delete(s.updates, id)
	case ledger.TypeAccountApprove, ledger.TypeAccountDeny:
		var b struct{ Account string }
		if json.Unmarshal(e.Body, &b) != nil {
			return
		}
		v := verdict{by: e.Key.Origin, approve: e.Type == ledger.TypeAccountApprove, ts: e.TS}
		if a := s.accounts[b.Account]; a != nil {
			s.decide(a, v)
		} else {
			s.verdicts[b.Account] = append(s.verdicts[b.Account], v)
		}
	case ledger.TypeAccountUpdate:
		var b struct {
			Account     string
			Join, Leave []string
		}
		if json.Unmarshal(e.Body, &b) != nil {
			return
		}
		u := membership{join: b.Join, leave: b.Leave, ts: e.TS}
		if a := s.accounts[b.Account]; a != nil {
			s.member(a, u)
		} else {
			s.updates[b.Account] = append(s.updates[b.Account], u)
		}
	case ledger.TypeGroupCreate:
		var b struct{ ID, Name string }
		if json.Unmarshal(e.Body, &b) == nil && b.ID != "" && s.groups[b.ID] == nil {
			s.groups[b.ID] = &Group{ID: b.ID, Name: b.Name}
		}
	case ledger.TypeMsgPost, ledger.TypeAlert:
		var b struct {
			From     string    `json:"from"`
			Audience string    `json:"audience"`
			Text     string    `json:"text"`
			Loc      *Location `json:"loc"`
		}
		if json.Unmarshal(e.Body, &b) != nil || b.From == "" || b.Text == "" {
			return
		}
		m := &Message{ID: AccountID(e.Key), Key: e.Key, From: b.From, Audience: b.Audience, Text: b.Text, Prio: e.Prio, Loc: b.Loc, TS: e.TS}
		if e.Type == ledger.TypeAlert {
			m.Alert, m.Audience = true, "everyone"
		}
		if m.Audience == "" {
			m.Audience = "everyone"
		}
		i := sort.Search(len(s.messages), func(i int) bool { return less(m, s.messages[i]) })
		s.messages = append(s.messages, nil)
		copy(s.messages[i+1:], s.messages[i:])
		s.messages[i] = m
	}
}

func less(a, b *Message) bool {
	if a.TS != b.TS {
		return a.TS.Less(b.TS)
	}
	return a.ID < b.ID
}

// decide applies a verdict if the account's own house wrote it. The latest valid verdict wins.
func (s *State) decide(a *Account, v verdict) {
	if v.by != a.Home {
		return
	}
	if v.approve {
		a.Status = "approved"
	} else {
		a.Status = "denied"
	}
}

func (s *State) member(a *Account, u membership) {
	for _, g := range u.join {
		a.Groups[g] = true
	}
	for _, g := range u.leave {
		delete(a.Groups, g)
	}
}

// Account returns an account or nil.
func (s *State) Account(id string) *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accounts[id]
}

// Approved reports whether the account exists and is approved.
func (s *State) Approved(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a := s.accounts[id]
	return a != nil && a.Status == "approved"
}

// AccountsNamed lists approved accounts with this name, any case.
func (s *State) AccountsNamed(name string) []*Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Account
	for _, a := range s.accounts {
		if a.Status == "approved" && strings.EqualFold(a.Name, strings.TrimSpace(name)) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Pending lists accounts waiting for this house's approval, oldest first.
func (s *State) Pending(home ledger.NodeID) []*Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Account
	for _, a := range s.accounts {
		if a.Home == home && a.Status == "pending" {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Since.Less(out[j].Since) })
	return out
}

// HouseName is a node's house name ("" if not announced).
func (s *State) HouseName(node ledger.NodeID) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.houses[node]
}

// Directory lists houses (by name) with their approved people (by name).
func (s *State) Directory() []HouseDir {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byNode := map[ledger.NodeID]*HouseDir{}
	for node, name := range s.houses {
		byNode[node] = &HouseDir{Node: hex.EncodeToString(node[:4]), Name: name}
	}
	for _, a := range s.accounts {
		if a.Status != "approved" {
			continue
		}
		h := byNode[a.Home]
		if h == nil {
			h = &HouseDir{Node: hex.EncodeToString(a.Home[:4]), Name: "?"}
			byNode[a.Home] = h
		}
		h.People = append(h.People, a)
	}
	var out []HouseDir
	for _, h := range byNode {
		sort.Slice(h.People, func(i, j int) bool { return h.People[i].Name < h.People[j].Name })
		out = append(out, *h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Groups lists groups by name.
func (s *State) Groups() []*Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Group
	for _, g := range s.groups {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// counts reports whether a message counts at all: an approved author, and an admin for alerts. Caller holds the lock.
func (s *State) counts(m *Message) bool {
	a := s.accounts[m.From]
	return a != nil && a.Status == "approved" && (!m.Alert || a.Admin)
}

// Visible reports whether acct may see m.
func (s *State) Visible(m *Message, acct string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.visible(m, acct)
}

func (s *State) visible(m *Message, acct string) bool {
	if !s.counts(m) {
		return false
	}
	switch {
	case m.Audience == "everyone":
		return true
	case strings.HasPrefix(m.Audience, "group:"):
		a := s.accounts[acct]
		return a != nil && a.Groups[strings.TrimPrefix(m.Audience, "group:")]
	case strings.HasPrefix(m.Audience, "dm:"):
		return acct == m.From || acct == strings.TrimPrefix(m.Audience, "dm:")
	}
	return false
}

// convKey is the conversation a visible message belongs to, for acct.
func convKey(m *Message, acct string) string {
	if strings.HasPrefix(m.Audience, "dm:") {
		if m.From == acct {
			return m.Audience
		}
		return "dm:" + m.From
	}
	return m.Audience
}

// Messages returns the last limit messages of one conversation that acct may see, oldest first.
func (s *State) Messages(acct, conv string, limit int) []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Message
	for _, m := range s.messages {
		if s.visible(m, acct) && convKey(m, acct) == conv {
			out = append(out, m)
		}
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// Conversations lists acct's conversations, newest activity first.
func (s *State) Conversations(acct string) []Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	by := map[string]*Conversation{}
	for _, m := range s.messages {
		if !s.visible(m, acct) {
			continue
		}
		k := convKey(m, acct)
		c := by[k]
		if c == nil {
			c = &Conversation{Key: k, Name: s.convName(k)}
			by[k] = c
		}
		c.Last = m
		c.Count++
	}
	var out []Conversation
	for _, c := range by {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return less(out[j].Last, out[i].Last) })
	return out
}

func (s *State) convName(k string) string {
	switch {
	case k == "everyone":
		return "Everyone"
	case strings.HasPrefix(k, "group:"):
		if g := s.groups[strings.TrimPrefix(k, "group:")]; g != nil {
			return g.Name
		}
	case strings.HasPrefix(k, "dm:"):
		if a := s.accounts[strings.TrimPrefix(k, "dm:")]; a != nil {
			return a.Name
		}
	}
	return k
}
```

- [ ] **Step 4: Run to see it pass, with the race detector.** Run: `go test -race ./internal/state/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/state
git commit -m "state: accounts, houses, groups and messages folded from the log, in any order, with authority from the log"
```

---

### Task 2: PINs, lockout, sessions

**Files:** Create `internal/auth/auth.go`, `internal/auth/auth_test.go`

**Interfaces — Produces:**
```go
func ValidPIN(pin string) bool
func HashPIN(pin string) (string, error)    // "$argon2id$v=19$m=19456,t=2,p=1$<salt b64>$<hash b64>"
func CheckPIN(encoded, pin string) bool
type Lockout struct{ /* … */ }
func NewLockout() *Lockout
func (l *Lockout) Wait(account string, now time.Time) time.Duration  // 0 if allowed
func (l *Lockout) Fail(account string, now time.Time)
func (l *Lockout) Success(account string)
type Sessions struct{ /* … */ }
func NewSessions() *Sessions
func (s *Sessions) New(account string, now time.Time) string         // token
func (s *Sessions) Get(token string, now time.Time) (string, bool)
func (s *Sessions) End(token string)
```

- [ ] **Step 1: Write the failing tests** — `internal/auth/auth_test.go`

```go
package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPIN(t *testing.T) {
	for _, p := range []string{"123456", "000000"} {
		if !ValidPIN(p) {
			t.Errorf("%q should be valid", p)
		}
	}
	for _, p := range []string{"", "12345", "1234567", "12a456", " 123456", "１２３４５６"} {
		if ValidPIN(p) {
			t.Errorf("%q should be invalid", p)
		}
	}
	h, err := HashPIN("482915")
	if err != nil || !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash %q, %v", h, err)
	}
	h2, _ := HashPIN("482915")
	if h == h2 {
		t.Fatal("two hashes of one PIN are equal: no salt")
	}
	if !CheckPIN(h, "482915") || CheckPIN(h, "482916") || CheckPIN("garbage", "482915") {
		t.Fatal("CheckPIN wrong")
	}
}

// Review focus 3.
func TestLockout(t *testing.T) {
	l := NewLockout()
	now := time.Unix(1_790_000_000, 0)
	for i := 0; i < 5; i++ {
		if l.Wait("anna", now) != 0 {
			t.Fatalf("locked after %d failures", i)
		}
		l.Fail("anna", now)
	}
	if w := l.Wait("anna", now); w != time.Minute {
		t.Fatalf("after 5 failures wait %v, want 1m", w)
	}
	if l.Wait("ben", now) != 0 {
		t.Fatal("another account locked")
	}
	now = now.Add(time.Minute)
	l.Fail("anna", now) // a sixth failure doubles
	if w := l.Wait("anna", now); w != 2*time.Minute {
		t.Fatalf("wait %v, want 2m", w)
	}
	l.Success("anna")
	if l.Wait("anna", now) != 0 {
		t.Fatal("success did not reset the lockout")
	}
}

func TestSessions(t *testing.T) {
	s := NewSessions()
	now := time.Unix(1_790_000_000, 0)
	tok := s.New("anna", now)
	if len(tok) < 40 {
		t.Fatalf("token %q too short", tok)
	}
	if a, ok := s.Get(tok, now.Add(300*24*time.Hour)); !ok || a != "anna" {
		t.Fatal("session lost within a year")
	}
	if _, ok := s.Get(tok, now.Add(366*24*time.Hour)); ok {
		t.Fatal("session alive after a year")
	}
	tok2 := s.New("ben", now)
	s.End(tok2)
	if _, ok := s.Get(tok2, now); ok {
		t.Fatal("ended session still valid")
	}
	if _, ok := s.Get("nonsense", now); ok {
		t.Fatal("unknown token valid")
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/auth/` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/auth/auth.go`

```go
// Package auth handles PINs (argon2id), lockouts after wrong PINs, and
// sessions (spec §8).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemKiB = 19456 // 19 MiB
	argonTime   = 2
	argonLanes  = 1
	sessionLife = 365 * 24 * time.Hour
	freeTries   = 5
)

var b64 = base64.RawStdEncoding

// ValidPIN: exactly six ASCII digits.
func ValidPIN(pin string) bool {
	if len(pin) != 6 {
		return false
	}
	for _, c := range []byte(pin) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// HashPIN returns an encoded argon2id hash with a random salt.
func HashPIN(pin string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(pin), salt, argonTime, argonMemKiB, argonLanes, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemKiB, argonTime, argonLanes, b64.EncodeToString(salt), b64.EncodeToString(h)), nil
}

// CheckPIN compares a PIN with an encoded hash in constant time.
func CheckPIN(encoded, pin string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || m > 1<<20 || t > 10 || p == 0 {
		return false
	}
	salt, err1 := b64.DecodeString(parts[4])
	want, err2 := b64.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// Lockout slows PIN guessing: 5 free tries per account, then 1 min, doubling.
type Lockout struct {
	mu    sync.Mutex
	fails map[string]int
	until map[string]time.Time
}

// NewLockout returns an empty lockout table.
func NewLockout() *Lockout { return &Lockout{fails: map[string]int{}, until: map[string]time.Time{}} }

// Wait is how long the account must wait before the next try (0: try now).
func (l *Lockout) Wait(account string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if u := l.until[account]; now.Before(u) {
		return u.Sub(now)
	}
	return 0
}

// Fail records a wrong PIN.
func (l *Lockout) Fail(account string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[account]++
	if n := l.fails[account]; n >= freeTries {
		l.until[account] = now.Add(time.Minute << (n - freeTries))
	}
}

// Success clears the account's failures.
func (l *Lockout) Success(account string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, account)
	delete(l.until, account)
}

// Sessions maps random tokens to accounts for a year.
type Sessions struct {
	mu sync.Mutex
	m  map[string]session
}

type session struct {
	account string
	expires time.Time
}

// NewSessions returns an empty session table.
func NewSessions() *Sessions { return &Sessions{m: map[string]session{}} }

// New starts a session and returns its token.
func (s *Sessions) New(account string, now time.Time) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[tok] = session{account: account, expires: now.Add(sessionLife)}
	return tok
}

// Get returns the session's account.
func (s *Sessions) Get(token string, now time.Time) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, ok := s.m[token]
	if !ok || now.After(x.expires) {
		return "", false
	}
	return x.account, true
}

// End signs a session out.
func (s *Sessions) End(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, token)
}
```

- [ ] **Step 4: Run to see it pass.** Run: `go test -race ./internal/auth/` — Expected: `ok` (the argon2 hashes take a moment).

- [ ] **Step 5: Commit**

```bash
git add internal/auth
git commit -m "auth: argon2id PINs, lockout per account doubling after five tries, year-long sessions"
```

---

### Task 3: The server, the captive portal and the event stream

**Files:** Create `internal/web/server.go`, `internal/web/captive.go`, `internal/web/web_test.go` (first part), `internal/web/app/index.html` (placeholder, replaced in Task 6)

**Interfaces — Produces:**
```go
type Config struct {
	House       string
	Author      *ledger.Author
	State       *state.State
	Publish     func(*ledger.Entry)   // to the mesh (and the LoRa bridge)
	Now         func() time.Time
	SecureCookie bool                 // false only for http://localhost (mock)
	HighPerHour int                   // default 3
}
type Server struct{ /* … */ }
func New(cfg Config) *Server
func (s *Server) Handler() http.Handler
func (s *Server) Write(t ledger.Type, p prio.Level, body any) (*ledger.Entry, error) // append, apply, publish, notify
func (s *Server) OnEntry(e *ledger.Entry)                                          // from the mesh: apply, notify
func (s *Server) Bootstrap(admin, pin string, groups []string) (string, error)       // first boot: announce the house, an approved admin, groups
```
Captive probes (`/hotspot-detect.html`, `/library/test/success.html`, `/generate_204`, `/gen_204`, `/connecttest.txt`, `/ncsi.txt`, `/success.txt`, `/canonical.html`) answer `302` to `/welcome`. `/welcome` is a short HTML page. `/api/events` is a server-sent-events stream for a signed-in, approved session: `event: message` with the message JSON (only messages visible to that account) and `event: state` (no data needed) when accounts, approvals or groups change.

- [ ] **Step 1: Write the failing tests** — `internal/web/web_test.go`

```go
package web

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/state"
)

// house is one house server with its own log; peers get each other's entries through link.
type house struct {
	srv   *Server
	ts    *httptest.Server
	peers []*house
}

var clock = time.Unix(1_790_000_000, 0)

func newHouse(t *testing.T, name string) *house {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(nil)
	au, _ := ledger.NewAuthor(priv, hlc.New(func() int64 { return clock.UnixMilli() }), ledger.NewMemStore())
	h := &house{}
	h.srv = New(Config{House: name, Author: au, State: state.New(), Now: func() time.Time { return clock },
		Publish: func(e *ledger.Entry) {
			for _, p := range h.peers {
				p.srv.OnEntry(e)
			}
		}})
	h.ts = httptest.NewServer(h.srv.Handler())
	t.Cleanup(h.ts.Close)
	return h
}

func link(hs ...*house) {
	for _, a := range hs {
		for _, b := range hs {
			if a != b {
				a.peers = append(a.peers, b)
			}
		}
	}
}

// client keeps one phone's cookie.
type client struct {
	t   *testing.T
	h   *house
	jar []*http.Cookie
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.h.ts.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range c.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if cks := resp.Cookies(); len(cks) > 0 {
		c.jar = cks
	}
	out := map[string]any{}
	raw, _ := io.ReadAll(resp.Body)
	json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestCaptivePortal(t *testing.T) {
	h := newHouse(t, "Miller")
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, p := range []string{"/hotspot-detect.html", "/generate_204", "/connecttest.txt", "/ncsi.txt", "/success.txt"} {
		resp, err := noFollow.Get(h.ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/welcome" {
			t.Errorf("%s: %d %q", p, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
	resp, _ := http.Get(h.ts.URL + "/welcome")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "Miller") || !strings.Contains(string(b), "https://batnet") {
		t.Fatal("welcome page lacks the house name or the address to open")
	}
}

func TestBootstrapAndEvents(t *testing.T) {
	h := newHouse(t, "Miller")
	if _, err := h.srv.Bootstrap("Anna", "482915", []string{"Medical", "Food"}); err != nil {
		t.Fatal(err)
	}
	anna := &client{t: t, h: h}
	if code, _ := anna.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "482915"}); code != 200 {
		t.Fatalf("admin login: %d", code)
	}
	req, _ := http.NewRequest("GET", h.ts.URL+"/api/events", nil)
	for _, ck := range anna.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("events: %v %v", resp, err)
	}
	defer resp.Body.Close()
	go anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "Water point open"})
	r := bufio.NewReader(resp.Body)
	deadline := time.After(3 * time.Second)
	got := make(chan string, 1)
	go func() {
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, "Water point open") {
				got <- line
				return
			}
		}
	}()
	select {
	case <-got:
	case <-deadline:
		t.Fatal("the new message did not arrive on the event stream")
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/web/` — Expected: FAIL, undefined names.

- [ ] **Step 3: Implement** — `internal/web/server.go`

```go
// Package web is the house Pi's HTTP server: the captive portal, the app, its
// JSON API and a server-sent-events stream (spec §8).
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/auth"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
	"github.com/johnusher/batnet/internal/state"
)

//go:embed app
var appFS embed.FS

// Config wires a server to its house node.
type Config struct {
	House        string
	Author       *ledger.Author
	State        *state.State
	Publish      func(*ledger.Entry)
	Now          func() time.Time
	SecureCookie bool
	HighPerHour  int
}

// Server serves one house.
type Server struct {
	cfg      Config
	sessions *auth.Sessions
	lockout  *auth.Lockout
	high     *prio.Limiter
	mu       sync.Mutex // serialises writes to the log
	hub      *hub
}

// New returns a server.
func New(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.HighPerHour == 0 {
		cfg.HighPerHour = 3
	}
	if cfg.Publish == nil {
		cfg.Publish = func(*ledger.Entry) {}
	}
	return &Server{cfg: cfg, sessions: auth.NewSessions(), lockout: auth.NewLockout(), high: prio.NewLimiter(cfg.HighPerHour), hub: newHub()}
}

// Write appends an entry from this house, applies it, publishes it, and tells listeners.
func (s *Server) Write(t ledger.Type, p prio.Level, body any) (*ledger.Entry, error) {
	s.mu.Lock()
	e, err := s.cfg.Author.Append(t, uint8(p), body)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	s.cfg.State.Apply(e)
	s.cfg.Publish(e)
	s.notify(e)
	return e, nil
}

// OnEntry takes an entry that arrived from the mesh or the radio.
func (s *Server) OnEntry(e *ledger.Entry) {
	s.cfg.State.Apply(e)
	s.notify(e)
}

func (s *Server) notify(e *ledger.Entry) {
	switch e.Type {
	case ledger.TypeMsgPost, ledger.TypeAlert:
		s.hub.message(s.cfg.State, state.AccountID(e.Key))
	default:
		s.hub.state()
	}
}

// Bootstrap is first boot: announce the house, create its approved admin and groups.
func (s *Server) Bootstrap(admin, pin string, groups []string) (string, error) {
	if !auth.ValidPIN(pin) {
		return "", fmt.Errorf("web: the admin PIN must be 6 digits")
	}
	if _, err := s.Write(ledger.TypeNodeAnnounce, prio.Housekeeping, map[string]string{"house": s.cfg.House}); err != nil {
		return "", err
	}
	h, err := auth.HashPIN(pin)
	if err != nil {
		return "", err
	}
	req, err := s.Write(ledger.TypeAccountRequest, prio.Normal, map[string]any{"name": admin, "pinHash": h, "admin": true})
	if err != nil {
		return "", err
	}
	id := state.AccountID(req.Key)
	if _, err := s.Write(ledger.TypeAccountApprove, prio.Normal, map[string]string{"account": id}); err != nil {
		return "", err
	}
	for _, g := range groups {
		slug := strings.ToLower(strings.ReplaceAll(g, " ", "-"))
		if _, err := s.Write(ledger.TypeGroupCreate, prio.Housekeeping, map[string]string{"id": slug, "name": g}); err != nil {
			return "", err
		}
	}
	return id, nil
}

// Handler routes every request.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.captive(mux)
	s.api(mux)
	sub, _ := fs.Sub(appFS, "app")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))
	return securityHeaders(mux)
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'")
		h.ServeHTTP(w, r)
	})
}

// ---- server-sent events ----

type listener struct {
	account string
	ch      chan []byte
}

type hub struct {
	mu        sync.Mutex
	listeners map[*listener]bool
}

func newHub() *hub { return &hub{listeners: map[*listener]bool{}} }

func (h *hub) add(account string) *listener {
	l := &listener{account: account, ch: make(chan []byte, 64)}
	h.mu.Lock()
	h.listeners[l] = true
	h.mu.Unlock()
	return l
}

func (h *hub) remove(l *listener) {
	h.mu.Lock()
	delete(h.listeners, l)
	h.mu.Unlock()
}

func (h *hub) send(l *listener, b []byte) {
	select {
	case l.ch <- b:
	default: // a slow phone misses events; it reloads conversations on reconnect
	}
}

func (h *hub) message(st *state.State, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := st.MessageByID(id)
	if m == nil {
		return
	}
	for l := range h.listeners {
		if st.Visible(m, l.account) {
			b, _ := json.Marshal(m)
			h.send(l, []byte("event: message\ndata: "+string(b)+"\n\n"))
		}
	}
}

func (h *hub) state() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for l := range h.listeners {
		h.send(l, []byte("event: state\ndata: {}\n\n"))
	}
}
```

The hub looks messages up with `MessageByID`; add it to `internal/state/state.go`:

```go
// MessageByID returns a message or nil.
func (s *State) MessageByID(id string) *Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].ID == id {
			return s.messages[i]
		}
	}
	return nil
}
```

`internal/web/captive.go`:

```go
package web

import (
	"html/template"
	"net/http"
)

// Phones and laptops check for internet by fetching these; answering with a
// redirect makes them open the sign-in pop-up on our welcome page.
var probes = []string{"/hotspot-detect.html", "/library/test/success.html", "/generate_204", "/gen_204",
	"/connecttest.txt", "/ncsi.txt", "/success.txt", "/canonical.html"}

var welcome = template.Must(template.New("w").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>batnet · {{.}}</title>
<style>body{font:17px/1.5 system-ui,sans-serif;max-width:32rem;margin:2rem auto;padding:0 1rem;color:#16212b}
.url{font:600 22px ui-monospace,monospace;background:#eef2f4;padding:.6rem;border-radius:10px;text-align:center}</style></head>
<body><h1>Welcome to batnet · {{.}}</h1>
<p>This WiFi has no internet. It connects you to the people in {{.}} and the houses around it.</p>
<p>This small window can't record voice or keep you signed in. Open your normal web browser and go to:</p>
<p class="url">https://batnet</p>
<p>Your browser will warn that the connection is "not private" once. That's expected without the internet: choose
<b>Advanced</b>, then <b>Proceed</b> (Android) or <b>Show details</b>, then <b>visit this website</b> (iPhone).</p>
</body></html>`))

func (s *Server) captive(mux *http.ServeMux) {
	for _, p := range probes {
		mux.HandleFunc("GET "+p, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/welcome", http.StatusFound)
		})
	}
	mux.HandleFunc("GET /welcome", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		welcome.Execute(w, s.cfg.House)
	})
}
```

`internal/web/app/index.html` (placeholder until Task 6): `<!doctype html><meta charset="utf-8"><title>batnet</title><p>batnet</p>`

Task 4 adds `api.go` (the `api` method and `/api/events`); to compile now, add a stub `api.go`:

```go
package web

import "net/http"

func (s *Server) api(mux *http.ServeMux) {}
```

- [ ] **Step 4: Run.** Run: `go test ./internal/web/ -run Captive` — Expected: `ok` (`TestBootstrapAndEvents` waits for Task 4 and 5's API; run it there).

- [ ] **Step 5: Commit**

```bash
git add internal/web internal/state
git commit -m "web: the house server - captive-portal probes, welcome page, log writes, an event hub, first-boot bootstrap"
```

---

### Task 4: Accounts API

**Files:** Replace `internal/web/api.go`; add tests to `internal/web/web_test.go`

**Interfaces — Produces:** `POST /api/signup {name, pin}` → 200 `{account, status:"pending"}` + cookie; `POST /api/login {name, pin}` → 200 `{account}` + cookie, 401 `{"error":"Name or PIN not recognised."}` for any wrong name or PIN, 429 `{"error":…, "retryAfter": seconds}` when locked; `POST /api/logout`; `GET /api/me` → `{account, name, house, status, admin}` or 401; `GET /api/admin/pending` (admins of this house) → `[{id,name}]`; `POST /api/admin/approve|deny {account}`. Bodies over 16 KB → 413. Unknown or bad JSON → 400.

- [ ] **Step 1: Write the failing tests** — append to `internal/web/web_test.go`

```go
func TestSignupApprovalLogin(t *testing.T) {
	miller, fern := newHouse(t, "Miller"), newHouse(t, "Fern")
	link(miller, fern)
	miller.srv.Bootstrap("Anna", "482915", []string{"Medical"})
	fern.srv.Bootstrap("Cleo", "111111", nil)

	ben := &client{t: t, h: miller}
	code, out := ben.do("POST", "/api/signup", map[string]string{"name": "Ben", "pin": "246810"})
	if code != 200 || out["status"] != "pending" {
		t.Fatalf("signup: %d %v", code, out)
	}
	if code, _ := ben.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "hi"}); code != http.StatusForbidden {
		t.Fatalf("a pending account posted: %d", code)
	}
	anna := &client{t: t, h: miller}
	anna.do("POST", "/api/login", map[string]string{"name": "anna", "pin": "482915"})
	_, pend := anna.do("GET", "/api/admin/pending", nil)
	if code, _ := anna.do("POST", "/api/admin/approve", map[string]string{"account": out["account"].(string)}); code != 200 {
		t.Fatalf("approve: %d (pending: %v)", code, pend)
	}
	if _, me := ben.do("GET", "/api/me", nil); me["status"] != "approved" {
		t.Fatalf("Ben after approval: %v", me)
	}
	// Ben can now sign in at another house too (accounts are copied to every house)
	benAtFern := &client{t: t, h: fern}
	if code, _ := benAtFern.do("POST", "/api/login", map[string]string{"name": "Ben", "pin": "246810"}); code != 200 {
		t.Fatalf("login at Fern: %d", code)
	}
	// Cleo, admin at Fern, cannot approve Miller's accounts
	dan := &client{t: t, h: miller}
	_, d := dan.do("POST", "/api/signup", map[string]string{"name": "Dan", "pin": "135790"})
	cleo := &client{t: t, h: fern}
	cleo.do("POST", "/api/login", map[string]string{"name": "Cleo", "pin": "111111"})
	if code, _ := cleo.do("POST", "/api/admin/approve", map[string]string{"account": d["account"].(string)}); code != http.StatusForbidden {
		t.Fatalf("another house's admin approved Dan: %d", code)
	}
}

// Review focus 3.
func TestLoginErrorsAlike(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	c := &client{t: t, h: h}
	c1, o1 := c.do("POST", "/api/login", map[string]string{"name": "Nobody", "pin": "482915"})
	c2, o2 := c.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "000000"})
	if c1 != 401 || c2 != 401 || o1["error"] != o2["error"] {
		t.Fatalf("wrong name %d %v, wrong PIN %d %v", c1, o1, c2, o2)
	}
	for i := 0; i < 5; i++ {
		c.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "000000"})
	}
	if code, out := c.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "482915"}); code != 429 || out["retryAfter"] == nil {
		t.Fatalf("not locked after six wrong PINs: %d %v", code, out)
	}
}

// Review focus 4.
func TestSignedOutCannotRead(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	c := &client{t: t, h: h}
	for _, p := range []string{"/api/me", "/api/conversations", "/api/messages?conv=everyone", "/api/directory", "/api/events", "/api/admin/pending"} {
		if code, _ := c.do("GET", p, nil); code != 401 {
			t.Errorf("%s without a session: %d", p, code)
		}
	}
	if code, _ := c.do("POST", "/api/signup", map[string]string{"name": "Ben", "pin": "12345"}); code != 400 {
		t.Errorf("a 5-digit PIN was accepted: %d", code)
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/web/ -run 'Signup|LoginErrors|SignedOut'` — Expected: FAIL (404s from the stub).

- [ ] **Step 3: Implement** — `internal/web/api.go`

```go
package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/johnusher/batnet/internal/auth"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
	"github.com/johnusher/batnet/internal/state"
)

const maxBody = 16 << 10

type apiError struct {
	code int
	msg  string
	wait time.Duration
}

func (e *apiError) Error() string { return e.msg }

func fail(code int, msg string) error { return &apiError{code: code, msg: msg} }

// handle wraps an API handler: JSON in and out, errors as {"error": …}.
func handle(f func(w http.ResponseWriter, r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		out, err := f(w, r)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			var ae *apiError
			var tooBig *http.MaxBytesError
			body := map[string]any{"error": "Something went wrong."}
			code := http.StatusInternalServerError
			switch {
			case errors.As(err, &ae):
				code, body["error"] = ae.code, ae.msg
				if ae.wait > 0 {
					body["retryAfter"] = int(math.Ceil(ae.wait.Seconds()))
				}
			case errors.As(err, &tooBig):
				code, body["error"] = http.StatusRequestEntityTooLarge, "Too much data."
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(body)
			return
		}
		json.NewEncoder(w).Encode(out)
	}
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return err
		}
		return fail(http.StatusBadRequest, "That request isn't valid.")
	}
	return nil
}

func (s *Server) setSession(w http.ResponseWriter, account string) {
	tok := s.sessions.New(account, s.cfg.Now())
	http.SetCookie(w, &http.Cookie{Name: "batnet", Value: tok, Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode, MaxAge: 365 * 24 * 3600})
}

// who returns the session's account (any status), or a 401.
func (s *Server) who(r *http.Request) (*state.Account, error) {
	ck, err := r.Cookie("batnet")
	if err != nil {
		return nil, fail(http.StatusUnauthorized, "Please sign in.")
	}
	id, ok := s.sessions.Get(ck.Value, s.cfg.Now())
	if !ok {
		return nil, fail(http.StatusUnauthorized, "Please sign in.")
	}
	a := s.cfg.State.Account(id)
	if a == nil {
		return nil, fail(http.StatusUnauthorized, "Please sign in.")
	}
	return a, nil
}

// member returns an approved account, or 401/403.
func (s *Server) member(r *http.Request) (*state.Account, error) {
	a, err := s.who(r)
	if err != nil {
		return nil, err
	}
	if a.Status != "approved" {
		return nil, fail(http.StatusForbidden, "Your account is waiting for approval.")
	}
	return a, nil
}

func (s *Server) admin(r *http.Request) (*state.Account, error) {
	a, err := s.member(r)
	if err != nil {
		return nil, err
	}
	if !a.Admin || a.Home != s.cfg.Author.ID() {
		return nil, fail(http.StatusForbidden, "Only this house's admin can do that.")
	}
	return a, nil
}

func validName(n string) bool {
	n = strings.TrimSpace(n)
	return n != "" && utf8.RuneCountInString(n) <= 40
}

func (s *Server) api(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/signup", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct{ Name, Pin string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if !validName(in.Name) || !auth.ValidPIN(in.Pin) {
			return nil, fail(http.StatusBadRequest, "Choose a name and a 6-digit PIN.")
		}
		h, err := auth.HashPIN(in.Pin)
		if err != nil {
			return nil, err
		}
		e, err := s.Write(ledger.TypeAccountRequest, prio.Normal, map[string]any{"name": strings.TrimSpace(in.Name), "pinHash": h})
		if err != nil {
			return nil, err
		}
		id := state.AccountID(e.Key)
		s.setSession(w, id)
		return map[string]string{"account": id, "status": "pending"}, nil
	}))
	mux.HandleFunc("POST /api/login", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct{ Name, Pin string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		now := s.cfg.Now()
		key := strings.ToLower(strings.TrimSpace(in.Name))
		if wait := s.lockout.Wait(key, now); wait > 0 {
			return nil, &apiError{code: http.StatusTooManyRequests, msg: "Too many wrong PINs. Try again later.", wait: wait}
		}
		for _, a := range s.cfg.State.AccountsNamed(in.Name) {
			if auth.CheckPIN(a.PinHash, in.Pin) {
				s.lockout.Success(key)
				s.setSession(w, a.ID)
				return map[string]string{"account": a.ID}, nil
			}
		}
		s.lockout.Fail(key, now)
		return nil, fail(http.StatusUnauthorized, "Name or PIN not recognised.")
	}))
	mux.HandleFunc("POST /api/logout", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if ck, err := r.Cookie("batnet"); err == nil {
			s.sessions.End(ck.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: "batnet", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteStrictMode})
		return map[string]bool{"ok": true}, nil
	}))
	mux.HandleFunc("GET /api/me", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.who(r)
		if err != nil {
			return nil, err
		}
		return map[string]any{"account": a.ID, "name": a.Name, "house": s.cfg.State.HouseName(a.Home), "here": s.cfg.House,
			"status": a.Status, "admin": a.Admin && a.Home == s.cfg.Author.ID()}, nil
	}))
	mux.HandleFunc("GET /api/admin/pending", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := s.admin(r); err != nil {
			return nil, err
		}
		out := []map[string]string{}
		for _, a := range s.cfg.State.Pending(s.cfg.Author.ID()) {
			out = append(out, map[string]string{"id": a.ID, "name": a.Name})
		}
		return out, nil
	}))
	for _, verb := range []string{"approve", "deny"} {
		typ := ledger.TypeAccountApprove
		if verb == "deny" {
			typ = ledger.TypeAccountDeny
		}
		mux.HandleFunc("POST /api/admin/"+verb, handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
			if _, err := s.admin(r); err != nil {
				return nil, err
			}
			var in struct{ Account string }
			if err := decode(r, &in); err != nil {
				return nil, err
			}
			a := s.cfg.State.Account(in.Account)
			if a == nil || a.Home != s.cfg.Author.ID() {
				return nil, fail(http.StatusForbidden, "That account belongs to another house.")
			}
			if _, err := s.Write(typ, prio.Normal, map[string]string{"account": a.ID}); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		}))
	}
	s.messagesAPI(mux)
}

var _ = fmt.Sprintf // used by messagesAPI's errors
```

Task 5 adds `messagesAPI`; to compile now, add at the end of `api.go`:

```go
func (s *Server) messagesAPI(mux *http.ServeMux) {}
```

- [ ] **Step 4: Run.** Run: `go test -race ./internal/web/ -run 'Captive|Signup|LoginErrors'` — Expected: `ok`. (`TestSignedOutCannotRead` needs Task 5's routes; run it there.)

- [ ] **Step 5: Commit**

```bash
git add internal/web
git commit -m "web: sign up, sign in with lockout, sign out, me, and admin approval by the house's own admin"
```

---

### Task 5: Messages API and the event stream

**Files:** Modify `internal/web/api.go` (replace the `messagesAPI` stub); add tests to `internal/web/web_test.go`

**Interfaces — Produces:** `GET /api/directory` → `{houses:[{node,name,people:[{id,name,admin}]}], groups:[{id,name}]}`; `GET /api/conversations` → `[{key,name,last,count}]`; `GET /api/messages?conv=…&limit=…` → `[message]`; `POST /api/post {conv, text, prio, loc}` → the message; `POST /api/groups/join|leave {group}`; `GET /api/events` (SSE, approved only). Post rules: approved only; `conv` is `everyone`, `group:<id>` (must be a member), or `dm:<account>` (an approved account); `prio` from `low|normal|high|alert` (default normal), `alert` only for this house's admin (writes an `alert` entry, audience everyone), High limited per account per hour (429 with a friendly message); text 1–2000 characters after trimming; `loc` optional and validated.

- [ ] **Step 1: Write the failing tests** — append to `internal/web/web_test.go`

```go
func signedIn(t *testing.T, h *house, name, pin string) *client {
	t.Helper()
	c := &client{t: t, h: h}
	if code, out := c.do("POST", "/api/login", map[string]string{"name": name, "pin": pin}); code != 200 {
		t.Fatalf("login %s: %d %v", name, code, out)
	}
	return c
}

func TestPostAndRead(t *testing.T) {
	miller, fern := newHouse(t, "Miller"), newHouse(t, "Fern")
	link(miller, fern)
	miller.srv.Bootstrap("Anna", "482915", []string{"Medical"})
	fern.srv.Bootstrap("Cleo", "111111", nil)
	anna, cleo := signedIn(t, miller, "Anna", "482915"), signedIn(t, fern, "Cleo", "111111")
	if code, out := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "Water point open", "prio": "high",
		"loc": map[string]any{"lat": 52.52, "lon": 13.40, "acc": 15, "source": "phone"}}); code != 200 || out["prio"].(float64) != 8 {
		t.Fatalf("post: %d %v", code, out)
	}
	code, _ := cleo.do("GET", "/api/messages?conv=everyone", nil)
	if code != 200 {
		t.Fatalf("read at Fern: %d", code)
	}
	req, _ := http.NewRequest("GET", fern.ts.URL+"/api/messages?conv=everyone", nil)
	for _, ck := range cleo.jar {
		req.AddCookie(ck)
	}
	resp, _ := http.DefaultClient.Do(req)
	var msgs []map[string]any
	json.NewDecoder(resp.Body).Decode(&msgs)
	resp.Body.Close()
	if len(msgs) != 1 || msgs[0]["text"] != "Water point open" || msgs[0]["loc"] == nil {
		t.Fatalf("Fern sees %v", msgs)
	}
	_, me := anna.do("GET", "/api/me", nil)
	if code, _ := cleo.do("POST", "/api/post", map[string]any{"conv": "dm:" + me["account"].(string), "text": "Got it"}); code != 200 {
		t.Fatalf("dm: %d", code)
	}
	if code, _ := cleo.do("POST", "/api/post", map[string]any{"conv": "group:medical", "text": "not a member"}); code != http.StatusForbidden {
		t.Fatalf("posted to a group without joining: %d", code)
	}
	cleo.do("POST", "/api/groups/join", map[string]string{"group": "medical"})
	if code, _ := cleo.do("POST", "/api/post", map[string]any{"conv": "group:medical", "text": "Nurse at the school"}); code != 200 {
		t.Fatalf("group post after joining: %d", code)
	}
}

func TestPriorityRules(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	ben := &client{t: t, h: h}
	_, b := ben.do("POST", "/api/signup", map[string]string{"name": "Ben", "pin": "246810"})
	anna.do("POST", "/api/admin/approve", map[string]string{"account": b["account"].(string)})
	for i := 0; i < 3; i++ {
		if code, _ := ben.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "urgent", "prio": "high"}); code != 200 {
			t.Fatalf("High %d refused", i+1)
		}
	}
	if code, out := ben.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "urgent again", "prio": "high"}); code != 429 || !strings.Contains(out["error"].(string), "High") {
		t.Fatalf("a fourth High in the hour: %d %v", code, out)
	}
	if code, _ := ben.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "evacuate", "prio": "alert"}); code != http.StatusForbidden {
		t.Fatalf("a non-admin sent an alert: %d", code)
	}
	if code, out := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "Flood on the low road", "prio": "alert"}); code != 200 || out["alert"] != true || out["prio"].(float64) != 9 {
		t.Fatalf("admin alert: %d %v", code, out)
	}
}

// Review focus 5.
func TestPostValidation(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	bad := []map[string]any{
		{"conv": "everyone", "text": "   "},
		{"conv": "everyone", "text": strings.Repeat("x", 2001)},
		{"conv": "nowhere", "text": "x"},
		{"conv": "dm:nobody", "text": "x"},
		{"conv": "everyone", "text": "x", "prio": "urgent"},
		{"conv": "everyone", "text": "x", "loc": map[string]any{"lat": 91, "lon": 0, "acc": 5, "source": "phone"}},
		{"conv": "everyone", "text": "x", "loc": map[string]any{"lat": 0, "lon": 0, "acc": 5, "source": "satellite"}},
		{"conv": "everyone", "text": "x", "extra": true},
	}
	for i, b := range bad {
		if code, _ := anna.do("POST", "/api/post", b); code != 400 {
			t.Errorf("bad post %d accepted or wrong code: %d", i, code)
		}
	}
	if code, _ := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": strings.Repeat("x", 20000)}); code != 413 {
		t.Errorf("an oversized body: %d", code)
	}
	if _, convs := anna.do("GET", "/api/conversations", nil); len(convs) != 0 {
		t.Fatalf("a rejected post was stored: %v", convs)
	}
}
```

- [ ] **Step 2: Run to see it fail.** Run: `go test ./internal/web/ -run 'PostAndRead|PriorityRules|PostValidation|SignedOut|Bootstrap'` — Expected: FAIL.

- [ ] **Step 3: Implement** — replace the `messagesAPI` stub in `api.go`:

```go
func validLoc(l *state.Location) bool {
	return l == nil || (l.Lat >= -90 && l.Lat <= 90 && l.Lon >= -180 && l.Lon <= 180 && l.Acc >= 0 && l.Acc <= 10000 &&
		(l.Source == "phone" || l.Source == "node"))
}

func (s *Server) messagesAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/directory", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := s.member(r); err != nil {
			return nil, err
		}
		return map[string]any{"houses": s.cfg.State.Directory(), "groups": s.cfg.State.Groups()}, nil
	}))
	mux.HandleFunc("GET /api/conversations", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.member(r)
		if err != nil {
			return nil, err
		}
		out := s.cfg.State.Conversations(a.ID)
		if out == nil {
			out = []state.Conversation{}
		}
		return out, nil
	}))
	mux.HandleFunc("GET /api/messages", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.member(r)
		if err != nil {
			return nil, err
		}
		limit := 100
		fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
		limit = max(1, min(limit, 500))
		out := s.cfg.State.Messages(a.ID, r.URL.Query().Get("conv"), limit)
		if out == nil {
			out = []*state.Message{}
		}
		return out, nil
	}))
	mux.HandleFunc("POST /api/post", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.member(r)
		if err != nil {
			return nil, err
		}
		var in struct {
			Conv string          `json:"conv"`
			Text string          `json:"text"`
			Prio string          `json:"prio"`
			Loc  *state.Location `json:"loc"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		text := strings.TrimSpace(in.Text)
		if text == "" || utf8.RuneCountInString(text) > 2000 {
			return nil, fail(http.StatusBadRequest, "Write between 1 and 2000 characters.")
		}
		if !validLoc(in.Loc) {
			return nil, fail(http.StatusBadRequest, "That location isn't valid.")
		}
		choice := prio.Choice(in.Prio)
		if choice != "" && choice != "low" && choice != "normal" && choice != "high" && choice != "alert" {
			return nil, fail(http.StatusBadRequest, "Priority must be low, normal, high or alert.")
		}
		audience, kind := in.Conv, ""
		switch {
		case in.Conv == "everyone":
			kind = "everyone"
		case strings.HasPrefix(in.Conv, "group:"):
			kind = "group"
			if !a.Groups[strings.TrimPrefix(in.Conv, "group:")] {
				return nil, fail(http.StatusForbidden, "Join the group to post in it.")
			}
		case strings.HasPrefix(in.Conv, "dm:"):
			kind = "dm"
			if !s.cfg.State.Approved(strings.TrimPrefix(in.Conv, "dm:")) {
				return nil, fail(http.StatusBadRequest, "No such person.")
			}
		default:
			return nil, fail(http.StatusBadRequest, "No such conversation.")
		}
		typ := ledger.TypeMsgPost
		switch choice {
		case "alert":
			if !a.Admin || a.Home != s.cfg.Author.ID() {
				return nil, fail(http.StatusForbidden, "Only a house admin can send an alert.")
			}
			typ, audience = ledger.TypeAlert, "everyone"
		case "high":
			if !s.high.Allow(a.ID, s.cfg.Now().UnixMicro()) {
				return nil, fail(http.StatusTooManyRequests, "You have used your 3 High messages for this hour. Send it as Normal.")
			}
		}
		level := prio.ForPost(choice, kind, false)
		body := map[string]any{"from": a.ID, "audience": audience, "text": text}
		if in.Loc != nil {
			body["loc"] = in.Loc
		}
		e, err := s.Write(typ, level, body)
		if err != nil {
			return nil, err
		}
		return s.cfg.State.MessageByID(state.AccountID(e.Key)), nil
	}))
	for _, verb := range []string{"join", "leave"} {
		mux.HandleFunc("POST /api/groups/"+verb, handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
			a, err := s.member(r)
			if err != nil {
				return nil, err
			}
			var in struct{ Group string }
			if err := decode(r, &in); err != nil {
				return nil, err
			}
			found := false
			for _, g := range s.cfg.State.Groups() {
				found = found || g.ID == in.Group
			}
			if !found {
				return nil, fail(http.StatusBadRequest, "No such group.")
			}
			body := map[string]any{"account": a.ID, verb: []string{in.Group}}
			if _, err := s.Write(ledger.TypeAccountUpdate, prio.Housekeeping, body); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		}))
	}
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		a, err := s.member(r)
		if err != nil {
			handle(func(http.ResponseWriter, *http.Request) (any, error) { return nil, err })(w, r)
			return
		}
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		fl.Flush()
		l := s.hub.add(a.ID)
		defer s.hub.remove(l)
		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case b := <-l.ch:
				w.Write(b)
				fl.Flush()
			case <-ping.C:
				w.Write([]byte(": ping\n\n"))
				fl.Flush()
			}
		}
	})
}
```

Remove the `var _ = fmt.Sprintf` line (fmt is used now) and the stub.

- [ ] **Step 4: Run.** Run: `go test -race ./internal/web/ ./internal/state/ ./internal/auth/` — Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/web
git commit -m "web: directory, conversations, messages, posting with priority, location and alerts, groups, and a live event stream"
```

---

### Task 6: The first real app, served by every mock house

**Files:** Replace `internal/web/app/index.html`; create `internal/web/app/app.js`, `internal/web/app/app.css`; modify `cmd/batnetd/main.go`

**Produces:** a working app: landing (sign in / create account / how this works), pending screen (polls `/api/me` every 5 s), chats (conversations + "Everyone" + groups + people, unread counts from the event stream), a conversation (messages with sender, time, High/Alert tags, location line; a box with priority select and a 📍 location toggle using `navigator.geolocation`), people (directory; tap to message), admin (pending requests: approve / deny; Alert everyone). `batnetd -mock N -web` runs N houses with a WiFi line between them, bootstraps each with admin `Admin` / PIN `123456` (placeholder, mock only) and groups Medical and Food, serves house i on `http://127.0.0.1:(8001+i)`, prints the URLs, and runs until interrupted (with `-for 0`).

- [ ] **Step 1: Write the app.** `index.html`: a `<main id="app">` and `<script type="module" src="app.js">`, linking `app.css`. `app.js` (ES2017, no framework): a tiny router over screens rendering into `#app`; `api(path, body)` using `fetch` with JSON and `credentials: 'same-origin'`; an `EventSource('/api/events')` that refreshes the open conversation or the chat list; every user-provided string inserted with `textContent` or an `esc()` helper (never raw `innerHTML` of server data); `navigator.geolocation.getCurrentPosition` only when the 📍 toggle is on, sending `{lat, lon, acc, source:'phone'}`; sound on new messages while open (WebAudio beep, louder for High, a siren pattern for Alert). `app.css`: the mockup's app styles (system fonts, light/dark via `prefers-color-scheme`), large tap targets.

- [ ] **Step 2: Test the app's safety rule** — append to `internal/web/web_test.go`:

```go
// The app must never insert server text as HTML.
func TestAppEscapes(t *testing.T) {
	b, err := appFS.ReadFile("app/app.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if strings.Contains(src, "innerHTML = m.") || strings.Contains(src, "innerHTML = msg.") || strings.Contains(src, "eval(") {
		t.Fatal("app.js inserts server data as HTML or evals")
	}
	if !strings.Contains(src, "function esc(") {
		t.Fatal("app.js lacks its escaping helper")
	}
	total := 0
	fs.WalkDir(appFS, "app", func(p string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			info, _ := d.Info()
			total += int(info.Size())
		}
		return nil
	})
	if total > 200<<10 {
		t.Fatalf("the app is %d bytes, over 200 KB", total)
	}
}
```

(import `io/fs`). Run: `go test ./internal/web/ -run AppEscapes` — Expected: `ok`.

- [ ] **Step 3: Serve it from `batnetd -mock`.** In `cmd/batnetd/main.go`, add `-web` (bool) and allow `-for 0` (run until Ctrl-C). For each mock house: create `state.New()`, a `web.Server` with `Publish` = the house's `Syncer.Publish`, `SecureCookie: false`, `House: name`; wire the Syncer's `OnEntry` to also call `srv.OnEntry`; call `srv.Bootstrap("Admin", "123456", []string{"Medical", "Food"})`; listen on `127.0.0.1:8001+i` with `http.Server{ReadHeaderTimeout: 10 * time.Second}`; print `Miller: http://127.0.0.1:8001  (admin: Admin / 123456)`. With `-web`, the random "message N" traffic stays off.

- [ ] **Step 4: Try it.** Run: `go run ./cmd/batnetd -mock 3 -web -for 0`, open two houses' URLs in two browser windows, sign up at one, approve with Admin, post, and see it arrive at the other. Screenshot both with headless Chrome. Stop with Ctrl-C.

- [ ] **Step 5: Commit**

```bash
git add internal/web/app internal/web/web_test.go cmd/batnetd
git commit -m "The first real app: sign up, approval, chats, posting with priority and location, admin; batnetd -mock -web serves it per house"
```

---

### Task 7: Docs, known problems, to-do

- [ ] README "For developers": `go run ./cmd/batnetd -mock 3 -web -for 0` and what to open.
- [ ] `knowledge/known-problems.md`: anything found (and: sessions live in memory, lost on reboot - people sign in again; the lockout is per house).
- [ ] `knowledge/todo.md`: tick plan 4a; plan 4b next.
- [ ] Run everything and commit:

```bash
go vet ./... && go test -race ./... && go run ./cmd/batnetd -sim scenarios/*.json && (cd tools/okflint && go run . ../../knowledge)
git add README.md knowledge
git commit -m "Docs: run the house server locally; known problems and to-do updated"
```

---

## Self-review

**Spec coverage:** §4 entry types and their bodies → Task 1; §8 captive portal (probes, welcome) → Task 3; accounts (name + 6-digit PIN, admin approval, lockout, year-long sessions, sign in at any house) → Tasks 2, 4; live connection (SSE instead of a WebSocket: standard library, one-way is enough because posts go by POST; recorded as a ruling) → Tasks 3, 5; screens → Task 6 (a first version; 4b completes them); §7 priority with the High limit and admin alerts → Task 5; decision 0019 location → Tasks 1, 5, 6; §12 authority from the log → Task 1. Not here: outbox in IndexedDB, bedside mode, the speaker, kiosk, i18n (4b and plan 5).

**Placeholders:** Task 6 Step 1 describes the app rather than listing it, because the app is UI code shaped by the mockup; its safety rules are tested in Step 2.

**Type consistency:** `state.AccountID`, `state.Location`, `state.Message`, `Server.Write`, `Server.OnEntry`, `Server.Bootstrap`, `prio.ForPost`, `prio.Limiter` used consistently.
