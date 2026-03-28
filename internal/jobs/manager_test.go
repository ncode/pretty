package jobs

import (
	"testing"
	"time"
)

func TestJobRetention(t *testing.T) {
	m := NewManager()
	m.CreateJob(JobTypeNormal, "whoami", []string{"host1"})
	m.CreateJob(JobTypeNormal, "uptime", []string{"host1"})
	if len(m.NormalJobs()) != 1 {
		t.Fatalf("expected only current normal job retained")
	}
	m.CreateJob(JobTypeAsync, "date", []string{"host1"})
	m.CreateJob(JobTypeAsync, "uname", []string{"host1"})
	m.CreateJob(JobTypeAsync, "id", []string{"host1"})
	if len(m.AsyncJobs()) != 2 {
		t.Fatalf("expected last two async jobs retained")
	}
}

func TestManagerSnapshotReused(t *testing.T) {
	m := NewManager()
	job := m.CreateJob(JobTypeNormal, "whoami", []string{"host1"})
	snap1 := m.Job(job.ID)
	snap2 := m.Job(job.ID)
	if snap1 == nil || snap2 == nil {
		t.Fatalf("expected snapshots to be present")
	}
	if snap1 != snap2 {
		t.Fatalf("expected snapshot reuse between reads")
	}
}

func TestManagerSnapshotUpdatesOnStatusChange(t *testing.T) {
	m := NewManager()
	job := m.CreateJob(JobTypeNormal, "whoami", []string{"host1"})
	m.MarkHostRunning(job.ID, "host1")
	if got := m.Job(job.ID).Hosts["host1"].State; got != HostRunning {
		t.Fatalf("unexpected state: %v", got)
	}
	m.MarkHostDone(job.ID, "host1", 0, true)
	if got := m.Job(job.ID).Hosts["host1"].State; got != HostSuccess {
		t.Fatalf("unexpected state: %v", got)
	}
}

func TestFindJobLockedAsyncAndMissing(t *testing.T) {
	m := NewManager()
	async := m.CreateJob(JobTypeAsync, "date", []string{"host1"})

	got := m.Job(async.ID)
	if got == nil {
		t.Fatalf("expected async job to be found")
	}
	if got.ID != async.ID {
		t.Fatalf("expected job ID %d, got %d", async.ID, got.ID)
	}

	if m.Job(999) != nil {
		t.Fatalf("expected nil for missing job ID")
	}
}

func TestJobLookupAsyncAndMissing(t *testing.T) {
	m := NewManager()
	normal := m.CreateJob(JobTypeNormal, "whoami", []string{"host1"})
	async := m.CreateJob(JobTypeAsync, "date", []string{"host1"})

	got := m.Job(async.ID)
	if got == nil || got.ID != async.ID {
		t.Fatalf("expected async job %d, got %v", async.ID, got)
	}

	got = m.Job(normal.ID)
	if got == nil || got.ID != normal.ID {
		t.Fatalf("expected normal job %d, got %v", normal.ID, got)
	}

	if m.Job(999) != nil {
		t.Fatalf("expected nil for non-existent job ID")
	}
}

func TestElapsed(t *testing.T) {
	t.Run("nil receiver returns 0", func(t *testing.T) {
		var hs *HostStatus
		if hs.Elapsed() != 0 {
			t.Fatalf("expected 0 for nil receiver, got %v", hs.Elapsed())
		}
	})

	t.Run("running state with non-zero startedAt", func(t *testing.T) {
		m := NewManager()
		job := m.CreateJob(JobTypeNormal, "sleep 10", []string{"host1"})
		m.MarkHostRunning(job.ID, "host1")
		time.Sleep(5 * time.Millisecond)

		snap := m.Job(job.ID)
		elapsed := snap.Hosts["host1"].Elapsed()
		if elapsed <= 0 {
			t.Fatalf("expected positive elapsed for running host, got %v", elapsed)
		}
	})

	t.Run("completed state returns Duration", func(t *testing.T) {
		m := NewManager()
		job := m.CreateJob(JobTypeNormal, "echo ok", []string{"host1"})
		m.MarkHostRunning(job.ID, "host1")
		time.Sleep(5 * time.Millisecond)
		m.MarkHostDone(job.ID, "host1", 0, true)

		snap := m.Job(job.ID)
		hs := snap.Hosts["host1"]
		if hs.Duration <= 0 {
			t.Fatalf("expected positive Duration, got %v", hs.Duration)
		}
		if hs.Elapsed() != hs.Duration {
			t.Fatalf("expected Elapsed() == Duration (%v), got %v", hs.Duration, hs.Elapsed())
		}
	})
}

func TestMarkHostDoneZeroStartedAt(t *testing.T) {
	m := NewManager()
	job := m.CreateJob(JobTypeNormal, "echo ok", []string{"host1"})

	// Skip MarkHostRunning so startedAt is zero.
	m.MarkHostDone(job.ID, "host1", 0, true)
	snap := m.Job(job.ID)
	hs := snap.Hosts["host1"]
	if hs.State != HostSuccess {
		t.Fatalf("expected HostSuccess, got %v", hs.State)
	}
	if hs.Duration < 0 {
		t.Fatalf("expected non-negative Duration after zero-startedAt backfill, got %v", hs.Duration)
	}

	// Same test for failure path.
	m2 := NewManager()
	job2 := m2.CreateJob(JobTypeNormal, "false", []string{"host1"})
	m2.MarkHostDone(job2.ID, "host1", 1, false)
	snap2 := m2.Job(job2.ID)
	hs2 := snap2.Hosts["host1"]
	if hs2.State != HostFailed {
		t.Fatalf("expected HostFailed, got %v", hs2.State)
	}
	if hs2.Duration < 0 {
		t.Fatalf("expected non-negative Duration for failed host, got %v", hs2.Duration)
	}
}

func TestMarkHostRunningEdgeCases(t *testing.T) {
	m := NewManager()
	job := m.CreateJob(JobTypeNormal, "whoami", []string{"host1"})

	// Wrong jobID — should not panic.
	m.MarkHostRunning(999, "host1")

	// Wrong hostname — should not panic.
	m.MarkHostRunning(job.ID, "no-such-host")

	// First call transitions to HostRunning.
	m.MarkHostRunning(job.ID, "host1")
	snap := m.Job(job.ID)
	if snap.Hosts["host1"].State != HostRunning {
		t.Fatalf("expected HostRunning, got %v", snap.Hosts["host1"].State)
	}

	// Second call is a no-op; state stays HostRunning.
	m.MarkHostRunning(job.ID, "host1")
	snap2 := m.Job(job.ID)
	if snap2.Hosts["host1"].State != HostRunning {
		t.Fatalf("expected HostRunning after second call, got %v", snap2.Hosts["host1"].State)
	}
}

func TestCloneJobNilStatusEntry(t *testing.T) {
	job := &Job{
		ID:         42,
		Type:       JobTypeNormal,
		Command:    "test",
		Created:    time.Now(),
		Hosts:      map[string]*HostStatus{"host1": nil, "host2": {Host: "host2", State: HostQueued}},
		HostsOrder: []string{"host1", "host2"},
	}

	clone := cloneJob(job)
	if clone == nil {
		t.Fatalf("expected non-nil clone")
	}
	if _, exists := clone.Hosts["host1"]; exists {
		t.Fatalf("expected nil status entry to be skipped in clone")
	}
	if clone.Hosts["host2"] == nil {
		t.Fatalf("expected host2 to be cloned")
	}
	if clone.Hosts["host2"].State != HostQueued {
		t.Fatalf("expected HostQueued for host2, got %v", clone.Hosts["host2"].State)
	}
}

func TestSnapshotCaching(t *testing.T) {
	t.Run("NormalJobs cached across calls", func(t *testing.T) {
		m := NewManager()
		m.CreateJob(JobTypeNormal, "whoami", []string{"host1"})

		first := m.NormalJobs()
		second := m.NormalJobs()
		if len(first) != 1 || len(second) != 1 {
			t.Fatalf("expected 1 normal job in both calls")
		}
		if first[0] != second[0] {
			t.Fatalf("expected same snapshot pointer when no mutation occurred")
		}
	})

	t.Run("AsyncJobs cached across calls", func(t *testing.T) {
		m := NewManager()
		m.CreateJob(JobTypeAsync, "date", []string{"host1"})

		first := m.AsyncJobs()
		second := m.AsyncJobs()
		if len(first) != 1 || len(second) != 1 {
			t.Fatalf("expected 1 async job in both calls")
		}
		if first[0] != second[0] {
			t.Fatalf("expected same snapshot pointer when no mutation occurred")
		}
	})
}
