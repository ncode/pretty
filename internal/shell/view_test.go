package shell

import "testing"

func TestViewReturnsEmptyWhenQuit(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.quit = true

	if got := m.View(); got != "" {
		t.Fatalf("expected empty view on quit, got %q", got)
	}
}

func TestViewIncludesViewportAndInput(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.viewport.SetContent("output-line")
	m.input.SetValue("echo hi")

	got := m.View()
	if got == "" {
		t.Fatalf("expected non-empty view")
	}
}
