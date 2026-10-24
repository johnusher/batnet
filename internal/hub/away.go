package hub

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/johnusher/batnet/internal/meshtastic"
)

// AwaySettings is the away note: a short answer to a private message the
// owner has not answered after a while (decision 0033).
type AwaySettings struct {
	On      bool   `json:"on"`
	Minutes int    `json:"minutes,omitempty"` // the wait; 0: AwayWait
	Text    string `json:"text,omitempty"`    // "": DefaultAwayText
}

// DefaultAwayText is the note unless the owner wrote their own: simple
// German, and plain that it is automatic.
const DefaultAwayText = "Hallo! Das ist eine automatische Antwort. Ich bin gerade nicht am Funkgerät. Ich schreibe dir bald zurück. Liebe Grüße"

const (
	AwayWait    = 15 * time.Minute   // the wait unless the owner chose another
	awayOncePer = 7 * 24 * time.Hour // one note per radio in this time
	awayPerDay  = 10                 // and at most this many notes a day in all
	awayGiveUp  = 2 * time.Hour      // a note the radio could not send by then is dropped
	awayTick    = 30 * time.Second
)

// AwayConfig wires the away note to the radio and the hub's settings.
type AwayConfig struct {
	Settings func() AwaySettings
	MyID     func() string // this hub's radio, like !a1b2c3d4
	// Send sends text to one radio as a reply to its message.
	Send func(to, text string, replyID uint32) error
	// NoContact is the radios that asked not to be written to.
	NoContact func() []string
	// File keeps when each radio last got a note ("": not across restarts).
	File string
	Now  func() time.Time
}

// Away waits after each private message for the owner's answer, and sends
// the note if none came: once per radio a week, never to a bare "ok" or to
// a message that looks automatic itself, never on a channel.
type Away struct {
	cfg AwayConfig

	mu      sync.Mutex
	pending map[string]awaiting  // radio: its unanswered message
	sent    map[string]time.Time // radio: when it last got a note
}

type awaiting struct {
	at time.Time // its first unanswered message
	id uint32    // its latest, which the note answers
}

// NewAway returns the away note, with the notes already sent read back.
func NewAway(cfg AwayConfig) *Away {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	a := &Away{cfg: cfg, pending: map[string]awaiting{}, sent: map[string]time.Time{}}
	if cfg.File != "" {
		if b, err := os.ReadFile(cfg.File); err == nil {
			json.Unmarshal(b, &a.sent)
		}
	}
	return a
}

// Event takes a radio event (meshtastic's OnEvent): a private message starts
// the wait, anything this hub sends to that radio ends it.
func (a *Away) Event(e meshtastic.Event) {
	switch e.Kind {
	case "packet":
		if e.Local || e.Port != "text" || e.Text == "" || e.From == "" || a.cfg.MyID == nil || e.To != a.cfg.MyID() {
			return
		}
		if !worthANote(e.Text) {
			return
		}
		a.mu.Lock()
		p, ok := a.pending[e.From]
		if !ok {
			p.at = a.cfg.Now()
		}
		p.id = e.ID
		a.pending[e.From] = p
		a.mu.Unlock()
	case "sent":
		a.mu.Lock()
		delete(a.pending, e.To) // answered: by the owner, a test, or the note itself
		a.mu.Unlock()
	}
}

// Run sends the notes that are due until ctx ends.
func (a *Away) Run(ctx context.Context) {
	t := time.NewTicker(awayTick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.tick()
		}
	}
}

// tick sends at most one note: the radio allows one message every few seconds.
func (a *Away) tick() {
	set := AwaySettings{}
	if a.cfg.Settings != nil {
		set = a.cfg.Settings()
	}
	now := a.cfg.Now()
	a.mu.Lock()
	if !set.On {
		clear(a.pending) // switched on later, it does not answer old messages
		a.mu.Unlock()
		return
	}
	wait := AwayWait
	if set.Minutes > 0 {
		wait = time.Duration(set.Minutes) * time.Minute
	}
	today := 0
	for _, t := range a.sent {
		if now.Sub(t) < 24*time.Hour {
			today++
		}
	}
	no := map[string]bool{}
	if a.cfg.NoContact != nil {
		for _, id := range a.cfg.NoContact() {
			no[id] = true
		}
	}
	to, id := "", uint32(0)
	for r, p := range a.pending {
		switch {
		case now.Sub(p.at) < wait:
			continue
		case no[r], now.Sub(a.sent[r]) < awayOncePer, today >= awayPerDay, now.Sub(p.at) > wait+awayGiveUp:
			delete(a.pending, r)
			continue
		}
		to, id = r, p.id
		break
	}
	a.mu.Unlock()
	if to == "" {
		return
	}
	text := strings.TrimSpace(set.Text)
	if text == "" {
		text = DefaultAwayText
	}
	if err := a.cfg.Send(to, text, id); err != nil {
		log.Printf("away: %v (tried again soon)", err)
		return
	}
	a.mu.Lock()
	delete(a.pending, to)
	a.sent[to] = now
	for r, t := range a.sent {
		if now.Sub(t) > awayOncePer {
			delete(a.sent, r)
		}
	}
	b, _ := json.Marshal(a.sent)
	a.mu.Unlock()
	if a.cfg.File != "" {
		tmp := a.cfg.File + ".tmp"
		err := os.WriteFile(tmp, append(b, '\n'), 0o600)
		if err == nil {
			err = os.Rename(tmp, a.cfg.File)
		}
		if err != nil {
			log.Printf("away: %v", err)
		}
	}
}

// Waiting is how many radios wait for an answer, for the panel.
func (a *Away) Waiting() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}

// acks are words that end a conversation rather than start one: a message
// of nothing else gets no note (the talking-on-the-mesh skill).
var acks = map[string]bool{
	"ok": true, "okay": true, "oki": true, "okidoki": true, "k": true, "kk": true,
	"danke": true, "dank": true, "dir": true, "dankeschön": true, "vielen": true, "thx": true, "thanks": true, "thank": true, "you": true, "ty": true,
	"angekommen": true, "empfangen": true, "erhalten": true, "gelesen": true, "da": true, "ist": true, "hier": true, "auch": true,
	"received": true, "copy": true, "roger": true, "ack": true, "got": true, "it": true,
	"ja": true, "jo": true, "jap": true, "yes": true, "yep": true, "top": true, "super": true, "prima": true, "gut": true,
	"klasse": true, "cool": true, "nice": true, "perfekt": true, "passt": true, "test": true, "ping": true, "73": true,
	"gerne": true, "gern": true, "bitte": true, "lg": true, "gruß": true, "grüße": true,
}

// greetings alone open a conversation; with an acknowledgement they end one
// ("Hallo, angekommen!").
var greetings = map[string]bool{"hallo": true, "hello": true, "hi": true, "hey": true, "moin": true, "servus": true, "huhu": true, "na": true}

// automatic is what a message from a machine says: a status post, another
// away note, a bot's answer.
var automatic = []string{"automatisch", "automatic", "auto-reply", "autoreply", "autoantwort", "abwesenheit", "out of office",
	"nodecount", "online for", "uptime", "bot", "pong", "traceroute"}

// worthANote says whether a private message gets the note if the owner does
// not answer: not a bare acknowledgement, not a machine's message.
func worthANote(text string) bool {
	low := strings.ToLower(text)
	words := strings.FieldsFunc(low, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' })
	for _, w := range automatic {
		if strings.Contains(w, " ") || strings.Contains(w, "-") {
			if strings.Contains(low, w) {
				return false
			}
			continue
		}
		for _, x := range words {
			if x == w || (len(w) > 4 && strings.HasPrefix(x, w)) {
				return false
			}
		}
	}
	ack := false
	for _, x := range words {
		x = strings.Trim(x, "-")
		switch {
		case acks[x]:
			ack = true
		case !greetings[x] && x != "":
			return true
		}
	}
	return !ack && len(words) > 0 // a bare hello; not emoji, signs and acknowledgements alone
}
