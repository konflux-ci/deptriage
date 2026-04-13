/*
Copyright 2025 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	claudeDefaultModel = "claude-sonnet-4-20250514"
	claudeTimeout      = 120 * time.Second
	claudeAPIVersion   = "2023-06-01"
)

// Claude implements LLMProvider for Anthropic's Claude API.
type Claude struct {
	apiKey string
	model  string
	client *http.Client
}

// NewClaude creates a Claude provider.
func NewClaude(apiKey, model string) *Claude {
	if model == "" {
		model = claudeDefaultModel
	}
	return &Claude{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: claudeTimeout},
	}
}

type claudeRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	Messages  []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (c *Claude) Analyze(ctx context.Context, prompt string) (string, error) {
	reqBody := claudeRequest{
		Model:     c.model,
		MaxTokens: 4096,
		Messages: []claudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling claude request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating claude request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", claudeAPIVersion)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude API call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading claude response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return "", fmt.Errorf("parsing claude response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("claude returned empty response")
	}

	// Concatenate all text blocks
	var result string
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			result += block.Text
		}
	}
	return result, nil
}
