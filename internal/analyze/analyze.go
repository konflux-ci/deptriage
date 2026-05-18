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

	"github.com/konflux-ci/deptriage/internal/analyze/provider"
	ghclient "github.com/konflux-ci/deptriage/internal/github"
	"github.com/konflux-ci/deptriage/internal/security"
	"github.com/konflux-ci/deptriage/internal/types"

	gh "github.com/google/go-github/v87/github"
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
	AutoMerge      bool
	ClassifyOutput string
	DryRun         bool
	WorkDir        string
}

// Run executes the full analysis pipeline. Always exits 0 — errors are reported as comments.
// Returns the assessed risk level (or RiskUnknown on error) and any error.
func Run(ctx context.Context, opts Options) (types.RiskLevel, error) {
	client := ghclient.NewClient(ctx, opts.Token, opts.Repo)

	// Read classify output
	classifyResult, err := readClassifyOutput(opts.ClassifyOutput)
	if err != nil {
		slog.Error("failed to read classify output", "error", err)
		return postFallbackOrDryRun(ctx, client, opts.PRNumber, fmt.Sprintf("Failed to read classify output: %v", err), opts.DryRun)
	}

	// Check API key
	if opts.APIKey == "" {
		return postFallbackOrDryRun(ctx, client, opts.PRNumber,
			fmt.Sprintf("The `%s` API key is not configured. Configure it to enable AI-assisted dependency impact analysis", opts.Provider), opts.DryRun)
	}

	// Create LLM provider
	llm, err := provider.New(opts.Provider, opts.APIKey, opts.Model)
	if err != nil {
		return postFallbackOrDryRun(ctx, client, opts.PRNumber, fmt.Sprintf("Invalid LLM provider: %v", err), opts.DryRun)
	}

	// Create raw GitHub client for advisory lookups
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Token})
	rawGHClient := gh.NewClient(oauth2.NewClient(ctx, ts))

	// Gather context
	ctxJSON := GatherContext(ctx, classifyResult, client, rawGHClient, opts.WorkDir)
	contextStr, err := ContextToJSON(ctxJSON)
	if err != nil {
		return postFallbackOrDryRun(ctx, client, opts.PRNumber, fmt.Sprintf("Failed to assemble context: %v", err), opts.DryRun)
	}

	// Render prompt
	prompt := RenderPrompt(classifyResult.BumpType, classifyResult.PRTitle, contextStr)

	// Call LLM
	slog.Info("calling LLM API", "provider", opts.Provider)
	response, err := llm.Analyze(ctx, prompt)
	if err != nil {
		slog.Error("LLM API call failed", "provider", opts.Provider, "error", err)
		return postFallbackOrDryRun(ctx, client, opts.PRNumber, fmt.Sprintf("The AI API call did not succeed (%v)", err), opts.DryRun)
	}

	// Redact secrets
	response = security.RedactSecrets(response)

	// Extract risk level
	riskLevel := ExtractRiskLevel(response)
	slog.Info("analysis complete", "riskLevel", riskLevel)

	// Post analysis comment
	if opts.DryRun {
		slog.Info("[DRY-RUN] would upsert analysis comment", types.LogKeyPR, opts.PRNumber, "bodyLen", len(response))
	} else if err := client.UpsertAnalysisComment(ctx, opts.PRNumber, response); err != nil {
		slog.Warn("failed to post comment", "error", err)
	}

	// Apply risk label
	applyRiskLabel(ctx, client, opts.PRNumber, riskLevel, opts.DryRun)

	// Submit review if applicable
	submitReview(ctx, client, opts, riskLevel, response)

	// Auto-merge if eligible
	if shouldAttemptMerge(opts.AutoMerge, opts.AutoApprove, riskLevel) {
		tryMerge(ctx, client, opts)
	}

	return riskLevel, nil
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

func applyRiskLabel(ctx context.Context, client *ghclient.Client, prNumber int, risk types.RiskLevel, dryRun bool) {
	if risk == types.RiskUnknown {
		return
	}

	// Remove existing risk labels
	for _, level := range []types.RiskLevel{types.RiskLow, types.RiskMedium, types.RiskHigh} {
		if dryRun {
			slog.Info("[DRY-RUN] would remove label", types.LogKeyLabel, level.Label(), types.LogKeyPR, prNumber)
		} else {
			_ = client.RemoveLabel(ctx, prNumber, level.Label())
		}
	}

	if dryRun {
		slog.Info("[DRY-RUN] would apply risk label", types.LogKeyLabel, risk.Label(), types.LogKeyPR, prNumber)
		return
	}

	desc := fmt.Sprintf("AI-assessed %s risk dependency update", risk)
	if err := client.EnsureLabel(ctx, prNumber, risk.Label(), risk.Color(), desc); err != nil {
		slog.Warn("failed to apply risk label", types.LogKeyLabel, risk.Label(), "error", err)
	}
}

func shouldAttemptMerge(autoMerge, autoApprove bool, risk types.RiskLevel) bool {
	return autoMerge && autoApprove && risk != types.RiskHigh
}

const deptriageCheckName = "Triage dependency PR"

func tryMerge(ctx context.Context, client *ghclient.Client, opts Options) {
	hasLabels, err := client.HasLabels(ctx, opts.PRNumber, []string{types.LabelApproved, types.LabelLGTM})
	if err != nil {
		slog.Warn("failed to check PR labels for auto-merge", "error", err)
		return
	}
	if !hasLabels {
		slog.Info("skipping auto-merge: approved/lgtm labels not present")
		return
	}

	checkStatus, err := client.ChecksAllPassed(ctx, opts.PRNumber, deptriageCheckName)
	if err != nil {
		slog.Warn("failed to check CI status for auto-merge", "error", err)
		return
	}
	if checkStatus != ghclient.ChecksPassed {
		slog.Info("skipping auto-merge: not all CI checks have passed")
		return
	}

	if opts.DryRun {
		slog.Info("[DRY-RUN] would merge PR", types.LogKeyPR, opts.PRNumber, "method", "squash")
		return
	}

	slog.Info("all merge conditions met, merging PR", types.LogKeyPR, opts.PRNumber)
	if err := client.MergePR(ctx, opts.PRNumber, "squash"); err != nil {
		slog.Warn("auto-merge failed", "error", err)
		return
	}
	slog.Info("PR merged successfully", types.LogKeyPR, opts.PRNumber)
}

func submitReview(ctx context.Context, client *ghclient.Client, opts Options, risk types.RiskLevel, body string) {
	if risk == types.RiskLow && opts.AutoApprove {
		if opts.DryRun {
			slog.Info("[DRY-RUN] would submit review", types.LogKeyEvent, types.ReviewApprove, types.LogKeyPR, opts.PRNumber)
			return
		}
		if err := client.SubmitReview(ctx, opts.PRNumber, types.ReviewApprove, body); err != nil {
			slog.Warn("failed to submit review", types.LogKeyEvent, types.ReviewApprove, "error", err)
		}
		return
	}
	if opts.DryRun {
		slog.Info("[DRY-RUN] would submit review", types.LogKeyEvent, types.ReviewComment, types.LogKeyPR, opts.PRNumber)
		return
	}
	if err := client.SubmitReview(ctx, opts.PRNumber, types.ReviewComment, body); err != nil {
		slog.Warn("failed to submit review", types.LogKeyEvent, types.ReviewComment, "error", err)
	}
}

func postFallbackOrDryRun(ctx context.Context, client *ghclient.Client, prNumber int, msg string, dryRun bool) (types.RiskLevel, error) {
	if dryRun {
		slog.Info("[DRY-RUN] would post fallback comment", types.LogKeyPR, prNumber, "reason", msg)
		return types.RiskUnknown, nil
	}
	return types.RiskUnknown, client.PostFallbackComment(ctx, prNumber, msg)
}
