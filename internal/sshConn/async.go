package sshConn

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

func RunCommand(host *Host, command string, jobID int, events chan<- OutputEvent) (int, error) {
	connection, err := Connection(host)
	if err != nil {
		emitSystem(events, host, fmt.Sprintf("error connection to host %s: %v", host.Hostname, err))
		return 1, err
	}
	defer connection.Close()

	session, err := connection.NewSession()
	if err != nil {
		emitSystem(events, host, fmt.Sprintf("unable to open session: %v", err))
		return 1, err
	}
	defer session.Close()

	stdoutWriter := NewProxyWriter(events, host, jobID)
	stderrWriter := NewProxyWriter(events, host, jobID)
	stderrWriter.system = true
	session.Stdout = stdoutWriter
	session.Stderr = stderrWriter

	err = session.Run(command)
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*ssh.ExitError); ok {
		return exitErr.ExitStatus(), nil
	}
	emitSystem(events, host, fmt.Sprintf("command failed on %s: %v", host.Hostname, err))
	return 1, err
}
