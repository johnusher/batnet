package meshtastic

import (
	"bufio"
	"context"
	"math"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRadio is a meshtasticd stand-in: it answers want_config with a small
// node list and records what the client sends.
type fakeRadio struct {
	ln  net.Listener
	got chan []byte
	out chan []byte
}

func newFakeRadio(t *testing.T) *fakeRadio {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRadio{ln: ln, got: make(chan []byte, 16), out: make(chan []byte, 16)}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			r := bufio.NewReader(conn)
			for {
				b, err := readFrame(r)
				if err != nil {
					return
				}
				f.got <- b
			}
		}()
		for b := range f.out {
			conn.Write(b)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return f
}

func (f *fakeRadio) send(pb []byte) { f.out <- frame(pb) }

func userMsg(id, long, short string) []byte {
	var u enc
	u.bytes(1, []byte(id))
	u.bytes(2, []byte(long))
	u.bytes(3, []byte(short))
	return u.b
}

func nodeInfo(num uint32, u []byte, snr float32, hops uint32) []byte {
	var n enc
	n.varint(1, uint64(num))
	n.bytes(2, u)
	n.fixed32(4, math.Float32bits(snr))
	n.varint(9, uint64(hops))
	var fr enc
	fr.bytes(frNodeInfo, n.b)
	return fr.b
}

func textPacket(from uint32, text string, snr float32, hopStart, hopLimit uint32) []byte {
	var d enc
	d.varint(dPortnum, PortText)
	d.bytes(dPayload, []byte(text))
	var mp enc
	mp.fixed32(mpFrom, from)
	mp.fixed32(mpTo, Broadcast)
	mp.bytes(mpDecoded, d.b)
	mp.fixed32(mpRxSNR, math.Float32bits(snr))
	rssi := int64(-97)
	mp.varint(mpRxRSSI, uint64(rssi)) // a negative int32 is a 10-byte varint
	mp.varint(mpHopLimit, uint64(hopLimit))
	mp.varint(mpHopStart, uint64(hopStart))
	var fr enc
	fr.bytes(frPacket, mp.b)
	return fr.b
}

func waitFor(t *testing.T, c *Client, ok func(Status) bool) Status {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st := c.Status(); ok(st) {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out; status %+v", c.Status())
	return Status{}
}

func TestClientReadsTheRadio(t *testing.T) {
	f := newFakeRadio(t)
	c := New(f.ln.Addr().String())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	// The client asks for the config first.
	fs, err := fields(<-f.got)
	if err != nil || len(fs) != 1 || fs[0].num != trWantConfigID {
		t.Fatalf("first message %+v %v", fs, err)
	}
	var my, myInfo enc
	my.varint(1, 0xdeadbeef)
	myInfo.bytes(frMyInfo, my.b)
	f.send(myInfo.b)
	f.out <- []byte("INFO | some log text on the stream\n") // skipped
	f.send(nodeInfo(0xdeadbeef, userMsg("!deadbeef", "batnet hub 1", "BH1"), 0, 0))
	f.send(nodeInfo(0xa1b2c3d4, userMsg("!a1b2c3d4", "Anna's handheld", "ANNA"), 6.5, 1))
	var lora, cfg, fr enc
	lora.varint(2, 0)   // LongFast
	lora.varint(7, 3)   // EU_868
	lora.varint(8, 3)   // hop limit
	lora.varint(9, 1)   // tx enabled
	lora.varint(10, 27) // dBm
	cfg.bytes(6, lora.b)
	fr.bytes(frConfig, cfg.b)
	f.send(fr.b)
	var chs, ch, chFr enc
	chs.bytes(3, nil)
	ch.varint(1, 0)
	ch.bytes(2, chs.b)
	ch.varint(3, 1)
	chFr.bytes(frChannel, ch.b)
	f.send(chFr.b)
	var done enc
	done.varint(frConfigComplete, 1)
	f.send(done.b)
	f.send(textPacket(0xa1b2c3d4, "hello from the field", 4.25, 3, 2))

	st := waitFor(t, c, func(s Status) bool { return s.Ready && len(s.Texts) == 1 })
	if st.Region != "EU_868" || st.Preset != "LongFast" || st.TxPower != 27 || !st.TxEnabled || st.HopLimit != 3 {
		t.Errorf("radio settings %+v", st)
	}
	if len(st.Channels) != 1 || st.Channels[0].Name != "LongFast" || st.Channels[0].Role != "primary" {
		t.Errorf("channels %+v", st.Channels)
	}
	if len(st.Nodes) != 2 || !st.Nodes[0].Self || !st.Nodes[0].Batnet || st.Nodes[1].Batnet {
		t.Fatalf("nodes %+v", st.Nodes)
	}
	anna := st.Nodes[1]
	if anna.LongName != "Anna's handheld" || anna.SNR != 4.25 || anna.RSSI != -97 || anna.Hops != 1 || anna.Packets != 1 {
		t.Errorf("anna %+v", anna)
	}
	if tx := st.Texts[0]; tx.Text != "hello from the field" || tx.From != 0xa1b2c3d4 || tx.Hops != 1 || tx.Mine {
		t.Errorf("text %+v", tx)
	}

	// Sending: a broadcast text packet with our hop limit, then a wait.
	if err := c.SendText("hi from the hub", 0); err != nil {
		t.Fatal(err)
	}
	if err := c.SendText("again", 0); err != ErrTooSoon {
		t.Errorf("second send: %v", err)
	}
	p := sentPacket(t, f)
	if p.to != Broadcast || p.port != PortText || string(p.payload) != "hi from the hub" || p.hopLimit != 3 {
		t.Errorf("sent %+v", p)
	}
	if err := c.SayHello(true); err != nil {
		t.Fatal(err)
	}
	if p := sentPacket(t, f); p.port != PortNodeInfo || !strings.Contains(string(p.payload), "batnet hub 1") {
		t.Errorf("hello %+v", p)
	}
	if err := c.SayHello(true); err == nil {
		t.Error("a second hello at once should wait")
	}
	if st := c.Status(); len(st.Texts) != 2 || !st.Texts[1].Mine || st.PacketsOut != 2 {
		t.Errorf("after sending %+v", st)
	}
}

// sentPacket reads ToRadio messages until a packet arrives (heartbeats are skipped).
func sentPacket(t *testing.T, f *fakeRadio) packet {
	t.Helper()
	for {
		select {
		case b := <-f.got:
			fs, err := fields(b)
			if err != nil {
				t.Fatal(err)
			}
			for _, fl := range fs {
				if fl.num == trPacket {
					p, err := parsePacket(fl.b)
					if err != nil {
						t.Fatal(err)
					}
					return p
				}
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no packet sent")
		}
	}
}

func TestOfflineAndBadInput(t *testing.T) {
	c := New("127.0.0.1:1")
	if err := c.SendText("x", 0); err != ErrOffline {
		t.Errorf("offline send: %v", err)
	}
	if err := c.SendText(strings.Repeat("x", MaxText+1), 0); err == nil {
		t.Error("too long accepted")
	}
	if _, err := fields([]byte{0x0a, 0x05, 'a'}); err == nil {
		t.Error("truncated field accepted")
	}
	// Garbage never panics.
	c.handle([]byte{0x12, 0x03, 0xff, 0xff, 0xff})
	c.handle([]byte{0xff})
}

// readyRadio connects a client to a fake radio holding this hub (0xdeadbeef)
// and one other radio (0xa1b2c3d4).
func readyRadio(t *testing.T) (*fakeRadio, *Client, *[]Event) {
	t.Helper()
	f := newFakeRadio(t)
	c := New(f.ln.Addr().String())
	var mu sync.Mutex
	evs := &[]Event{}
	c.OnEvent(func(e Event) { mu.Lock(); *evs = append(*evs, e); mu.Unlock() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go c.Run(ctx)
	<-f.got // want_config
	var my, myInfo enc
	my.varint(1, 0xdeadbeef)
	myInfo.bytes(frMyInfo, my.b)
	f.send(myInfo.b)
	f.send(nodeInfo(0xdeadbeef, userMsg("!deadbeef", "batnet hub 1", "BH1"), 0, 0))
	f.send(nodeInfo(0xa1b2c3d4, userMsg("!a1b2c3d4", "Anna's handheld", "ANNA"), 6.5, 1))
	var done enc
	done.varint(frConfigComplete, 1)
	f.send(done.b)
	waitFor(t, c, func(s Status) bool { return s.Ready && len(s.Nodes) == 2 })
	return f, c, evs
}

// routingPacket is a receipt from a radio about message id.
func routingPacket(from, id, errCode uint32) []byte {
	var rt enc
	if errCode != 0 {
		rt.varint(rtErrorReason, uint64(errCode))
	}
	var d enc
	d.varint(dPortnum, PortRouting)
	d.bytes(dPayload, rt.b)
	d.fixed32(dRequestID, id)
	var mp enc
	mp.fixed32(mpFrom, from)
	mp.fixed32(mpTo, 0xdeadbeef)
	mp.bytes(mpDecoded, d.b)
	var fr enc
	fr.bytes(frPacket, mp.b)
	return fr.b
}

func TestDirectMessagesAndReceipts(t *testing.T) {
	f, c, evs := readyRadio(t)

	// A message to one radio, as a reply, asks for a receipt.
	sent, err := c.Send(0xa1b2c3d4, 0, "hi Anna", 77)
	if err != nil {
		t.Fatal(err)
	}
	p := sentPacket(t, f)
	if p.to != 0xa1b2c3d4 || string(p.payload) != "hi Anna" || p.replyID != 77 || p.id != sent.ID || sent.Status != "sending" {
		t.Fatalf("sent %+v, text %+v", p, sent)
	}
	if _, err := c.Send(0xdeadbeef, 0, "to myself", 0); err == nil {
		t.Error("a message to this hub itself was accepted")
	}

	// Anna's radio confirms it: delivered, and a later receipt from our own
	// radio does not undo that.
	f.send(routingPacket(0xa1b2c3d4, sent.ID, 0))
	waitFor(t, c, func(s Status) bool { return s.Texts[0].Status == "delivered" })
	f.send(routingPacket(0xdeadbeef, sent.ID, 5))
	time.Sleep(50 * time.Millisecond)
	if st := c.Status(); st.Texts[0].Status != "delivered" {
		t.Errorf("after a late receipt %+v", st.Texts[0])
	}

	// A broadcast passed on by a radio nearby; another that failed.
	c.mu.Lock()
	c.lastSend = time.Time{}
	c.mu.Unlock()
	pub, _ := c.Send(Broadcast, 0, "hello all", 0)
	sentPacket(t, f)
	f.send(routingPacket(0xdeadbeef, pub.ID, 0))
	waitFor(t, c, func(s Status) bool { return s.Texts[1].Status == "passed on" })
	c.mu.Lock()
	c.lastSend = time.Time{}
	c.mu.Unlock()
	lost, _ := c.Send(0xa1b2c3d4, 0, "are you there?", 0)
	sentPacket(t, f)
	f.send(routingPacket(0xdeadbeef, lost.ID, 5))
	st := waitFor(t, c, func(s Status) bool { return s.Texts[2].Status == "failed" })
	if st.Texts[2].Detail != "no answer after three tries" {
		t.Errorf("failed %+v", st.Texts[2])
	}

	// An answer to this hub alone, end to end encrypted, relayed.
	var d enc
	d.varint(dPortnum, PortText)
	d.bytes(dPayload, []byte("yes, hi"))
	d.fixed32(dReplyID, lost.ID)
	var mp enc
	mp.fixed32(mpFrom, 0xa1b2c3d4)
	mp.fixed32(mpTo, 0xdeadbeef)
	mp.fixed32(mpID, 4242)
	mp.bytes(mpDecoded, d.b)
	mp.varint(mpPKI, 1)
	mp.varint(mpRelay, 0x1c)
	var fr enc
	fr.bytes(frPacket, mp.b)
	f.send(fr.b)
	st = waitFor(t, c, func(s Status) bool { return len(s.Texts) == 4 })
	if in := st.Texts[3]; in.To != 0xdeadbeef || in.ReplyID != lost.ID || !in.PKI || in.ID != 4242 || in.Mine {
		t.Errorf("answer %+v", in)
	}

	var kinds []string
	var relay string
	waitFor(t, c, func(Status) bool { return len(*evs) >= 12 })
	for _, e := range *evs {
		kinds = append(kinds, e.Kind)
		if e.Kind == "packet" && e.Text == "yes, hi" {
			relay = e.Relay
		}
	}
	if relay != "1c" || !strings.Contains(strings.Join(kinds, " "), "sent packet receipt") {
		t.Errorf("events %v, relay %q", kinds, relay)
	}
}

func TestLinksHeardStraight(t *testing.T) {
	c := New("127.0.0.1:1")
	c.st.MyNum = 0xdeadbeef
	c.node(0xa1b2c3d4)
	c.node(0x111111d4) // ends in the same byte as Anna's radio
	c.node(0x2222221c)
	c.mu.Lock()
	// Straight from Anna: three packets, the median is the middle one.
	for _, snr := range []float32{-12, -4, -7} {
		c.packet(packet{from: 0xa1b2c3d4, to: Broadcast, snr: snr, hopStart: 3, hopLimit: 3})
	}
	// Relayed by the only radio ending in 1c: that radio's signal.
	c.packet(packet{from: 0xa1b2c3d4, to: Broadcast, snr: -9, hopStart: 3, hopLimit: 2, relay: 0x1c})
	// Relayed by a radio ending in d4: two are known, Anna's was heard straight.
	c.packet(packet{from: 0x2222221c, to: Broadcast, snr: -2, hopStart: 7, hopLimit: 5, relay: 0xd4})
	// Unknown relay byte and our own packets count for no one.
	c.packet(packet{from: 0xa1b2c3d4, to: Broadcast, snr: -1, hopStart: 3, hopLimit: 1, relay: 0x77})
	c.packet(packet{from: 0xdeadbeef, to: Broadcast, snr: 5, hopStart: 3, hopLimit: 3})
	c.mu.Unlock()
	links := map[uint32]*Link{}
	for _, n := range c.Status().Nodes {
		links[n.Num] = n.Link
	}
	if l := links[0xa1b2c3d4]; l == nil || l.Count != 4 || l.SNR != -4 {
		t.Errorf("anna %+v", l)
	}
	if l := links[0x2222221c]; l == nil || l.Count != 1 || l.SNR != -9 {
		t.Errorf("relay 1c %+v", l)
	}
	if links[0x111111d4] != nil || links[0xdeadbeef] != nil {
		t.Errorf("links for radios not heard straight: %+v %+v", links[0x111111d4], links[0xdeadbeef])
	}
}

func TestLinksKeptBetweenRuns(t *testing.T) {
	a := New("127.0.0.1:1")
	a.node(0xa1b2c3d4)
	a.mu.Lock()
	a.packet(packet{from: 0xa1b2c3d4, to: Broadcast, snr: -8, hopStart: 3, hopLimit: 3})
	a.mu.Unlock()
	b := New("127.0.0.1:1")
	b.LoadLinks(a.SavedLinks())
	b.LoadLinks(map[string]SavedLink{"nonsense": {SNRs: []float32{1}}, "!deadbeef": {}})
	b.node(0xa1b2c3d4)
	st := b.Status()
	if len(st.Nodes) != 1 || st.Nodes[0].Link == nil || st.Nodes[0].Link.SNR != -8 || st.Nodes[0].Link.Count != 1 {
		t.Errorf("after loading %+v", st.Nodes)
	}
}

// Readings age out after linkKeep, and ForgetLinks starts the zone again (B68).
func TestLinksAgeAndForget(t *testing.T) {
	now := time.Date(2026, 10, 23, 14, 0, 0, 0, time.UTC)
	c := New("127.0.0.1:1")
	c.now = func() time.Time { return now }
	c.LoadLinks(map[string]SavedLink{
		"!a1b2c3d4": {SNRs: []float32{-8}, Count: 1, At: now.Add(-time.Hour)},
		"!deadbeef": {SNRs: []float32{-9}, Count: 1, At: now.Add(-linkKeep - time.Hour)},
	})
	c.node(0xa1b2c3d4)
	c.node(0xdeadbeef)
	got := map[string]bool{}
	for _, n := range c.Status().Nodes {
		got[n.ID] = n.Link != nil
	}
	if !got["!a1b2c3d4"] || got["!deadbeef"] {
		t.Errorf("links shown %v, want only the recent one", got)
	}
	if s := c.SavedLinks(); len(s) != 1 {
		t.Errorf("saved %d links, want the recent one only", len(s))
	}
	old := c.ForgetLinks()
	if len(old) != 1 || len(c.SavedLinks()) != 0 {
		t.Errorf("forgot %d, kept %d", len(old), len(c.SavedLinks()))
	}
}

// The primary channel with no name of its own is named by the preset, even
// when the radio sends the channels before the LoRa settings.
func TestPrimaryChannelNamedByPreset(t *testing.T) {
	c := New("127.0.0.1:1")
	c.channels[0] = Channel{Index: 0, Role: "primary"}
	c.channels[1] = Channel{Index: 1, Name: "BerlinMesh", Role: "secondary"}
	c.st.Preset = "MediumFast"
	if ch := c.Status().Channels; len(ch) != 2 || ch[0].Name != "MediumFast" || ch[1].Name != "BerlinMesh" {
		t.Errorf("channels %+v", ch)
	}
}

// The messages are kept across restarts (L16): saved, loaded before any heard
// since, and bounded.
func TestTextsKeptBetweenRuns(t *testing.T) {
	a := New("127.0.0.1:1")
	a.mu.Lock()
	a.addText(Text{At: time.Unix(100, 0), From: 7, To: Broadcast, Text: "hello"})
	a.addText(Text{At: time.Unix(200, 0), From: 8, To: Broadcast, Channel: 1, Text: "on the group channel"})
	a.mu.Unlock()
	b := New("127.0.0.1:1")
	b.mu.Lock()
	b.addText(Text{At: time.Unix(300, 0), From: 9, To: Broadcast, Text: "heard since"})
	b.mu.Unlock()
	b.LoadTexts(a.SavedTexts())
	got := b.Status().Texts
	if len(got) != 3 || got[0].Text != "hello" || got[1].Channel != 1 || got[2].Text != "heard since" {
		t.Errorf("after loading %+v", got)
	}
	many := make([]Text, keepTexts+50)
	for i := range many {
		many[i] = Text{At: time.Unix(int64(i), 0), Text: "x"}
	}
	b.LoadTexts(many)
	if n := len(b.Status().Texts); n != keepTexts {
		t.Errorf("%d messages kept, want %d", n, keepTexts)
	}
}

// A radio's names are its own claim: too long or not text, they are kept out.
// The hub knows which channels it has; the tables of radios and links are bounded.
func TestNamesChannelsAndBounds(t *testing.T) {
	f, c, _ := readyRadio(t)
	f.send(nodeInfo(0xa1b2c3d4, userMsg("!a1b2c3d4", strings.Repeat("x", MaxLongName+1), "\xff\xfe"), 6.5, 1))
	f.send(nodeInfo(0x0badf00d, userMsg("!00000000", "Fine name", "FN"), 2, 1))
	st := waitFor(t, c, func(s Status) bool { return len(s.Nodes) == 3 })
	for _, n := range st.Nodes {
		if n.Num == 0xa1b2c3d4 && (n.LongName != "Anna's handheld" || n.ShortName != "ANNA") {
			t.Fatalf("an over-long or non-text name replaced the old one: %q %q", n.LongName, n.ShortName)
		}
	}
	var chs, ch, chFr enc
	chs.bytes(3, []byte("BerlinMesh"))
	ch.varint(1, 1)
	ch.bytes(2, chs.b)
	ch.varint(3, 2) // secondary
	chFr.bytes(frChannel, ch.b)
	f.send(chFr.b)
	waitFor(t, c, func(s Status) bool { return c.HasChannel(1) })
	if c.HasChannel(2) || c.HasChannel(-1) {
		t.Fatal("a channel the radio has not got")
	}
	for i := uint32(0); i < maxNodes+20; i++ {
		f.send(textPacket(0x10000000+i, "spam", 1, 3, 2))
	}
	waitFor(t, c, func(s Status) bool { return len(s.Nodes) == maxNodes })
	c.mu.Lock()
	n, l := len(c.nodes), len(c.links)
	c.mu.Unlock()
	if n != maxNodes || l > maxLinks {
		t.Fatalf("%d radios and %d links kept, want at most %d and %d", n, l, maxNodes, maxLinks)
	}
}

func TestNoKeyAsksForItAndSendsAgain(t *testing.T) {
	f, c, _ := readyRadio(t)
	if st := c.Status(); st.Nodes[1].Key {
		t.Fatal("a radio whose name card had no key counts as keyed")
	}
	sent, err := c.Send(0xa1b2c3d4, 0, "hallo Anna", 0)
	if err != nil {
		t.Fatal(err)
	}
	sentPacket(t, f)

	// Our own radio refuses: no key for Anna. The hub asks her radio for its name card.
	f.send(routingPacket(0xdeadbeef, sent.ID, ErrNoKey))
	st := waitFor(t, c, func(s Status) bool { return s.Texts[0].Status == "failed" })
	if tx := st.Texts[0]; tx.Code != ErrNoKey || !tx.KeyAsked || tx.Detail != "no key for that radio yet" {
		t.Errorf("failed %+v", tx)
	}
	ask := sentPacket(t, f)
	if ask.to != 0xa1b2c3d4 || ask.port != PortNodeInfo || ask.payload == nil {
		t.Errorf("asked %+v", ask)
	}

	// Her name card comes with a key: the message goes again under a new ID.
	u := userMsg("!a1b2c3d4", "Anna's handheld", "ANNA")
	var k enc
	k.bytes(8, make([]byte, 32))
	var d enc
	d.varint(dPortnum, PortNodeInfo)
	d.bytes(dPayload, append(u, k.b...))
	var mp enc
	mp.fixed32(mpFrom, 0xa1b2c3d4)
	mp.fixed32(mpTo, 0xdeadbeef)
	mp.bytes(mpDecoded, d.b)
	var fr enc
	fr.bytes(frPacket, mp.b)
	f.send(fr.b)
	again := sentPacket(t, f)
	if again.to != 0xa1b2c3d4 || string(again.payload) != "hallo Anna" || again.id == sent.ID {
		t.Errorf("sent again %+v", again)
	}
	st = waitFor(t, c, func(s Status) bool { return s.Texts[0].ID == again.id })
	if tx := st.Texts[0]; tx.Status != "sending" || tx.Code != 0 || !st.Nodes[1].Key || len(st.Texts) != 1 {
		t.Errorf("after the key %+v, node %+v", tx, st.Nodes[1])
	}
	f.send(routingPacket(0xa1b2c3d4, again.id, 0))
	waitFor(t, c, func(s Status) bool { return s.Texts[0].Status == "delivered" })
}

func TestGarbledEchoCountsAsPassedOn(t *testing.T) {
	f, c, _ := readyRadio(t)
	sent, err := c.Send(0xa1b2c3d4, 0, "hallo Anna", 0)
	if err != nil {
		t.Fatal(err)
	}
	sentPacket(t, f)

	// Garbled packets that are not our message's echo change nothing.
	c.Echo(Unreadable{ID: sent.ID, From: "!a1b2c3d4", To: "!deadbeef"}) // another radio's packet with the same ID
	c.Echo(Unreadable{ID: sent.ID + 1, From: "!deadbeef", To: "!a1b2c3d4"})
	if tx := c.Status().Texts[0]; tx.Status != "sending" || tx.Echoed {
		t.Fatalf("after unrelated garbled packets %+v", tx)
	}

	// A neighbour passes it on; we hear that echo, too garbled for the radio to count it.
	c.Echo(Unreadable{ID: sent.ID, From: "!deadbeef", To: "!a1b2c3d4", Error: -7})
	st := waitFor(t, c, func(s Status) bool { return s.Texts[0].Status == "passed on" })
	if tx := st.Texts[0]; !tx.Echoed || tx.Code != 0 || tx.Detail != "" {
		t.Errorf("after the echo %+v", tx)
	}

	// The radio, which never counted the echo, gives up after three tries: it stays passed on.
	in := st.PacketsIn
	f.send(routingPacket(0xdeadbeef, sent.ID, ErrMaxRetransmit))
	st = waitFor(t, c, func(s Status) bool { return s.PacketsIn > in })
	if tx := st.Texts[0]; tx.Status != "passed on" || tx.Code != 0 {
		t.Errorf("after the radio gave up %+v", tx)
	}

	// The radio it was for confirms after all: delivered.
	f.send(routingPacket(0xa1b2c3d4, sent.ID, 0))
	waitFor(t, c, func(s Status) bool { return s.Texts[0].Status == "delivered" })
}

func TestGarbledEchoAfterTheRadioGaveUp(t *testing.T) {
	f, c, _ := readyRadio(t)
	sent, err := c.Send(0xa1b2c3d4, 0, "hallo Anna", 0)
	if err != nil {
		t.Fatal(err)
	}
	sentPacket(t, f)
	f.send(routingPacket(0xdeadbeef, sent.ID, ErrMaxRetransmit))
	waitFor(t, c, func(s Status) bool { return s.Texts[0].Status == "failed" })
	c.Echo(Unreadable{ID: sent.ID, From: "!deadbeef", To: "!a1b2c3d4"})
	if tx := c.Status().Texts[0]; tx.Status != "passed on" || tx.Code != 0 {
		t.Errorf("an echo logged after the radio gave up %+v", tx)
	}
}

// A radio's hops are those of the copy of its packet that came the shortest
// way, even when only a copy that took longer could be read (B76).
func TestHopsFromTheClosestCopy(t *testing.T) {
	c := New("127.0.0.1:1")
	c.node(0xa1b2c3d4)
	hops := func() Node {
		for _, n := range c.Status().Nodes {
			if n.Num == 0xa1b2c3d4 {
				return n
			}
		}
		t.Fatal("no node")
		return Node{}
	}

	// Copies after 2 and 4 relays, too garbled to read, then one read after 6.
	c.Echo(Unreadable{ID: 0x5b72d, From: "!a1b2c3d4", To: "everyone", Hops: 2, HopStart: 7, SNR: -15.75, RSSI: -126})
	c.Echo(Unreadable{ID: 0x5b72d, From: "!a1b2c3d4", To: "everyone", Hops: 4, HopStart: 7, SNR: -7.5, RSSI: -118})
	c.mu.Lock()
	c.packet(packet{from: 0xa1b2c3d4, to: Broadcast, id: 0x5b72d, snr: -12.25, rssi: -123, hopStart: 7, hopLimit: 1, relay: 0x07})
	c.mu.Unlock()
	if n := hops(); n.Hops != 2 || n.SNR != -15.75 || n.RSSI != -126 || !n.Garbled {
		t.Errorf("garbled copies first: %+v", n)
	}

	// The next packet is read after 3 relays; a garbled copy straight from the radio comes later.
	c.mu.Lock()
	c.packet(packet{from: 0xa1b2c3d4, to: Broadcast, id: 0x99, snr: -9, rssi: -120, hopStart: 7, hopLimit: 4})
	c.mu.Unlock()
	if n := hops(); n.Hops != 3 || n.Garbled {
		t.Errorf("read after 3: %+v", n)
	}
	c.Echo(Unreadable{ID: 0x99, From: "!a1b2c3d4", Hops: 0, HopStart: 7, SNR: -17, RSSI: -127})
	if n := hops(); n.Hops != 0 || n.SNR != -17 || !n.Garbled {
		t.Errorf("garbled copy afterwards: %+v", n)
	}

	// Copies that do not fit change nothing: another packet ID, another hop start
	// (a garbled header), more hops, nonsense, a radio never read.
	c.mu.Lock()
	c.packet(packet{from: 0xa1b2c3d4, to: Broadcast, id: 0x100, snr: -9, rssi: -120, hopStart: 7, hopLimit: 4})
	c.mu.Unlock()
	c.Echo(Unreadable{ID: 0x101, From: "!a1b2c3d4", Hops: 1, HopStart: 7})
	c.Echo(Unreadable{ID: 0x100, From: "!a1b2c3d4", Hops: 1, HopStart: 3})
	c.Echo(Unreadable{ID: 0x100, From: "!a1b2c3d4", Hops: 5, HopStart: 7})
	c.Echo(Unreadable{ID: 0x100, From: "!a1b2c3d4", Hops: -2, HopStart: 7})
	c.Echo(Unreadable{ID: 0x100, From: "!deadbeef", Hops: 0, HopStart: 7})
	if n := hops(); n.Hops != 3 || n.Garbled {
		t.Errorf("after copies that do not fit: %+v", n)
	}
	for _, n := range c.Status().Nodes {
		if n.Num == 0xdeadbeef {
			t.Errorf("a radio made up from a garbled header: %+v", n)
		}
	}
}

// The SOS channel is listen-only: no text by accident (Send, a reply, a
// message to one radio), one on purpose per OnPurposeGap.
func TestSOSChannelIsListenOnly(t *testing.T) {
	f := newFakeRadio(t)
	c := New(f.ln.Addr().String())
	now := time.Date(2026, 10, 25, 18, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	c.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	later := func(d time.Duration) { mu.Lock(); now = now.Add(d); mu.Unlock() }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)
	<-f.got
	var my, myInfo enc
	my.varint(1, 0xdeadbeef)
	myInfo.bytes(frMyInfo, my.b)
	f.send(myInfo.b)
	for _, x := range []struct {
		index uint64
		name  string
		role  uint64
	}{{0, "", 1}, {1, "BerlinMesh", 2}, {2, "SOS", 2}} {
		var chs, ch, fr enc
		chs.bytes(3, []byte(x.name))
		ch.varint(1, x.index)
		ch.bytes(2, chs.b)
		ch.varint(3, x.role)
		fr.bytes(frChannel, ch.b)
		f.send(fr.b)
	}
	var done enc
	done.varint(frConfigComplete, 1)
	f.send(done.b)
	st := waitFor(t, c, func(s Status) bool { return s.Ready && len(s.Channels) == 3 })
	if st.Channels[1].ListenOnly || !st.Channels[2].ListenOnly {
		t.Errorf("channels %+v", st.Channels)
	}
	if name, ok := c.ListenOnlyChannel(2); !ok || name != "SOS" {
		t.Errorf("ListenOnlyChannel(2) = %q %v", name, ok)
	}
	if err := c.SendText("oops", 2); err != ErrListenOnly {
		t.Errorf("by accident: %v", err)
	}
	if _, err := c.Send(Broadcast, 2, "a reply", 77); err != ErrListenOnly {
		t.Errorf("a reply: %v", err)
	}
	if _, err := c.SendOnPurpose(1, "not SOS"); err == nil {
		t.Error("on purpose on an ordinary channel went out")
	}
	if _, err := c.SendOnPurpose(2, "Need help at the corner"); err != nil {
		t.Fatal(err)
	}
	if p := sentPacket(t, f); p.channel != 2 || p.to != Broadcast || string(p.payload) != "Need help at the corner" {
		t.Errorf("sent %+v", p)
	}
	later(time.Minute)
	if _, err := c.SendOnPurpose(2, "again"); err != ErrTooSoonOnPurpose {
		t.Errorf("a second within the gap: %v", err)
	}
	if err := c.SendText("ordinary channels are not held up", 1); err != nil {
		t.Errorf("BerlinMesh: %v", err)
	}
	sentPacket(t, f)
	later(OnPurposeGap)
	if _, err := c.SendOnPurpose(2, "again"); err != nil {
		t.Errorf("after the gap: %v", err)
	}
}
