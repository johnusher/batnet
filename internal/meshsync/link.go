// Package meshsync keeps the log in step across the WiFi mesh: it broadcasts
// new entries, sends heartbeats, and repairs differences with version vectors.
package meshsync

import "context"

// Packet is a broadcast received from another node.
type Packet struct {
	From string
	Data []byte
}

// Link is how a node reaches the mesh: UDP and HTTP on bat0 on a Pi
// (plan 4), an in-process mesh in tests and the mock village.
type Link interface {
	Addr() string
	// Broadcast sends to every node reachable over the mesh; delivery is best-effort.
	Broadcast(pkt []byte) error
	// Request asks one node and waits for its answer.
	Request(ctx context.Context, to string, req []byte) ([]byte, error)
	Inbox() <-chan Packet
	// SetHandler answers requests from other nodes.
	SetHandler(h func(from string, req []byte) []byte)
}
