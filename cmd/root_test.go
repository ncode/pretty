package cmd

import (
	"strings"
	"testing"

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
