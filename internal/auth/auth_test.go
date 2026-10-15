package auth

import (
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
