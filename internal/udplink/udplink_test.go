package udplink

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/meshsync"
	"github.com/johnusher/batnet/internal/seal"
)

func freePort(t *testing.T, network string) int {
	t.Helper()
	if network == "udp" {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		return c.LocalAddr().(*net.UDPAddr).Port
	}
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// pair opens two links on loopback, each "broadcasting" to the other's port.
// Distinct addresses (127.0.0.2 and .3) need Linux; elsewhere both are 127.0.0.1.
func pair(t *testing.T, distinct bool) (*Link, *Link) {
	t.Helper()
	ipa, ipb := "127.0.0.1", "127.0.0.1"
	if distinct {
		ipa, ipb = "127.0.0.2", "127.0.0.3"
		l, err := net.Listen("tcp4", ipb+":0")
		if err != nil {
			t.Skip("needs 127.0.0.2 and 127.0.0.3 on loopback (Linux):", err)
		}
		l.Close()
	}
	ua, ub, ha, hb := freePort(t, "udp"), freePort(t, "udp"), freePort(t, "tcp"), freePort(t, "tcp")
	a, err := Open(Config{Addr: ipa, BroadcastTo: fmt.Sprintf("%s:%d", ipb, ub), UDPPort: ua, HTTPPort: ha, PeerHTTPPort: hb})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(Config{Addr: ipb, BroadcastTo: fmt.Sprintf("%s:%d", ipa, ua), UDPPort: ub, HTTPPort: hb, PeerHTTPPort: ha})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close(); b.Close() })
	return a, b
}

func TestBroadcastAndRequest(t *testing.T) {
	a, b := pair(t, false)
	if err := a.Broadcast([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-b.Inbox():
		if string(p.Data) != "hello" || p.From != "127.0.0.1" {
			t.Errorf("got %+v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast lost")
	}
	b.SetHandler(func(from string, req []byte) []byte { return append([]byte("answer to "), req...) })
	ans, err := a.Request(context.Background(), "127.0.0.1", []byte("q"))
	if err != nil || string(ans) != "answer to q" {
		t.Fatalf("request: %q %v", ans, err)
	}
	if err := a.Broadcast(make([]byte, maxPacket+1)); err == nil {
		t.Error("oversized packet sent")
	}
}

// Two real syncers over the link: an entry written at one arrives at the
// other by broadcast, and one written before the link was up is repaired by
// request after a heartbeat. (Syncers ignore heartbeats from their own address,
// so this needs two loopback addresses.)
func TestSyncOverLink(t *testing.T) {
	la, lb := pair(t, true)
	key, _ := seal.NewVillageKey()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type node struct {
		store  *ledger.MemStore
		author *ledger.Author
		sync   *meshsync.Syncer
	}
	mk := func(l *Link) *node {
		n := &node{store: ledger.NewMemStore()}
		_, priv, _ := ed25519.GenerateKey(nil)
		clock := hlc.New(func() int64 { return time.Now().UnixMilli() })
		n.author, _ = ledger.NewAuthor(priv, clock, n.store)
		sealer, _ := seal.NewSealer(key, seal.PurposeMesh)
		n.sync = meshsync.New(meshsync.Config{Link: l, Store: n.store, Clock: clock, Sealer: sealer, Heartbeat: 50 * time.Millisecond})
		return n
	}
	a, b := mk(la), mk(lb)
	a.author.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "written before the link was up"})
	go a.sync.Run(ctx)
	go b.sync.Run(ctx)
	e, _ := b.author.Append(ledger.TypeMsgPost, 5, map[string]string{"text": "live"})
	if err := b.sync.Publish(e); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ea, _ := a.store.All()
		eb, _ := b.store.All()
		if len(ea) == 2 && len(eb) == 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	ea, _ := a.store.All()
	eb, _ := b.store.All()
	t.Fatalf("a holds %d, b holds %d; want 2 each", len(ea), len(eb))
}
