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

	verdictTS hlc.Timestamp            // the verdict in force: the latest valid one wins
	groupTS   map[string]hlc.Timestamp // per group, the latest join or leave wins
}

// clone copies an account so callers never share its maps with the state.
func (a *Account) clone() *Account {
	c := *a
	c.Groups = make(map[string]bool, len(a.Groups))
	for g, in := range a.Groups {
		c.Groups[g] = in
	}
	c.groupTS = nil
	return &c
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
	ClientID string        `json:"clientId,omitempty"` // chosen by the phone; a retry with the same one is stored once
	origin   ledger.NodeID // the node that wrote it
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
func AccountID(k ledger.Key) string {
	return fmt.Sprintf("%s/%d", hex.EncodeToString(k.Origin[:]), k.Seq)
}

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
		a := &Account{ID: id, Name: b.Name, PinHash: b.PinHash, Home: e.Key.Origin, Admin: b.Admin, Status: "pending", Groups: map[string]bool{}, Since: e.TS, groupTS: map[string]hlc.Timestamp{}}
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
			ClientID string    `json:"clientId"`
		}
		if json.Unmarshal(e.Body, &b) != nil || b.From == "" || b.Text == "" {
			return
		}
		if b.ClientID != "" {
			for _, old := range s.messages {
				if old.From == b.From && old.ClientID == b.ClientID {
					return // a retry of a message already here
				}
			}
		}
		m := &Message{ID: AccountID(e.Key), Key: e.Key, From: b.From, Audience: b.Audience, Text: b.Text, Prio: e.Prio, Loc: b.Loc, TS: e.TS, ClientID: b.ClientID, origin: e.Key.Origin}
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
	if v.by != a.Home || (a.Status != "pending" && !a.verdictTS.Less(v.ts)) {
		return // not this account's house, or older than the verdict in force
	}
	a.verdictTS = v.ts
	if v.approve {
		a.Status = "approved"
	} else {
		a.Status = "denied"
	}
}

// member applies joins and leaves; for each group the latest change wins.
func (s *State) member(a *Account, u membership) {
	set := func(g string, in bool) {
		if last, ok := a.groupTS[g]; ok && !last.Less(u.ts) {
			return
		}
		a.groupTS[g] = u.ts
		if in {
			a.Groups[g] = true
		} else {
			delete(a.Groups, g)
		}
	}
	for _, g := range u.join {
		set(g, true)
	}
	for _, g := range u.leave {
		set(g, false)
	}
}

// Account returns an account or nil.
func (s *State) Account(id string) *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if a := s.accounts[id]; a != nil {
		return a.clone()
	}
	return nil
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
			out = append(out, a.clone())
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
			out = append(out, a.clone())
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
		h.People = append(h.People, a.clone())
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
	return a != nil && a.Status == "approved" && (!m.Alert || (a.Admin && m.origin == a.Home))
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

// FindClient returns account's message with this clientId, or nil.
func (s *State) FindClient(account, clientID string) *Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.messages) - 1; i >= 0; i-- {
		if m := s.messages[i]; m.From == account && m.ClientID == clientID {
			return m
		}
	}
	return nil
}
