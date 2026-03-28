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

func TestResolveHostAppliesMatchValues(t *testing.T) {
	cfg := "Host web\n  HostName web.internal\nMatch host=web.internal\n  User matched\n  Port 2201\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved, err := resolver.ResolveHost(HostSpec{Host: "web"}, "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.User != "matched" {
		t.Fatalf("expected match user, got %+v", resolved)
	}
	if resolved.Port != 2201 {
		t.Fatalf("expected match port, got %+v", resolved)
	}
}

func TestResolveHostMatchNonMatchingBlockDoesNotApply(t *testing.T) {
	cfg := "Host web\n  User host-user\nMatch host=db\n  User match-user\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved, err := resolver.ResolveHost(HostSpec{Host: "web"}, "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.User != "host-user" {
		t.Fatalf("unexpected resolved user: %+v", resolved)
	}
}

func TestResolveHostExplicitOverridesMatch(t *testing.T) {
	cfg := "Host web\n  User base\n  Port 2222\nMatch host=web\n  User matched\n  Port 2201\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spec := HostSpec{Host: "web", User: "explicit", UserSet: true, Port: 2022, PortSet: true}
	resolved, err := resolver.ResolveHost(spec, "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.User != "explicit" || resolved.Port != 2022 {
		t.Fatalf("explicit values should win, got %+v", resolved)
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

func TestGetValueUserConfigHit(t *testing.T) {
	userCfg := writeTempConfig(t, "Host web\n  HostName 10.0.0.1\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, err := resolver.getValue("web", "HostName")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %q", val)
	}
}

func TestGetValueFallsToSystem(t *testing.T) {
	userCfg := writeTempConfig(t, "Host web\n  User deploy\n")
	systemCfg := writeTempConfig(t, "Host web\n  ProxyJump bastion\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg, System: systemCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, err := resolver.getValue("web", "ProxyJump")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "bastion" {
		t.Fatalf("expected bastion from system config, got %q", val)
	}
}

func TestGetValueBothMissReturnsEmpty(t *testing.T) {
	userCfg := writeTempConfig(t, "Host web\n  User deploy\n")
	systemCfg := writeTempConfig(t, "Host db\n  Port 3333\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg, System: systemCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, err := resolver.getValue("web", "ProxyJump")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestGetAllValuesMergesBothConfigs(t *testing.T) {
	userCfg := writeTempConfig(t, "Host web\n  IdentityFile ~/.ssh/user_key\n")
	systemCfg := writeTempConfig(t, "Host web\n  IdentityFile ~/.ssh/system_key\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg, System: systemCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vals, err := resolver.getAllValues("web", "IdentityFile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d: %v", len(vals), vals)
	}
}

func TestGetAllValuesFiltersBlank(t *testing.T) {
	userCfg := writeTempConfig(t, "Host web\n  User deploy\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vals, err := resolver.getAllValues("web", "IdentityFile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, v := range vals {
		if v == "" {
			t.Fatalf("blank value should be filtered out")
		}
	}
}

func TestExpandPathEmpty(t *testing.T) {
	if got := expandPath(""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExpandPathTildeAlone(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	if got := expandPath("~"); got != home {
		t.Fatalf("expected %q, got %q", home, got)
	}
}

func TestExpandPathTildeSubdir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	want := filepath.Join(home, "subdir")
	if got := expandPath("~/subdir"); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestExpandPathAbsoluteUnchanged(t *testing.T) {
	if got := expandPath("/tmp/foo"); got != "/tmp/foo" {
		t.Fatalf("expected /tmp/foo, got %q", got)
	}
}

func TestCurrentUserReturnsNonEmpty(t *testing.T) {
	u := currentUser()
	if u == "" {
		t.Fatal("expected non-empty username from currentUser()")
	}
}

func TestLoadSSHConfigEmptyPaths(t *testing.T) {
	resolver, err := LoadSSHConfig(SSHConfigPaths{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestLoadConfigEmptyPath(t *testing.T) {
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for empty path")
	}
}

func TestLoadConfigNonExistentPath(t *testing.T) {
	cfg, err := loadConfig("/nonexistent/config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for non-existent path")
	}
}

func TestResolveNilConfig(t *testing.T) {
	resolver := &SSHConfigResolver{}
	result, err := resolver.resolve(nil, "web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for nil config")
	}
}

func TestGetValueWithNilConfigs(t *testing.T) {
	resolver := &SSHConfigResolver{}
	val, err := resolver.getValue("web", "HostName")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty, got %q", val)
	}
}

func TestGetAllValuesWithNilConfigs(t *testing.T) {
	resolver := &SSHConfigResolver{}
	vals, err := resolver.getAllValues("web", "IdentityFile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 0 {
		t.Fatalf("expected empty, got %v", vals)
	}
}

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}
