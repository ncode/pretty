package shell

import (
	"strings"
	"testing"
)

func TestWrapCommand(t *testing.T) {
	cmd := wrapCommand("whoami", 3)
	if !strings.Contains(cmd, "__PRETTY_EXIT__3") {
		t.Fatalf("expected sentinel in command: %s", cmd)
	}
}

func TestWrapCommand_NoLeadingNewline(t *testing.T) {
	cmd := wrapCommand("whoami", 3)
	if strings.Contains(cmd, "\\n__PRETTY_EXIT__") {
		t.Fatalf("expected no leading newline in sentinel wrapper: %s", cmd)
	}
}

func TestWrapCommand_TrailingOperators(t *testing.T) {
	cases := []struct {
		name    string
		command string
	}{
		{name: "trailing_ampersand", command: "sleep 1 &"},
		{name: "trailing_semicolon", command: "echo ok;"},
		{name: "trailing_and", command: "echo ok &&"},
		{name: "trailing_or", command: "echo ok ||"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := wrapCommand(tc.command, 7)
			if strings.Contains(cmd, "&;") {
				t.Fatalf("unexpected '&;' in wrapped command: %s", cmd)
			}
			if strings.Contains(cmd, ";;") {
				t.Fatalf("unexpected ';;' in wrapped command: %s", cmd)
			}
			if strings.Contains(cmd, "&&;") {
				t.Fatalf("unexpected '&&;' in wrapped command: %s", cmd)
			}
			if strings.Contains(cmd, "||;") {
				t.Fatalf("unexpected '||;' in wrapped command: %s", cmd)
			}
			if !strings.Contains(cmd, "__PRETTY_EXIT__7") {
				t.Fatalf("expected sentinel in command: %s", cmd)
			}
		})
	}
}
