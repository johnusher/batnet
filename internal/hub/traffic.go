package hub

import (
	"math"
	"sync"
	"time"

	"github.com/johnusher/batnet/internal/meshtastic"
)

// Traffic keeps the last hour of what happened on the air, for the panel's
// traffic view: every packet heard (readable or not), every packet this hub
// sent or passed on, and every time the channel was too busy to send.
type Traffic struct {
	preset func() string // the radio's preset now, for airtime

	mu     sync.Mutex
	seq    int64
	frames []Frame
}

// Frame is one moment on the air.
type Frame struct {
	Seq  int64     `json:"seq"`
	At   time.Time `json:"t"`
	Kind string    `json:"kind"` // heard, garbled, sent, relayed, busy
	From string    `json:"from,omitempty"`
	Name string    `json:"name,omitempty"`
	To   string    `json:"to,omitempty"` // a node ID, or "everyone"
	Port string    `json:"port,omitempty"`
	// Encrypted: heard whole, but for a channel this hub has no key for.
	Encrypted bool    `json:"encrypted,omitempty"`
	SNR       float64 `json:"snr"`
	RSSI      int     `json:"rssi,omitempty"`
	Hops      int     `json:"hops"`
	Relay     string  `json:"relay,omitempty"`
	Bytes     int     `json:"bytes,omitempty"`
	AirMs     int     `json:"airMs,omitempty"` // estimated time on the air
	// What the packet says, when it could be read: a text, a shared
	// position, and the decoded contents of other ports.
	Text string         `json:"text,omitempty"`
	Lat  *float64       `json:"lat,omitempty"`
	Lon  *float64       `json:"lon,omitempty"`
	Alt  *int32         `json:"alt,omitempty"`
	Info map[string]any `json:"info,omitempty"`
}

const (
	keepTraffic   = time.Hour
	maxFrames     = 4000
	framesPerPoll = 1000
)

// NewTraffic keeps traffic; preset reports the radio's current preset.
func NewTraffic(preset func() string) *Traffic { return &Traffic{preset: preset} }

func (t *Traffic) add(f Frame) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	f.Seq = t.seq
	if f.At.IsZero() {
		f.At = time.Now()
	}
	t.frames = append(t.frames, f)
	cut := 0
	for cut < len(t.frames) && (len(t.frames)-cut > maxFrames || time.Since(t.frames[cut].At) > keepTraffic) {
		cut++
	}
	t.frames = t.frames[cut:]
}

// Event records a radio event: a packet heard, or one this hub sent.
func (t *Traffic) Event(e meshtastic.Event) {
	switch {
	case e.Kind == "packet" && !e.Local:
		// Bytes is the MeshPacket as meshtasticd hands it over: about 17
		// bytes of receive details more than the payload, which goes on the
		// air after a 16-byte header.
		air := max(e.Bytes-1, 20)
		t.add(Frame{At: e.At, Kind: "heard", From: e.From, Name: e.FromName, To: e.To, Port: e.Port, Encrypted: e.Port == "",
			SNR: float64(e.SNR), RSSI: int(e.RSSI), Hops: e.Hops, Relay: e.Relay, Bytes: air, AirMs: t.airtime(air),
			Text: e.Text, Lat: e.Lat, Lon: e.Lon, Alt: e.Alt, Info: e.Info})
	case e.Kind == "sent":
		air := max(e.Bytes+12, 20) // the ToRadio wrapping off, the header and encryption on
		t.add(Frame{At: e.At, Kind: "sent", To: e.To, Port: e.Port, Hops: 0, Bytes: air, AirMs: t.airtime(air), Text: e.Text})
	}
}

// Garbled records a packet heard but not readable.
func (t *Traffic) Garbled(u meshtastic.Unreadable) {
	t.add(Frame{At: u.At, Kind: "garbled", From: u.From, To: u.To, SNR: u.SNR, RSSI: u.RSSI, Hops: u.Hops, Relay: u.Relay})
}

// Relayed records a packet this hub's radio passed on.
func (t *Traffic) Relayed(relay string) { t.add(Frame{Kind: "relayed", Relay: relay, Hops: -1}) }

// Busy records a moment the radio wanted to send but the channel was busy.
func (t *Traffic) Busy() { t.add(Frame{Kind: "busy", Hops: -1}) }

// TrafficOut is what the panel polls.
type TrafficOut struct {
	Seq    int64   `json:"seq"`
	Preset string  `json:"preset"`
	Limit  float64 `json:"limit"` // the weakest SNR the preset can still read, dB
	Frames []Frame `json:"frames"`
}

// Since returns the frames after seq (at most framesPerPoll, oldest first).
func (t *Traffic) Since(seq int64) TrafficOut {
	p := t.preset()
	t.mu.Lock()
	defer t.mu.Unlock()
	out := TrafficOut{Seq: t.seq, Preset: p, Limit: modems[p].limit, Frames: []Frame{}}
	i := len(t.frames)
	for i > 0 && t.frames[i-1].Seq > seq {
		i--
	}
	if len(t.frames)-i > framesPerPoll {
		i = len(t.frames) - framesPerPoll
	}
	out.Frames = append(out.Frames, t.frames[i:]...)
	return out
}

// modem is a Meshtastic preset's LoRa settings in the EU (bandwidth in kHz,
// coding rate 4/cr) and the SNR it decodes down to.
type modem struct {
	sf, cr int
	bw     float64
	limit  float64
}

var modems = map[string]modem{
	"ShortTurbo": {7, 5, 500, -7.5}, "ShortFast": {7, 5, 250, -7.5}, "ShortSlow": {8, 5, 250, -10},
	"MediumFast": {9, 5, 250, -12.5}, "MediumSlow": {10, 5, 250, -15}, "LongFast": {11, 5, 250, -17.5},
	"LongModerate": {11, 8, 125, -17.5}, "LongSlow": {12, 8, 125, -20}, "VeryLongSlow": {12, 8, 62.5, -20},
}

// airtime is LoRa's time on air (Semtech AN1200.13) for n bytes, with
// Meshtastic's 16-symbol preamble, an explicit header and a CRC.
func (t *Traffic) airtime(n int) int {
	m, ok := modems[t.preset()]
	if !ok {
		return 0
	}
	tsym := math.Pow(2, float64(m.sf)) / m.bw // ms
	de := 0.0
	if tsym > 16 {
		de = 1
	}
	pre := (16 + 4.25) * tsym
	num := 8*float64(n) - 4*float64(m.sf) + 28 + 16
	sym := 8 + math.Max(math.Ceil(num/(4*(float64(m.sf)-2*de)))*float64(m.cr), 0)
	return int(math.Round(pre + sym*tsym))
}
