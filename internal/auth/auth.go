// Package auth handles PINs (argon2id), lockouts after wrong PINs, and
// sessions (spec §8).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemKiB = 19456 // 19 MiB
	argonTime   = 2
	argonLanes  = 1
	sessionLife = 365 * 24 * time.Hour
	freeTries   = 5
)

var b64 = base64.RawStdEncoding

// ValidPIN: exactly six ASCII digits.
func ValidPIN(pin string) bool {
	if len(pin) != 6 {
		return false
	}
	for _, c := range []byte(pin) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// HashPIN returns an encoded argon2id hash with a random salt.
func HashPIN(pin string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(pin), salt, argonTime, argonMemKiB, argonLanes, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemKiB, argonTime, argonLanes, b64.EncodeToString(salt), b64.EncodeToString(h)), nil
}

// CheckPIN compares a PIN with an encoded hash in constant time.
func CheckPIN(encoded, pin string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || m > 1<<20 || t > 10 || p == 0 {
		return false
	}
	salt, err1 := b64.DecodeString(parts[4])
	want, err2 := b64.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// Lockout slows PIN guessing: 5 free tries per account, then 1 min, doubling.
type Lockout struct {
	mu    sync.Mutex
	fails map[string]int
	until map[string]time.Time
}

// NewLockout returns an empty lockout table.
func NewLockout() *Lockout { return &Lockout{fails: map[string]int{}, until: map[string]time.Time{}} }

// Wait is how long the account must wait before the next try (0: try now).
func (l *Lockout) Wait(account string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if u := l.until[account]; now.Before(u) {
		return u.Sub(now)
	}
	return 0
}

// Fail records a wrong PIN.
func (l *Lockout) Fail(account string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[account]++
	if n := l.fails[account]; n >= freeTries {
		l.until[account] = now.Add(time.Minute << (n - freeTries))
	}
}

// Success clears the account's failures.
func (l *Lockout) Success(account string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, account)
	delete(l.until, account)
}

// Sessions maps random tokens to accounts for a year.
type Sessions struct {
	mu sync.Mutex
	m  map[string]session
}

type session struct {
	account string
	expires time.Time
}

// NewSessions returns an empty session table.
func NewSessions() *Sessions { return &Sessions{m: map[string]session{}} }

// New starts a session and returns its token.
func (s *Sessions) New(account string, now time.Time) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[tok] = session{account: account, expires: now.Add(sessionLife)}
	return tok
}

// Get returns the session's account.
func (s *Sessions) Get(token string, now time.Time) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, ok := s.m[token]
	if !ok || now.After(x.expires) {
		return "", false
	}
	return x.account, true
}

// End signs a session out.
func (s *Sessions) End(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, token)
}
