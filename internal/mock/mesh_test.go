package mock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/meshsync"
)

var _ meshsync.Link = (*Link)(nil)

func recv(t *testing.T, l *Link) (meshsync.Packet, bool) {
	t.Helper()
	select {
	case p := <-l.Inbox():
		return p, true
	case <-time.After(50 * time.Millisecond):
		return meshsync.Packet{}, false
	}
}

func TestBroadcastFloodsTheComponent(t *testing.T) {
	m := NewMesh(1)
	a, b, c, d := m.Add("a"), m.Add("b"), m.Add("c"), m.Add("d")
	m.Connect("a", "b")
	m.Connect("b", "c") // d is on its own
	a.Broadcast([]byte("hi"))
	for _, l := range []*Link{b, c} {
		if p, ok := recv(t, l); !ok || p.From != "a" || string(p.Data) != "hi" {
			t.Fatalf("%s got %+v %v", l.Addr(), p, ok)
		}
	}
	if _, ok := recv(t, a); ok {
		t.Fatal("sender received its own broadcast")
	}
	if _, ok := recv(t, d); ok {
		t.Fatal("an unconnected node received the broadcast")
	}
}

func TestDownNodesNeitherSendNorReceive(t *testing.T) {
	m := NewMesh(1)
	a, b := m.Add("a"), m.Add("b")
	m.Connect("a", "b")
	m.SetUp("b", false)
	a.Broadcast([]byte("x"))
	if _, ok := recv(t, b); ok {
		t.Fatal("a node without power received a packet")
	}
	b.Broadcast([]byte("y"))
	if _, ok := recv(t, a); ok {
		t.Fatal("a node without power sent a packet")
	}
}

func TestRequest(t *testing.T) {
	m := NewMesh(1)
	a, b := m.Add("a"), m.Add("b")
	b.SetHandler(func(from string, req []byte) []byte { return append([]byte(from+":"), req...) })
	if _, err := a.Request(context.Background(), "b", []byte("q")); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("before connecting: %v", err)
	}
	m.Connect("a", "b")
	got, err := a.Request(context.Background(), "b", []byte("q"))
	if err != nil || string(got) != "a:q" {
		t.Fatalf("got %q, %v", got, err)
	}
	m.Disconnect("a", "b")
	if _, err := a.Request(context.Background(), "b", nil); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("after disconnecting: %v", err)
	}
}

func TestLoss(t *testing.T) {
	m := NewMesh(7)
	a, b := m.Add("a"), m.Add("b")
	m.Connect("a", "b")
	m.SetLoss(1)
	a.Broadcast([]byte("x"))
	if _, ok := recv(t, b); ok {
		t.Fatal("packet arrived at 100% loss")
	}
}

func TestBroadcastLocalOneHop(t *testing.T) {
	m := NewMesh(1)
	a, b, c := m.Add("a"), m.Add("b"), m.Add("c")
	m.Connect("a", "b")
	m.Connect("b", "c")
	a.BroadcastLocal([]byte("beat"))
	if _, ok := recv(t, b); !ok {
		t.Fatal("neighbour b missed a one-hop broadcast")
	}
	if _, ok := recv(t, c); ok {
		t.Fatal("c, two hops away, got a one-hop broadcast")
	}
}
