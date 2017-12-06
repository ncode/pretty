package sshConn

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func NewHostList() *HostList {
	hl := HostList{}
	hl.hosts = make([]*Host, 0)
	return &hl
}

type HostList struct {
	mu    sync.Mutex
	hosts []*Host
}

func (h *HostList) AddHost(host *Host) {
	h.mu.Lock()
	h.hosts = append(h.hosts, host)
	h.mu.Unlock()
}

func (h *HostList) Hosts() []*Host {
	return h.hosts
}

func (h *HostList) Len() int {
	return len(h.hosts)
}

func (h *HostList) State() (connected int, waiting int) {
	for _, host := range h.hosts {
		if atomic.LoadInt32(&host.IsConnected) == 1 {
			connected++
			if atomic.LoadInt32(&host.IsWaiting) == 1 {
				waiting++
			}
		}
	}
	return connected, waiting
}

type Host struct {
	Color       *color.Color
	Hostname    string
	IsConnected int32
	Channel     chan string
	ControlC    chan os.Signal
	IsWaiting   int32
}

func Agent() ssh.AuthMethod {
	if Agent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(Agent).Signers)
	}
	return nil
}

func PublicKeyFile(privateKey string) ssh.AuthMethod {
	key, err := ioutil.ReadFile(privateKey)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}
	return ssh.PublicKeys(signer)
}

func Connection(hostname string) (connection *ssh.Client, err error) {
	sshConfig := &ssh.ClientConfig{
		User: viper.GetString("username"),
		Auth: []ssh.AuthMethod{
			Agent(),
		},
		Timeout: 10 * time.Second,
	}

	connection, err = ssh.Dial("tcp", fmt.Sprintf("%s:22", hostname), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %s", err)
	}

	return connection, err
}

func Session(connection *ssh.Client, host *Host) (stdin io.WriteCloser, session *ssh.Session, err error) {
	session, err = connection.NewSession()
	if err != nil {
		return stdin, session, err
	}

	session.Stdout = NewProxyWriter(os.Stdout, host)
	session.Stderr = NewProxyWriter(os.Stderr, host)
	stdin, err = session.StdinPipe()
	if err != nil {
		return stdin, session, err
	}

	err = session.Shell()
	if err != nil {
		return stdin, session, err
	}

	return stdin, session, err
}
