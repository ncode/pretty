package shell

import (
	"testing"

	"github.com/ncode/pretty/internal/jobs"
)

func TestStatusLinesColorizeHosts(t *testing.T) {
	manager := jobs.NewManager()
	job := manager.CreateJob(jobs.JobTypeNormal, "whoami", []string{"host1"})
	status := job.Hosts["host1"]
	status.State = jobs.HostSuccess
	status.ExitCode = 0
	status.Duration = 0

	colorize := func(hostname, line string) string {
		return "COLOR(" + hostname + "):" + line
	}

	lines := statusLines(manager, job.ID, colorize)
	if len(lines) != 2 {
		t.Fatalf("unexpected lines: %#v", lines)
	}
	want := "COLOR(host1):  host1: succeeded exit=0 duration=-"
	if lines[1] != want {
		t.Fatalf("unexpected line: %q", lines[1])
	}
}
