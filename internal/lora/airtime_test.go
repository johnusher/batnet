package lora

import "testing"

func TestAirtimeMatchesTheJavaScriptModel(t *testing.T) {
	// the JavaScript model: LongFast, 130 bytes including overhead -> 1214 ms; ShortFast -> 112 ms
	lf, sf := Presets["LongFast"], Presets["ShortFast"]
	if got := AirtimeUs(130-Overhead, lf) / 1000; got < 1210 || got > 1218 {
		t.Fatalf("LongFast 130 B = %d ms, want about 1214", got)
	}
	if got := AirtimeUs(130-Overhead, sf) / 1000; got < 110 || got > 114 {
		t.Fatalf("ShortFast 130 B = %d ms, want about 112", got)
	}
	if AirtimeUs(MaxFrame, lf) <= AirtimeUs(10, lf) {
		t.Fatal("bigger frames must take longer")
	}
}
