// Package udplink is meshsync's Link over a real network: UDP broadcasts for
// new entries and heartbeats, HTTP for repair requests. On a Pi it runs on
// bat0, the batman-adv mesh interface, where every house is one hop away at
// layer 2 and broadcasts reach the whole mesh.
package udplink

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/meshsync"
)

const (
	inboxSize  = 4096
	maxPacket  = 65000
	maxRequest = 1 << 20
	maxAnswer  = 32 << 20
)

// Config says where the link listens and broadcasts.
type Config struct {
	// Addr is this node's IPv4 address on the mesh, e.g. 172.27.12.34.
	Addr string
	// BroadcastTo is where broadcasts go, e.g. 172.27.255.255:4200.
	BroadcastTo string
	// UDPPort and HTTPPort are where this node listens (4200 and 4201 on a Pi).
	UDPPort, HTTPPort int
	// PeerHTTPPort is where other nodes answer requests (HTTPPort if 0).
	PeerHTTPPort int
}

// Link is one node's connection to the mesh.
type Link struct {
	cfg     Config
	udp     *net.UDPConn // receives broadcasts
	send    *net.UDPConn // sends them, from Addr so receivers see who sent
	bcast   *net.UDPAddr
	srv     *http.Server
	client  *http.Client
	inbox   chan meshsync.Packet
	mu      sync.Mutex
	handler func(from string, req []byte) []byte
	done    chan struct{}
}

// Open starts listening.
func Open(cfg Config) (*Link, error) {
	if cfg.PeerHTTPPort == 0 {
		cfg.PeerHTTPPort = cfg.HTTPPort
	}
	bcast, err := net.ResolveUDPAddr("udp4", cfg.BroadcastTo)
	if err != nil {
		return nil, err
	}
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{Port: cfg.UDPPort})
	if err != nil {
		return nil, err
	}
	send, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(cfg.Addr)})
	if err != nil {
		udp.Close()
		return nil, err
	}
	ln, err := net.Listen("tcp4", net.JoinHostPort(cfg.Addr, strconv.Itoa(cfg.HTTPPort)))
	if err != nil {
		udp.Close()
		send.Close()
		return nil, err
	}
	l := &Link{cfg: cfg, udp: udp, send: send, bcast: bcast, inbox: make(chan meshsync.Packet, inboxSize), done: make(chan struct{}),
		client: &http.Client{Timeout: 20 * time.Second}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /meshsync", l.serve)
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go l.srv.Serve(ln)
	go l.read()
	return l, nil
}

// Close stops the link.
func (l *Link) Close() error {
	close(l.done)
	l.srv.Close()
	l.send.Close()
	return l.udp.Close()
}

func (l *Link) read() {
	self := l.send.LocalAddr().(*net.UDPAddr)
	buf := make([]byte, maxPacket+1)
	for {
		n, from, err := l.udp.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-l.done:
				return
			default:
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		addr := from.IP.String()
		if (from.IP.Equal(self.IP) && from.Port == self.Port) || n > maxPacket {
			continue // our own broadcast coming back, or too big
		}
		select {
		case l.inbox <- meshsync.Packet{From: addr, Data: append([]byte(nil), buf[:n]...)}:
		default: // full: meshsync repairs what is dropped
		}
	}
}

func (l *Link) serve(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "bad address", http.StatusBadRequest)
		return
	}
	req, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequest))
	if err != nil {
		http.Error(w, "too big", http.StatusRequestEntityTooLarge)
		return
	}
	l.mu.Lock()
	h := l.handler
	l.mu.Unlock()
	if h == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(h(host, req))
}

// Addr is this node's mesh address.
func (l *Link) Addr() string { return l.cfg.Addr }

// Broadcast sends to every node on the mesh.
func (l *Link) Broadcast(pkt []byte) error {
	if len(pkt) > maxPacket {
		return fmt.Errorf("udplink: packet of %d bytes is too big", len(pkt))
	}
	_, err := l.send.WriteToUDP(pkt, l.bcast)
	return err
}

// BroadcastLocal is Broadcast: batman-adv floods every broadcast through the
// whole mesh, so one-hop-only heartbeats (decision 0021) need batman's
// neighbour list; until then they reach everyone (known problem B34).
func (l *Link) BroadcastLocal(pkt []byte) error { return l.Broadcast(pkt) }

// Request asks one node and waits for its answer.
func (l *Link) Request(ctx context.Context, to string, req []byte) ([]byte, error) {
	url := "http://" + net.JoinHostPort(to, strconv.Itoa(l.cfg.PeerHTTPPort)) + "/meshsync"
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(req))
	if err != nil {
		return nil, err
	}
	resp, err := l.client.Do(hr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("udplink: %s answered %s", to, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxAnswer))
}

// Inbox delivers broadcasts from other nodes.
func (l *Link) Inbox() <-chan meshsync.Packet { return l.inbox }

// SetHandler answers requests from other nodes.
func (l *Link) SetHandler(h func(from string, req []byte) []byte) {
	l.mu.Lock()
	l.handler = h
	l.mu.Unlock()
}
