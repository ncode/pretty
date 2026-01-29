package shell

func (m model) View() string {
	if m.quit {
		return ""
	}
	return m.viewport.View() + "\n" + m.input.View()
}
