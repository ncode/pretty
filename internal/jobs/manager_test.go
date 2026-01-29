package jobs

import "testing"

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
