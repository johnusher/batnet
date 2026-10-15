package seal

import (
	"bytes"
	"errors"
	"testing"
)

func mustKey(t *testing.T) VillageKey {
	t.Helper()
	k, err := NewVillageKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpen(t *testing.T) {
	s, _ := NewSealer(mustKey(t), PurposeMesh)
	box, err := s.Seal([]byte("hello village"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(box, []byte("hello")) {
		t.Fatal("plaintext visible in the box")
	}
	got, err := s.Open(box)
	if err != nil || string(got) != "hello village" {
		t.Fatalf("Open = %q, %v", got, err)
	}
}

func TestOtherVillageCannotOpen(t *testing.T) {
	ours, _ := NewSealer(mustKey(t), PurposeMesh)
	theirs, _ := NewSealer(mustKey(t), PurposeMesh)
	box, _ := ours.Seal([]byte("private"))
	if _, err := theirs.Open(box); !errors.Is(err, ErrOpen) {
		t.Fatalf("got %v, want ErrOpen", err)
	}
}

func TestPurposesSeparateKeys(t *testing.T) {
	k := mustKey(t)
	if k.Derive(PurposeMesh) == k.Derive("batnet lora v1") {
		t.Fatal("two purposes derived the same key")
	}
	mesh, _ := NewSealer(k, PurposeMesh)
	lora, _ := NewSealer(k, "batnet lora v1")
	box, _ := mesh.Seal([]byte("x"))
	if _, err := lora.Open(box); !errors.Is(err, ErrOpen) {
		t.Fatalf("a box for one purpose opened under another: %v", err)
	}
}

func TestOpenRejectsShortAndTampered(t *testing.T) {
	s, _ := NewSealer(mustKey(t), PurposeMesh)
	if _, err := s.Open([]byte{1, 2, 3}); !errors.Is(err, ErrOpen) {
		t.Fatalf("short box: %v", err)
	}
	box, _ := s.Seal([]byte("message"))
	box[len(box)-1] ^= 1
	if _, err := s.Open(box); !errors.Is(err, ErrOpen) {
		t.Fatalf("tampered box: %v", err)
	}
}
