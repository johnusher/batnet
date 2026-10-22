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
	"strconv"
	"strings"
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
	adminGap = 5 * time.Second
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
	Hardware  string    `json:"hardware,omitempty"`
	// Position is where the radio last said it was; nil: it never said.
	Position *NodePosition `json:"position,omitempty"`
	// Link is how strongly the radio's own signal reaches this hub: heard
	// straight, or as the last relay of someone else's packet. nil: never.
	Link *Link `json:"link,omitempty"`
	// Batnet: sent batnet traffic (PRIVATE_APP) or names itself batnet.
	Batnet bool `json:"batnet"`
	Self   bool `json:"self"`
}

// Link is a radio's signal straight to this hub.
type Link struct {
	SNR   float32   `json:"snr"`   // the median of the last few
	Count int       `json:"count"` // packets heard straight from it in all
	At    time.Time `json:"at"`
}

// keepLinkSNRs is how many recent straight SNRs a link's median is taken over.
const keepLinkSNRs = 15

type link struct {
	snrs  []float32
	count int
	at    time.Time
}

// NodePosition is a radio's last shared position.
type NodePosition struct {
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Alt       *int32    `json:"alt,omitempty"`
	AccuracyM float64   `json:"accuracyM,omitempty"` // 0: exact
	At        time.Time `json:"at"`
}

func (c *Client) setPosition(n *Node, p Position) {
	at := c.now()
	if p.Time != 0 {
		at = time.Unix(int64(p.Time), 0)
	}
	n.Position = &NodePosition{Lat: p.Lat, Lon: p.Lon, Alt: p.Alt, AccuracyM: p.AccuracyM, At: at}
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
	ID      uint32    `json:"id"`
	ReplyID uint32    `json:"replyId,omitempty"` // the message this one answers
	PKI     bool      `json:"pki,omitempty"`     // encrypted end to end for this hub
	// Status of a message this hub sent: sending, passed on (a radio nearby
	// relayed it), delivered (the radio it was for confirmed it) or failed.
	Status string `json:"status,omitempty"`
	Detail string `json:"detail,omitempty"` // why it failed
}

// Channel is one of the radio's channels.
type Channel struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// Event is one thing that happened on the radio, for a log: every packet
// received (decoded or not), and the connection coming and going.
type Event struct {
	At       time.Time `json:"t"`
	Kind     string    `json:"kind"` // packet, sent, receipt, connected, disconnected
	From     string    `json:"from,omitempty"`
	FromName string    `json:"fromName,omitempty"`
	To       string    `json:"to,omitempty"`
	Channel  uint32    `json:"channel"`
	ID       uint32    `json:"id,omitempty"`
	Port     string    `json:"port,omitempty"` // empty: encrypted for a channel we cannot read
	SNR      float32   `json:"snr"`
	RSSI     int32     `json:"rssi,omitempty"`
	Hops     int       `json:"hops"`
	HopStart uint32    `json:"hopStart,omitempty"`
	Text     string    `json:"text,omitempty"`
	Lat      *float64  `json:"lat,omitempty"` // a position the sender shared
	Lon      *float64  `json:"lon,omitempty"`
	Alt      *int32    `json:"alt,omitempty"`
	Bytes    int       `json:"bytes"`
	// Info is what the packet says, decoded for the panel (battery, names,
	// a route, neighbours), by port.
	Info    map[string]any `json:"info,omitempty"`
	Relay   string         `json:"relay,omitempty"` // the last byte of the radio that passed it on
	PKI     bool           `json:"pki,omitempty"`
	ReplyID uint32         `json:"replyId,omitempty"`
	Status  string         `json:"status,omitempty"` // for a receipt: what became of message ID
	// Local is a packet from this hub's own radio (its telemetry, a receipt),
	// not one heard on the air.
	Local bool   `json:"local,omitempty"`
	Error string `json:"error,omitempty"`
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
	links     map[uint32]*link
	channels  map[int]Channel
	selfUser  []byte // our own User message, for "say hello"
	lastSend  time.Time
	lastHello time.Time
	lastAdmin time.Time
	loraRaw   []byte // the radio's LoRaConfig as it sent it, for changing a few fields
	listeners map[chan struct{}]bool
	onEvent   []func(Event)
}

// OnEvent adds a function called for every Event (from the client's own
// goroutine; it must not block for long). Add them before Run.
func (c *Client) OnEvent(f func(Event)) {
	c.mu.Lock()
	c.onEvent = append(c.onEvent, f)
	c.mu.Unlock()
}

func (c *Client) emit(e Event) {
	c.mu.Lock()
	fs := c.onEvent
	c.mu.Unlock()
	for _, f := range fs {
		f(e)
	}
}

// New returns a client for meshtasticd at addr (host:port, usually 127.0.0.1:4403).
func New(addr string) *Client {
	return &Client{addr: addr, now: time.Now, nodes: map[uint32]*Node{}, links: map[uint32]*link{}, channels: map[int]Channel{},
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
		ev := Event{At: c.now(), Kind: "disconnected"}
		if err != nil {
			ev.Error = err.Error()
		}
		c.emit(ev)
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
	c.emit(Event{At: c.now(), Kind: "connected"})

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
				receipt := c.packet(p)
				evs := []Event{c.event(p, len(f.b))}
				if receipt != nil {
					evs = append(evs, *receipt)
				}
				c.mu.Unlock()
				for _, ev := range evs {
					c.emit(ev)
				}
				c.mu.Lock()
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

// SavedLink is a link as kept between runs, by radio ID ("!deadbeef").
type SavedLink struct {
	SNRs  []float32 `json:"snrs"`
	Count int       `json:"count"`
	At    time.Time `json:"at"`
}

// SavedLinks returns the links to keep between runs.
func (c *Client) SavedLinks() map[string]SavedLink {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := make(map[string]SavedLink, len(c.links))
	for num, l := range c.links {
		m[fmt.Sprintf("!%08x", num)] = SavedLink{SNRs: append([]float32(nil), l.snrs...), Count: l.count, At: l.at}
	}
	return m
}

// LoadLinks puts back links kept from an earlier run.
func (c *Client) LoadLinks(m map[string]SavedLink) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, sl := range m {
		num, err := strconv.ParseUint(strings.TrimPrefix(id, "!"), 16, 32)
		if err != nil || len(sl.SNRs) == 0 || uint32(num) == c.st.MyNum {
			continue
		}
		if len(sl.SNRs) > keepLinkSNRs {
			sl.SNRs = sl.SNRs[len(sl.SNRs)-keepLinkSNRs:]
		}
		c.links[uint32(num)] = &link{snrs: sl.SNRs, count: sl.Count, at: sl.At}
	}
}

// heardStraight notes a packet that came straight from the radio num.
func (c *Client) heardStraight(num uint32, snr float32) {
	l := c.links[num]
	if l == nil {
		l = &link{}
		c.links[num] = l
	}
	l.snrs = append(l.snrs, snr)
	if len(l.snrs) > keepLinkSNRs {
		l.snrs = l.snrs[1:]
	}
	l.count++
	l.at = c.now()
}

// relayer finds the radio a packet's last relay byte names: the one known
// radio ending in that byte, or else the one of them already heard straight.
// Two or more either way: not known (256 bytes, many radios).
func (c *Client) relayer(b uint32) (uint32, bool) {
	if b == 0 {
		return 0, false
	}
	var all, heard []uint32
	for num := range c.nodes {
		if num&0xff == b&0xff && num != c.st.MyNum {
			all = append(all, num)
			if c.links[num] != nil {
				heard = append(heard, num)
			}
		}
	}
	switch {
	case len(all) == 1:
		return all[0], true
	case len(heard) == 1:
		return heard[0], true
	}
	return 0, false
}

func (c *Client) setUser(n *Node, u user) {
	if u.long != "" {
		n.LongName = u.long
	}
	if u.short != "" {
		n.ShortName = u.short
	}
	n.HWModel, n.Role, n.Hardware = u.hw, roles[u.role], HardwareName(u.hw)
	if l := strings.ToLower(u.long); strings.HasPrefix(l, "batnet") || strings.HasPrefix(l, "lorabatnet") {
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
		case 3:
			if p, ok := parsePosition(f.b); ok {
				c.setPosition(n, p)
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
		c.loraRaw = append([]byte(nil), f.b...)
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

// packet folds a received packet in; for a receipt about a message this hub
// sent it returns the event saying what became of it. Caller holds mu.
func (c *Client) packet(p packet) *Event {
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
	if p.snr != 0 && p.from != c.st.MyNum {
		if h := p.hops(); h == 0 {
			c.heardStraight(p.from, p.snr)
		} else if num, ok := c.relayer(p.relay); h > 0 && ok {
			c.heardStraight(num, p.snr)
		}
	}
	if !p.decoded {
		return nil
	}
	switch p.port {
	case PortText:
		t := string(p.payload)
		if !utf8.ValidString(t) {
			return nil
		}
		c.addText(Text{At: n.LastHeard, From: p.from, To: p.to, Channel: p.channel, Text: t, SNR: p.snr, RSSI: p.rssi, Hops: p.hops(),
			ID: p.id, ReplyID: p.replyID, PKI: p.pki})
	case PortNodeInfo:
		c.setUser(n, parseUser(p.payload))
	case PortPosition:
		if pos, ok := parsePosition(p.payload); ok {
			c.setPosition(n, pos)
		}
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
	case PortRouting:
		if p.requestID != 0 {
			return c.receipt(p)
		}
	}
	return nil
}

// receipt applies a Routing packet that answers a message this hub sent: from
// the radio it was for, it was delivered; from this hub's own radio with no
// error, a radio nearby passed it on; with an error, it failed. A message
// already delivered stays delivered. Caller holds mu.
func (c *Client) receipt(p packet) *Event {
	for i := len(c.st.Texts) - 1; i >= 0; i-- {
		t := &c.st.Texts[i]
		if !t.Mine || t.ID != p.requestID {
			continue
		}
		if t.Status == "delivered" {
			return nil
		}
		switch code := routingError(p.payload); {
		case code != 0:
			t.Status, t.Detail = "failed", routingErrors[code]
			if t.Detail == "" {
				t.Detail = fmt.Sprintf("error %d", code)
			}
		case p.from == t.To:
			t.Status, t.Detail = "delivered", ""
		default:
			t.Status, t.Detail = "passed on", ""
		}
		return &Event{At: c.now(), Kind: "receipt", ID: t.ID, To: nodeID(t.To), Status: t.Status, Error: t.Detail, Local: p.from == c.st.MyNum}
	}
	return nil
}

func nodeID(n uint32) string {
	if n == Broadcast {
		return "everyone"
	}
	return fmt.Sprintf("!%08x", n)
}

var portNames = map[uint32]string{PortText: "text", PortPosition: "position", PortNodeInfo: "nodeinfo", PortRouting: "routing",
	PortTelemetry: "telemetry", PortPrivate: "batnet", 6: "admin", 7: "text-compressed", 32: "reply", 33: "ip-tunnel",
	34: "paxcounter", 64: "serial", 65: "store-forward", 66: "range-test", 70: "traceroute", 71: "neighborinfo", 73: "map-report"}

// event describes a received packet for the log. Caller holds mu.
func (c *Client) event(p packet, n int) Event {
	e := Event{At: c.now(), Kind: "packet", From: nodeID(p.from), To: nodeID(p.to), Channel: p.channel,
		ID: p.id, SNR: p.snr, RSSI: p.rssi, Hops: p.hops(), HopStart: p.hopStart, Bytes: n,
		PKI: p.pki, ReplyID: p.replyID, Local: p.from == c.st.MyNum}
	if p.relay != 0 {
		e.Relay = fmt.Sprintf("%02x", p.relay&0xff)
	}
	if nd := c.nodes[p.from]; nd != nil {
		e.FromName = nd.LongName
	}
	if !p.decoded {
		return e
	}
	e.Port = portNames[p.port]
	if e.Port == "" {
		e.Port = fmt.Sprintf("port %d", p.port)
	}
	switch p.port {
	case PortText:
		if utf8.Valid(p.payload) {
			e.Text = string(p.payload)
		}
	case PortPosition:
		if pos, ok := parsePosition(p.payload); ok {
			e.Lat, e.Lon, e.Alt = &pos.Lat, &pos.Lon, pos.Alt
		}
	}
	e.Info = info(p.port, p.payload)
	return e
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
		if l := c.links[n.Num]; l != nil {
			s := append([]float32(nil), l.snrs...)
			sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
			cp.Link = &Link{SNR: s[len(s)/2], Count: l.count, At: l.at}
		}
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
	_, err := c.Send(Broadcast, channel, text, 0)
	return err
}

// Send sends text to one radio (to) or to everyone (Broadcast) on a channel,
// optionally as a reply to message replyID, asking for a receipt. A message
// to one radio is encrypted end to end by meshtasticd once it knows that
// radio's key. It returns the message as the panel shows it.
func (c *Client) Send(to, channel uint32, text string, replyID uint32) (Text, error) {
	if text == "" || len(text) > MaxText {
		return Text{}, fmt.Errorf("meshtastic: text must be 1-%d bytes", MaxText)
	}
	c.mu.Lock()
	if c.conn == nil || !c.st.Ready {
		c.mu.Unlock()
		return Text{}, ErrOffline
	}
	if to == c.st.MyNum || to == 0 {
		c.mu.Unlock()
		return Text{}, errors.New("meshtastic: that is this hub")
	}
	if c.now().Sub(c.lastSend) < SendGap {
		c.mu.Unlock()
		return Text{}, ErrTooSoon
	}
	c.lastSend = c.now()
	hop := c.st.HopLimit
	c.mu.Unlock()
	if hop == 0 {
		hop = 3
	}
	id := rand.Uint32()
	pb := encodePacket(to, channel, id, hop, PortText, []byte(text), sendOpts{wantAck: true, replyID: replyID})
	if err := c.write(pb); err != nil {
		return Text{}, err
	}
	c.mu.Lock()
	c.st.PacketsOut++
	t := Text{At: c.now(), From: c.st.MyNum, To: to, Channel: channel, Text: text, Hops: 0, Mine: true, ID: id, ReplyID: replyID, Status: "sending"}
	c.addText(t)
	c.changed()
	c.mu.Unlock()
	c.emit(Event{At: t.At, Kind: "sent", To: nodeID(to), Channel: channel, ID: id, Port: "text", Text: text, ReplyID: replyID, Bytes: len(pb), Local: true})
	return t, nil
}

// SayHello broadcasts this node's name. With ask, it asks the radios that
// hear it to answer with theirs: the quickest way to see who is out there,
// but the answers come at once and collide, so a hub's regular hello only
// says its name.
func (c *Client) SayHello(ask bool) error {
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
	id := rand.Uint32()
	pb := encodePacket(Broadcast, 0, id, hop, PortNodeInfo, u, sendOpts{wantResponse: ask})
	if err := c.write(pb); err != nil {
		return err
	}
	c.mu.Lock()
	c.st.PacketsOut++
	c.mu.Unlock()
	c.emit(Event{At: c.now(), Kind: "sent", To: "everyone", ID: id, Port: "nodeinfo", Bytes: len(pb), Local: true})
	return nil
}
