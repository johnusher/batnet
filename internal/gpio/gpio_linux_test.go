package gpio

import "testing"

// The ioctl numbers carry the struct sizes: they must match linux/gpio.h.
func TestSizes(t *testing.T) {
	if sizeLineRequest != 592 {
		t.Errorf("gpio_v2_line_request is %d bytes, want 592", sizeLineRequest)
	}
	if sizeLineValues != 16 {
		t.Errorf("gpio_v2_line_values is %d bytes, want 16", sizeLineValues)
	}
	if got := (ioctlGetLine >> 16) & 0x3fff; got != 592 {
		t.Errorf("GET_LINE size field %d", got)
	}
}
