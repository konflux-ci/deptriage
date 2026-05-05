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

package classify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	ghclient "github.com/konflux-ci/deptriage/internal/github"
	"github.com/konflux-ci/deptriage/internal/types"
)

// Options configures the classify pipeline.
type Options struct {
	PRNumber    int
	Repo        string
	Token       string
	OutputFile  string
	AutoApprove bool
	DryRun      bool
}

// Run executes the full classification pipeline.
func Run(ctx context.Context, opts Options) (*types.ClassifyResult, error) {
	client := ghclient.NewClient(ctx, opts.Token, opts.Repo)

	// Fetch PR data
	pr, err := client.FetchPR(ctx, opts.PRNumber)
	if err != nil {
		return nil, fmt.Errorf("fetching PR: %w", err)
	}

	// Detect semver bump type
	bumpType := DetectBumpType(pr.Title, pr.Body)
	slog.Info("detected bump type", "bumpType", bumpType)

	// Extract packages: prefer Dependency Review API, fall back to PR body regex
	depEntries, err := client.FetchDependencyReview(ctx, pr.BaseRef, pr.HeadRef)
	var packages []types.PackageInfo
	if err != nil {
		slog.Warn("dependency review API unavailable, falling back to PR body parsing", "error", err)
	} else {
		packages = ghclient.DepReviewToPackages(depEntries)
	}
	if len(packages) == 0 {
		packages = ExtractPackagesWithChangelog(pr.Body, pr.Title)
	}
	slog.Info("extracted packages", "count", len(packages))

	// Detect risk hints and apply risk-hint labels
	riskHintLabels := DetectRiskHintLabels(pr.Title, pr.Body)
	riskHints := DetectRiskHints(pr.Title, pr.Body)
	if riskHints != "" {
		slog.Warn("risk hints detected", "hint", strings.SplitN(riskHints, ":", 2)[0])
	}
	for _, hint := range riskHintLabels {
		if opts.DryRun {
			slog.Info("[DRY-RUN] would apply risk-hint label", types.LogKeyLabel, hint.Label, types.LogKeyPR, opts.PRNumber)
			continue
		}
		if err := client.EnsureLabel(ctx, opts.PRNumber, hint.Label, hint.Color, hint.LabelDesc); err != nil {
			slog.Warn("failed to apply risk-hint label", types.LogKeyLabel, hint.Label, "error", err)
		}
	}

	// Determine the dominant ecosystem across all packages
	ecosystem := dominantEcosystem(packages)

	// Apply semver label (ecosystem-aware for digest bumps)
	var appliedLabel string
	if bumpType != types.BumpUnknown {
		labelName := bumpType.Label()
		labelColor := bumpType.Color()
		if bumpType == types.BumpDigest {
			labelName = types.DigestLabel(ecosystem)
			labelColor = types.DigestLabelColor(ecosystem)
		}

		hasLabel := slices.ContainsFunc(pr.Labels, func(l string) bool {
			return strings.HasPrefix(l, "semver/")
		})
		if !hasLabel {
			if opts.DryRun {
				slog.Info("[DRY-RUN] would apply semver label", types.LogKeyLabel, labelName, types.LogKeyPR, opts.PRNumber)
				appliedLabel = labelName
			} else {
				desc := fmt.Sprintf("Semver %s version bump", bumpType)
				if bumpType == types.BumpDigest && ecosystem == "gomod" {
					desc = "Go module digest/pseudo-version bump (treated as minor)"
				}
				if err := client.EnsureLabel(ctx, opts.PRNumber, labelName, labelColor, desc); err != nil {
					slog.Warn("failed to apply label", types.LogKeyLabel, labelName, "error", err)
				} else {
					appliedLabel = labelName
				}
			}
		}
	}

	result := &types.ClassifyResult{
		BumpType:  bumpType,
		Packages:  packages,
		RiskHints: riskHints,
		PRTitle:   pr.Title,
		PRBody:    pr.Body,
		Repo:      opts.Repo,
		PRNumber:  opts.PRNumber,
		Label:     appliedLabel,
	}

	// Auto-approve eligible patches by applying approved/lgtm labels.
	// Gomod digest bumps are excluded — pseudo-versions have no semver guarantees.
	isAutoApproveEligible := bumpType == types.BumpPatch ||
		(bumpType == types.BumpDigest && ecosystem != "gomod")
	if opts.AutoApprove && isAutoApproveEligible && riskHints == "" {
		slog.Info("applying auto-approve labels for eligible PR")
		for _, label := range []string{types.LabelApproved, types.LabelLGTM} {
			if opts.DryRun {
				slog.Info("[DRY-RUN] would apply auto-approve label", types.LogKeyLabel, label, types.LogKeyPR, opts.PRNumber)
				continue
			}
			if err := client.EnsureLabel(ctx, opts.PRNumber, label, types.ColorGreen, "Auto-approved dependency update"); err != nil {
				slog.Warn("failed to apply auto-approve label", types.LogKeyLabel, label, "error", err)
			}
		}
	}

	// Write output
	if err := writeResult(result, opts.OutputFile); err != nil {
		return nil, fmt.Errorf("writing classify result: %w", err)
	}
	slog.Info("classify result written", "path", opts.OutputFile)

	return result, nil
}

// dominantEcosystem returns "gomod" if any package has a gomod ecosystem, empty otherwise.
func dominantEcosystem(pkgs []types.PackageInfo) string {
	for _, p := range pkgs {
		if p.Ecosystem == "gomod" {
			return "gomod"
		}
	}
	return ""
}

func writeResult(result *types.ClassifyResult, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
