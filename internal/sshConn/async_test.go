package sshConn

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// testSSHClient starts a minimal SSH server on a random TCP port and returns
// a connected *ssh.Client. The handler callback is invoked for each new
// channel (typically "session") opened by the client. The listener is closed
// when the test completes.
func testSSHClient(t *testing.T, handler func(ch ssh.NewChannel)) *ssh.Client {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}

	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		sConn, chans, reqs, err := ssh.NewServerConn(conn, serverConfig)
		if err != nil {
			conn.Close()
			return
		}
		go ssh.DiscardRequests(reqs)
		for newCh := range chans {
			handler(newCh)
		}
		sConn.Close()
	}()

	clientConfig := &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", ln.Addr().String(), clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestRunCommandConnectionError(t *testing.T) {
	prevConn := connectionFunc
	t.Cleanup(func() { connectionFunc = prevConn })

	wantErr := errors.New("connect failed")
	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return nil, wantErr
	}

	host := &Host{Hostname: "test-host"}
	events := make(chan OutputEvent, 1)
	exitCode, err := RunCommand(host, "uptime", 1, events)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped connect error, got %v", err)
	}

	select {
	case evt := <-events:
		if !evt.System {
			t.Fatal("expected system event")
		}
		if !strings.Contains(evt.Line, "connect failed") {
			t.Fatalf("expected error in event line, got %q", evt.Line)
		}
		if !strings.Contains(evt.Line, "test-host") {
			t.Fatalf("expected hostname in event line, got %q", evt.Line)
		}
	default:
		t.Fatal("expected system error event")
	}
}

func TestRunCommandSessionError(t *testing.T) {
	prevConn := connectionFunc
	t.Cleanup(func() { connectionFunc = prevConn })

	// Use testSSHClient with a handler that rejects the "session" channel.
	// This causes connection.NewSession() to return an error.
	handler := func(ch ssh.NewChannel) {
		ch.Reject(ssh.Prohibited, "no sessions allowed")
	}

	client := testSSHClient(t, handler)
	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return client, nil
	}

	host := &Host{Hostname: "sess-err-host"}
	events := make(chan OutputEvent, 4)
	exitCode, runErr := RunCommand(host, "whoami", 2, events)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if runErr == nil {
		t.Fatal("expected error from NewSession, got nil")
	}

	select {
	case evt := <-events:
		if !evt.System {
			t.Fatal("expected system event")
		}
		if !strings.Contains(evt.Line, "unable to open session") {
			t.Fatalf("unexpected event line: %q", evt.Line)
		}
	default:
		t.Fatal("expected system error event for session failure")
	}
}

func TestRunCommandSuccess(t *testing.T) {
	prevConn := connectionFunc
	t.Cleanup(func() { connectionFunc = prevConn })

	handler := func(ch ssh.NewChannel) {
		channel, reqs, err := ch.Accept()
		if err != nil {
			return
		}
		go func() {
			for req := range reqs {
				if req.Type == "exec" {
					req.Reply(true, nil)
					channel.Write([]byte("hello\n"))
					channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					channel.Close()
					return
				}
				req.Reply(false, nil)
			}
		}()
		go io.Copy(io.Discard, channel)
	}

	client := testSSHClient(t, handler)
	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return client, nil
	}

	host := &Host{Hostname: "success-host"}
	events := make(chan OutputEvent, 8)
	exitCode, err := RunCommand(host, "echo hello", 3, events)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	// Drain events; we should see the "hello" line.
	found := false
	for len(events) > 0 {
		evt := <-events
		if strings.Contains(evt.Line, "hello") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected stdout event containing 'hello'")
	}
}

func TestRunCommandExitError(t *testing.T) {
	prevConn := connectionFunc
	t.Cleanup(func() { connectionFunc = prevConn })

	handler := func(ch ssh.NewChannel) {
		channel, reqs, err := ch.Accept()
		if err != nil {
			return
		}
		go func() {
			for req := range reqs {
				if req.Type == "exec" {
					req.Reply(true, nil)
					channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{42}))
					channel.Close()
					return
				}
				req.Reply(false, nil)
			}
		}()
		go io.Copy(io.Discard, channel)
	}

	client := testSSHClient(t, handler)
	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return client, nil
	}

	host := &Host{Hostname: "exit-err-host"}
	events := make(chan OutputEvent, 4)
	exitCode, err := RunCommand(host, "false", 4, events)

	if err != nil {
		t.Fatalf("ExitError should not propagate as error, got: %v", err)
	}
	if exitCode != 42 {
		t.Fatalf("expected exit code 42, got %d", exitCode)
	}
}

func TestRunCommandNonExitError(t *testing.T) {
	prevConn := connectionFunc
	t.Cleanup(func() { connectionFunc = prevConn })

	handler := func(ch ssh.NewChannel) {
		// Accept the channel then immediately close the underlying
		// transport without sending an exit-status. This forces
		// session.Run to return a non-ExitError.
		channel, _, err := ch.Accept()
		if err != nil {
			return
		}
		channel.Close()
	}

	client := testSSHClient(t, handler)
	connectionFunc = func(host *Host) (*ssh.Client, error) {
		return client, nil
	}

	host := &Host{Hostname: "non-exit-host"}
	events := make(chan OutputEvent, 4)
	exitCode, err := RunCommand(host, "boom", 5, events)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if err == nil {
		t.Fatal("expected non-nil error for non-ExitError failure")
	}

	// Check for the "command failed" system event.
	found := false
	for len(events) > 0 {
		evt := <-events
		if evt.System && strings.Contains(evt.Line, "command failed") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected system event with 'command failed'")
	}
}
