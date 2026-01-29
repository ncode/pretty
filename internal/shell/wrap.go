package shell

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ncode/pretty/internal/jobs"
)

func wrapCommand(command string, jobID int) string {
	trimmed := strings.TrimRightFunc(command, unicode.IsSpace)
	sep := "; "
	if strings.HasSuffix(trimmed, "&&") || strings.HasSuffix(trimmed, "||") || strings.HasSuffix(trimmed, "&") || strings.HasSuffix(trimmed, ";") {
		sep = "\n"
	}
	return fmt.Sprintf("%s%sprintf '%s:%%d\\n' $?", command, sep, jobs.SentinelFor(jobID))
}
