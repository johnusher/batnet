package auth

import (
	"sync"
	"testing"
	"time"
)

// Review C2: parallel wrong PINs must not slip past the lockout, and long
// lockouts must not overflow back to zero.
func TestLockoutUnderParallelTries(t *testing.T) {
	l := NewLockout()
	now := time.Unix(1_790_000_000, 0)
	var mu sync.Mutex
	allowed := 0
	var wg sync.WaitGroup
	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Try("anna", now) == 0 {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed > 5 {
		t.Fatalf("%d parallel tries allowed, want at most 5", allowed)
	}
	for i := 0; i < 200; i++ {
		now = now.Add(l.Wait("anna", now) + time.Second)
		l.Try("anna", now)
	}
	if l.Wait("anna", now) <= 0 {
		t.Fatal("after many failures the lockout fell back to zero")
	}
}
