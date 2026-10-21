package meshtastic

import (
	"bufio"
	"context"
	"math"
	"net"
	"strings"
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
	if err := c.SayHello(); err != nil {
		t.Fatal(err)
	}
	if p := sentPacket(t, f); p.port != PortNodeInfo || !strings.Contains(string(p.payload), "batnet hub 1") {
		t.Errorf("hello %+v", p)
	}
	if err := c.SayHello(); err == nil {
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
