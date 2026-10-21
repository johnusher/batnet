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
	mpRxRSSI   = 12
	mpHopStart = 15

	// Data
	dPortnum      = 1
	dPayload      = 2
	dWantResponse = 3
)

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
	decoded               bool
	port                  uint32
	payload               []byte
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

// encodePacket builds a ToRadio carrying one decoded MeshPacket.
func encodePacket(to, channel, id, hopLimit, port uint32, payload []byte, wantResponse bool) []byte {
	var d enc
	d.varint(dPortnum, uint64(port))
	d.bytes(dPayload, payload)
	if wantResponse {
		d.varint(dWantResponse, 1)
	}
	var mp enc
	mp.fixed32(mpTo, to)
	if channel != 0 {
		mp.varint(mpChannel, uint64(channel))
	}
	mp.bytes(mpDecoded, d.b)
	mp.fixed32(mpID, id)
	mp.varint(mpHopLimit, uint64(hopLimit))
	var tr enc
	tr.bytes(trPacket, mp.b)
	return tr.b
}
