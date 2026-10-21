package hub

import (
	"testing"

	"github.com/johnusher/batnet/internal/meshtastic"
)

func TestClassify(t *testing.T) {
	me := "!deadbeef"
	mine := func(id uint32) bool { return id == 42 }
	cases := []struct {
		e    meshtastic.Event
		want string
	}{
		{meshtastic.Event{Kind: "packet", Port: "text", Text: "hi", To: me}, "direct"},
		{meshtastic.Event{Kind: "packet", Port: "text", Text: "hi", To: me, ReplyID: 42}, "direct"},
		{meshtastic.Event{Kind: "packet", Port: "text", Text: "yes", To: "everyone", ReplyID: 42}, "reply"},
		{meshtastic.Event{Kind: "packet", Port: "text", Text: "hi", To: "everyone"}, "public"},
		{meshtastic.Event{Kind: "packet", Port: "text", Text: "hi", To: "everyone", Channel: 1}, ""},
		{meshtastic.Event{Kind: "packet", Port: "text", Text: "hi", To: "!a1b2c3d4"}, ""},
		{meshtastic.Event{Kind: "packet", Port: "text", Text: "mine", To: "everyone", Local: true}, ""},
		{meshtastic.Event{Kind: "packet", Port: "position", To: me}, ""},
		{meshtastic.Event{Kind: "receipt", Status: "delivered"}, "delivered"},
		{meshtastic.Event{Kind: "receipt", Status: "passed on"}, ""},
		{meshtastic.Event{Kind: "receipt", Status: "failed"}, "failed"},
	}
	for i, c := range cases {
		if got := Classify(c.e, me, mine); got != c.want {
			t.Errorf("%d: %q, want %q", i, got, c.want)
		}
	}
}

func TestNotifierUnread(t *testing.T) {
	n := &Notifier{MyID: func() string { return "!deadbeef" }}
	n.init()
	n.handle(meshtastic.Event{Kind: "packet", Port: "text", Text: "hi", To: "!deadbeef"})
	n.handle(meshtastic.Event{Kind: "packet", Port: "text", Text: "all", To: "everyone"})
	if n.Unread() != 1 {
		t.Fatal(n.Unread())
	}
	n.Seen()
	if n.Unread() != 0 {
		t.Fatal("not cleared")
	}
}
