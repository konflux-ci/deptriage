/*
Copyright 2026 Red Hat, Inc.

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

package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/konflux-ci/dep-impact-analysis-action/internal/analyze/provider"
	ghclient "github.com/konflux-ci/dep-impact-analysis-action/internal/github"
	"github.com/konflux-ci/dep-impact-analysis-action/internal/security"
	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"

	gh "github.com/google/go-github/v85/github"
	"golang.org/x/oauth2"
)

// Options configures the analyze pipeline.
type Options struct {
	PRNumber       int
	Repo           string
	Token          string
	Provider       string
	APIKey         string
	Model          string
	AutoApprove    bool
	ClassifyOutput string
	WorkDir        string
}

// Run executes the full analysis pipeline. Always exits 0 — errors are reported as comments.
func Run(ctx context.Context, opts Options) error {
	client := ghclient.NewClient(ctx, opts.Token, opts.Repo)

	// Read classify output
	classifyResult, err := readClassifyOutput(opts.ClassifyOutput)
	if err != nil {
		slog.Error("failed to read classify output", "error", err)
		return client.PostFallbackComment(ctx, opts.PRNumber, fmt.Sprintf("Failed to read classify output: %v", err))
	}

	// Check API key
	if opts.APIKey == "" {
		return client.PostFallbackComment(ctx, opts.PRNumber,
			fmt.Sprintf("The `%s` API key is not configured. Configure it to enable AI-assisted dependency impact analysis", opts.Provider))
	}

	// Create LLM provider
	llm, err := provider.New(opts.Provider, opts.APIKey, opts.Model)
	if err != nil {
		return client.PostFallbackComment(ctx, opts.PRNumber, fmt.Sprintf("Invalid LLM provider: %v", err))
	}

	// Create raw GitHub client for advisory lookups
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Token})
	rawGHClient := gh.NewClient(oauth2.NewClient(ctx, ts))

	// Gather context
	ctxJSON := GatherContext(ctx, classifyResult, client, rawGHClient, opts.WorkDir)
	contextStr, err := ContextToJSON(ctxJSON)
	if err != nil {
		return client.PostFallbackComment(ctx, opts.PRNumber, fmt.Sprintf("Failed to assemble context: %v", err))
	}

	// Render prompt
	prompt := RenderPrompt(classifyResult.BumpType, classifyResult.PRTitle, contextStr)

	// Call LLM
	slog.Info("calling LLM API", "provider", opts.Provider)
	response, err := llm.Analyze(ctx, prompt)
	if err != nil {
		slog.Error("LLM API call failed", "provider", opts.Provider, "error", err)
		return client.PostFallbackComment(ctx, opts.PRNumber, fmt.Sprintf("The AI API call did not succeed (%v)", err))
	}

	// Redact secrets
	response = security.RedactSecrets(response)

	// Extract risk level
	riskLevel := ExtractRiskLevel(response)
	slog.Info("analysis complete", "riskLevel", riskLevel)

	// Post analysis comment
	if err := client.UpsertAnalysisComment(ctx, opts.PRNumber, response); err != nil {
		slog.Warn("failed to post comment", "error", err)
	}

	// Apply risk label
	applyRiskLabel(ctx, client, opts.PRNumber, riskLevel)

	// Submit review if applicable
	submitReview(ctx, client, opts, riskLevel, response)

	return nil
}

func readClassifyOutput(path string) (*types.ClassifyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var result types.ClassifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing classify output: %w", err)
	}
	return &result, nil
}

func applyRiskLabel(ctx context.Context, client *ghclient.Client, prNumber int, risk types.RiskLevel) {
	if risk == types.RiskUnknown {
		return
	}

	// Remove existing risk labels
	for _, level := range []string{"low", "medium", "high"} {
		_ = client.RemoveLabel(ctx, prNumber, "risk/"+level)
	}

	colors := map[types.RiskLevel]string{
		types.RiskLow:    "0e8a16",
		types.RiskMedium: "fbca04",
		types.RiskHigh:   "e11d48",
	}

	labelName := fmt.Sprintf("risk/%s", risk)
	color := colors[risk]
	desc := fmt.Sprintf("AI-assessed %s risk dependency update", risk)
	if err := client.EnsureLabel(ctx, prNumber, labelName, color, desc); err != nil {
		slog.Warn("failed to apply risk label", "label", labelName, "error", err)
	}
}

func submitReview(ctx context.Context, client *ghclient.Client, opts Options, risk types.RiskLevel, body string) {
	switch risk {
	case types.RiskHigh:
		if err := client.SubmitReview(ctx, opts.PRNumber, "REQUEST_CHANGES", body); err != nil {
			slog.Warn("failed to submit review", "event", "REQUEST_CHANGES", "error", err)
		}
	case types.RiskLow:
		if opts.AutoApprove {
			if err := client.SubmitReview(ctx, opts.PRNumber, "APPROVE", body); err != nil {
				slog.Warn("failed to submit review", "event", "APPROVE", "error", err)
			}
		}
	case types.RiskMedium:
		if err := client.SubmitReview(ctx, opts.PRNumber, "COMMENT", body); err != nil {
			slog.Warn("failed to submit review", "event", "COMMENT", "error", err)
		}
	}
}
