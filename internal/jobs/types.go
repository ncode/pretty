package jobs

import "time"

type JobType string

const (
	JobTypeNormal JobType = "normal"
	JobTypeAsync  JobType = "async"
)

type HostState string

const (
	HostQueued  HostState = "queued"
	HostRunning HostState = "running"
	HostSuccess HostState = "succeeded"
	HostFailed  HostState = "failed"
)

type HostStatus struct {
	Host      string
	State     HostState
	ExitCode  int
	Duration  time.Duration
	startedAt time.Time
}

type Job struct {
	ID         int
	Type       JobType
	Command    string
	Created    time.Time
	Hosts      map[string]*HostStatus
	HostsOrder []string
}

func (h *HostStatus) Elapsed() time.Duration {
	if h == nil {
		return 0
	}
	if h.State == HostRunning && !h.startedAt.IsZero() {
		return time.Since(h.startedAt)
	}
	return h.Duration
}
