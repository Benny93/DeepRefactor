package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *model) handleKeys(msg tea.KeyMsg) tea.Cmd {
	if m.logFocused {
		switch msg.String() {
		case "q", "esc":
			m.logFocused = false
		case "up", "k":
			m.logView.LineUp(1)
		case "down", "j":
			m.logView.LineDown(1)
		case "pgup":
			m.logView.HalfViewUp()
		case "pgdown":
			m.logView.HalfViewDown()
		case "g":
			m.logView.GotoTop()
		case "G":
			m.logView.GotoBottom()
		}
	} else {
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return tea.Quit
		case "up", "k":
			if m.table.cursor > 0 {
				m.table.cursor--
			}
			m.updateLogView()
		case "down", "j":
			if m.table.cursor < len(m.table.rows)-1 {
				m.table.cursor++
			}
			m.updateLogView()
		case "enter":
			m.logFocused = true
		}
	}
	return nil
}

func (m *model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	var cmd tea.Cmd
	if m.logFocused {
		m.logView, cmd = m.logView.Update(msg)
	}
	return cmd
}
