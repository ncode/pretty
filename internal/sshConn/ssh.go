package sshConn

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
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
	Color         *color.Color
	Hostname      string
	Alias         string
	Host          string
	Port          int
	User          string
	IdentityFiles []string
	ProxyJump     []ResolvedHost
	IsConnected   int32
	Channel       chan CommandRequest
	ControlC      chan os.Signal
	IsWaiting     int32
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

func hostKeyCallback() ssh.HostKeyCallback {
	if path := viper.GetString("known_hosts"); path != "" {
		if callback, err := knownhosts.New(path); err == nil {
			return callback
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".ssh", "known_hosts")
		if callback, err := knownhosts.New(path); err == nil {
			return callback
		}
	}
	return ssh.InsecureIgnoreHostKey()
}

func dialAddress(host *Host) string {
	return net.JoinHostPort(host.Host, strconv.Itoa(host.Port))
}

func resolvedAddress(host ResolvedHost) string {
	return net.JoinHostPort(host.Host, strconv.Itoa(host.Port))
}

func Connection(host *Host) (connection *ssh.Client, err error) {
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if agent := Agent(); agent != nil {
		authMethods = append(authMethods, agent)
	}
	if len(host.IdentityFiles) > 0 {
		fileMethods, err := LoadIdentityFiles(host.IdentityFiles)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, fileMethods...)
	}

	sshConfig := &ssh.ClientConfig{
		User:            host.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	if len(host.ProxyJump) > 0 {
		target := ResolvedHost{
			Alias:         host.Alias,
			Host:          host.Host,
			Port:          host.Port,
			User:          host.User,
			IdentityFiles: host.IdentityFiles,
		}
		configs := map[string]*ssh.ClientConfig{
			host.Alias: sshConfig,
		}
		for _, jump := range host.ProxyJump {
			jumpConfig, err := clientConfigFor(jump)
			if err != nil {
				return nil, err
			}
			configs[jump.Alias] = jumpConfig
		}
		connection, err = dialWithJumps(target, host.ProxyJump, configs)
		if err != nil {
			return nil, fmt.Errorf("failed to dial: %s", err)
		}
		return connection, nil
	}

	connection, err = ssh.Dial("tcp", dialAddress(host), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %s", err)
	}

	return connection, err
}

func clientConfigFor(host ResolvedHost) (*ssh.ClientConfig, error) {
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if agent := Agent(); agent != nil {
		authMethods = append(authMethods, agent)
	}
	if len(host.IdentityFiles) > 0 {
		fileMethods, err := LoadIdentityFiles(host.IdentityFiles)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, fileMethods...)
	}

	return &ssh.ClientConfig{
		User:            host.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback(),
		Timeout:         10 * time.Second,
	}, nil
}

func dialWithJumps(target ResolvedHost, jumps []ResolvedHost, configs map[string]*ssh.ClientConfig) (*ssh.Client, error) {
	if len(jumps) == 0 {
		return ssh.Dial("tcp", resolvedAddress(target), configs[target.Alias])
	}

	client, err := ssh.Dial("tcp", resolvedAddress(jumps[0]), configs[jumps[0].Alias])
	if err != nil {
		return nil, err
	}

	for _, jump := range jumps[1:] {
		conn, err := client.Dial("tcp", resolvedAddress(jump))
		if err != nil {
			return nil, err
		}
		ncc, chans, reqs, err := ssh.NewClientConn(conn, resolvedAddress(jump), configs[jump.Alias])
		if err != nil {
			return nil, err
		}
		client = ssh.NewClient(ncc, chans, reqs)
	}

	conn, err := client.Dial("tcp", resolvedAddress(target))
	if err != nil {
		return nil, err
	}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, resolvedAddress(target), configs[target.Alias])
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(ncc, chans, reqs), nil
}

func Session(connection *ssh.Client, host *Host, stdout, stderr io.Writer) (stdin io.WriteCloser, session *ssh.Session, err error) {
	session, err = connection.NewSession()
	if err != nil {
		return stdin, session, err
	}

	session.Stdout = stdout
	session.Stderr = stderr
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
