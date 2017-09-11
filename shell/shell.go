package shell

import (
	"fmt"
	"io"
	"strings"
	"sync/atomic"

	"github.com/chzyer/readline"
	"github.com/ncode/pretool/message"
	"github.com/ncode/pretool/ssh"
	"github.com/spf13/viper"
)

func usage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, completer.Tree("    "))
}

var completer = readline.NewPrefixCompleter(
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

func Spawn(hostList *ssh.HostList) {
	command := make(chan string)
	go message.Broker(hostList, command)
	prompt := "pretool(0)>> "

	rl, err := readline.NewEx(&readline.Config{
		Prompt:              prompt,
		HistoryFile:         viper.GetString("history_file"),
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

		connected, waiting := hostList.State()
		if waiting > 0 {
			prompt = fmt.Sprintf("pretool(%d)/(%d)>> ", waiting, connected)
		} else {
			prompt = fmt.Sprintf("pretool(%d)>> ", connected)
		}
		rl.SetPrompt(prompt)

		line = strings.TrimSpace(line)
		switch {
		case line == ":help":
			usage(rl.Stderr())
		case line == ":bye":
			goto exit
		case line == "exit":
			goto exit
		case line == ":list":
			for _, host := range hostList.Hosts() {
				var state bool
				if atomic.LoadInt32(&host.IsConnected) == 1 {
					state = true
				}
				fmt.Printf("%v: Connected(%v)\n", host.Hostname, state)
			}
		case line == ":status":
			fmt.Printf("Connected hosts (%d)\n", connected)
			fmt.Printf("Failed hosts (%d)\n", hostList.Len()-connected)
		case line == "":
		default:
			command <- line
			fmt.Printf(prompt)
		}
	}
exit:
}
