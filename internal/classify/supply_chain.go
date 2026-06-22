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
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	ghclient "github.com/konflux-ci/deptriage/internal/github"
	"github.com/konflux-ci/deptriage/internal/types"
)

// SupplyChainFinding represents a detected supply-chain concern.
type SupplyChainFinding struct {
	Key       string
	Label     string
	Color     string
	LabelDesc string
	Message   string
	Details   []string // e.g. mismatched commit SHAs or suspicious file paths
}

var defaultTrustedBots = []string{
	"renovate[bot]",
	"red-hat-konflux[bot]",
	"dependabot[bot]",
}

var defaultSuspiciousPaths = []string{
	".claude/",
	".vscode/",
	".github/workflows/",
	".github/actions/",
}

var suspiciousScriptExts = []string{
	".sh", ".mjs", ".js", ".py", ".rb", ".pl",
}

var gitHubActionPrefixes = []string{
	".github/workflows/",
	".github/actions/",
}

var vendoredPrefixes = []string{
	"vendor/",
	"third_party/",
	"node_modules/",
}

var defaultExpectedPatterns = []string{
	"go.mod",
	"go.sum",
	"Dockerfile",
	"Containerfile",
	"*.Dockerfile",
	"vendor/",
	".tekton/",
	"renovate.json",
	".renovaterc",
	".renovaterc.json",
	"package.json",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"requirements.txt",
	"poetry.lock",
	"Pipfile.lock",
	"Gemfile",
	"Gemfile.lock",
	"Cargo.toml",
	"Cargo.lock",
	".gitmodules",
}

// IsTrustedBot reports whether the given login is in the trusted bot list.
func IsTrustedBot(author string, extraBots []string) bool {
	return slices.Contains(defaultTrustedBots, author) || slices.Contains(extraBots, author)
}

// ValidateAuthor checks that every commit on the PR was authored by the same
// identity that opened it. Returns nil if the PR author is not a trusted bot
// (supply-chain checks only apply to bot PRs) or if all commits match.
func ValidateAuthor(prAuthor string, commits []ghclient.CommitInfo, extraBots []string) *SupplyChainFinding {
	if !IsTrustedBot(prAuthor, extraBots) {
		return nil
	}

	var mismatched []string
	for _, c := range commits {
		if c.Author != prAuthor {
			mismatched = append(mismatched, c.SHA+" (by "+c.Author+")")
		}
	}
	if len(mismatched) == 0 {
		return nil
	}

	return &SupplyChainFinding{
		Key:       "SUPPLY_CHAIN_AUTHOR_MISMATCH",
		Label:     types.LabelSupplyChainAuthorMismatch,
		Color:     types.ColorRed,
		LabelDesc: "PR commit author does not match the bot that opened the PR",
		Message:   "One or more commits on this PR were authored by an identity other than " + prAuthor,
		Details:   mismatched,
	}
}

// DetectSuspiciousFiles checks the changed file list against the suspicious
// path blocklist and flags executable scripts outside vendored directories.
func DetectSuspiciousFiles(files []string, extraPaths []string) *SupplyChainFinding {
	blocklist := append(append([]string{}, defaultSuspiciousPaths...), extraPaths...)

	var flagged []string
	for _, f := range files {
		if matchesAnyPrefix(f, blocklist) {
			flagged = append(flagged, f)
			continue
		}
		if isSuspiciousScript(f) {
			flagged = append(flagged, f)
		}
	}
	if len(flagged) == 0 {
		return nil
	}

	return &SupplyChainFinding{
		Key:       "SUPPLY_CHAIN_SUSPICIOUS_FILES",
		Label:     types.LabelSupplyChainSuspiciousFiles,
		Color:     types.ColorRed,
		LabelDesc: "PR contains changes to known attack vector paths",
		Message:   "This PR modifies files associated with known supply-chain attack vectors",
		Details:   flagged,
	}
}

// ValidateDiffScope verifies that all changed files match expected patterns for
// a dependency update. Returns nil if the PR author is not a trusted bot.
func ValidateDiffScope(prAuthor string, files []string, extraBots []string, extraPatterns []string) *SupplyChainFinding {
	if !IsTrustedBot(prAuthor, extraBots) {
		return nil
	}

	allowed := append(append([]string{}, defaultExpectedPatterns...), extraPatterns...)

	var unexpected []string
	for _, f := range files {
		if !matchesExpectedPattern(f, allowed) {
			unexpected = append(unexpected, f)
		}
	}
	if len(unexpected) == 0 {
		return nil
	}

	return &SupplyChainFinding{
		Key:       "SUPPLY_CHAIN_UNEXPECTED_SCOPE",
		Label:     types.LabelSupplyChainUnexpectedScope,
		Color:     types.ColorRed,
		LabelDesc: "PR changes files outside expected dependency update scope",
		Message:   "This dependency PR modifies files outside the expected scope for a dependency update",
		Details:   unexpected,
	}
}

// IsGitHubActionsUpdate reports whether the PR is a pure GitHub Actions
// dependency update — all changed files are under .github/workflows/ or
// .github/actions/. Returns false for an empty file list.
func IsGitHubActionsUpdate(files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !matchesAnyPrefix(f, gitHubActionPrefixes) {
			return false
		}
	}
	return true
}

func filterGitHubActionPaths(files []string) []string {
	var out []string
	for _, f := range files {
		if !matchesAnyPrefix(f, gitHubActionPrefixes) {
			out = append(out, f)
		}
	}
	return out
}

func matchesAnyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func isSuspiciousScript(path string) bool {
	if matchesAnyPrefix(path, vendoredPrefixes) {
		return false
	}
	return slices.Contains(suspiciousScriptExts, strings.ToLower(filepath.Ext(path)))
}

func matchesExpectedPattern(path string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(path, p) {
				return true
			}
			continue
		}
		if path == p {
			return true
		}
		base := filepath.Base(path)
		if base == p {
			return true
		}
		if matched, err := filepath.Match(p, base); err != nil {
			slog.Warn("invalid expected-file glob pattern", "pattern", p, "error", err)
		} else if matched {
			return true
		}
		if strings.Contains(p, "/") {
			if matched, err := filepath.Match(p, path); err != nil {
				slog.Warn("invalid expected-file glob pattern", "pattern", p, "error", err)
			} else if matched {
				return true
			}
		}
	}
	return false
}
