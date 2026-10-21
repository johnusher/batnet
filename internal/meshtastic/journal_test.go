package meshtastic

import (
	"strings"
	"testing"
)

func TestParseUnreadable(t *testing.T) {
	line := "ERROR | 13:59:57 2873 [RadioIf] Ignore received packet due to error=-7 (maybe id=0xe2b85b4c fr=0xdeadbeef to=0xa1b2c3d4 flags=0x61 rxSNR=-20.75 rxRSSI=-94 nextHop=0x0 relay=0xfc)"
	u, ok := ParseUnreadable(line)
	if !ok {
		t.Fatal("not parsed")
	}
	want := Unreadable{Error: -7, ID: 0xe2b85b4c, From: "!deadbeef", To: "!a1b2c3d4", Hops: 2, HopStart: 3, SNR: -20.75, RSSI: -94, Relay: "fc"}
	if u != want {
		t.Fatalf("got %+v, want %+v", u, want)
	}

	u, _ = ParseUnreadable("ERROR | 14:00:45 2920 [RadioIf] Ignore received packet due to error=-7 (maybe id=0x1 fr=0xdeadbeef to=0xffffffff flags=0xe3 rxSNR=-5 rxRSSI=-80 nextHop=0x0 relay=0x0)")
	if u.To != "everyone" || u.Hops != 4 || u.HopStart != 7 || u.Relay != "" {
		t.Fatalf("broadcast: %+v", u)
	}

	if _, ok := ParseUnreadable("WARN  | 14:20:41 4116 [RadioIf] Can not send yet, busyRx"); ok {
		t.Fatal("parsed a line that is not an unreadable packet")
	}
}

func TestJournalLines(t *testing.T) {
	var got []string
	h := JournalHandlers{
		Unreadable: func(u Unreadable) { got = append(got, "unreadable "+u.From) },
		Relayed:    func(r string) { got = append(got, "relayed "+r) },
		Busy:       func() { got = append(got, "busy") },
	}
	for _, l := range []string{
		"ERROR | 13:59:57 2873 [RadioIf] Ignore received packet due to error=-7 (maybe id=0x1 fr=0xdeadbeef to=0xffffffff flags=0x63 rxSNR=-5 rxRSSI=-80 nextHop=0x0 relay=0x0)",
		"INFO  | 15:21:21 783 [Router] Rebroadcast received message coming from 80",
		"INFO  | 15:21:24 787 [Router] Rebroadcast received message coming from 0",
		"WARN  | 14:20:41 4116 [RadioIf] Can not send yet, busyRx",
		"INFO  | 14:20:40 4116 Tell client we have new packets 96",
	} {
		h.line(l)
	}
	if want := "unreadable !deadbeef|relayed 50|relayed |busy"; strings.Join(got, "|") != want {
		t.Errorf("got %q, want %q", strings.Join(got, "|"), want)
	}
}
