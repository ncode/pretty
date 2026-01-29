package shell

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/ncode/pretty/internal/jobs"
	"github.com/ncode/pretty/internal/sshConn"
	"github.com/spf13/viper"
)

type outputMsg struct {
	events []sshConn.OutputEvent
}

const (
	defaultPrompt  = "> "
	maxOutputLines = 10000
)

type model struct {
	input      textinput.Model
	viewport   viewport.Model
	output     *outputBuffer
	quit       bool
	history    *historyState
	scrollMode bool

	lastCtrlCAt time.Time
	now         func() time.Time

	hostList   *sshConn.HostList
	hostColors map[string]*color.Color
	jobs       *jobs.Manager
	broker     chan<- sshConn.CommandRequest
	events     chan sshConn.OutputEvent
}

func initialModel(hostList *sshConn.HostList, broker chan<- sshConn.CommandRequest, events chan sshConn.OutputEvent) model {
	input := textinput.New()
	input.Prompt = promptFromConfig()
	input.Focus()

	vp := viewport.New(0, 0)
	vp.SetContent("")

	historyEntries, _ := loadHistory(viper.GetString("history_file"))
	history := newHistory(historyEntries)

	var hostColors map[string]*color.Color
	if hostList != nil {
		hostColors = make(map[string]*color.Color, hostList.Len())
		for _, host := range hostList.Hosts() {
			hostColors[host.Hostname] = host.Color
		}
	}

	return model{
		input:      input,
		viewport:   vp,
		output:     newOutputBuffer(maxOutputLines),
		history:    history,
		now:        time.Now,
		hostList:   hostList,
		hostColors: hostColors,
		jobs:       jobs.NewManager(),
		broker:     broker,
		events:     events,
	}
}

func promptFromConfig() string {
	if viper.IsSet("prompt") {
		return viper.GetString("prompt")
	}
	return defaultPrompt
}

func (m model) Init() tea.Cmd {
	return listenOutput(m.events)
}

func appendLine(lines []string, line string) []string {
	return append(lines, line)
}

func colorizeHostLine(hostColors map[string]*color.Color, hostname, line string) string {
	if hostname == "" || hostColors == nil {
		return line
	}
	hostColor := hostColors[hostname]
	if hostColor == nil {
		return line
	}
	return hostColor.Sprint(line)
}

func listenOutput(events <-chan sshConn.OutputEvent) tea.Cmd {
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-events
		if !ok {
			return nil
		}
		batch := make([]sshConn.OutputEvent, 1, 16)
		batch[0] = evt
		for {
			select {
			case next, ok := <-events:
				if !ok {
					return outputMsg{events: batch}
				}
				batch = append(batch, next)
			default:
				return outputMsg{events: batch}
			}
		}
	}
}

func (m *model) appendLines(lines ...string) {
	m.output.Append(lines...)
}

func (m *model) flushOutputs() {
	offset := m.viewport.YOffset
	m.viewport.SetContent(m.output.String())
	if m.scrollMode {
		m.viewport.SetYOffset(offset)
		return
	}
	m.viewport.GotoBottom()
}

func (m *model) appendOutputs(lines ...string) {
	m.appendLines(lines...)
	m.flushOutputs()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			now := m.now()
			if !m.lastCtrlCAt.IsZero() && now.Sub(m.lastCtrlCAt) <= 500*time.Millisecond {
				m.quit = true
				return m, tea.Quit
			}
			m.lastCtrlCAt = now
			request := sshConn.CommandRequest{Kind: sshConn.CommandKindControl, ControlByte: 0x03}
			return m, sendCommand(m.broker, request)
		case "ctrl+z":
			request := sshConn.CommandRequest{Kind: sshConn.CommandKindControl, ControlByte: 0x1a}
			return m, sendCommand(m.broker, request)
		case "up":
			if m.scrollMode {
				break
			}
			if next, ok := m.history.up(m.input.Value()); ok {
				m.input.SetValue(next)
				m.input.CursorEnd()
				return m, nil
			}
		case "down":
			if m.scrollMode {
				break
			}
			if next, ok := m.history.down(); ok {
				m.input.SetValue(next)
				m.input.CursorEnd()
				return m, nil
			}
		case "esc":
			if m.scrollMode {
				m.scrollMode = false
				m.input.Focus()
				m.viewport.GotoBottom()
				return m, nil
			}
		case "enter":
			line := m.input.Value()
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				historyPath := viper.GetString("history_file")
				if historyPath != "" {
					_ = appendHistory(historyPath, trimmed)
				}
				m.history.append(trimmed)
			}
			command := ParseCommand(line)
			if command.Kind == CommandScroll {
				m.scrollMode = true
				m.input.Blur()
				return m, nil
			}
			m.input.Reset()
			switch command.Kind {
			case CommandExit:
				m.quit = true
				return m, tea.Quit
			case CommandHelp:
				m.appendOutputs(
					"commands: :async <command>, :status [id], :list, :help, :scroll, :bye",
					"history: use Up/Down to navigate previous commands",
					"keys: Ctrl+C forwards interrupt; double Ctrl+C (500ms) quits; Ctrl+Z forwards suspend",
					"scroll: :scroll to enter, esc to return (output scroll only in scroll mode)",
				)
				return m, nil
			case CommandList:
				if m.hostList == nil {
					m.appendOutputs("no hosts configured")
					return m, nil
				}
				for _, host := range m.hostList.Hosts() {
					connected := atomic.LoadInt32(&host.IsConnected) == 1
					line := fmt.Sprintf("%s: Connected(%t)", host.Hostname, connected)
					m.appendOutputs(colorizeHostLine(m.hostColors, host.Hostname, line))
				}
				return m, nil
			case CommandStatus:
				lines := statusLines(m.jobs, command.JobID, func(hostname, line string) string {
					return colorizeHostLine(m.hostColors, hostname, line)
				})
				m.appendOutputs(lines...)
				return m, nil
			case CommandAsync:
				if command.Arg == "" {
					return m, nil
				}
				hosts := connectedHosts(m.hostList)
				if len(hosts) == 0 {
					m.appendOutputs("no connected hosts")
					return m, nil
				}
				hostnames := hostnames(hosts)
				job := m.jobs.CreateJob(jobs.JobTypeAsync, command.Arg, hostnames)
				for _, host := range hosts {
					m.jobs.MarkHostRunning(job.ID, host.Hostname)
				}
				return m, runAsync(job.ID, command.Arg, hosts, m.events, m.jobs)
			case CommandRun:
				if command.Arg == "" {
					return m, nil
				}
				hosts := connectedHosts(m.hostList)
				if len(hosts) == 0 {
					m.appendOutputs("no connected hosts")
					return m, nil
				}
				hostnames := hostnames(hosts)
				job := m.jobs.CreateJob(jobs.JobTypeNormal, command.Arg, hostnames)
				for _, host := range hosts {
					m.jobs.MarkHostRunning(job.ID, host.Hostname)
				}
				request := sshConn.CommandRequest{JobID: job.ID, Command: wrapCommand(command.Arg, job.ID)}
				return m, sendCommand(m.broker, request)
			}
		}
	case tea.WindowSizeMsg:
		height := msg.Height - 1
		if height < 0 {
			height = 0
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = height
		m.input.Width = msg.Width
		return m, nil
	case outputMsg:
		needsFlush := false
		for _, evt := range msg.events {
			if prefix, jobID, exitCode, ok := jobs.ExtractSentinel(evt.Line); ok {
				if prefix != "" {
					if evt.System {
						m.appendLines(prefix)
					} else {
						line := fmt.Sprintf("%s: %s", evt.Hostname, prefix)
						m.appendLines(colorizeHostLine(m.hostColors, evt.Hostname, line))
					}
					needsFlush = true
				}
				m.jobs.MarkHostDone(jobID, evt.Hostname, exitCode, exitCode == 0)
				continue
			}
			if evt.System {
				m.appendLines(evt.Line)
			} else {
				line := fmt.Sprintf("%s: %s", evt.Hostname, evt.Line)
				m.appendLines(colorizeHostLine(m.hostColors, evt.Hostname, line))
			}
			needsFlush = true
		}
		if needsFlush {
			m.flushOutputs()
		}
		return m, listenOutput(m.events)
	}

	var (
		inputCmd    tea.Cmd
		viewportCmd tea.Cmd
	)
	m.input, inputCmd = m.input.Update(msg)
	allowViewport := true
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		if !m.scrollMode {
			allowViewport = false
		}
	}
	if allowViewport {
		m.viewport, viewportCmd = m.viewport.Update(msg)
	}
	return m, tea.Batch(inputCmd, viewportCmd)
}

func sendCommand(broker chan<- sshConn.CommandRequest, request sshConn.CommandRequest) tea.Cmd {
	if broker == nil {
		return nil
	}
	return func() tea.Msg {
		broker <- request
		return nil
	}
}

func runAsync(jobID int, command string, hosts []*sshConn.Host, events chan<- sshConn.OutputEvent, manager *jobs.Manager) tea.Cmd {
	if len(hosts) == 0 {
		return nil
	}
	return func() tea.Msg {
		for _, host := range hosts {
			h := host
			go func() {
				exitCode, err := sshConn.RunCommand(h, command, jobID, events)
				if err != nil {
					manager.MarkHostDone(jobID, h.Hostname, exitCode, false)
					return
				}
				manager.MarkHostDone(jobID, h.Hostname, exitCode, exitCode == 0)
			}()
		}
		return nil
	}
}

func connectedHosts(hostList *sshConn.HostList) []*sshConn.Host {
	if hostList == nil {
		return nil
	}
	hosts := make([]*sshConn.Host, 0, hostList.Len())
	for _, host := range hostList.Hosts() {
		if atomic.LoadInt32(&host.IsConnected) == 1 {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func hostnames(hosts []*sshConn.Host) []string {
	names := make([]string, 0, len(hosts))
	for _, host := range hosts {
		names = append(names, host.Hostname)
	}
	return names
}
