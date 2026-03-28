package shell

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/ncode/pretty/internal/jobs"
	"github.com/ncode/pretty/internal/sshConn"
)

func TestOutputBufferSizeMinimum(t *testing.T) {
	if got := outputBufferSize(0); got != 128 {
		t.Fatalf("unexpected size: %d", got)
	}
}

func TestOutputBufferSizeScales(t *testing.T) {
	if got := outputBufferSize(10); got != 288 {
		t.Fatalf("unexpected size: %d", got)
	}
}

func TestAsyncCommandViaTeatest(t *testing.T) {
	prev := runCommandFunc
	t.Cleanup(func() { runCommandFunc = prev })

	runCommandFunc = func(host *sshConn.Host, command string, jobID int, events chan<- sshConn.OutputEvent) (int, error) {
		events <- sshConn.OutputEvent{JobID: jobID, Hostname: host.Hostname, Line: "output-from-" + host.Hostname}
		return 0, nil
	}

	origRun := runProgram
	t.Cleanup(func() { runProgram = origRun })

	var final tea.Model
	runProgram = func(m tea.Model) (tea.Model, error) {
		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
		tm.Type(":async uptime")
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
		// Give async goroutines time to complete and output to arrive
		time.Sleep(200 * time.Millisecond)
		tm.Type(":bye")
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
		final = tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
		return final, nil
	}

	hostList := sshConn.NewHostList()
	hostList.AddHost(&sshConn.Host{Hostname: "host1", IsConnected: 1})
	hostList.AddHost(&sshConn.Host{Hostname: "host2", IsConnected: 1})

	Spawn(hostList)

	m, ok := final.(model)
	if !ok {
		t.Fatalf("expected model, got %T", final)
	}
	if !m.quit {
		t.Fatal("expected model to be in quit state")
	}

	// Verify an async job was created
	asyncJobs := m.jobs.AsyncJobs()
	if len(asyncJobs) != 1 {
		t.Fatalf("expected 1 async job, got %d", len(asyncJobs))
	}
	if asyncJobs[0].Command != "uptime" {
		t.Fatalf("expected command 'uptime', got %q", asyncJobs[0].Command)
	}
	// Verify both hosts completed successfully
	for _, hostname := range []string{"host1", "host2"} {
		hs := asyncJobs[0].Hosts[hostname]
		if hs == nil {
			t.Fatalf("expected host status for %s", hostname)
		}
		if hs.State != jobs.HostSuccess {
			t.Fatalf("expected %s succeeded, got %v", hostname, hs.State)
		}
	}
}

func TestSpawnExitsOnBye(t *testing.T) {
	orig := runProgram
	t.Cleanup(func() { runProgram = orig })

	var final tea.Model
	runProgram = func(m tea.Model) (tea.Model, error) {
		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
		tm.Type(":bye")
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
		final = tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
		return final, nil
	}

	Spawn(sshConn.NewHostList())

	m, ok := final.(model)
	if !ok {
		t.Fatalf("expected model, got %T", final)
	}
	if !m.quit {
		t.Fatal("expected model to be in quit state")
	}
}
