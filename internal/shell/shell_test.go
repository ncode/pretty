package shell

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/ncode/pretty/internal/sshConn"
)

func TestOutputBufferSizeMinimum(t *testing.T) {
	if got := outputBufferSize(0); got != 128 {
		t.Fatalf("unexpected size: %d", got)
	}
}

func TestOutputBufferSizeScales(t *testing.T) {
	if got := outputBufferSize(10); got != 288 {
		t.Fatalf("unexpected size: %d", got)
	}
}

func TestSpawnExitsOnBye(t *testing.T) {
	orig := runProgram
	t.Cleanup(func() { runProgram = orig })

	var final tea.Model
	runProgram = func(m tea.Model) (tea.Model, error) {
		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
		tm.Type(":bye")
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
		final = tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
		return final, nil
	}

	Spawn(sshConn.NewHostList())

	m, ok := final.(model)
	if !ok {
		t.Fatalf("expected model, got %T", final)
	}
	if !m.quit {
		t.Fatal("expected model to be in quit state")
	}
}
