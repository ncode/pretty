package jobs

import (
	"fmt"
	"strconv"
	"strings"
)

const sentinelPrefix = "__PRETTY_EXIT__"

func SentinelFor(jobID int) string {
	return fmt.Sprintf("%s%d", sentinelPrefix, jobID)
}

func ExtractSentinel(line string) (string, int, int, bool) {
	idx := strings.Index(line, sentinelPrefix)
	if idx == -1 {
		return "", 0, 0, false
	}
	payload := line[idx+len(sentinelPrefix):]
	colon := strings.IndexByte(payload, ':')
	if colon == -1 {
		return "", 0, 0, false
	}
	jobPart := payload[:colon]
	exitPart := payload[colon+1:]
	jobID, err := strconv.Atoi(jobPart)
	if err != nil || strconv.Itoa(jobID) != jobPart {
		return "", 0, 0, false
	}
	exitCode, err := strconv.Atoi(exitPart)
	if err != nil || strconv.Itoa(exitCode) != exitPart {
		return "", 0, 0, false
	}
	return line[:idx], jobID, exitCode, true
}

func ParseSentinel(line string) (int, int, bool) {
	prefix, jobID, exitCode, ok := ExtractSentinel(line)
	if !ok || prefix != "" {
		return 0, 0, false
	}
	return jobID, exitCode, true
}
