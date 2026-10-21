package meshtastic

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Unreadable is a packet meshtasticd heard but threw away, usually because
// it failed its checksum: the signal was too weak to read. meshtasticd never
// passes these to its API, only to its own log, so WatchJournal reads them
// from there. The header fields are as received and may be garbled too.
type Unreadable struct {
	At       time.Time `json:"t"`
	Error    int       `json:"error"` // RadioLib's code: -7 is a failed checksum
	ID       uint32    `json:"id"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	Hops     int       `json:"hops"` // relays it had passed, from its header
	HopStart int       `json:"hopStart"`
	SNR      float64   `json:"snr"`
	RSSI     int       `json:"rssi"`
	Relay    string    `json:"relay,omitempty"` // the last byte of the radio that passed it on
}

var (
	ignoredRe = regexp.MustCompile(`Ignore received packet due to error=(-?\d+) \((.*)\)`)
	kvRe      = regexp.MustCompile(`(\w+)=(\S+)`)
)

// ParseUnreadable reads one meshtasticd log line such as
//
//	ERROR | 13:59:57 2873 [RadioIf] Ignore received packet due to error=-7 (maybe id=0x1 fr=0xdeadbeef to=0xffffffff flags=0x63 rxSNR=-20.75 rxRSSI=-94 nextHop=0x0 relay=0xef)
func ParseUnreadable(line string) (Unreadable, bool) {
	m := ignoredRe.FindStringSubmatch(line)
	if m == nil {
		return Unreadable{}, false
	}
	u := Unreadable{}
	u.Error, _ = strconv.Atoi(m[1])
	hex := func(s string) uint64 { v, _ := strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 32); return v }
	for _, kv := range kvRe.FindAllStringSubmatch(m[2], -1) {
		switch v := kv[2]; kv[1] {
		case "id":
			u.ID = uint32(hex(v))
		case "fr":
			u.From = fmt.Sprintf("!%08x", hex(v))
		case "to":
			if n := uint32(hex(v)); n == Broadcast {
				u.To = "everyone"
			} else {
				u.To = fmt.Sprintf("!%08x", n)
			}
		case "flags":
			f := int(hex(v))
			u.HopStart = f >> 5 & 7
			u.Hops = u.HopStart - f&7
		case "rxSNR":
			u.SNR, _ = strconv.ParseFloat(v, 64)
		case "rxRSSI":
			u.RSSI, _ = strconv.Atoi(v)
		case "relay":
			if n := hex(v); n != 0 {
				u.Relay = fmt.Sprintf("%02x", n)
			}
		}
	}
	return u, true
}

// WatchJournal follows meshtasticd's log in the systemd journal and calls
// on for every packet it heard but could not read, until ctx ends. The user
// running it needs to be in the systemd-journal group.
func WatchJournal(ctx context.Context, unit string, on func(Unreadable)) {
	warned := false
	for ctx.Err() == nil {
		cmd := exec.CommandContext(ctx, "journalctl", "-u", unit, "-f", "-n", "0", "-o", "cat")
		out, err := cmd.StdoutPipe()
		if err == nil {
			err = cmd.Start()
		}
		if err == nil {
			sc := bufio.NewScanner(out)
			for sc.Scan() {
				if u, ok := ParseUnreadable(sc.Text()); ok {
					u.At = time.Now()
					on(u)
				}
			}
			err = cmd.Wait()
		}
		if ctx.Err() != nil {
			return
		}
		if !warned {
			log.Printf("meshtasticd journal: %v (unreadable packets are not logged)", err)
			warned = true
		}
		select {
		case <-ctx.Done():
		case <-time.After(30 * time.Second):
		}
	}
}
