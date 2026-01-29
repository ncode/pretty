package sshConn

import "testing"

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
