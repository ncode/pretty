package shell

import (
	"fmt"
	"strings"
)

type CommandKind int

const (
	CommandRun CommandKind = iota
	CommandAsync
	CommandStatus
	CommandList
	CommandHelp
	CommandScroll
	CommandExit
)

type Command struct {
	Kind  CommandKind
	Arg   string
	JobID int
}

func ParseCommand(line string) Command {
	trimmed := strings.TrimSpace(line)
	switch {
	case trimmed == ":bye" || trimmed == "exit":
		return Command{Kind: CommandExit}
	case trimmed == ":help":
		return Command{Kind: CommandHelp}
	case trimmed == ":scroll":
		return Command{Kind: CommandScroll}
	case trimmed == ":list":
		return Command{Kind: CommandList}
	case strings.HasPrefix(trimmed, ":status"):
		parts := strings.Fields(trimmed)
		if len(parts) == 2 {
			var id int
			_, _ = fmt.Sscanf(parts[1], "%d", &id)
			return Command{Kind: CommandStatus, JobID: id}
		}
		return Command{Kind: CommandStatus}
	case strings.HasPrefix(trimmed, ":async"):
		return Command{Kind: CommandAsync, Arg: strings.TrimSpace(strings.TrimPrefix(trimmed, ":async"))}
	default:
		return Command{Kind: CommandRun, Arg: trimmed}
	}
}
