package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMainExitsOnExecuteError(t *testing.T) {
	if os.Getenv("PRETTY_MAIN_CHILD") == "1" {
		os.Args = []string{"pretty"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainExitsOnExecuteError")
	cmd.Env = append(os.Environ(), "PRETTY_MAIN_CHILD=1")
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected process exit error, got %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
	}
	if !strings.Contains(string(out), "requires at least one host") {
		t.Fatalf("expected error output, got %q", string(out))
	}
}
