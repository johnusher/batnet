package ledger

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/johnusher/batnet/internal/hlc"
)

func newKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func sampleEntry(priv ed25519.PrivateKey) *Entry {
	e := &Entry{
		Key:  Key{Origin: NodeIDFromPublic(priv.Public().(ed25519.PublicKey)), Seq: 3},
		TS:   hlc.Timestamp{Wall: 1_790_000_000_000, Logical: 2},
		Type: TypeMsgPost,
		Prio: 5,
		Body: []byte(`{"audience":"everyone","text":"Water point open until 18:00"}`),
	}
	e.Sign(priv)
	return e
}

func TestRoundTrip(t *testing.T) {
	e := sampleEntry(newKey(t))
	b, err := e.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var got Entry
	if err := got.UnmarshalBinary(b); err != nil {
		t.Fatal(err)
	}
	if got.Key != e.Key || got.TS != e.TS || got.Type != e.Type || got.Prio != e.Prio ||
		!bytes.Equal(got.Body, e.Body) || !bytes.Equal(got.Sig, e.Sig) {
		t.Fatalf("round trip changed the entry:\n got %+v\nwant %+v", got, *e)
	}
	if err := got.Verify(); err != nil {
		t.Fatalf("verify after round trip: %v", err)
	}
}

func TestVerifyCatchesTampering(t *testing.T) {
	e := sampleEntry(newKey(t))
	e.Body = []byte(`{"audience":"everyone","text":"Water point closed"}`)
	if err := e.Verify(); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestVerifyCatchesWrongSigner(t *testing.T) {
	e := sampleEntry(newKey(t))
	e.Sign(newKey(t)) // signed by someone who isn't the origin
	if err := e.Verify(); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestUnmarshalRejectsMalformed(t *testing.T) {
	good, _ := sampleEntry(newKey(t)).MarshalBinary()
	cases := map[string][]byte{
		"empty":         {},
		"wrong version": append([]byte{99}, good[1:]...),
		"truncated":     good[:len(good)-10],
		"trailing":      append(append([]byte{}, good...), 0),
	}
	for name, b := range cases {
		var e Entry
		if err := e.UnmarshalBinary(b); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: got %v, want ErrMalformed", name, err)
		}
	}
}

func TestUnmarshalRejectsBadFields(t *testing.T) {
	priv := newKey(t)
	for name, mutate := range map[string]func(*Entry){
		"seq zero":  func(e *Entry) { e.Key.Seq = 0 },
		"prio 10":   func(e *Entry) { e.Prio = 10 },
		"huge body": func(e *Entry) { e.Body = make([]byte, MaxBody+1) },
	} {
		e := sampleEntry(priv)
		mutate(e)
		e.Sign(priv)
		b, _ := e.MarshalBinary()
		var got Entry
		if err := got.UnmarshalBinary(b); !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: got %v, want ErrMalformed", name, err)
		}
	}
}

func TestNodeIDString(t *testing.T) {
	var id NodeID
	id[0], id[1], id[2], id[3] = 0xde, 0xad, 0xbe, 0xef
	if got := id.String(); got != "deadbeef" {
		t.Fatalf("String() = %q, want deadbeef", got)
	}
}

// One entry has one wire form: a padded varint (which still verifies) is
// refused, so a second copy of an entry cannot slip past duplicate suppression.
func TestUnmarshalRejectsNonCanonical(t *testing.T) {
	e := sampleEntry(newKey(t))
	b, _ := e.MarshalBinary()
	// The seq follows the version byte and the 32-byte origin, as a one-byte
	// varint here. Pad it to two bytes (0x80|v, 0x00 means the same number).
	i := 1 + len(e.Key.Origin)
	if b[i] >= 0x80 {
		t.Fatalf("expected a one-byte seq at byte %d, got %#x", i, b[i])
	}
	padded := append(append(append([]byte{}, b[:i]...), b[i]|0x80, 0x00), b[i+1:]...)
	var out Entry
	if err := out.UnmarshalBinary(padded); err == nil {
		t.Fatal("a padded varint was accepted")
	}
	if err := out.UnmarshalBinary(b); err != nil {
		t.Fatalf("the canonical form was refused: %v", err)
	}
}
