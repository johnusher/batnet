package meshtastic

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestSetRadioKeepsTheOtherSettings(t *testing.T) {
	f, c, _ := readyRadio(t)
	var lora, cfg, fr enc
	lora.varint(loUsePreset, 1)
	lora.varint(loPreset, 0) // LongFast
	lora.varint(7, 3)        // EU_868
	lora.varint(loHopLimit, 3)
	lora.varint(loTxEnabled, 1)
	lora.varint(loTxPower, 27)
	lora.varint(13, 1) // boosted receive gain: must survive
	cfg.bytes(cfgLoRa, lora.b)
	fr.bytes(frConfig, cfg.b)
	f.send(fr.b)
	waitFor(t, c, func(s Status) bool { return s.Region == "EU_868" })

	if err := c.SetRadio(RadioSettings{Preset: "Turbo"}); err == nil {
		t.Error("an unknown preset was accepted")
	}
	hops := uint32(9)
	if err := c.SetRadio(RadioSettings{HopLimit: &hops}); err == nil {
		t.Error("9 relays were accepted")
	}
	hops = 4
	if err := c.SetRadio(RadioSettings{Preset: "MediumFast", HopLimit: &hops}); err != nil {
		t.Fatal(err)
	}
	p := sentPacket(t, f)
	if p.to != 0xdeadbeef || p.port != PortAdmin {
		t.Fatalf("admin packet %+v", p)
	}
	adm, _ := fields(p.payload)
	if len(adm) != 1 || adm[0].num != adSetConfig {
		t.Fatalf("admin %+v", adm)
	}
	cf, _ := fields(adm[0].b)
	got := map[int]uint64{}
	lf, _ := fields(cf[0].b)
	for _, x := range lf {
		got[x.num] = x.v
	}
	want := map[int]uint64{loUsePreset: 1, loPreset: 4, 7: 3, loHopLimit: 4, loTxEnabled: 1, loTxPower: 27, 13: 1}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("field %d = %d, want %d (all %v)", k, got[k], v, got)
		}
	}
	if err := c.SetRadio(RadioSettings{Preset: "LongFast"}); err == nil {
		t.Error("a second change at once was accepted")
	}
}

func TestSetOwner(t *testing.T) {
	f, c, _ := readyRadio(t)
	if err := c.SetOwner("lorabatnet 🐰🦇", "🐰🦇"); err == nil {
		t.Error("an 8-byte short name was accepted")
	}
	if err := c.SetOwner("", "🦇"); err == nil {
		t.Error("an empty name was accepted")
	}
	if err := c.SetOwner("lorabatnet 🐰🦇", "🦇"); err != nil {
		t.Fatal(err)
	}
	p := sentPacket(t, f)
	adm, _ := fields(p.payload)
	u := parseUser(adm[0].b)
	if adm[0].num != adSetOwner || u.long != "lorabatnet 🐰🦇" || u.short != "🦇" {
		t.Errorf("owner %+v %+v", adm, u)
	}
	// The client drops the session, to read the new name back on the next.
	deadline := time.Now().Add(4 * time.Second)
	for c.Status().Connected && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if c.Status().Connected {
		t.Error("still on the old session after the change")
	}
}

func TestPositionsAndContents(t *testing.T) {
	var pos enc
	lat, lon := int32(524000000), int32(134000000)
	pos.fixed32(1, uint32(lat))
	pos.fixed32(2, uint32(lon))
	pos.varint(3, 40)
	pos.varint(23, 13)
	p, ok := parsePosition(pos.b)
	if !ok || p.Lat != 52.4 || p.Lon != 13.4 || *p.Alt != 40 || math.Abs(p.AccuracyM-2918) > 5 {
		t.Errorf("position %+v", p)
	}
	// meshtasticd's own copy of a radio leaves the precision out: read it off
	// the coordinates, the middle of a 16-bit cell; an exact one stays exact.
	var lite enc
	lite.fixed32(1, uint32(int32(524000000))&^(1<<16-1)|1<<15)
	lite.fixed32(2, uint32(int32(134000000))&^(1<<16-1)|1<<15)
	if p, _ := parsePosition(lite.b); p.PrecisionBits != 16 || math.Abs(p.AccuracyM-365) > 2 {
		t.Errorf("rough position from the node list %+v", p)
	}
	var exact enc
	exact.fixed32(1, 524001234)
	exact.fixed32(2, 134004567)
	if p, _ := parsePosition(exact.b); p.AccuracyM != 0 {
		t.Errorf("exact position read as rough %+v", p)
	}
	if _, ok := parsePosition(nil); ok {
		t.Error("an empty position was accepted")
	}

	var dm, tel enc
	dm.varint(1, 87)
	dm.fixed32(2, math.Float32bits(3.91))
	dm.fixed32(3, math.Float32bits(12.5))
	tel.bytes(2, dm.b)
	m := info(PortTelemetry, tel.b)
	if m["battery"] != uint32(87) || m["voltage"] != 3.91 || m["channelUse"] != 12.5 {
		t.Errorf("telemetry %v", m)
	}
	if m := info(PortNodeInfo, userMsg("!a1b2c3d4", "Anna's handheld", "ANNA")); m["longName"] != "Anna's handheld" {
		t.Errorf("nodeinfo %v", m)
	}
	var rd enc
	rd.fixed32(1, 0xa1b2c3d4)
	if m := info(PortTraceroute, rd.b); len(m["route"].([]string)) != 1 {
		t.Errorf("traceroute %v", m)
	}
	// The signals on the way, in quarter dB, packed: -12.25 and +6 dB there, -3 back.
	var snr enc
	for _, v := range []int32{-49, 24} {
		snr.b = binary.AppendUvarint(snr.b, uint64(int64(v)))
	}
	rd.bytes(2, snr.b)
	back := int64(-12)
	rd.varint(4, uint64(back))
	if m := info(PortTraceroute, rd.b); fmt.Sprint(m["snrTowards"], m["snrBack"]) != "[-12.25 6] [-3]" {
		t.Errorf("traceroute signals %v", m)
	}
}
