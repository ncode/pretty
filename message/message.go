package message

import (
	"fmt"
	"strings"

	"github.com/ncode/pretool/ssh"
)

func worker(host *ssh.Host, input <-chan string) {
	connection, err := ssh.Connection(host.Hostname)
	if err == nil {
		host.IsConnected = true
	}

	for command := range input {
		session, err := ssh.Session(connection)
		if err != nil {
			fmt.Printf("Unable to open session: %v\n", err)
		}

		output, _ := session.CombinedOutput(string(command))
		for pos, l := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
			if pos == 0 {
				fmt.Printf("\n")
			}
			host.Color.Printf("%s: %s\n", host.Hostname, l)
		}
		fmt.Printf("pretool>> ")
	}
}

func Broker(hosts []*ssh.Host, input <-chan string, status chan<- string) {
	for _, host := range hosts {
		host.Channel = make(chan string)
		go worker(host, host.Channel)
	}

	for cmd := range input {
		for _, host := range hosts {
			host.Channel <- cmd
		}
		status <- "done"
	}
}
