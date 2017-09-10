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
	readline.PcItem(":exit"),
	readline.PcItem(":bye"),
	readline.PcItem(":help"),
	readline.PcItem(":status"),
	readline.PcItem(":list"),
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
	go message.Broker(hosts, command)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:              fmt.Sprintf("pretool(0)/(%d)>> ", len(hosts)),
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

		connected := 0
		waiting := 0
		for _, host := range hosts {
			if host.IsConnected {
				connected++
				if host.IsWaiting {
					waiting++
				}
			}
		}

		promtp := ""
		if waiting > 0 {
			promtp = fmt.Sprintf("pretool(%d)/(%d)>> ", waiting, connected)
		} else {
			promtp = fmt.Sprintf("pretool(%d)>> ", connected)
		}
		rl.SetPrompt(promtp)

		line = strings.TrimSpace(line)
		switch {
		case line == ":help":
			usage(rl.Stderr())
		case line == ":bye":
			goto exit
		case line == ":exit":
			goto exit
		case line == ":list":
			for _, host := range hosts {
				fmt.Printf("%v: Connected(%v)\n", host.Hostname, host.IsConnected)
			}
		case line == ":status":
			fmt.Printf("Connected hosts (%d)\n", connected)
			fmt.Printf("Failed hosts (%d)\n", len(hosts)-connected)
		case line == "":
		default:
			command <- line
			fmt.Printf(promtp)
		}
	}
exit:
}
