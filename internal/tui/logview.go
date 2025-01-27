package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) renderLogView() string {
	logContent := lipgloss.JoinVertical(lipgloss.Left,
		m.logHeaderView(),
		m.logView.View(),
		m.logFooterView(),
	)

	return tableBorderStyle.
		Width(m.logView.Width + 2).
		Height(m.logView.Height + lipgloss.Height(m.logHeaderView()) + lipgloss.Height(m.logFooterView())).
		Render(logContent)
}

func (m model) logHeaderView() string {
	var title string
	if m.table.cursor >= 0 && m.table.cursor < len(m.items) {
		item := m.items[m.table.cursor]
		if item.Type == "file" {
			cmdName := strings.ReplaceAll(m.lintCmd, "{{filepath}}", filepath.Base(item.Path))
			title = cmdName
		}
	}
	if title == "" {
		title = "No file selected"
	}
	return logHeaderStyle.Render("📄 " + title)
}

func (m model) logFooterView() string {
	info := fmt.Sprintf(" %3.f%% ", m.logView.ScrollPercent()*100)
	if m.logFocused {
		info += " ↑/↓: scroll • ESC: back "
	}
	return logFooterStyle.Render(info)
}

func (m *model) updateLogView() {
	if m.table.cursor >= 0 && m.table.cursor < len(m.items) {
		item := m.items[m.table.cursor]
		if item.Type == "file" {
			var lines []string
			for i, log := range item.File.Logs {
				lines = append(lines, fmt.Sprintf("%4d │ %s", i+1, log))
			}
			content := strings.Join(lines, "\n")
			m.logView.SetContent(content)

			if m.logView.AtBottom() {
				m.logView.GotoBottom()
			}
		}
	}
}
