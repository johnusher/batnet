//go:build !linux

package gpio

import "errors"

// Line is one requested GPIO line (Linux only).
type Line struct{}

var errNoGPIO = errors.New("gpio: only on Linux")

// Output is Linux only.
func Output(chip string, offset int) (*Line, error) { return nil, errNoGPIO }

// Input is Linux only.
func Input(chip string, offset int) (*Line, error) { return nil, errNoGPIO }

// Set is Linux only.
func (l *Line) Set(on bool) error { return errNoGPIO }

// Get is Linux only.
func (l *Line) Get() (bool, error) { return false, errNoGPIO }

// Close is Linux only.
func (l *Line) Close() error { return nil }
