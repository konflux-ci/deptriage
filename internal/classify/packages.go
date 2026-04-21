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
	"regexp"
	"strings"

	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"
)

// goModuleDomainRe matches Go module hosting domains to detect gomod ecosystem
// from package names when the Dependency Review API is unavailable.
var goModuleDomainRe = regexp.MustCompile(`^(github\.com|k8s\.io|sigs\.k8s\.io|golang\.org|google\.golang\.org|go\.uber\.org|gopkg\.in)/`)

// detectEcosystem infers ecosystem from a package name using domain heuristics.
func detectEcosystem(pkg string) string {
	if goModuleDomainRe.MatchString(pkg) {
		return "gomod"
	}
	return ""
}

var (
	// Match module paths in Renovate markdown table rows.
	// After stripping markdown links, matches: | module.path/pkg |
	modulePathRe = regexp.MustCompile(`\|\s*([a-zA-Z0-9._-]+\.[a-zA-Z]{2,}/[a-zA-Z0-9._/-]+)\s*\|`)

	// Strip markdown links: [text](url) -> text
	markdownLinkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

	// Fallback: extract module path from arbitrary text (e.g., PR title)
	modulePathLooseRe = regexp.MustCompile(`([a-zA-Z0-9._-]+\.[a-zA-Z]{2,}/[a-zA-Z0-9._/-]+)`)

	// Renovate boilerplate markers
	configSectionRe    = regexp.MustCompile(`(?m)^### Configuration$`)
	renovateDebugRe    = regexp.MustCompile(`(?m)^<!--renovate-debug:`)
)

// ExtractPackages extracts dependency package names from a Renovate PR body,
// with fallback to the PR title.
func ExtractPackages(body, title string) []string {
	pkgs := extractFromBody(body)
	if len(pkgs) == 0 {
		pkgs = extractFromTitle(title)
	}
	return pkgs
}

func extractFromBody(body string) []string {
	// Strip markdown links before matching
	stripped := markdownLinkRe.ReplaceAllString(body, "$1")
	matches := modulePathRe.FindAllStringSubmatch(stripped, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		pkg := m[1]
		if !seen[pkg] {
			seen[pkg] = true
			result = append(result, pkg)
		}
	}
	return result
}

func extractFromTitle(title string) []string {
	matches := modulePathLooseRe.FindAllStringSubmatch(title, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		pkg := m[1]
		if !seen[pkg] {
			seen[pkg] = true
			result = append(result, pkg)
		}
	}
	return result
}

// ExtractPackagesWithChangelog extracts packages and their changelogs from the PR body.
func ExtractPackagesWithChangelog(body, title string) []types.PackageInfo {
	pkgNames := ExtractPackages(body, title)
	if len(pkgNames) == 0 {
		return nil
	}

	cleanedBody := CleanPRBody(body)
	var pkgs []types.PackageInfo
	for _, name := range pkgNames {
		changelog := extractChangelog(body, name, cleanedBody)
		pkgs = append(pkgs, types.PackageInfo{
			Name:      name,
			Ecosystem: detectEcosystem(name),
			Changelog: changelog,
		})
	}
	return pkgs
}

// extractChangelog extracts release notes for a specific package from the PR body.
func extractChangelog(body, pkg, cleanedBody string) string {
	// Try package-specific section: ### ...pkg...
	escapedPkg := regexp.QuoteMeta(pkg)
	sectionRe := regexp.MustCompile(`(?s)###[^\n]*` + escapedPkg + `[^\n]*\n(.*?)(?:###\s*[a-zA-Z]|\z)`)
	if m := sectionRe.FindStringSubmatch(body); len(m) > 1 {
		return truncateLines(strings.TrimSpace(m[1]), 100)
	}

	// Fallback: use cleaned body
	return truncateLines(cleanedBody, 100)
}

// CleanPRBody strips Renovate boilerplate from a PR body.
func CleanPRBody(body string) string {
	// Remove from ### Configuration onward
	if loc := configSectionRe.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	// Remove from <!--renovate-debug: onward
	if loc := renovateDebugRe.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	return strings.TrimSpace(body)
}

func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

