// Package lora is batnet's bridge to the LoRa radio: frames and fragments,
// the priority queue under the duty limit, and the gateway logic that decides
// what goes on air (spec §6, decisions 0008, 0015, 0017).
package lora

import "math"

// Preset is a Meshtastic modem preset.
type Preset struct {
	Name   string
	SF     int
	BW     float64 // Hz
	CR     int     // 1 = 4/5
	RangeM float64 // the model's typical reach
}

// Presets are the ones the simulator offers (the JavaScript model's numbers).
var Presets = map[string]Preset{
	"ShortFast":  {Name: "ShortFast", SF: 7, BW: 250e3, CR: 1, RangeM: 1200},
	"MediumFast": {Name: "MediumFast", SF: 9, BW: 250e3, CR: 1, RangeM: 2000},
	"LongFast":   {Name: "LongFast", SF: 11, BW: 250e3, CR: 1, RangeM: 3000},
}

const (
	Overhead    = 26          // Meshtastic header, protobuf and encryption, bytes per packet
	MaxFrame    = 200         // batnet bytes per packet, header included
	DutyLimitUs = 360_000_000 // 10% of an hour (EU 869.4-869.65 MHz)
	HourUs      = 3_600_000_000
)

// AirtimeUs is how long one packet with frameBytes of batnet data is on air
// (Semtech's formula, preamble 16, CRC on, explicit header).
func AirtimeUs(frameBytes int, p Preset) int64 {
	payload := frameBytes + Overhead
	tsym := math.Pow(2, float64(p.SF)) / p.BW * 1e6 // µs
	de := 0
	if tsym > 16000 {
		de = 1
	}
	num := float64(8*payload - 4*p.SF + 28 + 16)
	sym := 8 + math.Max(math.Ceil(num/float64(4*(p.SF-2*de)))*float64(p.CR+4), 0)
	return int64((16+4.25)*tsym + sym*tsym)
}
