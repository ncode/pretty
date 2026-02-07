package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/ncode/pretty/internal/jobs"
	"github.com/ncode/pretty/internal/sshConn"
	"github.com/spf13/viper"
)

func TestAppendLine(t *testing.T) {
	lines := []string{"one"}
	lines = appendLine(lines, "two")
	if len(lines) != 2 || lines[1] != "two" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestColorizeHostLineUsesAssignedColor(t *testing.T) {
	prevNoColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = prevNoColor }()

	hostColor := color.New(color.FgRed)
	line := "host1: whoami"
	got := colorizeHostLine(map[string]*color.Color{"host1": hostColor}, "host1", line)
	want := hostColor.Sprint(line)
	if got != want {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestOutputMsgInlineSentinelPreservesPrefix(t *testing.T) {
	prevNoColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = prevNoColor }()

	hostColor := color.New(color.FgRed)
	hostList := sshConn.NewHostList()
	hostList.AddHost(&sshConn.Host{Hostname: "host1", Color: hostColor})

	m := initialModel(hostList, nil, nil)
	job := m.jobs.CreateJob(jobs.JobTypeNormal, "whoami", []string{"host1"})
	m.jobs.MarkHostRunning(job.ID, "host1")

	line := "whoami" + jobs.SentinelFor(job.ID) + ":0"
	updated, _ := m.Update(outputMsg{events: []sshConn.OutputEvent{
		{JobID: job.ID, Hostname: "host1", Line: line},
	}})
	um := updated.(model)

	lines := um.output.Lines()
	if len(lines) != 1 {
		t.Fatalf("expected one output line, got %d", len(lines))
	}
	want := hostColor.Sprint("host1: whoami")
	if lines[0] != want {
		t.Fatalf("unexpected output: %q", lines[0])
	}
}

func TestListCommandColorsHostLines(t *testing.T) {
	prevNoColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = prevNoColor }()

	hostColor := color.New(color.FgGreen)
	hostList := sshConn.NewHostList()
	hostList.AddHost(&sshConn.Host{Hostname: "host1", Color: hostColor})

	m := initialModel(hostList, nil, nil)
	m.input.SetValue(":list")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(model)

	want := hostColor.Sprint("host1: Connected(false)")
	lines := um.output.Lines()
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("unexpected output: %#v", lines)
	}
}

func TestUpdateProcessesBatchOutput(t *testing.T) {
	hostList := sshConn.NewHostList()
	hostList.AddHost(&sshConn.Host{Hostname: "host1"})

	m := initialModel(hostList, nil, nil)
	msg := outputMsg{events: []sshConn.OutputEvent{
		{Hostname: "host1", Line: "one"},
		{Hostname: "host1", Line: "two"},
	}}
	updated, _ := m.Update(msg)
	um := updated.(model)
	lines := um.output.Lines()
	if len(lines) != 2 || lines[0] != "host1: one" || lines[1] != "host1: two" {
		t.Fatalf("unexpected output: %#v", lines)
	}
}

func TestInitialModelUsesPromptFromConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("prompt", "\u03bb ")

	m := initialModel(nil, nil, nil)
	if m.input.Prompt != "\u03bb " {
		t.Fatalf("unexpected prompt: %q", m.input.Prompt)
	}
}

func TestInitialModelUsesDefaultPromptWhenUnset(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	m := initialModel(nil, nil, nil)
	if m.input.Prompt != defaultPrompt {
		t.Fatalf("unexpected prompt: %q", m.input.Prompt)
	}
}

func TestModelAppendHistoryOnEnter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	prevHistory := viper.GetString("history_file")
	viper.Set("history_file", path)
	defer viper.Set("history_file", prevHistory)

	m := initialModel(nil, nil, nil)
	m.input.SetValue("ls")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated.(model)

	entries, err := loadHistory(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0] != "ls" {
		t.Fatalf("expected [ls], got %v", entries)
	}
}

func TestInitialModelLoadsHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	if err := os.WriteFile(path, []byte("one\n\ntwo\n"), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prevHistory := viper.GetString("history_file")
	viper.Set("history_file", path)
	defer viper.Set("history_file", prevHistory)

	m := initialModel(nil, nil, nil)
	if m.history == nil {
		t.Fatal("expected history to be initialized")
	}
	if len(m.history.entries) != 2 || m.history.entries[0] != "one" || m.history.entries[1] != "two" {
		t.Fatalf("unexpected history entries: %v", m.history.entries)
	}
}

func TestHistoryNavigationRestoresDraft(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.history = newHistory([]string{"echo one", "echo two"})
	m.input.SetValue("draft")

	m = pressKey(m, "up")
	if m.input.Value() != "echo two" {
		t.Fatalf("expected last history entry, got %q", m.input.Value())
	}

	m = pressKey(m, "down")
	if m.input.Value() != "draft" {
		t.Fatalf("expected draft restored, got %q", m.input.Value())
	}
}

func TestHistoryNavigationUsesNewEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	prevHistory := viper.GetString("history_file")
	viper.Set("history_file", path)
	defer viper.Set("history_file", prevHistory)

	m := initialModel(nil, nil, nil)
	m.input.SetValue("ls")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	m = pressKey(m, "up")
	if m.input.Value() != "ls" {
		t.Fatalf("expected new entry, got %q", m.input.Value())
	}
}

func TestScrollModeEscExits(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.input.SetValue(":scroll")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.scrollMode {
		t.Fatal("expected scroll mode")
	}
	m = pressKey(m, "esc")
	if m.scrollMode {
		t.Fatal("expected scroll mode to exit")
	}
}

func TestOutputDoesNotAutoFollowInScrollMode(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.scrollMode = true
	m.viewport.SetContent("one\n")
	m.viewport.SetYOffset(0)
	m.appendOutputs("two")
	if m.viewport.YOffset != 0 {
		t.Fatalf("expected viewport offset to remain, got %d", m.viewport.YOffset)
	}
}

func TestOutputAutoFollowsWhenNotInScrollMode(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.viewport.Height = 1
	m.appendOutputs("one")
	m.appendOutputs("two")
	if m.viewport.YOffset != 1 {
		t.Fatalf("expected viewport offset to be at bottom, got %d", m.viewport.YOffset)
	}
}

func TestOutputDoesNotScrollOnInputNavigation(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.history = newHistory(nil)
	m.viewport.Height = 1
	m.appendOutputs("one", "two", "three")
	start := m.viewport.YOffset
	if start == 0 {
		t.Fatalf("expected non-zero bottom offset, got %d", start)
	}

	m = pressKey(m, "up")
	if m.viewport.YOffset != start {
		t.Fatalf("expected offset unchanged on up, got %d", m.viewport.YOffset)
	}

	m = pressKey(m, "down")
	if m.viewport.YOffset != start {
		t.Fatalf("expected offset unchanged on down, got %d", m.viewport.YOffset)
	}

	m = pressKey(m, "left")
	if m.viewport.YOffset != start {
		t.Fatalf("expected offset unchanged on left, got %d", m.viewport.YOffset)
	}

	m = pressKey(m, "right")
	if m.viewport.YOffset != start {
		t.Fatalf("expected offset unchanged on right, got %d", m.viewport.YOffset)
	}
}

func TestScrollModeDoesNotChangePromptBuffer(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.history = newHistory([]string{"one"})
	m.input.SetValue("draft")
	m.scrollMode = true
	m = pressKey(m, "up")
	if m.input.Value() != "draft" {
		t.Fatalf("expected prompt buffer to remain, got %q", m.input.Value())
	}
}

func TestScrollModeScrollKeysAffectViewport(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.viewport.Height = 1
	m.appendOutputs("one", "two", "three")
	bottom := m.viewport.YOffset
	if bottom == 0 {
		t.Fatalf("expected non-zero bottom offset, got %d", bottom)
	}
	m = pressKey(m, "up")
	if m.viewport.YOffset != bottom {
		t.Fatalf("expected viewport offset unchanged outside scroll mode, got %d", m.viewport.YOffset)
	}
	m.scrollMode = true
	m = pressKey(m, "up")
	if m.viewport.YOffset >= bottom {
		t.Fatalf("expected viewport offset to decrease, got %d", m.viewport.YOffset)
	}
}

func TestHelpIncludesScroll(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.input.SetValue(":help")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(model)

	joined := strings.Join(um.output.Lines(), "\n")
	if !strings.Contains(joined, ":scroll") {
		t.Fatalf("expected help to include :scroll, got %q", joined)
	}
	if !strings.Contains(joined, "Up/Down") {
		t.Fatalf("expected help to mention Up/Down history, got %q", joined)
	}
	if !strings.Contains(joined, "Ctrl+C") || !strings.Contains(joined, "Ctrl+Z") {
		t.Fatalf("expected help to mention Ctrl+C/Ctrl+Z, got %q", joined)
	}
}

func TestCtrlCForwardsInterrupt(t *testing.T) {
	broker := make(chan sshConn.CommandRequest, 1)
	m := initialModel(nil, broker, nil)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um := updated.(model)
	if um.quit {
		t.Fatal("expected not to quit on first ctrl+c")
	}
	_ = runCmd(t, cmd)
	req := readRequest(t, broker)
	if req.Kind != sshConn.CommandKindControl {
		t.Fatalf("expected control kind, got %v", req.Kind)
	}
	if req.ControlByte != 0x03 {
		t.Fatalf("expected 0x03, got 0x%02x", req.ControlByte)
	}
}

func TestCtrlCDoublePressQuits(t *testing.T) {
	broker := make(chan sshConn.CommandRequest, 1)
	m := initialModel(nil, broker, nil)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return base }

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(model)
	_ = runCmd(t, cmd)
	_ = readRequest(t, broker)

	m.now = func() time.Time { return base.Add(400 * time.Millisecond) }
	updated, quitCmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um := updated.(model)
	if !um.quit {
		t.Fatal("expected quit on double ctrl+c")
	}
	if _, ok := quitCmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", quitCmd())
	}
	select {
	case <-broker:
		t.Fatal("did not expect ctrl+c to forward on quit press")
	default:
	}
}

func TestCtrlCAfterTimeoutForwardsAgain(t *testing.T) {
	broker := make(chan sshConn.CommandRequest, 1)
	m := initialModel(nil, broker, nil)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return base }

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = runCmd(t, cmd)
	_ = readRequest(t, broker)

	m.now = func() time.Time { return base.Add(600 * time.Millisecond) }
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um := updated.(model)
	if um.quit {
		t.Fatal("expected not to quit after timeout")
	}
	m = um
	_ = runCmd(t, cmd)
	req := readRequest(t, broker)
	if req.ControlByte != 0x03 {
		t.Fatalf("expected 0x03, got 0x%02x", req.ControlByte)
	}
}

func TestCtrlZForwardsSuspend(t *testing.T) {
	broker := make(chan sshConn.CommandRequest, 1)
	m := initialModel(nil, broker, nil)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	um := updated.(model)
	if um.quit {
		t.Fatal("expected not to quit on ctrl+z")
	}
	_ = runCmd(t, cmd)
	req := readRequest(t, broker)
	if req.ControlByte != 0x1a {
		t.Fatalf("expected 0x1a, got 0x%02x", req.ControlByte)
	}
}

func TestControlKeysForwardInScrollMode(t *testing.T) {
	broker := make(chan sshConn.CommandRequest, 2)
	m := initialModel(nil, broker, nil)
	m.scrollMode = true
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um := updated.(model)
	if um.quit {
		t.Fatal("expected not to quit on ctrl+c in scroll mode")
	}
	m = um
	_ = runCmd(t, cmd)
	req := readRequest(t, broker)
	if req.ControlByte != 0x03 {
		t.Fatalf("expected 0x03, got 0x%02x", req.ControlByte)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	um = updated.(model)
	if um.quit {
		t.Fatal("expected not to quit on ctrl+z in scroll mode")
	}
	if !um.scrollMode {
		t.Fatal("expected scroll mode to remain enabled")
	}
	_ = runCmd(t, cmd)
	req = readRequest(t, broker)
	if req.ControlByte != 0x1a {
		t.Fatalf("expected 0x1a, got 0x%02x", req.ControlByte)
	}
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected command")
	}
	return cmd()
}

func readRequest(t *testing.T, broker <-chan sshConn.CommandRequest) sshConn.CommandRequest {
	t.Helper()
	select {
	case req := <-broker:
		return req
	default:
		t.Fatal("expected broker request")
	}
	return sshConn.CommandRequest{}
}

func pressKey(m model, key string) model {
	var msg tea.KeyMsg
	switch key {
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(model)
}
func BenchmarkAppendOutputs(b *testing.B) {
	lines := make([]string, 0, 1000)
	for i := 0; i < 1000; i++ {
		lines = append(lines, fmt.Sprintf("line-%d", i))
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := initialModel(nil, nil, nil)
		m.appendOutputs(lines...)
	}
}

func TestConnectedHosts(t *testing.T) {
	hostList := sshConn.NewHostList()
	hostList.AddHost(&sshConn.Host{Hostname: "host1", IsConnected: 1})
	hostList.AddHost(&sshConn.Host{Hostname: "host2", IsConnected: 0})

	hosts := connectedHosts(hostList)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 connected host, got %d", len(hosts))
	}
	if hosts[0].Hostname != "host1" {
		t.Fatalf("unexpected host: %s", hosts[0].Hostname)
	}
}

func TestConnectedHostsNilList(t *testing.T) {
	if got := connectedHosts(nil); got != nil {
		t.Fatalf("expected nil hosts, got %#v", got)
	}
}

func TestHostnames(t *testing.T) {
	hosts := []*sshConn.Host{{Hostname: "alpha"}, {Hostname: "beta"}}
	got := hostnames(hosts)
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("unexpected hostnames: %#v", got)
	}
}

func TestInitReturnsNilCmdWithoutEvents(t *testing.T) {
	m := initialModel(nil, nil, nil)
	if cmd := m.Init(); cmd != nil {
		t.Fatalf("expected nil cmd when events channel is nil")
	}
}

func TestInitReturnsOutputBatchFromEvents(t *testing.T) {
	events := make(chan sshConn.OutputEvent, 2)
	m := initialModel(nil, nil, events)

	events <- sshConn.OutputEvent{Hostname: "h1", Line: "one"}
	events <- sshConn.OutputEvent{Hostname: "h1", Line: "two"}
	close(events)

	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("expected non-nil cmd")
	}
	msg := cmd()
	got, ok := msg.(outputMsg)
	if !ok {
		t.Fatalf("expected outputMsg, got %T", msg)
	}
	if len(got.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got.events))
	}
}
