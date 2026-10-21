package hub

import (
	"context"
	"sync"

	"github.com/johnusher/batnet/internal/meshtastic"
)

// Classify says what a radio event means for the hub's sound and LED: direct
// (a message for this hub), reply (an answer on the public channel to one of
// the hub's messages), public (a message to everyone on the public channel),
// delivered or failed (one of the hub's messages), or "" (nothing to tell).
func Classify(e meshtastic.Event, myID string, mine func(id uint32) bool) string {
	switch e.Kind {
	case "packet":
		if e.Local || e.Port != "text" || e.Text == "" {
			return ""
		}
		switch {
		case e.To == myID:
			return "direct"
		case e.ReplyID != 0 && mine != nil && mine(e.ReplyID):
			return "reply"
		case e.To == "everyone" && e.Channel == 0:
			return "public"
		}
	case "receipt":
		switch e.Status {
		case "delivered":
			return "delivered"
		case "failed":
			return "failed"
		}
	}
	return ""
}

// Notifier turns radio events into chimes, blinks and a waking screen.
type Notifier struct {
	MyID  func() string                // this hub's radio, like !a1b2c3d4
	Mine  func(id uint32) bool         // a message this hub sent
	Sound *Sound                       // nil: no sound
	Light *Light                       // nil: no LED
	Power *Power                       // nil: no screen to wake
	Said  func(kind, who, text string) // optional, for the panel

	ch     chan meshtastic.Event
	once   sync.Once
	mu     sync.Mutex
	unread int
}

func (n *Notifier) init() { n.once.Do(func() { n.ch = make(chan meshtastic.Event, 32) }) }

// Event takes a radio event (meshtastic's OnEvent): it never blocks the radio.
func (n *Notifier) Event(e meshtastic.Event) {
	n.init()
	if e.Kind != "packet" && e.Kind != "receipt" {
		return
	}
	select {
	case n.ch <- e:
	default:
	}
}

// Run handles the events until ctx ends.
func (n *Notifier) Run(ctx context.Context) {
	n.init()
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-n.ch:
			n.handle(e)
		}
	}
}

func (n *Notifier) handle(e meshtastic.Event) {
	my := ""
	if n.MyID != nil {
		my = n.MyID()
	}
	kind := Classify(e, my, n.Mine)
	if kind == "" {
		return
	}
	who := e.FromName
	if who == "" {
		who = e.From
	}
	if kind == "direct" {
		n.mu.Lock()
		n.unread++
		n.mu.Unlock()
	}
	if n.Sound != nil {
		n.Sound.Notify(kind, who, e.Text)
	}
	if n.Light != nil && kind != "delivered" && kind != "failed" {
		n.Light.Flash(kind)
	}
	if n.Power != nil {
		n.Power.Wake(kind)
	}
	if n.Said != nil {
		n.Said(kind, who, e.Text)
	}
}

// Seen marks the messages read: the LED stops blinking.
func (n *Notifier) Seen() {
	n.mu.Lock()
	n.unread = 0
	n.mu.Unlock()
	if n.Light != nil {
		n.Light.Seen()
	}
}

// Unread is the messages for the hub since they were last seen.
func (n *Notifier) Unread() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.unread
}
