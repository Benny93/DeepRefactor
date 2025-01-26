package tui

import (
	"deeprefactor/internal/types"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	items      []types.TableItem
	table      tableModel
	logView    viewport.Model
	quitting   bool
	updateChan chan types.FileUpdate
	width      int
	height     int
	logFocused bool
}

type tableModel struct {
	columns   []string
	rows      []types.Row
	cursor    int
	styles    tableStyles
	maxWidth  int
	maxHeight int
}

type tableStyles struct {
	Header       lipgloss.Style
	Cell         lipgloss.Style
	Selected     lipgloss.Style
	Border       lipgloss.Style
	HeaderBorder lipgloss.Style
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

	styles := tableStyles{
		Header:       lipgloss.NewStyle().Bold(true).Padding(0, 1),
		Cell:         lipgloss.NewStyle().Padding(0, 1),
		Selected:     lipgloss.NewStyle().Background(lipgloss.Color("#3C3C3C")).Padding(0, 1),
		Border:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 0),
		HeaderBorder: lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false),
	}

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)
	vp.MouseWheelEnabled = true

	return model{
		items: items,
		table: tableModel{
			columns:   columns,
			rows:      rows,
			styles:    styles,
			maxWidth:  60,
			maxHeight: 20,
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
		m.width = msg.Width
		m.height = msg.Height
		tableWidth := m.width/2 - 4
		logWidth := m.width/2 - 6
		logHeight := m.height - 6

		m.table.maxWidth = tableWidth
		m.table.maxHeight = logHeight
		m.logView.Width = logWidth
		m.logView.Height = logHeight
		m.updateLogView()

	case tea.KeyMsg:
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
				return m, tea.Quit
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

	case tea.MouseMsg:
		if m.logFocused {
			m.logView, cmd = m.logView.Update(msg)
			cmds = append(cmds, cmd)
		}

	case types.FileUpdate:
		m.updateFileStatus(msg)
		m.updateLogView()
		return m, func() tea.Msg { return <-m.updateChan }
	}

	if m.logFocused {
		m.logView, cmd = m.logView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) updateLogView() {
	if m.table.cursor >= 0 && m.table.cursor < len(m.items) {
		item := m.items[m.table.cursor]
		if item.Type == "file" {
			content := strings.Join(item.File.Logs, "\n")
			m.logView.SetContent(content)
			if !m.logFocused {
				m.logView.GotoBottom()
			}
		}
	}
}

func (m *model) updateFileStatus(update types.FileUpdate) {
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
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	table := m.table.View()
	logStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(m.logView.Width).
		Height(m.logView.Height)

	if m.logFocused {
		logStyle = logStyle.BorderForeground(lipgloss.Color("62"))
	}

	logView := logStyle.Render(m.logView.View())

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			table,
			"  ",
			logView,
		),
	)
}

func (m tableModel) View() string {
	header := m.renderHeader()
	rows := m.renderRows()

	tableContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Join(rows, "\n"),
	)

	return m.styles.Border.
		Width(m.maxWidth).
		Height(m.maxHeight).
		Render(tableContent)
}

func (m tableModel) renderHeader() string {
	var cols []string
	for _, col := range m.columns {
		cols = append(cols, m.styles.Header.Render(col))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Left, cols...)
	return m.styles.HeaderBorder.Render(header) + "\n"
}

func (m tableModel) renderRows() []string {
	var rows []string
	for i, row := range m.rows {
		var cells []string
		style := m.styles.Cell
		if i == m.cursor {
			style = m.styles.Selected.
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderLeftForeground(lipgloss.Color("#FFFFFF"))

		}

		for _, d := range row.Data {
			cells = append(cells, style.Render(d))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, cells...))
	}
	return rows
}
