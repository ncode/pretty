package sshConn

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
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

func TestWorkerSessionError(t *testing.T) {
	prevConn := connectionFunc
	prevSess := sessionFunc
	t.Cleanup(func() {
		connectionFunc = prevConn
		sessionFunc = prevSess
	})

	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return &ssh.Client{}, nil
	}
	sessionFunc = func(connection *ssh.Client, host *Host, stdout, stderr io.Writer) (io.WriteCloser, *ssh.Session, error) {
		return nil, nil, errors.New("session failed")
	}

	host := &Host{Hostname: "host1"}
	events := make(chan OutputEvent, 2)
	input := make(chan CommandRequest)
	close(input)

	worker(host, input, events)

	if atomic.LoadInt32(&host.IsConnected) != 0 {
		t.Fatal("expected host disconnected after session error")
	}

	select {
	case evt := <-events:
		if !evt.System {
			t.Fatal("expected system event")
		}
		if !strings.Contains(evt.Line, "session failed") {
			t.Fatalf("unexpected event line: %q", evt.Line)
		}
	default:
		t.Fatal("expected session error event")
	}
}

type errorWriteCloser struct{}

func (w *errorWriteCloser) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w *errorWriteCloser) Close() error { return nil }

func TestWorkerControlByteWriteError(t *testing.T) {
	prevConn := connectionFunc
	prevSess := sessionFunc
	t.Cleanup(func() {
		connectionFunc = prevConn
		sessionFunc = prevSess
	})

	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return &ssh.Client{}, nil
	}
	sessionFunc = func(connection *ssh.Client, host *Host, stdout, stderr io.Writer) (io.WriteCloser, *ssh.Session, error) {
		return &errorWriteCloser{}, nil, nil
	}

	host := &Host{Hostname: "host1"}
	events := make(chan OutputEvent, 2)
	input := make(chan CommandRequest, 1)
	input <- CommandRequest{Kind: CommandKindControl, JobID: 1, ControlByte: 0x03}
	close(input)

	worker(host, input, events)

	found := false
	for {
		select {
		case evt := <-events:
			if evt.System && strings.Contains(evt.Line, "unable to send control byte") {
				found = true
			}
		default:
			if !found {
				t.Fatal("expected system error event about control byte write failure")
			}
			return
		}
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

func BenchmarkBrokerDispatch(b *testing.B) {
	for _, mode := range []struct {
		name   string
		buffer int
	}{
		{name: "unbuffered", buffer: 0},
		{name: "buffered_1", buffer: 1},
	} {
		for _, hostCount := range []int{1, 8, 32, 128} {
			b.Run(fmt.Sprintf("%s/hosts_%d", mode.name, hostCount), func(b *testing.B) {
				benchmarkBrokerDispatchMode(b, hostCount, mode.buffer)
			})
		}
	}
}

func benchmarkBrokerDispatchMode(b *testing.B, hostCount, bufferSize int) {
	prevWorker := workerRunner
	defer func() {
		workerRunner = prevWorker
	}()

	hostList := NewHostList()
	for i := 0; i < hostCount; i++ {
		hostList.AddHost(&Host{
			Hostname:    fmt.Sprintf("host-%d", i),
			IsConnected: 1,
		})
	}

	var ready sync.WaitGroup
	var drained sync.WaitGroup
	ready.Add(hostCount)
	drained.Add(hostCount)

	workerRunner = func(host *Host, input <-chan CommandRequest, events chan<- OutputEvent) {
		ready.Done()
		for i := 0; i < b.N; i++ {
			<-input
		}
		drained.Done()
	}

	input := make(chan CommandRequest)
	done := make(chan struct{})
	go func() {
		brokerWithBuffer(hostList, input, nil, bufferSize)
		close(done)
	}()

	ready.Wait()

	request := CommandRequest{Kind: CommandKindRun, JobID: 1, Command: "date"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input <- request
	}
	b.StopTimer()

	close(input)
	<-done
	drained.Wait()
}

func brokerWithBuffer(hostList *HostList, input <-chan CommandRequest, events chan<- OutputEvent, bufferSize int) {
	for _, host := range hostList.Hosts() {
		host.Channel = make(chan CommandRequest, bufferSize)
		go workerRunner(host, host.Channel, events)
	}

	for request := range input {
		for _, host := range hostList.Hosts() {
			if atomic.LoadInt32(&host.IsConnected) == 1 {
				host.Channel <- request
			}
		}
	}
}
