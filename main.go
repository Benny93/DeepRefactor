package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/kong"
)

// Define the CLI command structure
type CLI struct {
	LintCommand string   `arg:"" help:"The Go lint command to run."`
	Args        []string `arg:"" optional:""`
}

// Define the request body for the Ollama API
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// Define the response structure for the Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
}

// ExtractFilePath extracts a file path from a given string.
// It supports both Windows-style paths (e.g., `C:\path\to\file.go`) and
// Unix-style paths (e.g., `/home/user/project/file.go`).
func ExtractFilePath(input string) string {
	// Regular expression to match file paths (both absolute and relative)
	re := regexp.MustCompile(`([A-Za-z]:)?([\\/]?[^:\s\\/]+[\\/]?)+\.go`)
	return re.FindString(input)
}

func (cli *CLI) Run() error {
	iteration := 1
	for {
		startTime := time.Now()
		fmt.Printf("Starting iteration %d...\n", iteration)

		// Run the linting command
		lintOutput, err := cli.runLintCommand()
		if err != nil {
			// Handle linting errors
			if err := cli.handleLintingErrors(lintOutput); err != nil {
				return err
			}
		} else {
			// No linting errors found
			fmt.Println("No linting errors found.")
			break
		}

		// Log the time taken for the iteration
		duration := time.Since(startTime)
		fmt.Printf("Iteration %d completed in %v.\n", iteration, duration)
		iteration++
	}

	return nil
}

// runLintCommand runs the linting command and returns its output.
func (cli *CLI) runLintCommand() (string, error) {
	cmd := exec.Command(cli.LintCommand, cli.Args...)

	// Capture standard output and standard error
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	if err := cmd.Run(); err != nil {
		// Combine stdout and stderr for error handling
		return fmt.Sprintf("%s\n%s", stderr.String(), stdout.String()), err
	}

	// Return the combined output if no errors
	return stdout.String(), nil
}

// handleLintingErrors processes linting errors and fixes the affected file.
func (cli *CLI) handleLintingErrors(lintOutput string) error {
	fmt.Printf("Linting errors:\n%s", lintOutput)

	// Extract file path from error message
	filePath := ExtractFilePath(lintOutput)
	if filePath == "" {
		fmt.Println("No file path found.")
		return fmt.Errorf("no file path found in linting output")
	}
	fmt.Println("Filepath found at " + filePath)

	// Read the content of the affected file
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	// Call Ollama server to fix the errors
	if err := callOllamaServer(filePath, string(fileContent), lintOutput); err != nil {
		return fmt.Errorf("failed to call Ollama server: %v", err)
	}

	return nil
}

func callOllamaServer(fileName, fileContent, errorOutput string) error {
	// Prepare the prompt for Ollama
	prompt := fmt.Sprintf(`I have the following Go linting errors in the file %s:
%s

Here is the content of the file:
%s

Please provide a complete fixed version of the file. Do not include any explanations or additional text. Only return the complete code. Add comments to the code where you applied a fix per line. Also add a comment that summarized all the linter issues. All your comments should have the prefix [DeepRefactor]`, fileName, errorOutput, fileContent)

	// Create the request body
	requestBody := OllamaRequest{
		Model:  "deepseek-coder-v2", // Replace with your desired model
		Prompt: prompt,
		Stream: false,
	}

	// Marshal the request body to JSON
	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Send the request to the local Ollama server
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(requestBodyJSON))
	if err != nil {
		return fmt.Errorf("failed to send request to Ollama server: %v", err)
	}
	defer resp.Body.Close()

	// Decode the response
	var ollamaResponse OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResponse); err != nil {
		return fmt.Errorf("failed to decode response from Ollama server: %v", err)
	}

	// Remove escaping backticks (```) from the response
	fixedContent := removeEscapingBackticks(ollamaResponse.Response)

	// Output the suggested fix
	fmt.Printf("Fixed version of the file:\n%s\n", fixedContent)

	// Write the fixed content back to the file
	if err := os.WriteFile(fileName, []byte(fixedContent), 0644); err != nil {
		return fmt.Errorf("failed to write fixed content to file %s: %v", fileName, err)
	}

	fmt.Printf("File %s has been updated with the fixed version.\n", fileName)
	return nil
}

// removeEscapingBackticks removes the escaping backticks (```) from the response
func removeEscapingBackticks(response string) string {
	// Trim leading and trailing whitespace
	response = strings.TrimSpace(response)

	// Remove leading and trailing backticks (```)
	response = strings.TrimPrefix(response, "```go")
	response = strings.TrimSuffix(response, "```")

	// Trim any remaining whitespace
	response = strings.TrimSpace(response)

	return response
}

func main() {
	var cli CLI
	kongContext := kong.Parse(&cli)
	if err := kongContext.Run(&cli); err != nil {
		kongContext.Fatalf(err.Error())
	}
}
