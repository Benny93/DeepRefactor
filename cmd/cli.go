package cmd

import (
	"context"
	"deeprefactor/internal/ai"
	"deeprefactor/internal/processor"
	"deeprefactor/internal/tui"
	"deeprefactor/internal/types"
	"fmt"
	"strings"
	"sync"
	"time"
)

type CLI struct {
	Dir        string `flag:"" default:"." help:"Directory to search for Go files"`
	MaxRetries int    `flag:"" default:"5" help:"Maximum fix attempts per file"`
	OllamaURL  string `flag:"" default:"http://localhost:11434" help:"Ollama server URL"`
	Model      string `flag:"" default:"deepseek-coder-v2" help:"Ollama model to use"`
	LintCmd    string `flag:"" default:"golangci-lint run {{filepath}}" help:"Lint command template (use {{filepath}})"`
}

func (cli *CLI) Run() error {
	files, err := processor.FindGoFiles(cli.Dir)
	if err != nil {
		return fmt.Errorf("error finding Go files: %w", err)
	}

	tui.Create(files, func(updates chan<- types.FileUpdate, items []types.TableItem) {
		go cli.processFiles(updates, items)
	})
	return nil
}

func (cli *CLI) processFiles(updates chan<- types.FileUpdate, items []types.TableItem) {
	var wg sync.WaitGroup

	for _, item := range items {
		if item.Type != "file" {
			continue
		}
		wg.Add(1)
		go func(file *types.FileProcess) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			for attempt := 1; attempt <= cli.MaxRetries; attempt++ {
				updates <- types.FileUpdate{
					Path:   file.Path,
					Status: fmt.Sprintf("Attempt %d/%d", attempt, cli.MaxRetries),
				}

				lintCmd := strings.Replace(cli.LintCmd, "{{filepath}}", file.Path, 1)
				output, err := processor.RunLintCommand(ctx, lintCmd)
				if err == nil {
					updates <- types.FileUpdate{Path: file.Path, Status: "Fixed", Log: "Lint passed"}
					return
				}

				updates <- types.FileUpdate{Path: file.Path, Log: fmt.Sprintf("Lint errors:\n%s", output)}
				if err := cli.fixFile(ctx, file.Path, output, updates); err != nil {
					updates <- types.FileUpdate{Path: file.Path, Log: fmt.Sprintf("Fix error: %v", err)}
				}
			}
			updates <- types.FileUpdate{Path: file.Path, Status: "Failed"}
		}(item.File)
	}

	wg.Wait()
	close(updates)
}

func (cli *CLI) fixFile(ctx context.Context, path string, lintOutput string, updates chan<- types.FileUpdate) error {
	aiClient := ai.NewClient(cli.OllamaURL, cli.Model)

	err := aiClient.FixFile(ctx, path, lintOutput, updates)
	return err
}
