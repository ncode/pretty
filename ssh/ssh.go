package ssh

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Host struct {
	Color       *color.Color
	Hostname    string
	IsConnected bool
	Channel     chan string
	IsWaiting   bool
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
		log.Fatalf("Unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("Unable to parse private key: %v", err)
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
		return nil, fmt.Errorf("Failed to dial: %s", err)
	}

	return connection, err
}

func Session(connection *ssh.Client) (session *ssh.Session, err error) {
	session, err = connection.NewSession()
	if err != nil {
		return nil, fmt.Errorf("Failed to create session: %s", err)
	}
	return session, err
}
