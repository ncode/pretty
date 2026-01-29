package shell

import "testing"

func TestParseCommandAsync(t *testing.T) {
	cmd := ParseCommand(":async uptime")
	if cmd.Kind != CommandAsync || cmd.Arg != "uptime" {
		t.Fatalf("unexpected: %+v", cmd)
	}
}

func TestParseCommandStatus(t *testing.T) {
	cmd := ParseCommand(":status 7")
	if cmd.Kind != CommandStatus || cmd.JobID != 7 {
		t.Fatalf("unexpected: %+v", cmd)
	}
}

func TestParseCommandDefault(t *testing.T) {
	cmd := ParseCommand("whoami")
	if cmd.Kind != CommandRun || cmd.Arg != "whoami" {
		t.Fatalf("unexpected: %+v", cmd)
	}
}
