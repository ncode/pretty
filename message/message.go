package message

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/ncode/pretty/ssh"
)

func worker(host *ssh.Host, input <-chan string) {
	connection, err := ssh.Connection(host.Hostname)
	if err != nil {
		fmt.Printf("Error connection to host %s: %v\n", host.Hostname, err)
	} else {
		atomic.StoreInt32(&host.IsConnected, 1)
	}
	for command := range input {
		session, err := ssh.Session(connection)
		if err != nil {
			fmt.Printf("Unable to open session: %v\n", err)
			atomic.StoreInt32(&host.IsConnected, 0)
			continue
		}

		atomic.StoreInt32(&host.IsWaiting, 1)
		output, _ := session.CombinedOutput(string(command))
		for pos, l := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
			if pos == 0 {
				fmt.Printf("\r")
			}
			host.Color.Printf("%s: %s\n", host.Hostname, l)
		}
		atomic.StoreInt32(&host.IsWaiting, 0)
	}
}

func Broker(hostList *ssh.HostList, input <-chan string) {
	for _, host := range hostList.Hosts() {
		host.Channel = make(chan string)
		go worker(host, host.Channel)
	}

	for cmd := range input {
		for _, host := range hostList.Hosts() {
			if atomic.LoadInt32(&host.IsConnected) == 1 {
				host.Channel <- cmd
			}
		}
	}
}
