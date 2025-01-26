package tui

import (
	"deeprefactor/internal/types"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	items      []types.TableItem
	table      tableModel
	logView    string
	quitting   bool
	updateChan chan types.FileUpdate
	width      int
	height     int
}

type tableModel struct {
	columns   []string
	rows      []types.Row
	cursor    int
	selected  int
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
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}

func InitialModel(files []*types.FileProcess) model {
	// Group files by directory
	dirMap := make(map[string][]*types.FileProcess)
	for _, f := range files {
		dir := filepath.Dir(f.Path)
		dirMap[dir] = append(dirMap[dir], f)
	}

	// Create sorted list of directories
	var dirs []string
	for dir := range dirMap {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	// Create table items with directory groups
	var items []types.TableItem
	for _, dir := range dirs {
		// Add directory row
		items = append(items, types.TableItem{
			Type:   "directory",
			Path:   dir + string(filepath.Separator),
			Indent: 0,
		})

		// Add file rows
		for _, f := range dirMap[dir] {
			items = append(items, types.TableItem{
				Type:   "file",
				Path:   filepath.Base(f.Path),
				File:   f,
				Indent: 2,
			})
		}
	}

	// Create table columns
	columns := []string{"Path", "Status", "Attempts"}

	// Create initial rows for table
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
		Border:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).Padding(0, 0),
		HeaderBorder: lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false),
	}

	return model{
		items: items,
		table: tableModel{
			columns:   columns,
			rows:      rows,
			styles:    styles,
			maxWidth:  60, // Adjust based on terminal width
			maxHeight: 20,
		},
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.maxWidth = msg.Width - 4
		m.table.maxHeight = msg.Height - 6
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.table.cursor > 0 {
				m.table.cursor--
			}
		case "down", "j":
			if m.table.cursor < len(m.table.rows)-1 {
				m.table.cursor++
			}
		case "enter":
			m.logView = strings.Join(m.items[m.table.cursor].File.Logs, "\n")
		}
	case types.FileUpdate:
		m.updateFileStatus(msg)
		return m, func() tea.Msg { return <-m.updateChan }
	}

	return m, nil
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

	// Calculate layout dimensions
	tableWidth := m.width/2 - 4
	logWidth := m.width/2 - 4
	m.table.maxWidth = tableWidth
	m.table.maxHeight = m.height - 4

	// Render table and logs
	table := m.table.View()
	logStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		Padding(1, 2).
		Width(logWidth).
		Height(m.height - 4)

	logView := logStyle.Render(m.logView)

	// Create master-detail layout
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			table,
			"  ", // Spacer between panels
			logView,
		),
	)
}

func (m tableModel) View() string {
	header := m.renderHeader()
	rows := m.renderRows()

	// Combine header and rows
	tableContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Join(rows, "\n"),
	)

	// Add border around the entire table
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
