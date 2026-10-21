package hub

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner runs a command and returns what it printed (stdout and stderr). The
// hub's parts take one so tests can pretend to be nmcli, systemctl or aplay.
type Runner func(ctx context.Context, name string, args ...string) (string, error)

// Exec is the real Runner.
func Exec(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil && s != "" {
		err = fmt.Errorf("%s", firstLine(s))
	}
	return s, err
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// runFor runs name with a time limit.
func runFor(run Runner, d time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return run(ctx, name, args...)
}

// StartUnit starts (or with restart, restarts) a systemd unit batnetd may
// start through polkit (deploy/pi/50-batnet.rules).
func StartUnit(run Runner, unit string, restart bool) error {
	verb := "start"
	if restart {
		verb = "restart"
	}
	return Systemctl(run, verb, unit)
}

// Systemctl runs systemctl verb unit (start, restart, stop), without waiting
// for a password prompt.
func Systemctl(run Runner, verb, unit string) error {
	_, err := runFor(run, 30*time.Second, "systemctl", "--no-ask-password", verb, unit)
	return err
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
