package shell

import (
	"fmt"
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
	readline.PcItem("exit"),
	readline.PcItem("bye"),
	readline.PcItem("help"),
	readline.PcItem("status"),
	readline.PcItem("list"),
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
		Prompt:              "pretool>> ",
		HistoryFile:         "/tmp/readline.tmp",
		AutoComplete:        completer,
		InterruptPrompt:     "^C",
		EOFPrompt:           "exit",
		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
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
		case line == "list":
			for _, host := range hosts {
				fmt.Printf("%v: Connected(%v)\n", host.Hostname, host.IsConnected)
			}
		case line == "status":
			count := 0
			for _, host := range hosts {
				if host.IsConnected == true {
					count++
				}
			}
			fmt.Printf("Connected hosts (%d)\n", count)
			fmt.Printf("Failed hosts (%d)\n", len(hosts)-count)
		case line == "":
		default:
			command <- line
			<-status
		}
	}
exit:
}
