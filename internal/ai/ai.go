// internal/ai/ai.go
package ai

import (
	"bytes"
	"context"
	"deeprefactor/internal/types"
	"deeprefactor/pkg/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type AIClient struct {
	OllamaURL string
	Model     string
}

func NewClient(ollamaURL, model string) *AIClient {
	return &AIClient{
		OllamaURL: ollamaURL,
		Model:     model,
	}
}

func (c *AIClient) FixFile(ctx context.Context, path string, lintOutput string, updates chan<- types.FileUpdate) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	fixed, err := c.GetFixedCode(ctx, path, string(content), lintOutput)
	if err != nil {
		return fmt.Errorf("AI fix: %w", err)
	}

	if err := utils.SafeWriteFile(path, fixed); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	updates <- types.FileUpdate{Path: path, Log: "Applied AI fix"}
	return nil
}

func (c *AIClient) GetFixedCode(ctx context.Context, path, content, errors string) (string, error) {
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
		Model:  c.Model,
		Prompt: prompt,
		Stream: false,
	}

	resp, err := c.SendOllamaRequest(ctx, reqBody)
	if err != nil {
		return "", err
	}

	return utils.ExtractCodeBlock(resp), nil
}

func (c *AIClient) SendOllamaRequest(ctx context.Context, reqBody interface{}) (string, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.OllamaURL+"/api/generate", bytes.NewReader(body))
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
