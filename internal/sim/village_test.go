package sim

import (
	"reflect"
	"testing"

	"github.com/johnusher/batnet/internal/lora"
)

const testEpoch = 1_790_000_000_000

func smallVillage(t *testing.T, seed int64) *Village {
	t.Helper()
	v, err := NewVillage(seed, testEpoch, lora.Presets["LongFast"])
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []HouseSpec{
		{Name: "Miller", X: 0, Y: 0, Pi: "Pi 4", WiFi: "dongle", People: 3},
		{Name: "Fern", X: 300, Y: 0, Pi: "Pi 4", WiFi: "dongle", People: 2},
		{Name: "Okafor", X: 600, Y: 0, Pi: "Pi 5", WiFi: "dongle", People: 4},
		{Name: "Far", X: 5000, Y: 0, Pi: "Pi 4", WiFi: "dongle", People: 1},
	} {
		if _, err := v.AddHouse(h); err != nil {
			t.Fatal(err)
		}
	}
	v.Start()
	return v
}

func TestPostReachesItsGroup(t *testing.T) {
	v := smallVillage(t, 1)
	p, err := v.Post("Miller", "Water point open until 18:00", 5)
	if err != nil {
		t.Fatal(err)
	}
	v.RunFor(5 * Second)
	for _, h := range []string{"Fern", "Okafor"} {
		at, ok := p.Arrived[h]
		if !ok {
			t.Fatalf("%s never got the post", h)
		}
		if at-p.Created > 500*Ms {
			t.Fatalf("%s got it after %d µs", h, at-p.Created)
		}
	}
	if _, ok := p.Arrived["Far"]; ok {
		t.Fatal("a house out of WiFi range got the post")
	}
}

func TestPowerCutCatchUp(t *testing.T) {
	v := smallVillage(t, 2)
	if err := v.SetPower("Okafor", false); err != nil {
		t.Fatal(err)
	}
	p, _ := v.Post("Miller", "while Okafor was off", 5)
	v.RunFor(30 * Second)
	if _, ok := p.Arrived["Okafor"]; ok {
		t.Fatal("a house without power got the post")
	}
	v.SetPower("Okafor", true)
	v.RunFor(15 * Second) // a few heartbeats
	if _, ok := p.Arrived["Okafor"]; !ok {
		t.Fatal("Okafor did not catch up after power returned")
	}
}

// Review focus 1.
func TestDeterministic(t *testing.T) {
	run := func() []map[string]int64 {
		v := smallVillage(t, 7)
		for i := 0; i < 20; i++ {
			v.Post([]string{"Miller", "Fern", "Okafor"}[i%3], "hello", 5)
			v.RunFor(700 * Ms)
		}
		v.RunFor(20 * Second)
		var out []map[string]int64
		for _, p := range v.Posts {
			out = append(out, p.Arrived)
		}
		return out
	}
	if a, b := run(), run(); !reflect.DeepEqual(a, b) {
		t.Fatal("two runs with the same seed differ")
	}
}

func TestErrors(t *testing.T) {
	v := smallVillage(t, 1)
	if _, err := v.AddHouse(HouseSpec{Name: "Miller", Pi: "Pi 4", WiFi: "dongle"}); err == nil {
		t.Error("duplicate house name accepted")
	}
	if _, err := v.Post("Nobody", "x", 5); err == nil {
		t.Error("post from an unknown house accepted")
	}
	if err := v.SetPower("Nobody", false); err == nil {
		t.Error("power change for an unknown node accepted")
	}
}

func TestPostCrossesByLoRa(t *testing.T) {
	v, err := NewVillage(3, testEpoch, lora.Presets["LongFast"])
	if err != nil {
		t.Fatal(err)
	}
	v.AddHouse(HouseSpec{Name: "Miller", X: 0, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2})
	v.AddHouse(HouseSpec{Name: "Fern", X: 2500, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2}) // far apart: no WiFi link
	v.Start()
	v.RunFor(16 * Minute) // beacons tell each gateway the other group exists
	p, err := v.Post("Miller", "Bread at the bakery", 5)
	if err != nil {
		t.Fatal(err)
	}
	v.RunFor(2 * Minute)
	if _, ok := p.Arrived["Fern"]; !ok {
		t.Fatal("the post did not reach the other house by LoRa")
	}
}

// Review I6: a power cut reboots a gateway: its queue is gone and it beacons within a minute.
func TestPowerCutRestartsGateway(t *testing.T) {
	v, _ := NewVillage(5, testEpoch, lora.Presets["LongFast"])
	v.AddHouse(HouseSpec{Name: "Miller", X: 0, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2})
	v.AddHouse(HouseSpec{Name: "Fern", X: 2500, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2})
	v.Start()
	v.RunFor(20 * Minute)
	n := v.Node("Miller")
	beacons := n.Bridge.Stats.Beacons
	v.SetPower("Miller", false)
	v.RunFor(5 * Minute)
	v.SetPower("Miller", true)
	if n.Bridge.Queue.Len() != 0 {
		t.Fatal("the queue survived a power cut")
	}
	v.RunFor(61 * Second)
	if n.Bridge.Stats.Beacons == beacons {
		t.Fatal("no beacon within a minute of power returning")
	}
}

// Review follow-up: two gateways that both hold what a beacon lacks must not
// both answer (on real airtime their answers collide and the backlog grows).
func TestBeaconAnswersNotDuplicated(t *testing.T) {
	v, _ := NewVillage(9, testEpoch, lora.Presets["LongFast"])
	v.LoRa.BaseLoss, v.LoRa.EdgeLoss = 0, 0
	v.AddHouse(HouseSpec{Name: "Miller", X: 0, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2})
	v.AddHouse(HouseSpec{Name: "Fern", X: 200, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2})
	v.AddHouse(HouseSpec{Name: "Haddad", X: 2500, Y: 0, Pi: "Pi 4", WiFi: "dongle", LoRa: true, People: 2})
	v.SetPower("Haddad", false)
	v.Start()
	for i := 0; i < 6; i++ {
		v.Post("Miller", "while Haddad was off", 5)
	}
	v.RunFor(2 * Minute)
	v.SetPower("Haddad", true)
	v.RunFor(20 * Minute)
	// Only one of the two gateways may answer; resends after a collision on the air are fine.
	if m, f := v.Node("Miller").Bridge.Stats.Sent, v.Node("Fern").Bridge.Stats.Sent; m > 0 && f > 0 {
		t.Fatalf("both gateways answered the beacon: Miller sent %d, Fern %d", m, f)
	}
	for _, p := range v.Posts {
		if _, ok := p.Arrived["Haddad"]; !ok {
			t.Fatal("Haddad did not recover every post")
		}
	}
}

// Review focus 3.
func TestSimPhonePostTravels(t *testing.T) {
	v := smallVillage(t, 4)
	miller, okafor := v.Node("Miller"), v.Node("Okafor")
	if len(miller.People) != 3 || len(okafor.People) != 4 {
		t.Fatalf("people: %d %d", len(miller.People), len(okafor.People))
	}
	p, err := v.Post("Miller", "Water point open", 5)
	if err != nil {
		t.Fatal(err)
	}
	v.RunFor(5 * Second)
	if _, ok := p.Arrived["Okafor"]; !ok {
		t.Fatal("the post didn't arrive")
	}
	got := okafor.Web.State().Messages(okafor.People[0], "everyone", 10)
	if len(got) != 1 || got[0].Text != "Water point open" {
		t.Fatalf("Okafor's people see %v", got)
	}
}

// Review focus 5.
func TestSeededDeterministic(t *testing.T) {
	a, b := smallVillage(t, 9), smallVillage(t, 9)
	if a.Air.Stats.Frames != 0 {
		t.Fatalf("seeding sent %d frames; it should write stores directly", a.Air.Stats.Frames)
	}
	for i := range a.Nodes {
		va, _ := a.Nodes[i].Store.Vector()
		vb, _ := b.Nodes[i].Store.Vector()
		if len(va) != len(vb) || len(va) < 4 {
			t.Fatalf("node %d: vectors differ or empty: %d vs %d", i, len(va), len(vb))
		}
	}
}
