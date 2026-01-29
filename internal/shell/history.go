package shell

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const maxHistoryEntries = 5000

type historyState struct {
	entries []string
	index   int
	draft   string
}

func newHistory(entries []string) *historyState {
	h := &historyState{entries: entries}
	h.index = len(entries)
	return h
}

func (h *historyState) up(current string) (string, bool) {
	if h == nil || len(h.entries) == 0 {
		return "", false
	}
	if h.index == len(h.entries) {
		h.draft = current
		h.index = len(h.entries) - 1
		return h.entries[h.index], true
	}
	if h.index > 0 {
		h.index--
		return h.entries[h.index], true
	}
	return h.entries[h.index], false
}

func (h *historyState) down() (string, bool) {
	if h == nil || len(h.entries) == 0 {
		return "", false
	}
	if h.index == len(h.entries) {
		return "", false
	}
	if h.index < len(h.entries)-1 {
		h.index++
		return h.entries[h.index], true
	}
	h.index = len(h.entries)
	return h.draft, true
}

func (h *historyState) append(line string) {
	if h == nil {
		return
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	h.entries = append(h.entries, line)
	h.index = len(h.entries)
	h.draft = ""
}

func loadHistory(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entries = append(entries, line)
		if len(entries) > maxHistoryEntries {
			entries = entries[len(entries)-maxHistoryEntries:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func appendHistory(path, line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, line)
	return err
}
