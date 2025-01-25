# DeepRefactor

DeepRefactor is a CLI tool designed to automatically fix Go linting errors using the power of AI. It integrates with a local Ollama server and the `deepseek-coder-v2` model to analyze and refactor your Go code, ensuring it adheres to best practices and resolves linting issues.

---

## Features

- **Automated Linting Fixes**: Automatically fixes Go linting errors by leveraging AI.
- **Integration with Ollama**: Connects to a local Ollama server for AI-powered code refactoring.
- **Customizable Prompts**: Provides detailed prompts to the AI model to ensure high-quality fixes.
- **Loop Until Clean**: Repeats the linting and fixing process until no more errors are found.
- **Comment Annotations**: Adds `[DeepRefactor]` comments to the code to highlight fixes and summarize issues.

---

## Prerequisites

Before using DeepRefactor, ensure you have the following installed:

1. **Go**: Install Go from [https://golang.org/dl/](https://golang.org/dl/).
2. **Ollama**: Install Ollama from [https://ollama.ai/](https://ollama.ai/).
3. **DeepSeek Coder v2 Model**: Pull the `deepseek-coder-v2` model using Ollama.

---

## Installation

1. Clone the DeepRefactor repository:
   ```bash
   git clone https://github.com/your-username/DeepRefactor.git
   cd DeepRefactor
   ```

2. Build the CLI tool:
   ```bash
   go build -o deeprefactor
   ```

3. Move the binary to a directory in your `PATH` (optional):
   ```bash
   sudo mv deeprefactor /usr/local/bin/
   ```

---

## Setting Up Ollama with DeepSeek Coder v2

1. **Install Ollama**:
   - Follow the installation instructions for your operating system from the [Ollama website](https://ollama.ai/).

2. **Pull the DeepSeek Coder v2 Model**:
   - Run the following command to download the `deepseek-coder-v2` model:
     ```bash
     ollama pull deepseek-coder-v2
     ```

3. **Start the Ollama Server**:
   - Ensure the Ollama server is running locally:
     ```bash
     ollama serve
     ```

---

## Usage

### Basic Command

Run DeepRefactor with your preferred Go linting tool (e.g., `golangci-lint`):

```bash
deeprefactor golangci-lint run <filename>.go
```

### Example

1. Navigate to your Go project directory:
   ```bash
   cd /path/to/your/project
   ```

2. Run DeepRefactor:
   ```bash
   deeprefactor golangci-lint run
   ```

3. DeepRefactor will:
   - Run the linting tool.
   - Detect linting errors.
   - Fix the errors using the Ollama server.
   - Repeat the process until no more errors are found.

### Output Example

```
Linting errors:
testdata\mistakes.go:9:2: S1021: should merge variable declaration with assignment on next line (gosimple)
        var x int
        ^
Filepath found at testdata\mistakes.go
Fixed version of the file:
package main

import (
    "fmt"
)

func main() {
    var x int = 0 // [DeepRefactor] Merged declaration and assignment
    fmt.Println(x)
}
File testdata\mistakes.go has been updated with the fixed version.

No linting errors found.
```

---

## Configuration

### CLI Arguments

- **LintCommand**: The Go linting command to run (e.g., `golangci-lint`).
- **Args**: Additional arguments to pass to the linting command (optional).

Example:
```bash
deeprefactor golangci-lint run --enable-all
```

---

## How It Works

1. **Run Linting Tool**: DeepRefactor executes the specified linting tool and captures its output.
2. **Extract File Path**: It extracts the file path from the linting errors.
3. **Call Ollama Server**: The tool sends the file content and linting errors to the Ollama server for fixing.
4. **Apply Fixes**: The fixed code is written back to the file.
5. **Repeat**: The process repeats until no more linting errors are found.

---
## Lama info
Lama api can be called like this
```bash
curl http://localhost:11434/api/generate -d '{ "model": "deepseek-coder-v2", "prompt": "How are you today?", "stream": "false"}'
```

---

## Acknowledgments

- **Ollama**: For providing the infrastructure to run AI models locally.
- **DeepSeek**: For the powerful code refactoring capabilities.


