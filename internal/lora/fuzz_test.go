package lora

import (
	"testing"

	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
)

// Security review 2026-10-22: frames come off the air from any radio holding
// the channel key. Parsing, reassembly, beacon vectors and the bridge itself
// must survive any bytes.

// FuzzFrame: Parse and the Reassembler on arbitrary frames.
func FuzzFrame(f *testing.F) {
	frames, _ := Split(KindEntry, prio.Normal, 0xa1b2c3d4, make([]byte, 500))
	for _, x := range frames {
		f.Add(x)
	}
	f.Add([]byte{})
	f.Add([]byte{0x40, 0, 0, 0, 1, 0, 0, 1, 0, 1})
	f.Add([]byte{0x40, 0, 0, 0, 1, 0, 0, 1, 254, 255, 'x'})
	f.Fuzz(func(t *testing.T, b []byte) {
		fr, err := Parse(b)
		if err != nil {
			return
		}
		r := NewReassembler(10_000_000, 4)
		for i := int64(0); i < 6; i++ {
			fr.Index = uint8((int(fr.Index) + int(i)) % int(fr.Count))
			r.Add(fr, i*1_000_000)
			r.Stalled(i*1_000_000, 500_000)
		}
		if len(r.pending) > 4 {
			t.Fatalf("%d partial messages pending, max 4", len(r.pending))
		}
	})
}

// FuzzCompactVector: a beacon's vector on arbitrary bytes.
func FuzzCompactVector(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x01, 1, 2, 3, 4, 0x05})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0x0f})
	f.Add([]byte{0x02, 1, 2, 3, 4})
	f.Fuzz(func(t *testing.T, b []byte) {
		v, err := ParseCompactVector(b)
		if err == nil && len(v) > len(b) {
			t.Fatalf("%d origins from %d bytes", len(v), len(b))
		}
	})
}

// FuzzBridgeReceive: a gateway hearing arbitrary frames on air, then ticking.
func FuzzBridgeReceive(f *testing.F) {
	frames, _ := Split(KindBeacon, prio.Beacon, 0xa1b2c3d4, []byte{0x01, 1, 2, 3, 4, 0x05})
	f.Add(frames[0], uint8(2))
	f.Add([]byte{0x40, 0, 0, 0, 1, 0, 0, 1, 0, 1, 1, 2, 3}, uint8(1))
	f.Add([]byte{0x60, 0, 0, 0, 1, 0, 0, 1, 0, 3, 1, 2, 3}, uint8(3))
	f.Fuzz(func(t *testing.T, b []byte, reps uint8) {
		now := int64(1_000_000)
		st := ledger.NewMemStore()
		var self ledger.NodeID
		self[0], self[1] = 0xee, 0xff
		bridge := NewBridge(BridgeConfig{Self: self, Preset: Presets["LongFast"], Store: st,
			Radio: &fakeRadio{air: &fakeAir{}}, Now: func() int64 { return now },
			Inject:    func(e *ledger.Entry) bool { added, _ := st.Put(e); return added },
			InMyGroup: func(short uint32) bool { return short == 0x01020304 }})
		for i := 0; i <= int(reps%8); i++ {
			bridge.Receive(b)
			now += 500_000
			bridge.Tick()
		}
		now += 20 * 60_000_000
		bridge.Tick()
	})
}
