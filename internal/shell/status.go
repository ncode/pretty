package shell

import (
	"fmt"
	"time"

	"github.com/ncode/pretty/internal/jobs"
)

type hostLineColorizer func(hostname, line string) string

func statusLines(manager *jobs.Manager, jobID int, colorize hostLineColorizer) []string {
	if manager == nil {
		return []string{"status unavailable"}
	}
	if jobID > 0 {
		job := manager.Job(jobID)
		if job == nil {
			return []string{fmt.Sprintf("job %d not found", jobID)}
		}
		return formatJob(job, colorize)
	}

	lines := make([]string, 0)
	for _, job := range manager.NormalJobs() {
		lines = append(lines, formatJob(job, colorize)...)
	}
	for _, job := range manager.AsyncJobs() {
		lines = append(lines, formatJob(job, colorize)...)
	}
	if len(lines) == 0 {
		return []string{"no jobs recorded"}
	}
	return lines
}

func formatJob(job *jobs.Job, colorize hostLineColorizer) []string {
	if job == nil {
		return nil
	}
	lines := []string{fmt.Sprintf("job %d [%s] %s", job.ID, job.Type, job.Command)}
	for _, host := range job.HostsOrder {
		status := job.Hosts[host]
		line := formatHostStatus(status, colorize)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func formatHostStatus(status *jobs.HostStatus, colorize hostLineColorizer) string {
	if status == nil {
		return ""
	}
	exit := "-"
	if status.State == jobs.HostSuccess || status.State == jobs.HostFailed {
		exit = fmt.Sprintf("%d", status.ExitCode)
	}

	duration := "-"
	if elapsed := status.Elapsed(); elapsed > 0 {
		duration = elapsed.Truncate(time.Millisecond).String()
	}

	line := fmt.Sprintf("  %s: %s exit=%s duration=%s", status.Host, status.State, exit, duration)
	if colorize == nil {
		return line
	}
	return colorize(status.Host, line)
}
