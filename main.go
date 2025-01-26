package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	codeBlockRe = regexp.MustCompile(`(?s)\x60\x60\x60go(.*?)\x60\x60\x60`)
)

type CLI struct {
	Dir        string `flag:"" default:"." help:"Directory to search for Go files"`
	MaxRetries int    `flag:"" default:"5" help:"Maximum fix attempts per file"`
	OllamaURL  string `flag:"" default:"http://localhost:11434" help:"Ollama server URL"`
	Model      string `flag:"" default:"deepseek-coder-v2" help:"Ollama model to use"`
	LintCmd    string `flag:"" default:"golangci-lint run {{filepath}}" help:"Lint command template (use {{filepath}})"`
}

type FileProcess struct {
	Path     string
	Status   string
	Logs     []string
	Retries  int
	Selected bool
	mu       sync.Mutex
}

type model struct {
	files      []*FileProcess
	table      tableModel
	logView    string
	quitting   bool
	updateChan chan FileUpdate
	width      int
	height     int
}

type FileUpdate struct {
	Path   string
	Status string
	Log    string
}

type tableModel struct {
	columns   []string
	rows      []Row
	cursor    int
	selected  int
	styles    tableStyles
	maxWidth  int
	maxHeight int
}

type Row struct {
	Key  string
	Data []string
}

// Update the tableStyles struct
type tableStyles struct {
	Header       lipgloss.Style
	Cell         lipgloss.Style
	Selected     lipgloss.Style
	Border       lipgloss.Style
	HeaderBorder lipgloss.Style
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("golint-fixer"),
		kong.Description("AI-powered Go lint fixer"),
		kong.UsageOnError(),
	)

	if err := ctx.Run(&cli); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := cli.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func (cli *CLI) Run() error {
	files, err := findGoFiles(cli.Dir)
	if err != nil {
		return fmt.Errorf("error finding Go files: %w", err)
	}

	m := initialModel(files)
	m.updateChan = make(chan FileUpdate, 100)

	go cli.processFiles(m.updateChan, files)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}

// Update the initialModel function
func initialModel(files []*FileProcess) model {
	columns := []string{"File", "Status", "Attempts"}
	var rows []Row
	for _, f := range files {
		rows = append(rows, Row{
			Key:  f.Path,
			Data: []string{shortPath(f.Path), f.Status, fmt.Sprintf("%d/%d", f.Retries, 5)},
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
		files: files,
		table: tableModel{
			columns:   columns,
			rows:      rows,
			styles:    styles,
			maxWidth:  60, // Adjust based on terminal width
			maxHeight: 20,
		},
		updateChan: make(chan FileUpdate, 100),
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
			m.logView = strings.Join(m.files[m.table.cursor].Logs, "\n")
		}
	case FileUpdate:
		m.updateFileStatus(msg)
		return m, func() tea.Msg { return <-m.updateChan }
	}

	return m, nil
}

func (m *model) updateFileStatus(update FileUpdate) {
	for i, f := range m.files {
		if f.Path == update.Path {
			f.mu.Lock()
			if update.Status != "" {
				f.Status = update.Status
			}
			if update.Log != "" {
				f.Logs = append(f.Logs, update.Log)
			}
			if strings.Contains(update.Status, "Attempt") {
				f.Retries++
			}
			f.mu.Unlock()

			m.table.rows[i].Data = []string{
				shortPath(f.Path),
				f.Status,
				fmt.Sprintf("%d/%d", f.Retries, 5),
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

func (cli *CLI) processFiles(updates chan<- FileUpdate, files []*FileProcess) {
	var wg sync.WaitGroup

	for _, file := range files {
		wg.Add(1)
		go func(file *FileProcess) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			for attempt := 1; attempt <= cli.MaxRetries; attempt++ {
				updates <- FileUpdate{
					Path:   file.Path,
					Status: fmt.Sprintf("Attempt %d/%d", attempt, cli.MaxRetries),
				}

				lintCmd := strings.Replace(cli.LintCmd, "{{filepath}}", file.Path, 1)
				output, err := runLintCommand(ctx, lintCmd)
				if err == nil {
					updates <- FileUpdate{Path: file.Path, Status: "Fixed", Log: "Lint passed"}
					return
				}

				updates <- FileUpdate{Path: file.Path, Log: fmt.Sprintf("Lint errors:\n%s", output)}
				if err := cli.fixFile(ctx, file.Path, output, updates); err != nil {
					updates <- FileUpdate{Path: file.Path, Log: fmt.Sprintf("Fix error: %v", err)}
				}
			}
			updates <- FileUpdate{Path: file.Path, Status: "Failed"}
		}(file)
	}

	wg.Wait()
	close(updates)
}

func (cli *CLI) fixFile(ctx context.Context, path string, lintOutput string, updates chan<- FileUpdate) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	fixed, err := cli.getFixedCode(ctx, path, string(content), lintOutput)
	if err != nil {
		return fmt.Errorf("AI fix: %w", err)
	}

	if err := safeWriteFile(path, fixed); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	updates <- FileUpdate{Path: path, Log: "Applied AI fix"}
	return nil
}

func (cli *CLI) getFixedCode(ctx context.Context, path, content, errors string) (string, error) {
	prompt := fmt.Sprintf(`Fix these Go lint errors in %s:
%s

File content:
%s

Return only the corrected Go code with [DeepRefactor] comments. Use code blocks.`, path, errors, content)

	reqBody := struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}{
		Model:  cli.Model,
		Prompt: prompt,
		Stream: false,
	}

	resp, err := cli.sendOllamaRequest(ctx, reqBody)
	if err != nil {
		return "", err
	}

	return extractCodeBlock(resp), nil
}

func (cli *CLI) sendOllamaRequest(ctx context.Context, reqBody interface{}) (string, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cli.OllamaURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response failed: %w", err)
	}

	return response.Response, nil
}

func safeWriteFile(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func findGoFiles(dir string) ([]*FileProcess, error) {
	var files []*FileProcess
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, &FileProcess{
				Path:   path,
				Status: "Pending",
			})
		}
		return nil
	})
	return files, err
}

func runLintCommand(ctx context.Context, cmd string) (string, error) {
	parts := strings.Split(cmd, " ")
	var stdout, stderr bytes.Buffer
	c := exec.CommandContext(ctx, parts[0], parts[1:]...)
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	output := strings.TrimSpace(stderr.String() + stdout.String())

	if err != nil {
		return output, fmt.Errorf("lint failed: %w", err)
	}
	return output, nil
}

func shortPath(path string) string {
	if len(path) > 50 {
		return "..." + path[len(path)-47:]
	}
	return path
}

func extractCodeBlock(response string) string {
	matches := codeBlockRe.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return response
}
