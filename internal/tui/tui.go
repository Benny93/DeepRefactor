package tui

import (
	"deeprefactor/internal/types"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Table styling
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#5A5A5A")).
				Padding(0, 1)

	tableDirectoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7D56F4")).
				Padding(0, 1)

	tableFileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D0D0D0")).
			Padding(0, 1)

	tableSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#3C3C3C")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1)

	tableBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#4A4A4A")).
				Padding(0, 0)

	// Log view styling
	logHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5A5A5A")).
			Padding(0, 1)

	logFooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")).
			Background(lipgloss.Color("#2B2B2B")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")).
			Background(lipgloss.Color("#2B2B2B")).
			Padding(0, 1)
)

type model struct {
	items         []types.TableItem
	table         tableModel
	logView       viewport.Model
	quitting      bool
	updateChan    chan types.FileUpdate
	width         int
	height        int
	logFocused    bool
	lastUpdate    sync.Mutex
	statusMessage string
}

type tableModel struct {
	columns    []string
	rows       []types.Row
	cursor     int
	maxWidth   int
	maxHeight  int
	totalItems int
}

func Create(files []*types.FileProcess, processFunc func(updates chan<- types.FileUpdate, items []types.TableItem)) error {
	m := InitialModel(files)
	m.updateChan = make(chan types.FileUpdate, 100)

	processFunc(m.updateChan, m.items)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}

func InitialModel(files []*types.FileProcess) model {
	dirMap := make(map[string][]*types.FileProcess)
	for _, f := range files {
		dir := filepath.Dir(f.Path)
		dirMap[dir] = append(dirMap[dir], f)
	}

	var dirs []string
	for dir := range dirMap {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var items []types.TableItem
	for _, dir := range dirs {
		items = append(items, types.TableItem{
			Type:   "directory",
			Path:   dir + string(filepath.Separator),
			Indent: 0,
		})

		for _, f := range dirMap[dir] {
			items = append(items, types.TableItem{
				Type:   "file",
				Path:   filepath.Base(f.Path),
				File:   f,
				Indent: 2,
			})
		}
	}

	columns := []string{"Path", "Status", "Attempts"}
	var rows []types.Row
	for _, item := range items {
		var status, attempts string
		if item.Type == "file" {
			status = item.File.Status
			attempts = fmt.Sprintf("%d/%d", item.File.Retries, 5)
		}

		rows = append(rows, types.Row{
			Key:  item.Path,
			Data: []string{item.Path, status, attempts},
		})
	}

	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.Style = tableBorderStyle

	return model{
		items: items,
		table: tableModel{
			columns:    columns,
			rows:       rows,
			maxWidth:   60,
			maxHeight:  20,
			totalItems: len(items),
		},
		logView:    vp,
		updateChan: make(chan types.FileUpdate, 100),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		func() tea.Msg { return <-m.updateChan },
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleResize(msg)

	case tea.KeyMsg:
		cmds = append(cmds, m.handleKeys(msg))

	case tea.MouseMsg:
		cmds = append(cmds, m.handleMouse(msg))

	case types.FileUpdate:
		m.handleFileUpdate(msg)
		return m, func() tea.Msg { return <-m.updateChan }
	}

	if m.logFocused {
		m.logView, cmd = m.logView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) handleResize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	headerHeight := lipgloss.Height(m.logHeaderView())
	footerHeight := lipgloss.Height(m.logFooterView())
	verticalMargin := headerHeight + footerHeight

	tableWidth := m.width/2 - 4
	logWidth := m.width/2 - 4
	logHeight := m.height - verticalMargin - 4

	m.table.maxWidth = tableWidth
	m.table.maxHeight = logHeight + verticalMargin
	m.logView.Width = logWidth
	m.logView.Height = logHeight
	m.logView.YPosition = headerHeight + 1
	m.updateLogView()
}

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

func (m *model) handleFileUpdate(update types.FileUpdate) {
	m.lastUpdate.Lock()
	defer m.lastUpdate.Unlock()

	for i, item := range m.items {
		if item.Type == "file" && item.File.Path == update.Path {
			item.File.Mutex.Lock()
			if update.Status != "" {
				item.File.Status = update.Status
			}
			if update.Log != "" {
				item.File.Logs = append(item.File.Logs, update.Log)
			}
			if strings.Contains(update.Status, "Attempt") {
				item.File.Retries++
			}
			item.File.Mutex.Unlock()

			m.table.rows[i].Data = []string{
				strings.Repeat(" ", item.Indent) + filepath.Base(item.File.Path),
				item.File.Status,
				fmt.Sprintf("%d/%d", item.File.Retries, 5),
			}
			break
		}
	}
	m.updateLogView()
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

func (m model) View() string {
	if m.quitting {
		return ""
	}

	table := m.renderTable()
	logView := m.renderLogView()

	mainView := lipgloss.JoinHorizontal(
		lipgloss.Top,
		table,
		"  ",
		logView,
	)

	statusBar := statusBarStyle.Render(fmt.Sprintf(
		" %d items | %s | %s ",
		m.table.totalItems,
		m.getStatusMessage(),
		"↑/↓: Navigate • Enter: Logs • Q: Quit",
	))

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Left,
			mainView,
			statusBar,
		),
	)
}

func (m model) renderTable() string {
	header := m.renderTableHeader()
	rows := m.renderTableRows()

	return tableBorderStyle.
		Width(m.table.maxWidth).
		Height(m.table.maxHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, rows))
}

func (m model) renderTableHeader() string {
	var headers []string
	for i, col := range m.table.columns {
		width := 10
		if i == 0 {
			width = m.table.maxWidth - 40
		}
		header := tableHeaderStyle.Width(width).Render(col)
		headers = append(headers, header)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, headers...)
}

func (m model) renderTableRows() string {
	var renderedRows []string
	for i, row := range m.table.rows {
		item := m.items[i]
		var cells []string

		for j, d := range row.Data {
			style := tableFileStyle
			if item.Type == "directory" {
				style = tableDirectoryStyle
			}
			if i == m.table.cursor {
				style = tableSelectedStyle
			}

			// Apply different width constraints
			switch m.table.columns[j] {
			case "Path":
				cells = append(cells, style.Width(m.table.maxWidth-40).Render(d))
			case "Status":
				cells = append(cells, style.Width(18).Render(d))
			case "Attempts":
				cells = append(cells, style.Width(10).Render(d))
			}
		}
		renderedRows = append(renderedRows, lipgloss.JoinHorizontal(lipgloss.Left, cells...))
	}
	return strings.Join(renderedRows, "\n")
}

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
			title = filepath.Base(item.File.Path)
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

func (m *model) getStatusMessage() string {
	if m.statusMessage != "" {
		return m.statusMessage
	}
	if len(m.items) == 0 {
		return "No files processed"
	}
	return "OK"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
