package sshConn

import (
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"
)

type captureWriteCloser struct {
	buf []byte
}

func (w *captureWriteCloser) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *captureWriteCloser) Close() error { return nil }

func TestWorkerEmitsConnectionError(t *testing.T) {
	prevConnection := connectionFunc
	prevSession := sessionFunc
	t.Cleanup(func() {
		connectionFunc = prevConnection
		sessionFunc = prevSession
	})

	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return nil, errors.New("dial failed")
	}
	sessionFunc = prevSession

	host := &Host{Hostname: "host1"}
	events := make(chan OutputEvent, 1)
	input := make(chan CommandRequest)
	close(input)

	worker(host, input, events)

	select {
	case evt := <-events:
		if !evt.System {
			t.Fatalf("expected system event")
		}
		if !strings.Contains(evt.Line, "dial failed") {
			t.Fatalf("unexpected line: %q", evt.Line)
		}
	default:
		t.Fatalf("expected connection error event")
	}
}

func TestWorkerHandlesRequestsWithStubSession(t *testing.T) {
	prevConnection := connectionFunc
	prevSession := sessionFunc
	t.Cleanup(func() {
		connectionFunc = prevConnection
		sessionFunc = prevSession
	})

	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return &ssh.Client{}, nil
	}
	stdin := &captureWriteCloser{}
	sessionFunc = func(connection *ssh.Client, host *Host, stdout, stderr io.Writer) (io.WriteCloser, *ssh.Session, error) {
		return stdin, nil, nil
	}

	host := &Host{Hostname: "host1"}
	events := make(chan OutputEvent, 1)
	input := make(chan CommandRequest, 2)
	input <- CommandRequest{Kind: CommandKindRun, JobID: 7, Command: "uptime"}
	input <- CommandRequest{Kind: CommandKindControl, JobID: 8, ControlByte: 0x03}
	close(input)

	worker(host, input, events)

	written := string(stdin.buf)
	if !strings.Contains(written, "uptime\n") {
		t.Fatalf("expected command write, got %q", written)
	}
	if !strings.Contains(written, string([]byte{0x03})) {
		t.Fatalf("expected control byte write, got %q", written)
	}
	if atomic.LoadInt32(&host.IsConnected) != 1 {
		t.Fatalf("expected host connected")
	}
	if atomic.LoadInt32(&host.IsWaiting) != 0 {
		t.Fatalf("expected host not waiting")
	}
}

func TestBrokerDispatchesOnlyToConnectedHosts(t *testing.T) {
	prevWorker := workerRunner
	t.Cleanup(func() {
		workerRunner = prevWorker
	})

	dispatched := make(chan string, 2)
	workerRunner = func(host *Host, input <-chan CommandRequest, events chan<- OutputEvent) {
		request := <-input
		dispatched <- host.Hostname + ":" + request.Command
	}

	hostList := NewHostList()
	host1 := &Host{Hostname: "host1", IsConnected: 1}
	host2 := &Host{Hostname: "host2", IsConnected: 0}
	hostList.AddHost(host1)
	hostList.AddHost(host2)

	input := make(chan CommandRequest, 1)
	done := make(chan struct{})
	go func() {
		Broker(hostList, input, nil)
		close(done)
	}()

	input <- CommandRequest{Kind: CommandKindRun, JobID: 1, Command: "date"}
	close(input)
	<-done

	got := <-dispatched
	if got != "host1:date" {
		t.Fatalf("unexpected dispatch %q", got)
	}
	select {
	case extra := <-dispatched:
		t.Fatalf("unexpected extra dispatch: %s", extra)
	default:
	}
}
