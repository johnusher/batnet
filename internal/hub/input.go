package hub

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"
)

// WatchInput calls onActivity (at most twice a second) when any keyboard,
// mouse or touch screen under dir (/dev/input) is used, picking up devices
// plugged in later. It only listens: the kiosk still gets every key. The hub
// needs the input group.
func WatchInput(ctx context.Context, dir string, onActivity func()) {
	var mu sync.Mutex
	open := map[string]*os.File{}
	var last time.Time
	activity := func() {
		mu.Lock()
		now := time.Now()
		fire := now.Sub(last) >= 500*time.Millisecond
		if fire {
			last = now
		}
		mu.Unlock()
		if fire {
			onActivity()
		}
	}
	// struct input_event: a timeval (two longs), then type, code (uint16) and value (int32).
	size := int(2*unsafe.Sizeof(uintptr(0))) + 8
	read := func(path string, f *os.File) {
		buf := make([]byte, size*16)
		for {
			n, err := f.Read(buf)
			if err != nil {
				mu.Lock()
				delete(open, path)
				mu.Unlock()
				f.Close()
				return
			}
			for i := 0; i+size <= n; i += size {
				switch binary.LittleEndian.Uint16(buf[i+size-8:]) {
				case 1, 2, 3: // EV_KEY, EV_REL, EV_ABS
					activity()
				}
			}
		}
	}
	defer func() {
		mu.Lock()
		for _, f := range open {
			f.Close()
		}
		mu.Unlock()
	}()
	for {
		paths, _ := filepath.Glob(filepath.Join(dir, "event*"))
		for _, p := range paths {
			mu.Lock()
			_, ok := open[p]
			mu.Unlock()
			if ok {
				continue
			}
			f, err := os.Open(p)
			if err != nil {
				continue
			}
			mu.Lock()
			open[p] = f
			mu.Unlock()
			go read(p, f)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
