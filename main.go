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
	prompt := fmt.Sprintf(`I have the following Go linting error in the file %s:
%s

Here is the relevant portion of the file:
%s

Please fix the issue by either removing the unused variable or adding code that uses it. Do not include any explanations or additional text. Only return the fixed code. Add a comment with the prefix [DeepRefactor] to indicate the fix.`, fileName, errorOutput, fileContent)
	// Create the request body
	requestBody := OllamaRequest{
		Model:  "deepseek-coder-v2", // Replace with your desired model
		Prompt: prompt,
		Stream: true, // Enable streaming
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

	// Open the file for writing (truncate it to start fresh)
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", fileName, err)
	}
	defer file.Close()

	// Create a decoder to read the streaming response
	decoder := json.NewDecoder(resp.Body)

	// Process the streaming response
	for {
		var ollamaResponse OllamaResponse
		if err := decoder.Decode(&ollamaResponse); err != nil {
			if err.Error() == "EOF" {
				break // End of stream
			}
			return fmt.Errorf("failed to decode streaming response: %v", err)
		}

		// Remove escaping backticks (```) from the response
		fixedContent := removeEscapingBackticks(ollamaResponse.Response)

		// Write the fixed content to the file
		if _, err := file.WriteString(fixedContent); err != nil {
			return fmt.Errorf("failed to write to file %s: %v", fileName, err)
		}

		// Print the fixed content to the console (optional)
		fmt.Print(fixedContent)
	}

	fmt.Printf("\nFile %s has been updated with the fixed version.\n", fileName)
	return nil
}

// removeEscapingBackticks removes the escaping backticks (```) from the response
func removeEscapingBackticks(response string) string {

	// Remove leading and trailing backticks (```)
	response = strings.TrimPrefix(response, "```go")
	response = strings.TrimPrefix(response, "go")
	response = strings.TrimSuffix(response, "```")
	return response
}

func main() {
	var cli CLI
	kongContext := kong.Parse(&cli)
	if err := kongContext.Run(&cli); err != nil {
		kongContext.Fatalf(err.Error())
	}
}
