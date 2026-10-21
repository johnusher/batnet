package meshtastic

import (
	"fmt"
	"math"
)

// Ports whose contents the panel shows.
const (
	PortTraceroute   = 70
	PortNeighborInfo = 71
)

// Position is a position a radio shared (mesh.proto Position). Many radios
// share a deliberately rough one: Precision says how rough.
type Position struct {
	Lat, Lon float64
	Alt      *int32
	// PrecisionBits is how many bits of the coordinates were kept (32: all);
	// AccuracyM is the radius that leaves, in metres (0: exact).
	PrecisionBits uint32
	AccuracyM     float64
	Time          uint32 // when the radio was there (Unix seconds); 0: not said
	Sats          uint32
}

func parsePosition(b []byte) (Position, bool) {
	fs, err := fields(b)
	if err != nil {
		return Position{}, false
	}
	var p Position
	var haveLat, haveLon bool
	for _, f := range fs {
		switch f.num {
		case 1:
			p.Lat, haveLat = float64(int32(f.v))/1e7, true
		case 2:
			p.Lon, haveLon = float64(int32(f.v))/1e7, true
		case 3:
			v := f.i32()
			p.Alt = &v
		case 4:
			p.Time = f.u32()
		case 19:
			p.Sats = f.u32()
		case 23:
			p.PrecisionBits = f.u32()
		}
	}
	if !haveLat || !haveLon || (p.Lat == 0 && p.Lon == 0) {
		return Position{}, false
	}
	if p.PrecisionBits > 0 && p.PrecisionBits < 32 {
		// Half the cell left after dropping bits: 10 bits keep about 23 km.
		p.AccuracyM = math.Round(math.Pow(2, float64(31-p.PrecisionBits)) * 1e-7 * 111320)
	}
	return p, true
}

// Hardware names for the boards seen most often (mesh.proto HardwareModel).
var hardware = map[uint32]string{4: "LilyGO T-Beam", 7: "LilyGO T-Echo", 9: "RAK WisBlock 4631", 37: "Linux (meshtasticd)",
	43: "Heltec V3", 71: "Seeed T1000-E"}

// HardwareName names a board, or gives its number.
func HardwareName(hw uint32) string {
	if n, ok := hardware[hw]; ok {
		return n
	}
	if hw == 0 {
		return ""
	}
	return fmt.Sprintf("board model %d", hw)
}

// info decodes a packet's contents for the panel: what it says, port by port.
func info(port uint32, payload []byte) map[string]any {
	m := map[string]any{}
	fs, err := fields(payload)
	if err != nil && port != PortText {
		return nil
	}
	switch port {
	case PortPosition:
		if p, ok := parsePosition(payload); ok {
			if p.AccuracyM > 0 {
				m["accuracyM"] = p.AccuracyM
			}
			if p.Sats > 0 {
				m["sats"] = p.Sats
			}
		}
	case PortNodeInfo:
		u := parseUser(payload)
		m["longName"], m["shortName"] = u.long, u.short
		if h := HardwareName(u.hw); h != "" {
			m["hardware"] = h
		}
		if r := roles[u.role]; r != "" {
			m["role"] = r
		}
	case PortTelemetry:
		for _, f := range fs {
			sub, _ := fields(f.b)
			switch f.num {
			case 2: // device metrics
				for _, s := range sub {
					switch s.num {
					case 1:
						m["battery"] = s.u32()
					case 2:
						m["voltage"] = round2(s.f32())
					case 3:
						m["channelUse"] = round2(s.f32())
					case 4:
						m["airtime"] = round2(s.f32())
					case 5:
						m["uptimeS"] = s.u32()
					}
				}
			case 3: // environment
				for _, s := range sub {
					switch s.num {
					case 1:
						m["temperatureC"] = round2(s.f32())
					case 2:
						m["humidity"] = round2(s.f32())
					case 3:
						m["pressureHPa"] = round2(s.f32())
					}
				}
			}
		}
	case PortTraceroute:
		var route, back []string
		for _, f := range fs {
			switch {
			case f.num == 1 && f.typ == wireI32:
				route = append(route, nodeID(f.u32()))
			case f.num == 3 && f.typ == wireI32:
				back = append(back, nodeID(f.u32()))
			case (f.num == 1 || f.num == 3) && f.typ == wireLen: // packed
				for i := 0; i+4 <= len(f.b); i += 4 {
					n := uint32(f.b[i]) | uint32(f.b[i+1])<<8 | uint32(f.b[i+2])<<16 | uint32(f.b[i+3])<<24
					if f.num == 1 {
						route = append(route, nodeID(n))
					} else {
						back = append(back, nodeID(n))
					}
				}
			}
		}
		m["route"], m["routeBack"] = route, back
	case PortNeighborInfo:
		var ns []map[string]any
		for _, f := range fs {
			if f.num != 4 {
				continue
			}
			sub, _ := fields(f.b)
			n := map[string]any{}
			for _, s := range sub {
				switch s.num {
				case 1:
					n["id"] = nodeID(s.u32())
				case 2:
					n["snr"] = round2(s.f32())
				}
			}
			ns = append(ns, n)
		}
		m["neighbors"] = ns
	case PortRouting:
		if code := routingError(payload); code != 0 {
			m["error"] = routingErrors[code]
		}
	default:
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func round2(f float32) float64 { return math.Round(float64(f)*100) / 100 }
