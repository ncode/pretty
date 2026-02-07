package cmd

import (
	"errors"
	"os"
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
