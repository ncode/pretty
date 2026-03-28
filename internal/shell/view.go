package shell

import tea "charm.land/bubbletea/v2"

func (m model) View() tea.View {
	var content string
	if !m.quit {
		content = m.viewport.View() + "\n" + m.input.View()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
