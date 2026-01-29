package shell

import (
	"reflect"
	"testing"
)

func TestOutputBufferKeepsOrder(t *testing.T) {
	buf := newOutputBuffer(3)
	buf.Append("one", "two")
	if got, want := buf.Lines(), []string{"one", "two"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected lines: %#v", got)
	}
}

func TestOutputBufferCapsLines(t *testing.T) {
	buf := newOutputBuffer(2)
	buf.Append("one", "two", "three")
	if got, want := buf.Lines(), []string{"two", "three"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected lines: %#v", got)
	}
}
