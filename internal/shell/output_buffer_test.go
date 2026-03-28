package shell

import (
	"reflect"
	"testing"
)

func TestOutputBufferZeroCapacity(t *testing.T) {
	buf := newOutputBuffer(0)
	buf.Append("one", "two", "three")

	if lines := buf.Lines(); lines != nil {
		t.Fatalf("expected nil lines, got %#v", lines)
	}
	if s := buf.String(); s != "" {
		t.Fatalf("expected empty string, got %q", s)
	}
}

func TestOutputBufferStringJoinsWithNewlines(t *testing.T) {
	buf := newOutputBuffer(10)
	buf.Append("alpha", "beta", "gamma")

	want := "alpha\nbeta\ngamma"
	if got := buf.String(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestOutputBufferKeepsOrder(t *testing.T) {
	buf := newOutputBuffer(3)
	buf.Append("one", "two")
	if got, want := buf.Lines(), []string{"one", "two"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected lines: %#v", got)
	}
}

func TestOutputBufferNegativeMax(t *testing.T) {
	buf := newOutputBuffer(-1)
	buf.Append("one", "two")

	if buf.max != 0 {
		t.Fatalf("expected max=0 for negative input, got %d", buf.max)
	}
	if lines := buf.Lines(); lines != nil {
		t.Fatalf("expected nil lines, got %#v", lines)
	}
	if s := buf.String(); s != "" {
		t.Fatalf("expected empty string, got %q", s)
	}
}

func TestOutputBufferCapsLines(t *testing.T) {
	buf := newOutputBuffer(2)
	buf.Append("one", "two", "three")
	if got, want := buf.Lines(), []string{"two", "three"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected lines: %#v", got)
	}
}
