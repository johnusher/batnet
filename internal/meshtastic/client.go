package meshtastic

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	start1, start2 = 0x94, 0xc3
	maxFrame       = 512
	keepTexts      = 200
	heartbeatEvery = time.Minute
	redialAfter    = 5 * time.Second
	// MaxText is the most a Meshtastic text packet carries.
	MaxText = 200
	// SendGap and HelloGap keep a person at the panel from hogging the shared
	// channel; meshtasticd enforces the EU868 duty cycle on top.
	SendGap  = 5 * time.Second
	HelloGap = 5 * time.Minute
)

// ErrTooSoon means the last send was too recent.
var ErrTooSoon = errors.New("meshtastic: wait a few seconds before sending again")

// ErrOffline means meshtasticd is not connected.
var ErrOffline = errors.New("meshtastic: radio not connected")

// Node is one radio meshtasticd knows: itself or one it heard.
type Node struct {
	Num       uint32    `json:"num"`
	ID        string    `json:"id"`
	LongName  string    `json:"longName"`
	ShortName string    `json:"shortName"`
	Role      string    `json:"role"`
	HWModel   uint32    `json:"hwModel"`
	LastHeard time.Time `json:"lastHeard"`
	SNR       float32   `json:"snr"`
	RSSI      int32     `json:"rssi,omitempty"`
	Hops      int       `json:"hops"` // -1: unknown
	Battery   *uint32   `json:"battery,omitempty"`
	ChanUtil  *float32  `json:"chanUtil,omitempty"`
	AirUtilTx *float32  `json:"airUtilTx,omitempty"`
	Packets   int       `json:"packets"`
	// Batnet: sent batnet traffic (PRIVATE_APP) or names itself batnet.
	Batnet bool `json:"batnet"`
	Self   bool `json:"self"`
}

// Text is one text message on a channel this radio can read.
type Text struct {
	At      time.Time `json:"at"`
	From    uint32    `json:"from"`
	To      uint32    `json:"to"`
	Channel uint32    `json:"channel"`
	Text    string    `json:"text"`
	SNR     float32   `json:"snr"`
	RSSI    int32     `json:"rssi,omitempty"`
	Hops    int       `json:"hops"`
	Mine    bool      `json:"mine"`
}

// Channel is one of the radio's channels.
type Channel struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// Status is a copy of everything the client knows.
type Status struct {
	Addr       string    `json:"addr"`
	Connected  bool      `json:"connected"`
	Ready      bool      `json:"ready"` // the node list and settings have arrived
	Since      time.Time `json:"since"`
	LastError  string    `json:"lastError,omitempty"`
	MyNum      uint32    `json:"myNum"`
	Firmware   string    `json:"firmware"`
	Region     string    `json:"region"`
	Preset     string    `json:"preset"`
	TxPower    int32     `json:"txPower"`
	HopLimit   uint32    `json:"hopLimit"`
	TxEnabled  bool      `json:"txEnabled"`
	Channels   []Channel `json:"channels"`
	Nodes      []Node    `json:"nodes"`
	Texts      []Text    `json:"texts"`
	PacketsIn  int       `json:"packetsIn"`
	PacketsOut int       `json:"packetsOut"`
}

// Client keeps a connection to meshtasticd and a picture of the radio.
type Client struct {
	addr string
	now  func() time.Time

	mu        sync.Mutex
	conn      net.Conn
	st        Status
	nodes     map[uint32]*Node
	channels  map[int]Channel
	selfUser  []byte // our own User message, for "say hello"
	lastSend  time.Time
	lastHello time.Time
	listeners map[chan struct{}]bool
}

// New returns a client for meshtasticd at addr (host:port, usually 127.0.0.1:4403).
func New(addr string) *Client {
	return &Client{addr: addr, now: time.Now, nodes: map[uint32]*Node{}, channels: map[int]Channel{},
		listeners: map[chan struct{}]bool{}, st: Status{Addr: addr}}
}

// Changed returns a channel that gets a signal when something changes, and a function to stop.
func (c *Client) Changed() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	c.mu.Lock()
	c.listeners[ch] = true
	c.mu.Unlock()
	return ch, func() {
		c.mu.Lock()
		delete(c.listeners, ch)
		c.mu.Unlock()
	}
}

func (c *Client) changed() { // caller holds mu
	for ch := range c.listeners {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Run connects, and reconnects after every failure, until ctx ends.
func (c *Client) Run(ctx context.Context) {
	for ctx.Err() == nil {
		err := c.session(ctx)
		c.mu.Lock()
		c.conn = nil
		c.st.Connected, c.st.Ready = false, false
		if err != nil && ctx.Err() == nil {
			c.st.LastError = err.Error()
		}
		c.changed()
		c.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-time.After(redialAfter):
		}
	}
}

func (c *Client) session(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	c.mu.Lock()
	c.conn = conn
	c.st.Connected, c.st.Since, c.st.LastError = true, c.now(), ""
	c.changed()
	c.mu.Unlock()

	var want enc
	want.varint(trWantConfigID, uint64(rand.Uint32()))
	if err := c.write(want.b); err != nil {
		return err
	}
	beat := time.NewTicker(heartbeatEvery)
	defer beat.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-beat.C:
				var hb enc
				hb.bytes(trHeartbeat, nil)
				if c.write(hb.b) != nil {
					conn.Close()
					return
				}
			}
		}
	}()
	r := bufio.NewReader(conn)
	for {
		frame, err := readFrame(r)
		if err != nil {
			return err
		}
		c.handle(frame)
	}
}

// readFrame finds the next 0x94 0xc3 header and returns the protobuf after it.
// meshtasticd also writes plain log text on the same stream; it is skipped.
func readFrame(r *bufio.Reader) ([]byte, error) {
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != start1 {
			continue
		}
		if b, err = r.ReadByte(); err != nil {
			return nil, err
		}
		if b != start2 {
			r.UnreadByte()
			continue
		}
		var hdr [2]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return nil, err
		}
		n := int(binary.BigEndian.Uint16(hdr[:]))
		if n > maxFrame {
			continue // a false start inside other bytes
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf, nil
	}
}

func frame(pb []byte) []byte {
	return append([]byte{start1, start2, byte(len(pb) >> 8), byte(len(pb))}, pb...)
}

func (c *Client) write(pb []byte) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return ErrOffline
	}
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := conn.Write(frame(pb))
	return err
}

// handle folds one FromRadio message in.
func (c *Client) handle(b []byte) {
	fs, err := fields(b)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.changed()
	for _, f := range fs {
		switch f.num {
		case frMyInfo:
			sub, _ := fields(f.b)
			for _, s := range sub {
				if s.num == 1 {
					c.st.MyNum = s.u32()
				}
			}
		case frNodeInfo:
			c.nodeInfo(f.b)
		case frConfig:
			c.config(f.b)
		case frChannel:
			c.channel(f.b)
		case frMetadata:
			sub, _ := fields(f.b)
			for _, s := range sub {
				if s.num == 1 {
					c.st.Firmware = string(s.b)
				}
			}
		case frConfigComplete:
			c.st.Ready = true
		case frRebooted:
			c.st.Ready = false
		case frPacket:
			if p, err := parsePacket(f.b); err == nil {
				c.packet(p)
			}
		}
	}
}

func (c *Client) node(num uint32) *Node {
	n := c.nodes[num]
	if n == nil {
		n = &Node{Num: num, ID: fmt.Sprintf("!%08x", num), Hops: -1}
		c.nodes[num] = n
	}
	return n
}

func (c *Client) setUser(n *Node, u user) {
	if u.long != "" {
		n.LongName = u.long
	}
	if u.short != "" {
		n.ShortName = u.short
	}
	n.HWModel, n.Role = u.hw, roles[u.role]
	if len(u.long) >= 6 && (u.long[:6] == "batnet" || u.long[:6] == "Batnet") {
		n.Batnet = true
	}
}

func (c *Client) nodeInfo(b []byte) {
	fs, err := fields(b)
	if err != nil {
		return
	}
	var n *Node
	for _, f := range fs {
		if f.num == 1 {
			n = c.node(f.u32())
		}
	}
	if n == nil {
		return
	}
	for _, f := range fs {
		switch f.num {
		case 2:
			c.setUser(n, parseUser(f.b))
			if n.Num == c.st.MyNum {
				c.selfUser = append([]byte(nil), f.b...)
			}
		case 4:
			n.SNR = f.f32()
		case 5:
			if f.u32() != 0 {
				n.LastHeard = time.Unix(int64(f.u32()), 0)
			}
		case 6:
			m := parseMetrics(f.b)
			n.Battery, n.ChanUtil, n.AirUtilTx = m.battery, m.chanUtil, m.airUtilTx
		case 9:
			n.Hops = int(f.u32())
		}
	}
}

func (c *Client) config(b []byte) {
	fs, _ := fields(b)
	for _, f := range fs {
		if f.num != 6 { // lora
			continue
		}
		c.st.Preset, c.st.TxEnabled = presets[0], false
		sub, _ := fields(f.b)
		for _, s := range sub {
			switch s.num {
			case 2:
				c.st.Preset = presets[s.u32()]
			case 7:
				c.st.Region = regions[s.u32()]
				if c.st.Region == "" {
					c.st.Region = fmt.Sprintf("region %d", s.u32())
				}
			case 8:
				c.st.HopLimit = s.u32()
			case 9:
				c.st.TxEnabled = s.v != 0
			case 10:
				c.st.TxPower = s.i32()
			}
		}
		if c.st.Region == "" {
			c.st.Region = regions[0]
		}
	}
}

func (c *Client) channel(b []byte) {
	fs, _ := fields(b)
	ch := Channel{}
	for _, f := range fs {
		switch f.num {
		case 1:
			ch.Index = int(f.i32())
		case 2:
			sub, _ := fields(f.b)
			for _, s := range sub {
				if s.num == 3 {
					ch.Name = string(s.b)
				}
			}
		case 3:
			ch.Role = map[uint32]string{0: "disabled", 1: "primary", 2: "secondary"}[f.u32()]
		}
	}
	if ch.Role == "" {
		ch.Role = "disabled"
	}
	if ch.Role == "primary" && ch.Name == "" {
		ch.Name = c.st.Preset
		if ch.Name == "" {
			ch.Name = "LongFast"
		}
	}
	c.channels[ch.Index] = ch
}

func (c *Client) packet(p packet) {
	c.st.PacketsIn++
	n := c.node(p.from)
	n.Packets++
	n.LastHeard = c.now()
	if p.rxTime != 0 {
		n.LastHeard = time.Unix(int64(p.rxTime), 0)
	}
	if p.snr != 0 || p.rssi != 0 {
		n.SNR, n.RSSI = p.snr, p.rssi
	}
	if h := p.hops(); h >= 0 {
		n.Hops = h
	}
	if !p.decoded {
		return
	}
	switch p.port {
	case PortText:
		t := string(p.payload)
		if !utf8.ValidString(t) {
			return
		}
		c.addText(Text{At: n.LastHeard, From: p.from, To: p.to, Channel: p.channel, Text: t, SNR: p.snr, RSSI: p.rssi, Hops: p.hops()})
	case PortNodeInfo:
		c.setUser(n, parseUser(p.payload))
	case PortTelemetry:
		fs, _ := fields(p.payload)
		for _, f := range fs {
			if f.num == 2 {
				m := parseMetrics(f.b)
				n.Battery, n.ChanUtil, n.AirUtilTx = m.battery, m.chanUtil, m.airUtilTx
			}
		}
	case PortPrivate:
		n.Batnet = true
	}
}

func (c *Client) addText(t Text) {
	c.st.Texts = append(c.st.Texts, t)
	if len(c.st.Texts) > keepTexts {
		c.st.Texts = c.st.Texts[len(c.st.Texts)-keepTexts:]
	}
}

// Status returns a copy of what the client knows; nodes most recently heard first.
func (c *Client) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.st
	st.Texts = append([]Text(nil), c.st.Texts...)
	st.Nodes = make([]Node, 0, len(c.nodes))
	for _, n := range c.nodes {
		cp := *n
		cp.Self = n.Num == c.st.MyNum
		st.Nodes = append(st.Nodes, cp)
	}
	sort.Slice(st.Nodes, func(i, j int) bool {
		a, b := st.Nodes[i], st.Nodes[j]
		if a.Self != b.Self {
			return a.Self
		}
		if !a.LastHeard.Equal(b.LastHeard) {
			return a.LastHeard.After(b.LastHeard)
		}
		return a.Num < b.Num
	})
	st.Channels = nil
	for _, ch := range c.channels {
		if ch.Role != "disabled" {
			st.Channels = append(st.Channels, ch)
		}
	}
	sort.Slice(st.Channels, func(i, j int) bool { return st.Channels[i].Index < st.Channels[j].Index })
	return st
}

// SendText sends text to everyone on a channel (0 is the primary).
func (c *Client) SendText(text string, channel uint32) error {
	if text == "" || len(text) > MaxText {
		return fmt.Errorf("meshtastic: text must be 1-%d bytes", MaxText)
	}
	c.mu.Lock()
	if c.conn == nil || !c.st.Ready {
		c.mu.Unlock()
		return ErrOffline
	}
	if c.now().Sub(c.lastSend) < SendGap {
		c.mu.Unlock()
		return ErrTooSoon
	}
	c.lastSend = c.now()
	hop := c.st.HopLimit
	c.mu.Unlock()
	if hop == 0 {
		hop = 3
	}
	if err := c.write(encodePacket(Broadcast, channel, rand.Uint32(), hop, PortText, []byte(text), false)); err != nil {
		return err
	}
	c.mu.Lock()
	c.st.PacketsOut++
	c.addText(Text{At: c.now(), From: c.st.MyNum, To: Broadcast, Channel: channel, Text: text, Hops: 0, Mine: true})
	c.changed()
	c.mu.Unlock()
	return nil
}

// SayHello broadcasts this node's name and asks the radios that hear it to
// answer with theirs: the quickest way to see who is out there.
func (c *Client) SayHello() error {
	c.mu.Lock()
	if c.conn == nil || !c.st.Ready || c.selfUser == nil {
		c.mu.Unlock()
		return ErrOffline
	}
	if !c.lastHello.IsZero() && c.now().Sub(c.lastHello) < HelloGap {
		c.mu.Unlock()
		return fmt.Errorf("meshtastic: one hello every %v; the last was %v ago", HelloGap, c.now().Sub(c.lastHello).Round(time.Second))
	}
	c.lastHello = c.now()
	u, hop := c.selfUser, c.st.HopLimit
	c.mu.Unlock()
	if hop == 0 {
		hop = 3
	}
	if err := c.write(encodePacket(Broadcast, 0, rand.Uint32(), hop, PortNodeInfo, u, true)); err != nil {
		return err
	}
	c.mu.Lock()
	c.st.PacketsOut++
	c.mu.Unlock()
	return nil
}
