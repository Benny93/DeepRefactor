package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/alecthomas/kong"
)

// Define the CLI command structure
type CLI struct {
	LintCommand string   `arg:"" help:"The Go lint command to run."`
	Args        []string `arg:"" optional:""`
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
		fmt.Printf("failed to run lint command: %v\n%s", err, stderr.String())
	}

	// Output the standard output and errors if there are any issues
	if len(stderr.String()) > 0 {
		fmt.Printf("Linting errors:\n%s\n", stderr.String())
		// Call Ollama server to fix the errors
		if err := callOllamaServer(stderr.String()); err != nil {
			return fmt.Errorf("failed to call Ollama server: %v", err)
		}
	} else {
		fmt.Println("No linting errors found.")
	}
	return nil
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// Define the response structure for the Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
}

func callOllamaServer(errorOutput string) error {
	// Prepare the prompt for Ollama
	prompt := fmt.Sprintf("I have the following Go linting errors:\n%s\nCan you suggest a fix?", errorOutput)

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
	fmt.Printf("Suggested fix from Ollama:\n%s\n", ollamaResponse.Response)

	return nil
}

func main() {
	var cli CLI
	kongContext := kong.Parse(&cli)
	if err := kongContext.Run(&cli); err != nil {
		kongContext.Fatalf(err.Error())
	}
}
