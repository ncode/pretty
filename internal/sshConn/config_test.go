package sshConn

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
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

func TestResolveHostUsesCurrentUserWhenNoFallback(t *testing.T) {
	// With no explicit user, no User directive, and an empty fallbackUser,
	// ResolveHost must fall back to the OS current user. This exercises the
	// `userValue = currentUser()` branch that is otherwise masked by the
	// explicit fallback used in every other test in this file.
	resolver, err := LoadSSHConfig(SSHConfigPaths{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolved, err := resolver.ResolveHost(HostSpec{Host: "host-no-user"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.User == "" {
		t.Fatalf("expected a non-empty OS user when fallback is empty, got %+v", resolved)
	}
	if resolved.User != currentUser() {
		t.Fatalf("expected OS current user %q, got %q", currentUser(), resolved.User)
	}
}

func TestResolveHostPrefersExplicitAliasOverHost(t *testing.T) {
	// HostSpec.Alias takes precedence over Host for config lookup; this
	// covers the `alias = spec.Alias` branch that the other tests never hit
	// because they always set Host directly.
	cfg := "Host canonical\n  HostName canonical.internal\n  User deploy\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolved, err := resolver.ResolveHost(HostSpec{Host: "ignored", Alias: "canonical"}, "me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Alias != "canonical" {
		t.Fatalf("expected alias to win over host, got %+v", resolved)
	}
	if resolved.Host != "canonical.internal" {
		t.Fatalf("expected HostName to resolve via alias, got %+v", resolved)
	}
}

func TestLoadIdentityFilesSkipsMissing(t *testing.T) {
	methods, err := LoadIdentityFiles([]string{"/does/not/exist"})
	if err != nil {
		t.Fatalf("expected missing identity files to be skipped silently, got error: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected no auth methods for missing file, got %d", len(methods))
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

func TestLoadIdentityFilesSkipsPublicKey(t *testing.T) {
	// Public key only files (e.g. hardware-token identities served by an agent
	// such as yubikey-agent) must not abort the connection attempt. We
	// synthesize an authorized_keys line from a freshly generated ed25519 key
	// so the test has no static key material embedded in the source tree.
	pubKey := generateTestPublicKeyLine(t)
	path := filepath.Join(t.TempDir(), "hardware_token.pub")
	if err := os.WriteFile(path, []byte(pubKey), 0o600); err != nil {
		t.Fatalf("failed to write pub key: %v", err)
	}

	methods, err := LoadIdentityFiles([]string{path})
	if err != nil {
		t.Fatalf("expected public key identity to be skipped silently, got error: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected no auth methods from public key, got %d", len(methods))
	}
}

func TestLoadIdentityFilesMixedSkipsUnusable(t *testing.T) {
	// A private key is returned, while a co-located public key is ignored so
	// that authentication keeps working when the agent handles that identity.
	privPath := writeTempKey(t)
	pubPath := filepath.Join(t.TempDir(), "hardware.pub")
	pubKey := generateTestPublicKeyLine(t)
	if err := os.WriteFile(pubPath, []byte(pubKey), 0o600); err != nil {
		t.Fatalf("failed to write pub key: %v", err)
	}

	methods, err := LoadIdentityFiles([]string{pubPath, privPath, "/does/not/exist"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected only the parseable private key to produce an auth method, got %d", len(methods))
	}
}

func TestLoadIdentityFilesSkipsPassphraseProtected(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	encrypted, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", der, []byte("secret"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatalf("failed to encrypt key: %v", err)
	}
	data := pem.EncodeToMemory(encrypted)
	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	methods, err := LoadIdentityFiles([]string{path})
	if err != nil {
		t.Fatalf("expected passphrase-protected identity to be skipped silently, got error: %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("expected no auth methods from passphrase-protected key, got %d", len(methods))
	}
}

func TestLoadIdentityFilesReturnsErrorForMalformedPrivateKey(t *testing.T) {
	// Malformed PEM (starts with a PEM header but is not a valid key) should
	// still bubble up an error so operators notice the misconfigured file.
	path := filepath.Join(t.TempDir(), "id_rsa")
	malformed := "-----BEGIN RSA PRIVATE KEY-----\nnot-base64-valid\n-----END RSA PRIVATE KEY-----\n"
	if err := os.WriteFile(path, []byte(malformed), 0o600); err != nil {
		t.Fatalf("failed to write malformed key: %v", err)
	}
	_, err := LoadIdentityFiles([]string{path})
	if err == nil {
		t.Fatalf("expected error for malformed private key")
	}
}

func TestLoadIdentityFilesWrapsNonNotExistReadError(t *testing.T) {
	// Passing a directory where a file is expected produces a read error
	// that is not os.ErrNotExist, which must be surfaced with a wrapped,
	// path-qualified message rather than swallowed.
	dir := t.TempDir()
	_, err := LoadIdentityFiles([]string{dir})
	if err == nil {
		t.Fatalf("expected read error for directory path")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("expected wrapped error to include the offending path, got %q", err.Error())
	}
}

func TestResolveHostMatchExecApplies(t *testing.T) {
	// Models the production pattern where a jump alias resolves to a concrete
	// HostName only when the probe command succeeds, e.g.:
	//   Match host jump-alias exec "nc -zG 1 primary.example.net 22"
	//       HostName primary.example.net
	cfg := `Match host jump-alias exec "probe-primary"
  HostName primary.example.net
Match host jump-alias exec "probe-secondary"
  HostName secondary.example.net
`
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var seen []string
	stubExec(t, func(cmd string) (bool, error) {
		seen = append(seen, cmd)
		return cmd == "probe-primary", nil
	})

	resolved, err := resolver.ResolveHost(HostSpec{Host: "jump-alias"}, "me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Host != "primary.example.net" {
		t.Fatalf("expected primary HostName from matching exec block, got %q", resolved.Host)
	}
	if len(seen) == 0 {
		t.Fatalf("expected exec callback to be invoked, got no calls")
	}
}

func TestResolveHostMatchExecAllFailKeepsAlias(t *testing.T) {
	cfg := `Match host jump-alias exec "probe-primary"
  HostName primary.example.net
Match host jump-alias exec "probe-secondary"
  HostName secondary.example.net
`
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stubExec(t, func(cmd string) (bool, error) { return false, nil })

	resolved, err := resolver.ResolveHost(HostSpec{Host: "jump-alias"}, "me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With every exec probe failing, no HostName override applies and the
	// alias stays as-is - this is the same state that previously surfaced as
	// a DNS "no such host" when wiring was missing entirely.
	if resolved.Host != "jump-alias" {
		t.Fatalf("expected alias to remain when all exec probes fail, got %q", resolved.Host)
	}
}

func TestShellMatchExecSucceedsForTrue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell not available on Windows CI images")
	}
	ok, err := shellMatchExec("true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true command to succeed")
	}
}

func TestShellMatchExecFailsForFalse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell not available on Windows CI images")
	}
	ok, err := shellMatchExec("false")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false command to report non-match")
	}
}

func TestShellMatchExecEmptyCmd(t *testing.T) {
	ok, err := shellMatchExec("   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected empty command to report non-match")
	}
}

// stubExec replaces the package-level matchExecFunc for the duration of a test
// and restores it on cleanup. This keeps Match exec evaluation deterministic
// without shelling out.
func stubExec(t *testing.T, fn func(cmd string) (bool, error)) {
	t.Helper()
	orig := matchExecFunc
	matchExecFunc = fn
	t.Cleanup(func() { matchExecFunc = orig })
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

func TestParseProxyJumpNoneDisablesJumps(t *testing.T) {
	// OpenSSH: `ProxyJump none` cancels any inherited ProxyJump; it must not
	// be treated as a literal hop.
	for _, in := range []string{"none", "None", "NONE", "  none  "} {
		if got := ParseProxyJump(in); len(got) != 0 {
			t.Fatalf("ParseProxyJump(%q) = %v, want empty", in, got)
		}
	}
}

func TestParseProxyJumpSkipsEmptyComponents(t *testing.T) {
	// Empty/whitespace-only segments between commas are dropped rather than
	// surfacing as blank jump entries to the dialer.
	got := ParseProxyJump(" ,jump1, ,jump2,")
	want := []string{"jump1", "jump2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseProxyJump skipped-empty: got %v, want %v", got, want)
	}
}

func TestResolveHostProxyJumpNoneClearsJumps(t *testing.T) {
	// OpenSSH: first matching directive wins for single-valued options, so
	// the specific block declaring `ProxyJump none` must come before the
	// wildcard block that sets a jump. The resolved value must collapse to
	// no jumps rather than the literal host "none".
	cfg := `Host solo.internal
  ProxyJump none
Host *.internal
  ProxyJump jump1
`
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolved, err := resolver.ResolveHost(HostSpec{Host: "solo.internal"}, "me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved.ProxyJump) != 0 {
		t.Fatalf("expected no proxy jumps when ProxyJump=none wins, got %v", resolved.ProxyJump)
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

// generateTestPublicKeyLine produces a freshly generated authorized_keys-style
// line (type + base64 blob + comment) so tests do not ship static key material
// in-tree. The key is ephemeral and has no corresponding private half on disk.
func generateTestPublicKeyLine(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to wrap public key: %v", err)
	}
	return string(ssh.MarshalAuthorizedKey(sshPub))
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

func TestLoadConfigSurfacesNonNotExistOpenError(t *testing.T) {
	// Using a regular file as an intermediate path component produces an
	// ENOTDIR error from os.Open, which is not os.IsNotExist. loadConfig
	// must surface that error instead of silently returning nil.
	base := filepath.Join(t.TempDir(), "not_a_dir")
	if err := os.WriteFile(base, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("failed to seed placeholder file: %v", err)
	}
	cfg, err := loadConfig(filepath.Join(base, "config"))
	if err == nil {
		t.Fatalf("expected non-nil error for ENOTDIR path")
	}
	if cfg != nil {
		t.Fatalf("expected nil config when open errors, got %+v", cfg)
	}
}

func TestLoadSSHConfigPropagatesUserLoadError(t *testing.T) {
	base := filepath.Join(t.TempDir(), "user_placeholder")
	if err := os.WriteFile(base, []byte("x"), 0o600); err != nil {
		t.Fatalf("failed to seed placeholder: %v", err)
	}
	_, err := LoadSSHConfig(SSHConfigPaths{User: filepath.Join(base, "config")})
	if err == nil {
		t.Fatalf("expected user loadConfig error to propagate")
	}
}

func TestLoadSSHConfigPropagatesSystemLoadError(t *testing.T) {
	base := filepath.Join(t.TempDir(), "sys_placeholder")
	if err := os.WriteFile(base, []byte("x"), 0o600); err != nil {
		t.Fatalf("failed to seed placeholder: %v", err)
	}
	_, err := LoadSSHConfig(SSHConfigPaths{System: filepath.Join(base, "config")})
	if err == nil {
		t.Fatalf("expected system loadConfig error to propagate")
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

func TestResolveHostInvalidPortSurfacesError(t *testing.T) {
	cfg := "Host web\n  Port not-a-number\n"
	userCfg := writeTempConfig(t, cfg)
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.ResolveHost(HostSpec{Host: "web"}, "me")
	if err == nil {
		t.Fatalf("expected invalid Port value to produce a wrapped error")
	}
	if !strings.Contains(err.Error(), "invalid port") {
		t.Fatalf("expected wrapped error to mention invalid port, got %q", err.Error())
	}
}

func TestResolveHostEmptyAliasSurfacesResolverError(t *testing.T) {
	// The underlying ssh_config library requires a non-empty HostArg and
	// returns an error otherwise. ResolveHost must surface that rather than
	// continuing to connect to a nameless host.
	userCfg := writeTempConfig(t, "Host web\n  User deploy\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = resolver.ResolveHost(HostSpec{}, "")
	if err == nil {
		t.Fatalf("expected error for empty host/alias")
	}
}

func TestGetValueSurfacesUserResolverError(t *testing.T) {
	userCfg := writeTempConfig(t, "Host web\n  User deploy\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := resolver.getValue("", "User"); err == nil {
		t.Fatalf("expected resolver error for empty alias via user config")
	}
}

func TestGetValueSurfacesSystemResolverError(t *testing.T) {
	// Only a system config is loaded so the error must come from the system
	// branch of getValue rather than the user branch.
	systemCfg := writeTempConfig(t, "Host web\n  User deploy\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{System: systemCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := resolver.getValue("", "User"); err == nil {
		t.Fatalf("expected resolver error for empty alias via system config")
	}
}

func TestGetAllValuesSurfacesUserResolverError(t *testing.T) {
	userCfg := writeTempConfig(t, "Host web\n  IdentityFile ~/.ssh/id\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{User: userCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := resolver.getAllValues("", "IdentityFile"); err == nil {
		t.Fatalf("expected resolver error for empty alias via user config")
	}
}

func TestGetAllValuesSurfacesSystemResolverError(t *testing.T) {
	systemCfg := writeTempConfig(t, "Host web\n  IdentityFile ~/.ssh/id\n")
	resolver, err := LoadSSHConfig(SSHConfigPaths{System: systemCfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := resolver.getAllValues("", "IdentityFile"); err == nil {
		t.Fatalf("expected resolver error for empty alias via system config")
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
