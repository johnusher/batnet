package meshtastic

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
	"unicode/utf8"
)

// Admin messages (admin.proto) change this hub's own radio: its LoRa settings
// and its name. They go only to our own node, over the local API.
const (
	PortAdmin   = 6
	adSetOwner  = 32
	adSetConfig = 34
	cfgLoRa     = 6

	// LoRaConfig fields
	loUsePreset = 1
	loPreset    = 2
	loHopLimit  = 8
	loTxEnabled = 9
	loTxPower   = 10

	// MaxLongName and MaxShortName are what a node's names hold, in bytes
	// (an emoji takes four).
	MaxLongName  = 39
	MaxShortName = 4
	// MaxTxPower is the EU868 limit (27 dBm ERP); the radio itself may give less.
	MaxTxPower = 27
)

// RadioSettings are the LoRa settings the panel can change; nil and "" keep
// what the radio has.
type RadioSettings struct {
	Preset    string  `json:"preset,omitempty"`
	HopLimit  *uint32 `json:"hopLimit,omitempty"`
	TxPower   *int32  `json:"txPower,omitempty"`
	TxEnabled *bool   `json:"txEnabled,omitempty"`
}

// Presets lists the presets the panel offers, from the longest reach to the fastest.
var Presets = []string{"LongSlow", "LongModerate", "LongFast", "MediumSlow", "MediumFast", "ShortSlow", "ShortFast", "ShortTurbo"}

func presetNum(name string) (uint32, bool) {
	for n, p := range presets {
		if p == name && name != "VeryLongSlow" {
			return n, true
		}
	}
	return 0, false
}

var errNoConfig = errors.New("meshtastic: the radio's settings have not arrived yet")

// SetRadio changes the radio's LoRa settings. meshtasticd restarts its radio
// to apply them (about 10 s) and the client reconnects by itself.
func (c *Client) SetRadio(s RadioSettings) error {
	set := map[int]uint64{}
	if s.Preset != "" {
		n, ok := presetNum(s.Preset)
		if !ok {
			return fmt.Errorf("meshtastic: no preset %q", s.Preset)
		}
		set[loUsePreset], set[loPreset] = 1, uint64(n)
	}
	if s.HopLimit != nil {
		if *s.HopLimit < 1 || *s.HopLimit > 7 {
			return errors.New("meshtastic: relays allowed are 1 to 7")
		}
		set[loHopLimit] = uint64(*s.HopLimit)
	}
	if s.TxPower != nil {
		if *s.TxPower < 1 || *s.TxPower > MaxTxPower {
			return fmt.Errorf("meshtastic: transmit power is 1 to %d dBm", MaxTxPower)
		}
		set[loTxPower] = uint64(int64(*s.TxPower))
	}
	if s.TxEnabled != nil {
		set[loTxEnabled] = 0
		if *s.TxEnabled {
			set[loTxEnabled] = 1
		}
	}
	if len(set) == 0 {
		return nil
	}
	c.mu.Lock()
	raw := c.loraRaw
	c.mu.Unlock()
	if raw == nil {
		return errNoConfig
	}
	// set_config replaces the whole LoRaConfig: keep every field the radio
	// sent and change only ours.
	var lora enc
	fs, err := fields(raw)
	if err != nil {
		return err
	}
	for _, f := range fs {
		if _, ok := set[f.num]; !ok {
			lora.put(f)
		}
	}
	for _, num := range []int{loUsePreset, loPreset, loHopLimit, loTxEnabled, loTxPower} {
		if v, ok := set[num]; ok {
			lora.varint(num, v)
		}
	}
	var cfg, adm enc
	cfg.bytes(cfgLoRa, lora.b)
	adm.bytes(adSetConfig, cfg.b)
	return c.admin(adm.b)
}

// SetOwner gives this hub's radio a new long and short name.
func (c *Client) SetOwner(long, short string) error {
	long, short = strings.TrimSpace(long), strings.TrimSpace(short)
	if long == "" || len(long) > MaxLongName || !utf8.ValidString(long) {
		return fmt.Errorf("meshtastic: the name is 1 to %d bytes", MaxLongName)
	}
	if short == "" || len(short) > MaxShortName || !utf8.ValidString(short) {
		return fmt.Errorf("meshtastic: the short name is 1 to %d bytes (one emoji is 4)", MaxShortName)
	}
	var u, adm enc
	u.bytes(2, []byte(long))
	u.bytes(3, []byte(short))
	adm.bytes(adSetOwner, u.b)
	return c.admin(adm.b)
}

// admin sends an AdminMessage to our own node, then reconnects to read the
// settings back.
func (c *Client) admin(payload []byte) error {
	c.mu.Lock()
	if c.conn == nil || !c.st.Ready {
		c.mu.Unlock()
		return ErrOffline
	}
	if c.now().Sub(c.lastAdmin) < adminGap {
		c.mu.Unlock()
		return errors.New("meshtastic: one settings change every few seconds")
	}
	c.lastAdmin = c.now()
	me, conn := c.st.MyNum, c.conn
	c.mu.Unlock()
	if err := c.write(encodePacket(me, 0, rand.Uint32(), 0, PortAdmin, payload, sendOpts{wantResponse: true})); err != nil {
		return err
	}
	// A fresh session reads the changed settings and names.
	time.AfterFunc(2*time.Second, func() { conn.Close() })
	return nil
}
