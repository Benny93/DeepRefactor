package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"

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
// It supports Windows-style paths (e.g., `C:\path\to\file.go`) and optionally
// includes line and column numbers (e.g., `C:\path\to\file.go:21:6`).
func ExtractFilePath(input string) string {
	// Regular expression to match file paths
	re := regexp.MustCompile(`[A-Za-z]:(\\[^:]+)+\.go`)
	return re.FindString(input)
}

func (cli *CLI) Run() error {

	// Construct the full command to execute
	cmd := exec.Command(cli.LintCommand, cli.Args...)

	// Capture standard output and standard error
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	if err := cmd.Run(); err != nil {
		fmt.Printf("Linting errors:\n%s\n", stderr.String())
		// extract file path from error message "[linters_context] typechecking error: D:\\dev\\DeepRefactor\\testdata\\mistakes.go:21:6: y declared and not used"
		filePath := ExtractFilePath(stderr.String())
		if filePath == "" {
			fmt.Println("No file path found.")
		}
		fmt.Println("Filepath found at " + filePath)

		// Read the content of the affected file
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %v", filePath, err)
		}

		// Call Ollama server to fix the errors
		if err := callOllamaServer(filePath, string(fileContent), stderr.String()); err != nil {
			return fmt.Errorf("failed to call Ollama server: %v", err)
		}
		return fmt.Errorf("failed to run lint command: %v", err)
	}

	// Output the standard output and errors if there are any issues
	if len(stderr.String()) > 0 {
		fmt.Printf("Linting errors:\n%s\n", stderr.String())
	} else {
		fmt.Println("No linting errors found.")
	}

	return nil
}

func callOllamaServer(fileName, fileContent, errorOutput string) error {
	// Prepare the prompt for Ollama
	prompt := fmt.Sprintf(`I have the following Go linting errors in the file %s:
%s

Here is the content of the file:
%s

Please provide a complete fixed version of the file. Do not include any explanations or additional text. Only return the complete code.`, fileName, errorOutput, fileContent)

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

	// Output the suggested fix
	fmt.Printf("Fixed version of the file:\n%s\n", ollamaResponse.Response)

	return nil
}

func main() {
	var cli CLI
	kongContext := kong.Parse(&cli)
	if err := kongContext.Run(&cli); err != nil {
		kongContext.Fatalf(err.Error())
	}
}
