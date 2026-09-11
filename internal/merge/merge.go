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
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/konflux-ci/deptriage/internal/classify"
	ghclient "github.com/konflux-ci/deptriage/internal/github"
	"github.com/konflux-ci/deptriage/internal/types"
)

const mergeCheckName = "Merge if all checks pass"

// Options configures the merge command.
type Options struct {
	PRNumber        int
	HeadSHA         string
	Repo            string
	Token           string
	DryRun          bool
	TrustedBots     []string
	SuspiciousPaths []string
	ExpectedFiles   []string
}

// Run attempts to merge eligible PRs. Always exits 0.
func Run(ctx context.Context, opts Options) error {
	client := ghclient.NewClient(ctx, opts.Token, opts.Repo)

	if opts.HeadSHA != "" {
		return runForSHA(ctx, client, opts)
	}
	if opts.PRNumber > 0 {
		tryMergePR(ctx, client, opts)
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
		prOpts := opts
		prOpts.PRNumber = pr
		tryMergePR(ctx, client, prOpts)
	}
	return nil
}

// isMergeEligible checks if the PR labels indicate merge eligibility.
// Returns true if approved + lgtm are present and neither risk/high nor any
// supply-chain/* label is present.
func isMergeEligible(labels []string) bool {
	labelSet := make(map[string]bool, len(labels))
	for _, l := range labels {
		labelSet[l] = true
		if strings.HasPrefix(l, types.SupplyChainLabelPrefix) {
			return false
		}
	}
	return labelSet[types.LabelApproved] && labelSet[types.LabelLGTM] && !labelSet[types.LabelRiskHigh]
}

// isDeferredApprovalEligible checks if the PR is eligible for deferred approval:
// a patch or minor bump with risk hints that wasn't auto-approved early, but can
// be approved now that CI has proven the build is safe. PRs with supply-chain
// labels are excluded — passing CI does not prove a tampered PR is safe.
func isDeferredApprovalEligible(labels []string) bool {
	labelSet := make(map[string]bool, len(labels))
	hasRiskHint := false
	for _, l := range labels {
		labelSet[l] = true
		if strings.HasPrefix(l, types.RiskHintLabelPrefix) {
			hasRiskHint = true
		}
		if strings.HasPrefix(l, types.SupplyChainLabelPrefix) {
			return false
		}
	}
	isPatchOrMinor := labelSet[types.LabelSemverPatch] || labelSet[types.LabelSemverMinor]
	return isPatchOrMinor && hasRiskHint && !labelSet[types.LabelRiskHigh]
}

func tryMergePR(ctx context.Context, client *ghclient.Client, opts Options) {
	prNumber := opts.PRNumber
	dryRun := opts.DryRun
	slog.Info("evaluating PR for auto-merge", types.LogKeyPR, prNumber)

	pr, err := client.FetchPR(ctx, prNumber)
	if err != nil {
		slog.Warn("failed to fetch PR", types.LogKeyPR, prNumber, "error", err)
		return
	}
	if opts.HeadSHA != "" && pr.HeadSHA != opts.HeadSHA {
		slog.Warn("skipping: PR head changed since check_suite event", types.LogKeyPR, prNumber, "eventSHA", opts.HeadSHA, "currentSHA", pr.HeadSHA)
		return
	}
	if !isTrustedMergeAuthor(pr.Author, opts.TrustedBots) {
		slog.Warn("skipping: PR author is not a trusted dependency bot", types.LogKeyPR, prNumber, "author", pr.Author)
		return
	}
	if !validateSupplyChain(ctx, client, pr, opts) {
		return
	}

	eligible := isMergeEligible(pr.Labels)

	if !eligible && isDeferredApprovalEligible(pr.Labels) {
		checkStatus, err := waitForChecks(ctx, client, prNumber, pr.HeadSHA, mergeCheckName)
		if err != nil {
			slog.Warn("failed to check CI status for deferred approval", types.LogKeyPR, prNumber, "error", err)
			return
		}
		if checkStatus == ghclient.ChecksPassed {
			slog.Info("CI checks passed, allowing deferred merge for patch with risk hints", types.LogKeyPR, prNumber)
			eligible = true
		}
	}

	if !eligible {
		slog.Info("skipping: labels do not meet merge criteria", types.LogKeyPR, prNumber, "labels", pr.Labels)
		return
	}

	checkStatus, err := waitForChecks(ctx, client, prNumber, pr.HeadSHA, mergeCheckName)
	if err != nil {
		slog.Warn("failed to check CI status", types.LogKeyPR, prNumber, "error", err)
		return
	}
	if checkStatus != ghclient.ChecksPassed {
		slog.Info("skipping: not all CI checks have passed", types.LogKeyPR, prNumber)
		return
	}
	currentPR, err := client.FetchPR(ctx, prNumber)
	if err != nil {
		slog.Warn("skipping: unable to recheck PR head before approval", types.LogKeyPR, prNumber, "error", err)
		return
	}
	if currentPR.HeadSHA != pr.HeadSHA {
		slog.Warn("skipping: PR head changed after checks completed", types.LogKeyPR, prNumber, "checkedSHA", pr.HeadSHA, "currentSHA", currentPR.HeadSHA)
		return
	}

	slog.Info("all merge conditions met, merging PR", types.LogKeyPR, prNumber)
	if dryRun {
		slog.Info("[DRY-RUN] would submit approval review", types.LogKeyPR, prNumber)
		slog.Info("[DRY-RUN] would merge PR", types.LogKeyPR, prNumber, "method", "squash")
		return
	}
	if err := client.SubmitReviewAtSHA(ctx, prNumber, types.ReviewApprove, "All merge conditions met — auto-approved by deptriage.", pr.HeadSHA); err != nil {
		slog.Warn("failed to submit approval review", types.LogKeyPR, prNumber, "error", err)
		return
	}
	if err := client.MergePRAtSHA(ctx, prNumber, "squash", pr.HeadSHA); err != nil {
		if errors.Is(err, ghclient.ErrMergeQueueRequired) {
			currentPR, fetchErr := client.FetchPR(ctx, prNumber)
			if fetchErr != nil || currentPR.HeadSHA != pr.HeadSHA {
				slog.Warn("skipping queue entry: PR head changed or could not be verified", types.LogKeyPR, prNumber, "error", fetchErr)
				return
			}
			slog.Info("merge queue detected, enqueuing PR", types.LogKeyPR, prNumber)
			if err := client.EnqueuePR(ctx, pr.NodeID); err != nil {
				slog.Warn("enqueue failed", types.LogKeyPR, prNumber, "error", err)
			} else {
				slog.Info("PR enqueued successfully", types.LogKeyPR, prNumber)
			}
			return
		}
		slog.Warn("auto-merge failed", types.LogKeyPR, prNumber, "error", err)
		return
	}
	slog.Info("PR merged successfully", types.LogKeyPR, prNumber)
}

func isTrustedMergeAuthor(author string, trustedBots []string) bool {
	return classify.IsTrustedBot(author, trustedBots)
}

// validateSupplyChain repeats the provenance and scope checks here because the
// labels checked below are mutable status indicators, not authorization.
func validateSupplyChain(ctx context.Context, client *ghclient.Client, pr *ghclient.PRData, opts Options) bool {
	commits, files, err := client.FetchPRSnapshot(ctx, pr)
	if err != nil {
		slog.Warn("skipping: unable to verify PR snapshot", types.LogKeyPR, pr.Number, "error", err)
		return false
	}
	if finding := classify.ValidateAuthor(pr.Author, commits, opts.TrustedBots); finding != nil {
		slog.Warn("skipping: PR commit author validation failed", types.LogKeyPR, pr.Number, "finding", finding.Key)
		return false
	}

	expectedFiles := append([]string{}, opts.ExpectedFiles...)
	headSubmodulePaths, err := client.FetchSubmodulePathsForRepo(ctx, pr.HeadOwner, pr.HeadRepo, pr.HeadSHA)
	if err != nil {
		slog.Warn("skipping: unable to verify head submodule paths", types.LogKeyPR, pr.Number, "error", err)
		return false
	}
	baseSubmodulePaths, err := client.FetchSubmodulePaths(ctx, pr.BaseRef)
	if err != nil {
		slog.Warn("skipping: unable to verify base submodule paths", types.LogKeyPR, pr.Number, "error", err)
		return false
	}
	submodulePaths := append(headSubmodulePaths, baseSubmodulePaths...)
	for _, submodulePath := range submodulePaths {
		if slices.Contains(files, submodulePath) {
			slog.Warn("skipping: PR updates a submodule and requires human review", types.LogKeyPR, pr.Number, "path", submodulePath)
			return false
		}
	}
	expectedFiles = append(expectedFiles, submodulePaths...)
	if findings := classify.ValidateFileSafety(pr.Author, files, opts.TrustedBots, opts.SuspiciousPaths, expectedFiles); len(findings) > 0 {
		slog.Warn("skipping: PR file safety validation failed", types.LogKeyPR, pr.Number, "finding", findings[0].Key)
		return false
	}
	return true
}

const (
	checkRetryAttempts = 10
	checkRetryInterval = 90 * time.Second
)

func waitForChecks(ctx context.Context, client *ghclient.Client, prNumber int, headSHA, excludeWorkflow string) (ghclient.CheckStatus, error) {
	for attempt := range checkRetryAttempts {
		status, err := client.ChecksAllPassedForSHA(ctx, headSHA, excludeWorkflow)
		if err != nil {
			return status, err
		}
		if status != ghclient.ChecksInProgress {
			return status, nil
		}
		if attempt < checkRetryAttempts-1 {
			slog.Info("checks still in progress, retrying", types.LogKeyPR, prNumber, "attempt", attempt+1, "retryIn", checkRetryInterval)
			select {
			case <-time.After(checkRetryInterval):
			case <-ctx.Done():
				return ghclient.ChecksInProgress, ctx.Err()
			}
		}
	}
	slog.Info("checks still in progress after retries, giving up", types.LogKeyPR, prNumber)
	return ghclient.ChecksInProgress, nil
}
