package sshConn

import (
	"bytes"
	"fmt"
	"sync/atomic"
)

type ProxyWriter struct {
	events chan<- OutputEvent
	host   *Host
	jobID  int
	system bool
	buf    []byte
}

func NewProxyWriter(events chan<- OutputEvent, host *Host, jobID int) *ProxyWriter {
	return &ProxyWriter{
		events: events,
		host:   host,
		jobID:  jobID,
	}
}

func (w *ProxyWriter) Write(output []byte) (int, error) {
	if w.events == nil {
		return len(output), nil
	}

	w.buf = append(w.buf, output...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx == -1 {
			break
		}
		line := w.buf[:idx]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		w.events <- OutputEvent{
			JobID:    w.jobID,
			Hostname: w.host.Hostname,
			Line:     string(line),
			System:   w.system,
		}
		w.buf = w.buf[idx+1:]
	}
	return len(output), nil
}

func emitSystem(events chan<- OutputEvent, host *Host, line string) {
	if events == nil {
		fmt.Println(line)
		return
	}

	events <- OutputEvent{
		Hostname: host.Hostname,
		Line:     line,
		System:   true,
	}
}

func worker(host *Host, input <-chan CommandRequest, events chan<- OutputEvent) {
	connection, err := Connection(host)
	if err != nil {
		emitSystem(events, host, fmt.Sprintf("error connection to host %s: %v", host.Hostname, err))
		return
	} else {
		atomic.StoreInt32(&host.IsConnected, 1)
	}
	stdoutWriter := NewProxyWriter(events, host, 0)
	stderrWriter := NewProxyWriter(events, host, 0)
	stderrWriter.system = true
	stdin, session, err := Session(connection, host, stdoutWriter, stderrWriter)
	if err != nil {
		emitSystem(events, host, fmt.Sprintf("unable to open session: %v", err))
		atomic.StoreInt32(&host.IsConnected, 0)
		return
	}
	_ = session

	for request := range input {
		atomic.StoreInt32(&host.IsWaiting, 1)
		stdoutWriter.jobID = request.JobID
		stderrWriter.jobID = request.JobID
		if request.Kind == CommandKindControl {
			if request.ControlByte != 0 {
				if _, err := stdin.Write([]byte{request.ControlByte}); err != nil {
					emitSystem(events, host, fmt.Sprintf("unable to send control byte: %v", err))
				}
			}
		} else {
			fmt.Fprintf(stdin, "%s\n", request.Command)
		}
		atomic.StoreInt32(&host.IsWaiting, 0)
	}
}

func Broker(hostList *HostList, input <-chan CommandRequest, events chan<- OutputEvent) {
	for _, host := range hostList.Hosts() {
		host.Channel = make(chan CommandRequest)
		go worker(host, host.Channel, events)
	}

	for request := range input {
		for _, host := range hostList.Hosts() {
			if atomic.LoadInt32(&host.IsConnected) == 1 {
				host.Channel <- request
			}
		}
	}
}
