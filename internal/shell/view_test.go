package shell

import "testing"

func TestViewReturnsEmptyWhenQuit(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.quit = true

	got := m.View()
	if got.Content != "" {
		t.Fatalf("expected empty view on quit, got %q", got.Content)
	}
	if !got.AltScreen {
		t.Fatal("expected AltScreen to be true")
	}
}

func TestViewIncludesViewportAndInput(t *testing.T) {
	m := initialModel(nil, nil, nil)
	m.viewport.SetContent("output-line")
	m.input.SetValue("echo hi")

	got := m.View()
	if got.Content == "" {
		t.Fatalf("expected non-empty view")
	}
	if !got.AltScreen {
		t.Fatal("expected AltScreen to be true")
	}
}
