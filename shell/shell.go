package shell

import (
	"io"
	"strings"

	"github.com/chzyer/readline"
	"github.com/ncode/pretool/message"
	"github.com/ncode/pretool/ssh"
)

func usage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, completer.Tree("    "))
}

var completer = readline.NewPrefixCompleter(
	readline.PcItem("bye"),
	readline.PcItem("help"),
)

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

func Spawn(hosts []*ssh.Host) {
	command := make(chan string)
	status := make(chan string)
	go message.Broker(hosts, command, status)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:              "\033[31m»\033[0m ",
		HistoryFile:         "/tmp/readline.tmp",
		AutoComplete:        completer,
		InterruptPrompt:     "^C",
		EOFPrompt:           "exit",
		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	rl.SetPrompt("pretool>> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		switch {
		case line == "help":
			usage(rl.Stderr())
		case line == "bye":
			goto exit
		case line == "exit":
			goto exit
		case line == "":
		default:
			command <- line
			<-status
		}
	}
exit:
}
