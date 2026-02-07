package sshConn

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestProxyWriterEmitsLines(t *testing.T) {
	events := make(chan OutputEvent, 1)
	host := &Host{Hostname: "host1"}
	w := NewProxyWriter(events, host, 99)
	_, _ = w.Write([]byte("hello\n"))
	select {
	case evt := <-events:
		if evt.Hostname != "host1" || evt.JobID != 99 || evt.Line != "hello" {
			t.Fatalf("unexpected event: %+v", evt)
		}
	default:
		t.Fatalf("expected output event")
	}
}

func TestProxyWriterBuffersPartialLine(t *testing.T) {
	events := make(chan OutputEvent, 2)
	host := &Host{Hostname: "host1"}
	w := NewProxyWriter(events, host, 99)

	_, _ = w.Write([]byte("hello"))
	select {
	case <-events:
		t.Fatalf("unexpected event for partial line")
	default:
	}

	_, _ = w.Write([]byte(" world\nnext\n"))
	evt1 := <-events
	evt2 := <-events
	if evt1.Line != "hello world" || evt2.Line != "next" {
		t.Fatalf("unexpected events: %#v %#v", evt1, evt2)
	}
}

func BenchmarkProxyWriterWrite(b *testing.B) {
	events := make(chan OutputEvent, 4096)
	host := &Host{Hostname: "host1"}
	w := NewProxyWriter(events, host, 99)
	payload := []byte("line1\nline2\nline3\n")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = w.Write(payload)
		for len(events) > 0 {
			<-events
		}
	}
}

func TestEmitSystemSendsEventWhenChannelExists(t *testing.T) {
	events := make(chan OutputEvent, 1)
	host := &Host{Hostname: "host1"}

	emitSystem(events, host, "failed")

	select {
	case evt := <-events:
		if !evt.System {
			t.Fatalf("expected system event")
		}
		if evt.Hostname != "host1" || evt.Line != "failed" {
			t.Fatalf("unexpected event: %+v", evt)
		}
	default:
		t.Fatalf("expected event")
	}
}

func TestEmitSystemPrintsWhenNoChannel(t *testing.T) {
	host := &Host{Hostname: "host1"}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("unexpected pipe error: %v", err)
	}
	os.Stdout = w

	emitSystem(nil, host, "hello")

	_ = w.Close()
	os.Stdout = oldStdout

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Fatalf("expected output to contain message, got %q", string(data))
	}
}
