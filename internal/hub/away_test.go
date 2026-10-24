package hub

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
)

type awayRig struct {
	a    *Away
	now  time.Time
	set  AwaySettings
	no   []string
	sent []string // to:replyID:text
}

func newAwayRig(t *testing.T, file string) *awayRig {
	r := &awayRig{now: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC), set: AwaySettings{On: true}}
	r.a = NewAway(AwayConfig{
		Settings:  func() AwaySettings { return r.set },
		MyID:      func() string { return "!a1b2c3d4" },
		NoContact: func() []string { return r.no },
		Now:       func() time.Time { return r.now },
		File:      file,
		Send: func(to, text string, reply uint32) error {
			r.sent = append(r.sent, to+":"+text)
			if reply == 0 {
				t.Errorf("the note is not a reply")
			}
			// the radio reports what it sent, as meshtastic does
			r.a.Event(meshtastic.Event{Kind: "sent", To: to, Port: "text", Text: text, Local: true})
			return nil
		},
	})
	return r
}

func (r *awayRig) dm(from, text string) {
	r.a.Event(meshtastic.Event{Kind: "packet", From: from, To: "!a1b2c3d4", Port: "text", Text: text, ID: 7})
}

func (r *awayRig) after(d time.Duration) { r.now = r.now.Add(d); r.a.tick() }

func TestAwayAnswersAfterTheWait(t *testing.T) {
	r := newAwayRig(t, "")
	r.dm("!deadbeef", "Hallo, wer bist du?")
	r.after(14 * time.Minute)
	if len(r.sent) != 0 {
		t.Fatalf("sent before the wait: %v", r.sent)
	}
	r.after(time.Minute)
	if len(r.sent) != 1 || r.sent[0] != "!deadbeef:"+DefaultAwayText {
		t.Fatalf("sent %v", r.sent)
	}
	// once a week: a second message the next day gets none
	r.dm("!deadbeef", "Bist du noch da?")
	r.after(24 * time.Hour)
	r.after(time.Minute)
	if len(r.sent) != 1 {
		t.Fatalf("a second note within the week: %v", r.sent)
	}
	r.dm("!deadbeef", "Hallo nochmal")
	r.after(7 * 24 * time.Hour)
	r.after(time.Minute)
	if len(r.sent) != 1 { // that one waited past giving up
		t.Fatalf("an old message answered: %v", r.sent)
	}
	r.dm("!deadbeef", "Hallo nochmal")
	r.after(16 * time.Minute)
	if len(r.sent) != 2 {
		t.Fatalf("no note a week later: %v", r.sent)
	}
}

func TestAwayNotWhenAnswered(t *testing.T) {
	r := newAwayRig(t, "")
	r.dm("!deadbeef", "Hallo, wer bist du?")
	r.after(5 * time.Minute)
	r.a.Event(meshtastic.Event{Kind: "sent", To: "!deadbeef", Port: "text", Text: "Hi!", Local: true})
	r.after(20 * time.Minute)
	if len(r.sent) != 0 {
		t.Fatalf("answered, yet a note: %v", r.sent)
	}
}

func TestAwayNotWhenOffOrOptedOut(t *testing.T) {
	r := newAwayRig(t, "")
	r.set.On = false
	r.dm("!deadbeef", "Hallo, wer bist du?")
	r.after(20 * time.Minute)
	r.set.On = true
	r.after(time.Minute)
	r.no = []string{"!00000000"}
	r.dm("!00000000", "Hallo, wer bist du?")
	r.after(20 * time.Minute)
	if len(r.sent) != 0 {
		t.Fatalf("sent %v", r.sent)
	}
}

func TestAwayOwnWaitAndText(t *testing.T) {
	r := newAwayRig(t, "")
	r.set = AwaySettings{On: true, Minutes: 5, Text: "Bin gleich da."}
	r.dm("!deadbeef", "Hallo?")
	r.after(5 * time.Minute)
	if len(r.sent) != 1 || r.sent[0] != "!deadbeef:Bin gleich da." {
		t.Fatalf("sent %v", r.sent)
	}
}

func TestAwayNotOnChannels(t *testing.T) {
	r := newAwayRig(t, "")
	r.a.Event(meshtastic.Event{Kind: "packet", From: "!deadbeef", To: "everyone", Port: "text", Text: "Hallo zusammen", ID: 3})
	r.after(20 * time.Minute)
	if len(r.sent) != 0 {
		t.Fatalf("sent %v", r.sent)
	}
}

func TestAwayAtMostTenADay(t *testing.T) {
	r := newAwayRig(t, "")
	for i := 0; i < 12; i++ {
		r.dm("!0000"+string(rune('a'+i))+"000", "Hallo, wer bist du?")
	}
	r.after(16 * time.Minute)
	for i := 0; i < 12; i++ {
		r.after(awayTick)
	}
	if len(r.sent) != awayPerDay {
		t.Fatalf("%d notes", len(r.sent))
	}
}

func TestAwayRemembersAcrossRestarts(t *testing.T) {
	f := filepath.Join(t.TempDir(), "away.json")
	r := newAwayRig(t, f)
	r.dm("!deadbeef", "Hallo, wer bist du?")
	r.after(16 * time.Minute)
	r2 := newAwayRig(t, f)
	r2.now = r.now.Add(time.Hour)
	r2.dm("!deadbeef", "Hallo?")
	r2.after(16 * time.Minute)
	if len(r.sent) != 1 || len(r2.sent) != 0 {
		t.Fatalf("sent %v then %v", r.sent, r2.sent)
	}
}

func TestWorthANote(t *testing.T) {
	for text, want := range map[string]bool{
		"Hallo, wer bist du?": true,
		"Hallo":               true,
		"Angekommen! Grüße aus dem Norden, etwa 12 km": true,
		"ok":                                 false,
		"👍":                                  false,
		"Angekommen 👍":                       false,
		"Danke dir!":                         false,
		"Hallo, angekommen!":                 false,
		"Test":                               false,
		"Automatische Antwort: bin weg":      false,
		"online for 2d 10h | 📳 Nodecount:97": false,
		"I'm a bot":                          false,
	} {
		if got := worthANote(text); got != want {
			t.Errorf("worthANote(%q) = %v", text, got)
		}
	}
}

func TestAwayDefaultMatchesThePanel(t *testing.T) {
	b, err := panelFS.ReadFile("panel/panel.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"`+DefaultAwayText+`"`) {
		t.Error("panel.js's AWAY_TEXT is not DefaultAwayText")
	}
}
