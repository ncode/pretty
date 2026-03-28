package shell

import (
	tea "charm.land/bubbletea/v2"
	"github.com/ncode/pretty/internal/sshConn"
)

const (
	minOutputBuffer     = 128
	outputBufferPerHost = 16
)

func outputBufferSize(hostCount int) int {
	size := minOutputBuffer + hostCount*outputBufferPerHost
	if size < minOutputBuffer {
		return minOutputBuffer
	}
	return size
}

var runProgram = func(m tea.Model) (tea.Model, error) {
	return tea.NewProgram(m).Run()
}

func Spawn(hostList *sshConn.HostList) {
	broker := make(chan sshConn.CommandRequest)
	hostCount := 0
	if hostList != nil {
		hostCount = hostList.Len()
	}
	events := make(chan sshConn.OutputEvent, outputBufferSize(hostCount))
	go sshConn.Broker(hostList, broker, events)

	if _, err := runProgram(initialModel(hostList, broker, events)); err != nil {
		panic(err)
	}
}
