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
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/kong"
)

// CLI command structure
type CLI struct {
	LintCommand string   `arg:"" help:"Go lint command to run"`
	Args        []string `arg:"" optional:""`
	MaxRetries  int      `flag:"" default:"5" help:"Maximum fix attempts"`
	OllamaURL   string   `flag:"" default:"http://localhost:11434" help:"Ollama server URL"`
	Model       string   `flag:"" default:"deepseek-coder-v2" help:"Ollama model to use"`
}

// Ollama API request/response structures
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error"`
}

var (
	filePathRegex = regexp.MustCompile(`([A-Za-z]:)?([\\/]?[^:\s\\/]+[\\/]?)+\.go`)
	codeBlockRe   = regexp.MustCompile(`(?s)\x60\x60\x60go(.*?)\x60\x60\x60`)
)

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("golint-fixer"),
		kong.Description("AI-powered Go lint fixer"),
		kong.UsageOnError(),
	)

	if err := cli.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func (cli *CLI) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	for iteration := 1; iteration <= cli.MaxRetries; iteration++ {
		start := time.Now()
		fmt.Printf("Iteration %d/%d starting...\n", iteration, cli.MaxRetries)

		output, err := cli.runLintCommand(ctx)
		if err == nil {
			fmt.Println("Lint checks passed!")
			return nil
		}

		fmt.Printf("Lint errors:\n%s\n", output)
		if handleErr := cli.handleLintingErrors(ctx, output); handleErr != nil {
			return fmt.Errorf("error handling lint issues: %w", handleErr)
		}

		fmt.Printf("Iteration %d completed in %v\n", iteration, time.Since(start))
	}

	return fmt.Errorf("failed to resolve issues after %d attempts", cli.MaxRetries)
}

func (cli *CLI) runLintCommand(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, cli.LintCommand, cli.Args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stderr.String() + stdout.String())

	if err != nil {
		return output, fmt.Errorf("lint command failed: %w", err)
	}
	return output, nil
}

func (cli *CLI) handleLintingErrors(ctx context.Context, lintOutput string) error {
	filePaths := ExtractFilePaths(lintOutput)
	if len(filePaths) == 0 {
		return fmt.Errorf("no Go files found in lint output")
	}

	for _, filePath := range filePaths {
		if err := cli.processFile(ctx, filePath, lintOutput); err != nil {
			return err
		}
	}

	return nil
}

func (cli *CLI) processFile(ctx context.Context, path, lintOutput string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	fixed, err := cli.getFixedCode(ctx, path, string(content), lintOutput)
	if err != nil {
		return fmt.Errorf("failed to get fix for %s: %w", path, err)
	}

	if err := safeWriteFile(path, fixed); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	fmt.Printf("Updated %s with AI-generated fix\n", path)
	return nil
}

func (cli *CLI) getFixedCode(ctx context.Context, path, content, errors string) (string, error) {
	prompt := fmt.Sprintf(`Fix these Go lint errors in %s:
%s

File content:
%s

Return only the corrected Go code with [DeepRefactor] comments. Use code blocks.`, path, errors, content)

	reqBody := OllamaRequest{
		Model:  cli.Model,
		Prompt: prompt,
		Stream: true,
	}

	resp, err := cli.sendOllamaRequest(ctx, reqBody)
	if err != nil {
		return "", err
	}

	return extractCodeBlock(resp), nil
}

func (cli *CLI) sendOllamaRequest(ctx context.Context, req OllamaRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", cli.OllamaURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result strings.Builder
	dec := json.NewDecoder(resp.Body)

	for {
		var chunk OllamaResponse
		if err := dec.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("decode error: %w", err)
		}

		if chunk.Error != "" {
			return "", fmt.Errorf("API error: %s", chunk.Error)
		}
		result.WriteString(chunk.Response)
	}

	return result.String(), nil
}

func safeWriteFile(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ExtractFilePaths(input string) []string {
	return filePathRegex.FindAllString(input, -1)
}

func extractCodeBlock(response string) string {
	matches := codeBlockRe.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return response
}

func (cli *CLI) verifyFix(ctx context.Context) error {
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		if output, err := cli.runLintCommand(ctx); err == nil {
			return nil
		} else {
			fmt.Printf("Verification failed (attempt %d):\n%s\n", i+1, output)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("fix verification failed after %d attempts", maxRetries)
}
