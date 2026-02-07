package cmd

import (
	"strings"
	"testing"
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
