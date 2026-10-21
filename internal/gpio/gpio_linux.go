// Package gpio drives single GPIO lines through the Linux GPIO character
// device (/dev/gpiochipN, uAPI v2), in pure Go: an output for an LED, an input
// with a pull-up for a button.
package gpio

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// The v2 uAPI (linux/gpio.h).
const (
	flagActiveLow = 1 << 1
	flagInput     = 1 << 2
	flagOutput    = 1 << 3
	flagPullUp    = 1 << 8

	attrOutputValues = 2

	ioctlGetLine   = 0xC250B407 // GPIO_V2_GET_LINE_IOCTL, struct of 592 bytes
	ioctlGetValues = 0xC010B40E // GPIO_V2_LINE_GET_VALUES_IOCTL
	ioctlSetValues = 0xC010B40F // GPIO_V2_LINE_SET_VALUES_IOCTL
)

type lineAttribute struct {
	ID      uint32
	Padding uint32
	Value   uint64 // flags, output values or debounce period, by ID
}

type lineConfigAttribute struct {
	Attr lineAttribute
	Mask uint64
}

type lineConfig struct {
	Flags    uint64
	NumAttrs uint32
	Padding  [5]uint32
	Attrs    [10]lineConfigAttribute
}

type lineRequest struct {
	Offsets         [64]uint32
	Consumer        [32]byte
	Config          lineConfig
	NumLines        uint32
	EventBufferSize uint32
	Padding         [5]uint32
	Fd              int32
}

type lineValues struct {
	Bits uint64
	Mask uint64
}

// Line is one requested GPIO line.
type Line struct {
	f *os.File
}

// Output requests line offset on chip (e.g. "/dev/gpiochip0") as an output,
// starting low.
func Output(chip string, offset int) (*Line, error) {
	return request(chip, offset, flagOutput, true)
}

// Input requests line offset as an input with the pull-up on, active low: a
// button between the pin and ground reads true while pressed.
func Input(chip string, offset int) (*Line, error) {
	return request(chip, offset, flagInput|flagPullUp|flagActiveLow, false)
}

func request(chip string, offset int, flags uint64, out bool) (*Line, error) {
	if offset < 0 || offset > 63 {
		return nil, fmt.Errorf("gpio: line %d out of range", offset)
	}
	c, err := os.OpenFile(chip, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	var r lineRequest
	r.Offsets[0] = uint32(offset)
	copy(r.Consumer[:], "batnet")
	r.NumLines = 1
	r.Config.Flags = flags
	if out {
		r.Config.NumAttrs = 1
		r.Config.Attrs[0] = lineConfigAttribute{Attr: lineAttribute{ID: attrOutputValues, Value: 0}, Mask: 1}
	}
	if err := ioctl(c.Fd(), ioctlGetLine, unsafe.Pointer(&r)); err != nil {
		return nil, fmt.Errorf("gpio: line %d on %s: %w", offset, chip, err)
	}
	return &Line{f: os.NewFile(uintptr(r.Fd), fmt.Sprintf("%s line %d", chip, offset))}, nil
}

// Set drives an output line high (true) or low.
func (l *Line) Set(on bool) error {
	v := lineValues{Mask: 1}
	if on {
		v.Bits = 1
	}
	return ioctl(l.f.Fd(), ioctlSetValues, unsafe.Pointer(&v))
}

// Get reads the line (for an input: true while the button is pressed).
func (l *Line) Get() (bool, error) {
	v := lineValues{Mask: 1}
	if err := ioctl(l.f.Fd(), ioctlGetValues, unsafe.Pointer(&v)); err != nil {
		return false, err
	}
	return v.Bits&1 != 0, nil
}

// Close releases the line.
func (l *Line) Close() error { return l.f.Close() }

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

// sizes the uAPI fixes; checked by the test.
var (
	sizeLineRequest = unsafe.Sizeof(lineRequest{})
	sizeLineValues  = unsafe.Sizeof(lineValues{})
)
