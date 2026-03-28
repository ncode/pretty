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

func Spawn(hostList *sshConn.HostList) {
	broker := make(chan sshConn.CommandRequest)
	hostCount := 0
	if hostList != nil {
		hostCount = hostList.Len()
	}
	events := make(chan sshConn.OutputEvent, outputBufferSize(hostCount))
	go sshConn.Broker(hostList, broker, events)

	p := tea.NewProgram(initialModel(hostList, broker, events))
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
