package shell

import "testing"

func TestOutputBufferSizeMinimum(t *testing.T) {
	if got := outputBufferSize(0); got != 128 {
		t.Fatalf("unexpected size: %d", got)
	}
}

func TestOutputBufferSizeScales(t *testing.T) {
	if got := outputBufferSize(10); got != 288 {
		t.Fatalf("unexpected size: %d", got)
	}
}
