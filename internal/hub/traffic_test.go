package hub

import (
	"testing"

	"github.com/johnusher/batnet/internal/meshtastic"
)

func TestTraffic(t *testing.T) {
	preset := "LongFast"
	tr := NewTraffic(func() string { return preset })
	tr.Event(meshtastic.Event{Kind: "packet", From: "!a1b2c3d4", To: "everyone", Port: "position", SNR: -9.5, Hops: 2, Relay: "50", Bytes: 60})
	tr.Event(meshtastic.Event{Kind: "packet", From: "!deadbeef", Port: "telemetry", Local: true}) // our own radio: not on the air
	tr.Event(meshtastic.Event{Kind: "packet", From: "!a1b2c3d4", To: "everyone", Bytes: 70})      // a channel we have no key for
	tr.Event(meshtastic.Event{Kind: "sent", To: "!a1b2c3d4", Port: "text", Bytes: 40})
	tr.Garbled(meshtastic.Unreadable{From: "!deadbeef", SNR: -20.75})
	tr.Relayed("50")
	tr.Busy()

	out := tr.Since(0)
	kinds := ""
	for _, f := range out.Frames {
		kinds += f.Kind + " "
	}
	if kinds != "heard heard sent garbled relayed busy " || out.Seq != 6 || out.Limit != -17.5 {
		t.Fatalf("frames %q, seq %d, limit %v", kinds, out.Seq, out.Limit)
	}
	if !out.Frames[1].Encrypted || out.Frames[0].Encrypted || out.Frames[0].Relay != "50" {
		t.Errorf("frames %+v", out.Frames[:2])
	}
	if got := tr.Since(4).Frames; len(got) != 2 || got[0].Kind != "relayed" {
		t.Errorf("since 4: %+v", got)
	}

	// LongFast, 59 bytes on the air: about 0.6 s; MediumFast about a third.
	lf := out.Frames[0].AirMs
	preset = "MediumFast"
	mf := tr.airtime(59)
	if lf < 450 || lf > 750 || mf < lf/4 || mf > lf/2 {
		t.Errorf("airtime LongFast %d ms, MediumFast %d ms", lf, mf)
	}
}
