package auth

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPIN(t *testing.T) {
	for _, p := range []string{"123456", "000000"} {
		if !ValidPIN(p) {
			t.Errorf("%q should be valid", p)
		}
	}
	for _, p := range []string{"", "12345", "1234567", "12a456", " 123456", "１２３４５６"} {
		if ValidPIN(p) {
			t.Errorf("%q should be invalid", p)
		}
	}
	h, err := HashPIN("482915")
	if err != nil || !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash %q, %v", h, err)
	}
	h2, _ := HashPIN("482915")
	if h == h2 {
		t.Fatal("two hashes of one PIN are equal: no salt")
	}
	if !CheckPIN(h, "482915") || CheckPIN(h, "482916") || CheckPIN("garbage", "482915") {
		t.Fatal("CheckPIN wrong")
	}
}

// B21: a hash from the log with its own, costlier parameters is refused, not run.
func TestCheckPINRefusesOtherParameters(t *testing.T) {
	h, _ := HashPIN("482915")
	costly := strings.Replace(h, "m=19456,t=2,p=1", "m=1048576,t=2,p=1", 1)
	start := time.Now()
	if CheckPIN(costly, "482915") {
		t.Fatal("a hash with foreign parameters was accepted")
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("a hash with foreign parameters was computed")
	}
	for _, bad := range []string{
		strings.Replace(h, "t=2", "t=1", 1),
		strings.Replace(h, "p=1", "p=2", 1),
		strings.Replace(h, "v=19", "v=16", 1),
	} {
		if CheckPIN(bad, "482915") {
			t.Errorf("accepted %q", bad)
		}
	}
}

// B22: made-up names don't grow the lockout table without bound.
func TestLockoutTableIsBounded(t *testing.T) {
	l := NewLockout()
	now := time.Unix(1_790_000_000, 0)
	for i := 0; i < 3*maxTracked; i++ {
		l.Fail(fmt.Sprintf("nobody-%d", i), now)
	}
	if len(l.fails) > maxTracked+1 {
		t.Fatalf("%d names tracked, want at most %d", len(l.fails), maxTracked+1)
	}
	for i := 0; i < 5; i++ {
		l.Fail("anna", now) // locked: must survive the pruning
	}
	for i := 0; i < 2*maxTracked; i++ {
		l.Fail(fmt.Sprintf("other-%d", i), now)
	}
	if l.Wait("anna", now) == 0 {
		t.Fatal("a live lockout was pruned")
	}
}

// Review focus 3.
func TestLockout(t *testing.T) {
	l := NewLockout()
	now := time.Unix(1_790_000_000, 0)
	for i := 0; i < 5; i++ {
		if l.Wait("anna", now) != 0 {
			t.Fatalf("locked after %d failures", i)
		}
		l.Fail("anna", now)
	}
	if w := l.Wait("anna", now); w != time.Minute {
		t.Fatalf("after 5 failures wait %v, want 1m", w)
	}
	if l.Wait("ben", now) != 0 {
		t.Fatal("another account locked")
	}
	now = now.Add(time.Minute)
	l.Fail("anna", now) // a sixth failure doubles
	if w := l.Wait("anna", now); w != 2*time.Minute {
		t.Fatalf("wait %v, want 2m", w)
	}
	l.Success("anna")
	if l.Wait("anna", now) != 0 {
		t.Fatal("success did not reset the lockout")
	}
}

func TestSessions(t *testing.T) {
	s := NewSessions()
	now := time.Unix(1_790_000_000, 0)
	tok := s.New("anna", now)
	if len(tok) < 40 {
		t.Fatalf("token %q too short", tok)
	}
	if a, ok := s.Get(tok, now.Add(300*24*time.Hour)); !ok || a != "anna" {
		t.Fatal("session lost within a year")
	}
	if _, ok := s.Get(tok, now.Add(366*24*time.Hour)); ok {
		t.Fatal("session alive after a year")
	}
	tok2 := s.New("ben", now)
	s.End(tok2)
	if _, ok := s.Get(tok2, now); ok {
		t.Fatal("ended session still valid")
	}
	if _, ok := s.Get("nonsense", now); ok {
		t.Fatal("unknown token valid")
	}
}
