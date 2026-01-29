package jobs

import (
	"sync"
	"time"
)

type Manager struct {
	mu             sync.Mutex
	nextID         int
	normalJob      *Job
	asyncJobs      []*Job
	snapshotDirty  bool
	normalSnapshot *Job
	asyncSnapshots []*Job
}

func NewManager() *Manager {
	return &Manager{nextID: 1, snapshotDirty: true}
}

func (m *Manager) markDirty() {
	m.snapshotDirty = true
}

func (m *Manager) ensureSnapshotsLocked() {
	if !m.snapshotDirty {
		return
	}
	if m.normalJob != nil {
		m.normalSnapshot = cloneJob(m.normalJob)
	} else {
		m.normalSnapshot = nil
	}
	if cap(m.asyncSnapshots) < len(m.asyncJobs) {
		m.asyncSnapshots = make([]*Job, 0, len(m.asyncJobs))
	} else {
		m.asyncSnapshots = m.asyncSnapshots[:0]
	}
	for _, job := range m.asyncJobs {
		m.asyncSnapshots = append(m.asyncSnapshots, cloneJob(job))
	}
	m.snapshotDirty = false
}

func (m *Manager) CreateJob(t JobType, command string, hosts []string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := &Job{
		ID:         m.nextID,
		Type:       t,
		Command:    command,
		Created:    time.Now(),
		Hosts:      map[string]*HostStatus{},
		HostsOrder: make([]string, 0, len(hosts)),
	}
	m.nextID++

	for _, host := range hosts {
		job.Hosts[host] = &HostStatus{Host: host, State: HostQueued}
		job.HostsOrder = append(job.HostsOrder, host)
	}

	if t == JobTypeNormal {
		m.normalJob = job
	} else {
		m.asyncJobs = append(m.asyncJobs, job)
		if len(m.asyncJobs) > 2 {
			m.asyncJobs = m.asyncJobs[len(m.asyncJobs)-2:]
		}
	}

	m.markDirty()
	return job
}

func (m *Manager) NormalJobs() []*Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSnapshotsLocked()
	if m.normalSnapshot == nil {
		return nil
	}
	return []*Job{m.normalSnapshot}
}

func (m *Manager) AsyncJobs() []*Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSnapshotsLocked()
	if len(m.asyncSnapshots) == 0 {
		return nil
	}
	jobs := make([]*Job, len(m.asyncSnapshots))
	copy(jobs, m.asyncSnapshots)
	return jobs
}

func (m *Manager) Job(jobID int) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSnapshotsLocked()
	if m.normalSnapshot != nil && m.normalSnapshot.ID == jobID {
		return m.normalSnapshot
	}
	for _, job := range m.asyncSnapshots {
		if job != nil && job.ID == jobID {
			return job
		}
	}
	return nil
}

func (m *Manager) MarkHostRunning(jobID int, host string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.findJobLocked(jobID)
	if job == nil {
		return
	}
	status := job.Hosts[host]
	if status == nil {
		return
	}
	if status.State == HostQueued {
		status.State = HostRunning
		status.startedAt = time.Now()
		m.markDirty()
	}
}

func (m *Manager) MarkHostDone(jobID int, host string, exitCode int, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.findJobLocked(jobID)
	if job == nil {
		return
	}
	status := job.Hosts[host]
	if status == nil {
		return
	}
	if status.startedAt.IsZero() {
		status.startedAt = time.Now()
	}
	status.Duration = time.Since(status.startedAt)
	status.ExitCode = exitCode
	if success {
		status.State = HostSuccess
	} else {
		status.State = HostFailed
	}
	m.markDirty()
}

func (m *Manager) findJobLocked(jobID int) *Job {
	if m.normalJob != nil && m.normalJob.ID == jobID {
		return m.normalJob
	}
	for _, job := range m.asyncJobs {
		if job.ID == jobID {
			return job
		}
	}
	return nil
}

func cloneJob(job *Job) *Job {
	if job == nil {
		return nil
	}
	clone := &Job{
		ID:         job.ID,
		Type:       job.Type,
		Command:    job.Command,
		Created:    job.Created,
		Hosts:      make(map[string]*HostStatus, len(job.Hosts)),
		HostsOrder: append([]string(nil), job.HostsOrder...),
	}
	for host, status := range job.Hosts {
		if status == nil {
			continue
		}
		clone.Hosts[host] = &HostStatus{
			Host:      status.Host,
			State:     status.State,
			ExitCode:  status.ExitCode,
			Duration:  status.Duration,
			startedAt: status.startedAt,
		}
	}
	return clone
}
