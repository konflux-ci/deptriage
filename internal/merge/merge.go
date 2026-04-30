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

package merge

import (
	"context"
	"log/slog"
	"strings"

	ghclient "github.com/konflux-ci/dep-impact-analysis-action/internal/github"
	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"
)

const mergeCheckName = "Merge if all checks pass"

// Options configures the merge command.
type Options struct {
	PRNumber int
	HeadSHA  string
	Repo     string
	Token    string
}

// Run attempts to merge eligible PRs. Always exits 0.
func Run(ctx context.Context, opts Options) error {
	client := ghclient.NewClient(ctx, opts.Token, opts.Repo)

	if opts.HeadSHA != "" {
		return runForSHA(ctx, client, opts)
	}
	if opts.PRNumber > 0 {
		tryMergePR(ctx, client, opts.PRNumber)
	}
	return nil
}

func runForSHA(ctx context.Context, client *ghclient.Client, opts Options) error {
	prs, err := client.FindOpenPRsForSHA(ctx, opts.HeadSHA)
	if err != nil {
		slog.Warn("failed to find PRs for SHA", "sha", opts.HeadSHA, "error", err)
		return nil
	}
	if len(prs) == 0 {
		slog.Info("no open PRs found for SHA", "sha", opts.HeadSHA)
		return nil
	}
	for _, pr := range prs {
		tryMergePR(ctx, client, pr)
	}
	return nil
}

// isMergeEligible checks if the PR labels indicate merge eligibility.
// Returns true if approved + lgtm are present and risk/high is not.
func isMergeEligible(labels []string) bool {
	labelSet := make(map[string]bool, len(labels))
	for _, l := range labels {
		labelSet[l] = true
	}
	return labelSet[types.LabelApproved] && labelSet[types.LabelLGTM] && !labelSet[types.LabelRiskHigh]
}

// isDeferredApprovalEligible checks if the PR is eligible for deferred approval:
// a patch bump with risk hints that wasn't auto-approved early, but can be
// approved now that CI has proven the build is safe.
func isDeferredApprovalEligible(labels []string) bool {
	labelSet := make(map[string]bool, len(labels))
	hasRiskHint := false
	for _, l := range labels {
		labelSet[l] = true
		if strings.HasPrefix(l, types.RiskHintLabelPrefix) {
			hasRiskHint = true
		}
	}
	return labelSet[types.LabelSemverPatch] && hasRiskHint && !labelSet[types.LabelRiskHigh]
}

func tryMergePR(ctx context.Context, client *ghclient.Client, prNumber int) {
	slog.Info("evaluating PR for auto-merge", "pr", prNumber)

	pr, err := client.FetchPR(ctx, prNumber)
	if err != nil {
		slog.Warn("failed to fetch PR", "pr", prNumber, "error", err)
		return
	}

	eligible := isMergeEligible(pr.Labels)

	if !eligible && isDeferredApprovalEligible(pr.Labels) {
		allPassed, err := client.ChecksAllPassed(ctx, prNumber, mergeCheckName)
		if err != nil {
			slog.Warn("failed to check CI status for deferred approval", "pr", prNumber, "error", err)
			return
		}
		if allPassed {
			slog.Info("CI checks passed, granting deferred approval for patch with risk hints", "pr", prNumber)
			for _, label := range []string{types.LabelApproved, types.LabelLGTM} {
				if err := client.EnsureLabel(ctx, prNumber, label, types.ColorGreen, "Deferred approval: CI checks passed"); err != nil {
					slog.Warn("failed to apply deferred approval label", "label", label, "error", err)
					return
				}
			}
			eligible = true
		}
	}

	if !eligible {
		slog.Info("skipping: labels do not meet merge criteria", "pr", prNumber, "labels", pr.Labels)
		return
	}

	allPassed, err := client.ChecksAllPassed(ctx, prNumber, mergeCheckName)
	if err != nil {
		slog.Warn("failed to check CI status", "pr", prNumber, "error", err)
		return
	}
	if !allPassed {
		slog.Info("skipping: not all CI checks have passed", "pr", prNumber)
		return
	}

	slog.Info("all merge conditions met, merging PR", "pr", prNumber)
	if err := client.MergePR(ctx, prNumber, "squash"); err != nil {
		slog.Warn("auto-merge failed", "pr", prNumber, "error", err)
		return
	}
	slog.Info("PR merged successfully", "pr", prNumber)
}
