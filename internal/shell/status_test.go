package shell

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ncode/pretty/internal/jobs"
)

func TestStatusLinesNilManager(t *testing.T) {
	lines := statusLines(nil, 0, nil)
	if len(lines) != 1 || lines[0] != "status unavailable" {
		t.Fatalf("expected [status unavailable], got %#v", lines)
	}
}

func TestStatusLinesSpecificJobFound(t *testing.T) {
	manager := jobs.NewManager()
	job := manager.CreateJob(jobs.JobTypeNormal, "uptime", []string{"host1"})
	status := job.Hosts["host1"]
	status.State = jobs.HostSuccess
	status.ExitCode = 0

	colorize := func(hostname, line string) string { return line }
	lines := statusLines(manager, job.ID, colorize)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if !strings.Contains(lines[0], fmt.Sprintf("job %d", job.ID)) {
		t.Fatalf("expected job header, got %q", lines[0])
	}
}

func TestStatusLinesSpecificJobNotFound(t *testing.T) {
	manager := jobs.NewManager()
	lines := statusLines(manager, 999, nil)
	if len(lines) != 1 || lines[0] != "job 999 not found" {
		t.Fatalf("expected [job 999 not found], got %#v", lines)
	}
}

func TestStatusLinesNoJobs(t *testing.T) {
	manager := jobs.NewManager()
	lines := statusLines(manager, 0, nil)
	if len(lines) != 1 || lines[0] != "no jobs recorded" {
		t.Fatalf("expected [no jobs recorded], got %#v", lines)
	}
}

func TestFormatJobNil(t *testing.T) {
	colorize := func(hostname, line string) string { return line }
	lines := formatJob(nil, colorize)
	if lines != nil {
		t.Fatalf("expected nil, got %#v", lines)
	}
}

func TestFormatHostStatusNil(t *testing.T) {
	colorize := func(hostname, line string) string { return line }
	result := formatHostStatus(nil, colorize)
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestFormatHostStatusRunning(t *testing.T) {
	manager := jobs.NewManager()
	job := manager.CreateJob(jobs.JobTypeAsync, "uptime", []string{"host1"})
	manager.MarkHostRunning(job.ID, "host1")

	snap := manager.Job(job.ID)
	status := snap.Hosts["host1"]
	if status.State != jobs.HostRunning {
		t.Fatalf("expected running state, got %v", status.State)
	}

	colorize := func(hostname, line string) string { return line }
	result := formatHostStatus(status, colorize)

	// Running state should show exit="-"
	if !strings.Contains(result, "exit=-") {
		t.Fatalf("expected exit=- for running host, got %q", result)
	}
	// Running state with a non-zero elapsed should show a duration
	if !strings.Contains(result, "running") {
		t.Fatalf("expected 'running' in output, got %q", result)
	}
}

func TestStatusLinesWithNormalAndAsyncJobs(t *testing.T) {
	manager := jobs.NewManager()
	nj := manager.CreateJob(jobs.JobTypeNormal, "whoami", []string{"host1"})
	nj.Hosts["host1"].State = jobs.HostSuccess
	aj := manager.CreateJob(jobs.JobTypeAsync, "date", []string{"host1"})
	aj.Hosts["host1"].State = jobs.HostSuccess

	lines := statusLines(manager, 0, nil)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (headers for both jobs), got %d: %#v", len(lines), lines)
	}
	found := 0
	for _, l := range lines {
		if strings.Contains(l, "whoami") || strings.Contains(l, "date") {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected both jobs in output, got %d matches in %#v", found, lines)
	}
}

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
