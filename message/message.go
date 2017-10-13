package message

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/ncode/pretty/ssh"
)

type ProxyWriter struct {
	file *os.File
	host *ssh.Host
}

func NewProxyWriter(file *os.File, host *ssh.Host) *ProxyWriter {
	return &ProxyWriter{
		file: file,
		host: host,
	}
}

func (w *ProxyWriter) Write(output []byte) (int, error) {
	for pos, l := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		if pos == 0 {
			fmt.Printf("\r")
		}
		w.host.Color.Printf("%s: %s\n", w.host.Hostname, l)
	}
	return len(output), nil
}

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
		session.Stdout = NewProxyWriter(os.Stdout, host)
		session.Stderr = NewProxyWriter(os.Stderr, host)
		err = session.Start(string(command))
		if err != nil {
			fmt.Println(err)
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
