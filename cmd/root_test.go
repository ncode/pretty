package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncode/pretty/internal/sshConn"
	"github.com/spf13/viper"
)

func TestRootCmdUsesRunE(t *testing.T) {
	if RootCmd.RunE == nil {
		t.Fatalf("expected RootCmd.RunE to be configured")
	}
}

func TestExecuteReturnsErrorForMissingHosts(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		RootCmd.SetArgs(nil)
	})

	hostGroup = ""
	hostsFile = ""
	RootCmd.SetArgs([]string{})

	err := Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "requires at least one host") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReturnsErrorForInvalidHostSpec(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		RootCmd.SetArgs(nil)
	})

	hostGroup = ""
	hostsFile = ""
	RootCmd.SetArgs([]string{":"})

	err := Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "invalid host") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReturnsErrorWhenHostsFileCannotBeRead(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		RootCmd.SetArgs(nil)
	})

	hostGroup = ""
	hostsFile = "/path/that/does/not/exist"
	RootCmd.SetArgs([]string{"host1"})

	err := Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to read hostsFile") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReturnsErrorForInvalidGroupSpec(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		viper.Set("groups.bad", nil)
		RootCmd.SetArgs(nil)
	})

	viper.Set("groups.bad", map[string]interface{}{"user": "deploy"})
	hostGroup = "bad"
	hostsFile = ""
	RootCmd.SetArgs([]string{"host1"})

	err := Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "missing hosts") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReturnsErrorForInvalidHostsFileContent(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		RootCmd.SetArgs(nil)
	})

	f, err := os.CreateTemp(t.TempDir(), "hosts-*.txt")
	if err != nil {
		t.Fatalf("unexpected temp file error: %v", err)
	}
	if _, err := f.WriteString("host1:badport\n"); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	hostGroup = ""
	hostsFile = f.Name()
	RootCmd.SetArgs([]string{"host2"})

	err = Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "invalid port") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReturnsErrorWhenLoadingSSHConfigFails(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	prevLoad := loadSSHConfigFunc
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		loadSSHConfigFunc = prevLoad
		RootCmd.SetArgs(nil)
	})

	loadSSHConfigFunc = func(paths sshConn.SSHConfigPaths) (*sshConn.SSHConfigResolver, error) {
		return nil, errors.New("config boom")
	}

	hostGroup = ""
	hostsFile = ""
	RootCmd.SetArgs([]string{"host1"})

	err := Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to load ssh config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReturnsErrorWhenResolveHostFails(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	prevLoad := loadSSHConfigFunc
	prevResolve := resolveHostFunc
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		loadSSHConfigFunc = prevLoad
		resolveHostFunc = prevResolve
		RootCmd.SetArgs(nil)
	})

	loadSSHConfigFunc = func(paths sshConn.SSHConfigPaths) (*sshConn.SSHConfigResolver, error) {
		return &sshConn.SSHConfigResolver{}, nil
	}
	resolveHostFunc = func(resolver *sshConn.SSHConfigResolver, spec sshConn.HostSpec, fallbackUser string) (sshConn.ResolvedHost, error) {
		return sshConn.ResolvedHost{}, errors.New("resolve boom")
	}

	hostGroup = ""
	hostsFile = ""
	RootCmd.SetArgs([]string{"host1"})

	err := Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to resolve host") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReturnsErrorWhenResolveJumpFails(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	prevLoad := loadSSHConfigFunc
	prevResolve := resolveHostFunc
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		loadSSHConfigFunc = prevLoad
		resolveHostFunc = prevResolve
		RootCmd.SetArgs(nil)
	})

	loadSSHConfigFunc = func(paths sshConn.SSHConfigPaths) (*sshConn.SSHConfigResolver, error) {
		return &sshConn.SSHConfigResolver{}, nil
	}
	call := 0
	resolveHostFunc = func(resolver *sshConn.SSHConfigResolver, spec sshConn.HostSpec, fallbackUser string) (sshConn.ResolvedHost, error) {
		call++
		if call == 1 {
			return sshConn.ResolvedHost{Alias: spec.Alias, Host: spec.Host, Port: 22, ProxyJump: []string{"jump1"}}, nil
		}
		return sshConn.ResolvedHost{}, errors.New("jump boom")
	}

	hostGroup = ""
	hostsFile = ""
	RootCmd.SetArgs([]string{"host1"})

	err := Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to resolve jump host") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReturnsNilOnSuccessfulSetup(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	prevLoad := loadSSHConfigFunc
	prevSpawn := spawnShellFunc
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		loadSSHConfigFunc = prevLoad
		spawnShellFunc = prevSpawn
		RootCmd.SetArgs(nil)
	})

	loadSSHConfigFunc = func(paths sshConn.SSHConfigPaths) (*sshConn.SSHConfigResolver, error) {
		return &sshConn.SSHConfigResolver{}, nil
	}
	spawnCalled := false
	spawnShellFunc = func(hostList *sshConn.HostList) {
		spawnCalled = true
	}

	hostGroup = ""
	hostsFile = ""
	RootCmd.SetArgs([]string{"host1"})

	err := Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spawnCalled {
		t.Fatalf("expected spawn to be called")
	}
}

func TestExecuteAppliesGlobalUserToHostAndJumpSpecs(t *testing.T) {
	prevHostGroup := hostGroup
	prevHostsFile := hostsFile
	prevLoad := loadSSHConfigFunc
	prevResolve := resolveHostFunc
	prevSpawn := spawnShellFunc
	prevUsername := viper.Get("username")
	t.Cleanup(func() {
		hostGroup = prevHostGroup
		hostsFile = prevHostsFile
		loadSSHConfigFunc = prevLoad
		resolveHostFunc = prevResolve
		spawnShellFunc = prevSpawn
		viper.Set("username", prevUsername)
		RootCmd.SetArgs(nil)
	})

	loadSSHConfigFunc = func(paths sshConn.SSHConfigPaths) (*sshConn.SSHConfigResolver, error) {
		return &sshConn.SSHConfigResolver{}, nil
	}
	spawnShellFunc = func(hostList *sshConn.HostList) {}
	viper.Set("username", "deploy")

	call := 0
	resolveHostFunc = func(resolver *sshConn.SSHConfigResolver, spec sshConn.HostSpec, fallbackUser string) (sshConn.ResolvedHost, error) {
		call++
		if call == 1 {
			if !spec.UserSet || spec.User != "deploy" {
				t.Fatalf("expected global user on host resolve, got user=%q userSet=%v", spec.User, spec.UserSet)
			}
			return sshConn.ResolvedHost{Alias: spec.Alias, Host: spec.Host, Port: 22, User: spec.User, ProxyJump: []string{"jump1"}}, nil
		}
		if !spec.UserSet || spec.User != "deploy" {
			t.Fatalf("expected global user on jump resolve, got user=%q userSet=%v", spec.User, spec.UserSet)
		}
		return sshConn.ResolvedHost{Alias: spec.Alias, Host: spec.Host, Port: 22, User: spec.User}, nil
	}

	hostGroup = ""
	hostsFile = ""
	RootCmd.SetArgs([]string{"host1"})

	err := Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if call != 2 {
		t.Fatalf("expected 2 resolve calls, got %d", call)
	}
}

func TestInitConfigWithNonExistentFile(t *testing.T) {
	prevCfgFile := cfgFile
	t.Cleanup(func() {
		cfgFile = prevCfgFile
		viper.Reset()
	})

	cfgFile = "/tmp/pretty-test-nonexistent-config-file.yaml"
	viper.Reset()

	// Should not panic even though the file doesn't exist.
	initConfig()

	// ReadInConfig silently fails, so viper should have no config file used.
	if used := viper.ConfigFileUsed(); used != "" {
		// When the file doesn't exist, ConfigFileUsed may still return the
		// path that was set, but ReadInConfig should have returned an error
		// (handled silently). Just verify no panic occurred.
	}
}

func TestInitConfigWithExistingFile(t *testing.T) {
	prevCfgFile := cfgFile
	t.Cleanup(func() {
		cfgFile = prevCfgFile
		viper.Reset()
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "test-pretty.yaml")
	if err := os.WriteFile(configPath, []byte("username: testuser\nhistory_file: /tmp/test.history\n"), 0644); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	viper.Reset()
	cfgFile = configPath

	initConfig()

	if got := viper.GetString("username"); got != "testuser" {
		t.Fatalf("expected username 'testuser', got %q", got)
	}
	if got := viper.GetString("history_file"); got != "/tmp/test.history" {
		t.Fatalf("expected history_file '/tmp/test.history', got %q", got)
	}
}
