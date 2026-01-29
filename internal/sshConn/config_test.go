package sshConn

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/crypto/ssh/agent"
)

func TestResolveHostPrecedence(t *testing.T) {
	cfg := "Host web\n  User deploy\n  Port 2222\n  HostName web.internal\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spec := HostSpec{Host: "web", Port: 22, PortSet: true}
	resolved, err := resolver.ResolveHost(spec, "current")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Host != "web.internal" || resolved.Port != 22 || resolved.User != "deploy" {
		t.Fatalf("unexpected resolved host: %+v", resolved)
	}
}

func TestResolveHostPatternMatchExact(t *testing.T) {
	cfg := "Host myserver\n  HostName 10.0.0.5\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved, err := resolver.ResolveHost(HostSpec{Host: "myserver"}, "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Host != "10.0.0.5" {
		t.Fatalf("unexpected resolved host: %+v", resolved)
	}
}

func TestResolveHostPatternMatchWildcard(t *testing.T) {
	cfg := "Host *.prod\n  User deploy\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved, err := resolver.ResolveHost(HostSpec{Host: "web.prod"}, "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.User != "deploy" {
		t.Fatalf("unexpected resolved user: %+v", resolved)
	}
}

func TestResolveHostFallbackUser(t *testing.T) {
	resolver, err := LoadSSHConfig(SSHConfigPaths{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolved, err := resolver.ResolveHost(HostSpec{Host: "host1"}, "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.User != "fallback" {
		t.Fatalf("unexpected resolved user: %+v", resolved)
	}
}

func TestLoadIdentityFilesFailFast(t *testing.T) {
	_, err := LoadIdentityFiles([]string{"/does/not/exist"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadIdentityFilesSuccess(t *testing.T) {
	keyPath := writeTempKey(t)
	methods, err := LoadIdentityFiles([]string{keyPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected one auth method, got %d", len(methods))
	}
}

func TestClientConfigForCombinedAuth(t *testing.T) {
	socketPath := startTestAgent(t)
	t.Setenv("SSH_AUTH_SOCK", socketPath)
	keyPath := writeTempKey(t)

	config, err := clientConfigFor(ResolvedHost{
		User:          "deploy",
		IdentityFiles: []string{keyPath},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(config.Auth) != 2 {
		t.Fatalf("expected agent + identity auth methods, got %d", len(config.Auth))
	}
}

func TestParseProxyJump(t *testing.T) {
	got := ParseProxyJump("jump1,jump2")
	want := []string{"jump1", "jump2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected jumps: %+v", got)
	}
}

func TestResolveProxyJumpSingle(t *testing.T) {
	cfg := "Host target\n  ProxyJump jump1\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolved, err := resolver.ResolveHost(HostSpec{Host: "target"}, "current")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(resolved.ProxyJump, []string{"jump1"}) {
		t.Fatalf("unexpected jumps: %+v", resolved.ProxyJump)
	}
}

func TestResolveProxyJumpChain(t *testing.T) {
	cfg := "Host target\n  ProxyJump jump1,jump2\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolved, err := resolver.ResolveHost(HostSpec{Host: "target"}, "current")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(resolved.ProxyJump, []string{"jump1", "jump2"}) {
		t.Fatalf("unexpected jumps: %+v", resolved.ProxyJump)
	}
}

func startTestAgent(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "pretty-agent")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	socketPath := filepath.Join(dir, "agent.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen on socket: %v", err)
	}
	keyring := agent.NewKeyring()
	done := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			go agent.ServeAgent(keyring, conn)
		}
	}()
	t.Cleanup(func() {
		close(done)
		listener.Close()
		_ = os.RemoveAll(dir)
	})
	return socketPath
}

func writeTempKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	data := pem.EncodeToMemory(block)
	if data == nil {
		t.Fatalf("failed to encode key")
	}
	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}
	return path
}

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}
