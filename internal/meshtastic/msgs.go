package meshtastic

// Field numbers from meshtastic/protobufs (mesh.proto, config.proto,
// channel.proto, telemetry.proto, portnums.proto).
const (
	// FromRadio
	frPacket         = 2
	frMyInfo         = 3
	frNodeInfo       = 4
	frConfig         = 5
	frConfigComplete = 7
	frRebooted       = 8
	frChannel        = 10
	frMetadata       = 13

	// ToRadio
	trPacket       = 1
	trWantConfigID = 3
	trHeartbeat    = 7

	// MeshPacket
	mpFrom     = 1
	mpTo       = 2
	mpChannel  = 3
	mpDecoded  = 4
	mpID       = 6
	mpRxTime   = 7
	mpRxSNR    = 8
	mpHopLimit = 9
	mpWantAck  = 10
	mpRxRSSI   = 12
	mpHopStart = 15
	mpPKI      = 17
	mpRelay    = 19

	// Data
	dPortnum      = 1
	dPayload      = 2
	dWantResponse = 3
	dRequestID    = 6
	dReplyID      = 7

	// Routing
	rtErrorReason = 3
)

// routingErrors says in plain words why a message was not delivered
// (Routing.Error in mesh.proto).
var routingErrors = map[uint32]string{1: "no route to that radio", 2: "refused", 3: "timed out",
	4: "no radio interface", 5: "no answer after three tries", 6: "no channel for it", 7: "too large",
	8: "nobody answered", 9: "the radio's airtime limit was reached", 32: "bad request", 33: "not allowed",
	34: "encryption failed", 35: "that radio does not know this hub's key yet", 36: "the remote admin session ran out",
	37: "not one of that radio's admins", 38: "too many messages: rate limited", 39: "no key for that radio yet"}

// ErrNoKey is Routing.Error PKI_SEND_FAIL_PUBLIC_KEY: the radio refused to
// send a private message because it has not heard that radio's public key
// (its name card), and it no longer sends private messages unencrypted.
const ErrNoKey = 39

// Ports (portnums.proto).
const (
	PortText      = 1
	PortPosition  = 3
	PortNodeInfo  = 4
	PortRouting   = 5
	PortTelemetry = 67
	// PortPrivate is PRIVATE_APP: batnet's envelopes and beacons (spec §6).
	PortPrivate = 256
)

// Broadcast is the "to everyone" node number.
const Broadcast = 0xffffffff

var regions = map[uint32]string{0: "not set", 1: "US", 2: "EU_433", 3: "EU_868", 4: "CN", 5: "JP", 6: "ANZ",
	7: "KR", 8: "TW", 9: "RU", 10: "IN", 11: "NZ_865", 12: "TH", 13: "LORA_24", 14: "UA_433", 15: "UA_868"}

var presets = map[uint32]string{0: "LongFast", 1: "LongSlow", 2: "VeryLongSlow", 3: "MediumSlow", 4: "MediumFast",
	5: "ShortSlow", 6: "ShortFast", 7: "LongModerate", 8: "ShortTurbo"}

var roles = map[uint32]string{0: "client", 1: "client mute", 2: "router", 3: "router client", 4: "repeater",
	5: "tracker", 6: "sensor", 7: "TAK", 8: "client hidden", 9: "lost and found", 10: "TAK tracker", 11: "router late", 12: "client base"}

// user is the User message: who a node says it is.
type user struct {
	id, long, short string
	hw, role        uint32
	key             bool // carries its public key
}

func parseUser(b []byte) (u user) {
	fs, err := fields(b)
	if err != nil {
		return u
	}
	for _, f := range fs {
		switch f.num {
		case 1:
			u.id = string(f.b)
		case 2:
			u.long = string(f.b)
		case 3:
			u.short = string(f.b)
		case 5:
			u.hw = f.u32()
		case 7:
			u.role = f.u32()
		case 8:
			u.key = len(f.b) == 32
		}
	}
	return u
}

// metrics is DeviceMetrics; nil fields were not sent.
type metrics struct {
	battery, uptime   *uint32
	voltage, chanUtil *float32
	airUtilTx         *float32
}

func parseMetrics(b []byte) (m metrics) {
	fs, _ := fields(b)
	for _, f := range fs {
		u, fl := f.u32(), f.f32()
		switch f.num {
		case 1:
			m.battery = &u
		case 2:
			m.voltage = &fl
		case 3:
			m.chanUtil = &fl
		case 4:
			m.airUtilTx = &fl
		case 5:
			m.uptime = &u
		}
	}
	return m
}

// packet is a MeshPacket with its Data decoded, if it was.
type packet struct {
	from, to, channel, id uint32
	rxTime                uint32
	snr                   float32
	rssi                  int32
	hopLimit, hopStart    uint32
	relay                 uint32 // the last byte of the radio that passed it on; 0: unknown
	pki                   bool   // encrypted end to end for this one radio
	decoded               bool
	port                  uint32
	payload               []byte
	requestID, replyID    uint32
}

func parsePacket(b []byte) (p packet, err error) {
	fs, err := fields(b)
	if err != nil {
		return p, err
	}
	for _, f := range fs {
		switch f.num {
		case mpFrom:
			p.from = f.u32()
		case mpTo:
			p.to = f.u32()
		case mpChannel:
			p.channel = f.u32()
		case mpID:
			p.id = f.u32()
		case mpRxTime:
			p.rxTime = f.u32()
		case mpRxSNR:
			p.snr = f.f32()
		case mpRxRSSI:
			p.rssi = f.i32()
		case mpHopLimit:
			p.hopLimit = f.u32()
		case mpHopStart:
			p.hopStart = f.u32()
		case mpRelay:
			p.relay = f.u32()
		case mpPKI:
			p.pki = f.v != 0
		case mpDecoded:
			p.decoded = true
			ds, err := fields(f.b)
			if err != nil {
				return p, err
			}
			for _, d := range ds {
				switch d.num {
				case dPortnum:
					p.port = d.u32()
				case dPayload:
					p.payload = d.b
				case dRequestID:
					p.requestID = d.u32()
				case dReplyID:
					p.replyID = d.u32()
				}
			}
		}
	}
	return p, nil
}

// hops is how many relays a packet passed, or -1 if it does not say.
func (p packet) hops() int {
	if p.hopStart == 0 || p.hopLimit > p.hopStart {
		return -1
	}
	return int(p.hopStart - p.hopLimit)
}

// routingError reads a Routing payload: 0 means delivered (or passed on).
func routingError(b []byte) uint32 {
	fs, _ := fields(b)
	for _, f := range fs {
		if f.num == rtErrorReason {
			return f.u32()
		}
	}
	return 0
}

// sendOpts are the optional parts of an outgoing packet.
type sendOpts struct {
	wantResponse bool   // ask the radios that hear it to answer (a hello)
	wantAck      bool   // ask for a delivery receipt
	replyID      uint32 // the message this one answers
}

// encodePacket builds a ToRadio carrying one decoded MeshPacket.
func encodePacket(to, channel, id, hopLimit, port uint32, payload []byte, o sendOpts) []byte {
	var d enc
	d.varint(dPortnum, uint64(port))
	d.bytes(dPayload, payload)
	if o.wantResponse {
		d.varint(dWantResponse, 1)
	}
	if o.replyID != 0 {
		d.fixed32(dReplyID, o.replyID)
	}
	var mp enc
	mp.fixed32(mpTo, to)
	if channel != 0 {
		mp.varint(mpChannel, uint64(channel))
	}
	mp.bytes(mpDecoded, d.b)
	mp.fixed32(mpID, id)
	mp.varint(mpHopLimit, uint64(hopLimit))
	if o.wantAck {
		mp.varint(mpWantAck, 1)
	}
	var tr enc
	tr.bytes(trPacket, mp.b)
	return tr.b
}
