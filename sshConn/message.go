package sshConn

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"golang.org/x/crypto/ssh"
)

type ProxyWriter struct {
	file *os.File
	host *Host
}

func NewProxyWriter(file *os.File, host *Host) *ProxyWriter {
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

func worker(host *Host, input <-chan string) {
	connection, err := Connection(host.Hostname)
	if err != nil {
		fmt.Printf("error connection to host %s: %v\n", host.Hostname, err)
		return
	} else {
		atomic.StoreInt32(&host.IsConnected, 1)
	}
	stdin, session, err := Session(connection, host)
	if err != nil {
		fmt.Printf("unable to open session: %v\n", err)
		atomic.StoreInt32(&host.IsConnected, 0)
		return
	}

	for command := range input {
		atomic.StoreInt32(&host.IsWaiting, 1)
		// TODO: This is unfortunately not supported by OpenSSH yet.
		//  ref: https://github.com/golang/go/issues/16597
		//  ref: https://bugzilla.mindrot.org/show_bug.cgi?id=1424
		//  I will need to look for a less classy solution :(
		if command == "ˆC" {
			session.Signal(ssh.SIGINT)
		} else {
			fmt.Fprint(stdin, fmt.Sprintf("%s\n", command))
		}
		atomic.StoreInt32(&host.IsWaiting, 0)
	}
}

func Broker(hostList *HostList, input <-chan string, sent chan <- bool) {
	for _, host := range hostList.Hosts() {
		host.Channel = make(chan string)
		go worker(host, host.Channel)
	}

	for cmd := range input {
		for _, host := range hostList.Hosts() {
			if atomic.LoadInt32(&host.IsConnected) == 1 {
				host.Channel <- cmd
				sent <- true
			}
		}
	}
}
