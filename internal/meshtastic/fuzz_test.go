package meshtastic

import (
	"testing"
	"time"
)

// Security review 2026-10-22: every byte of a FromRadio packet is chosen by a
// stranger's radio on the public mesh. None of the hand-written protobuf
// decoders may panic on any input.

// FuzzFields: the protobuf splitter on arbitrary bytes.
func FuzzFields(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x08, 0x96, 0x01})                                                 // varint
	f.Add([]byte{0x12, 0x03, 'a', 'b', 'c'})                                        // bytes
	f.Add([]byte{0x12, 0xff, 0xff, 0xff, 0xff, 0x0f})                               // length far beyond the buffer
	f.Add([]byte{0x0d, 1, 2, 3})                                                    // short fixed32
	f.Add([]byte{0x09, 1, 2, 3, 4, 5, 6, 7})                                        // short fixed64
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}) // overlong varint key
	f.Fuzz(func(t *testing.T, b []byte) {
		fs, err := fields(b)
		if err != nil {
			return
		}
		var e enc
		for _, x := range fs {
			e.put(x)
		}
		again, err := fields(e.b)
		if err != nil || len(again) != len(fs) {
			t.Fatalf("re-encoding %d fields gave %d (%v)", len(fs), len(again), err)
		}
	})
}

// FuzzPacket: a MeshPacket's every decoder, as handle would run them, plus the
// whole client on the same bytes as a FromRadio (nodeInfo, config, channels).
func FuzzPacket(f *testing.F) {
	f.Add([]byte{})
	f.Add(textPacket(0xdeadbeef, "hello", 7.5, 3, 2))
	f.Add(nodeInfo(0xdeadbeef, userMsg("!deadbeef", "Anna", "AN"), 3, 1))
	f.Add(routingPacket(0xdeadbeef, 42, 0))
	for _, port := range []uint32{PortText, PortPosition, PortNodeInfo, PortRouting, PortTelemetry, PortTraceroute, PortNeighborInfo, PortPrivate} {
		f.Add(encodePacket(Broadcast, 0, 1, 3, port, []byte{0x0d, 1, 2, 3, 4, 0x15, 5, 6, 7, 8, 0x22, 2, 0x08, 0x01}, sendOpts{}))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if p, err := parsePacket(b); err == nil {
			_ = info(p.port, p.payload)
			parsePosition(p.payload)
			parseUser(p.payload)
			parseMetrics(p.payload)
			routingError(p.payload)
			_ = p.hops()
		}
		c := New("127.0.0.1:1")
		c.st.MyNum = 0x11223344
		c.st.Ready = true
		c.now = func() time.Time { return time.Unix(1_790_000_000, 0) }
		c.OnEvent(func(Event) {})
		c.handle(b)
		// And as the inner packet of a FromRadio, as meshtasticd frames it.
		var fr enc
		fr.bytes(frPacket, b)
		c.handle(fr.b)
		fr = enc{}
		fr.bytes(frNodeInfo, b)
		c.handle(fr.b)
		fr = enc{}
		fr.bytes(frConfig, b)
		fr.bytes(frChannel, b)
		fr.bytes(frMyInfo, b)
		fr.bytes(frMetadata, b)
		c.handle(fr.b)
		st := c.Status()
		if len(st.Nodes) > 8 { // handled as a packet, a node and a config; a few `from` values each
			t.Fatalf("%d nodes from one packet", len(st.Nodes))
		}
		c.SavedLinks()
	})
}

// FuzzUnreadable: meshtasticd's log lines (the header fields in them come off the air).
func FuzzUnreadable(f *testing.F) {
	f.Add("ERROR | 13:59:57 2873 [RadioIf] Ignore received packet due to error=-7 (maybe id=0x1 fr=0xdeadbeef to=0xffffffff flags=0x63 rxSNR=-20.75 rxRSSI=-94 nextHop=0x0 relay=0xef)")
	f.Add("Ignore received packet due to error=-7 (flags=0xffffffffffffffffffff fr=zz to=)")
	f.Add("Rebroadcast received message coming from 99999999999999999999")
	f.Fuzz(func(t *testing.T, s string) {
		ParseUnreadable(s)
		JournalHandlers{Unreadable: func(Unreadable) {}, Relayed: func(string) {}, Busy: func() {}}.line(s)
	})
}
