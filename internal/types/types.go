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

package types

// Well-known label names used across classify, analyze, and merge phases.
const (
	LabelApproved  = "approved"
	LabelLGTM      = "lgtm"
	LabelRiskHigh  = "risk/high"
	LabelSemverPatch = "semver/patch"
	LabelSemverMinor = "semver/minor"

	RiskHintLabelPrefix = "risk-hint/"

	LabelSupplyChainAuthorMismatch  = "supply-chain/author-mismatch"
	LabelSupplyChainSuspiciousFiles = "supply-chain/suspicious-files"
	LabelSupplyChainUnexpectedScope = "supply-chain/unexpected-scope"
	SupplyChainLabelPrefix          = "supply-chain/"
)

// Well-known label colors.
const (
	ColorGreen  = "0e8a16"
	ColorYellow = "fbca04"
	ColorRed    = "e11d48"
)

// GitHub pull request review event types.
const (
	ReviewApprove = "APPROVE"
	ReviewComment = "COMMENT"
)

// Shared structured log keys.
const (
	LogKeyPR    = "pr"
	LogKeyEvent = "event"
	LogKeyLabel = "label"
)

// BumpType represents a semver bump classification.
type BumpType string

func (b BumpType) String() string { return string(b) }

const (
	BumpMajor   BumpType = "major"
	BumpMinor   BumpType = "minor"
	BumpPatch   BumpType = "patch"
	BumpDigest  BumpType = "digest"
	BumpUnknown BumpType = "unknown"
)

// Label returns the GitHub label name for the bump type (e.g. "semver/patch").
// For digest bumps, use DigestLabel() which requires ecosystem context.
// Returns empty string for unknown.
func (b BumpType) Label() string {
	switch b {
	case BumpPatch, BumpMinor, BumpMajor:
		return "semver/" + string(b)
	case BumpDigest:
		return "semver/patch" // default; callers should use DigestLabel for ecosystem-aware labeling
	default:
		return ""
	}
}

// DigestLabel returns the correct label for a digest bump based on ecosystem.
// Gomod digests are labeled as minor because pseudo-versions have no semver guarantees.
// Non-gomod digests (container images, etc.) are labeled as patch.
func DigestLabel(ecosystem string) string {
	if ecosystem == "gomod" {
		return "semver/minor"
	}
	return "semver/patch"
}

// DigestLabelColor returns the label color for a digest bump based on ecosystem.
func DigestLabelColor(ecosystem string) string {
	if ecosystem == "gomod" {
		return ColorYellow
	}
	return ColorGreen
}

// Priority returns the bump severity for highest-wins comparison.
func (b BumpType) Priority() int {
	switch b {
	case BumpDigest:
		return 1
	case BumpPatch:
		return 2
	case BumpMinor:
		return 3
	case BumpMajor:
		return 4
	default:
		return 0
	}
}

// Color returns the GitHub label hex color for the bump type.
// Returns empty string for unknown.
func (b BumpType) Color() string {
	switch b {
	case BumpPatch, BumpDigest:
		return ColorGreen
	case BumpMinor:
		return ColorYellow
	case BumpMajor:
		return ColorRed
	default:
		return ""
	}
}

// RiskLevel represents an AI-assessed risk level.
type RiskLevel string

func (r RiskLevel) String() string { return string(r) }
func (r RiskLevel) Label() string  { return "risk/" + string(r) }

func (r RiskLevel) Color() string {
	switch r {
	case RiskLow:
		return ColorGreen
	case RiskMedium:
		return ColorYellow
	case RiskHigh:
		return ColorRed
	default:
		return ""
	}
}

const (
	RiskLow     RiskLevel = "low"
	RiskMedium  RiskLevel = "medium"
	RiskHigh    RiskLevel = "high"
	RiskUnknown RiskLevel = "unknown"
)

// SupplyChainFindingResult is a serializable supply-chain finding for ClassifyResult.
type SupplyChainFindingResult struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

// ClassifyResult is the output of the classify subcommand.
type ClassifyResult struct {
	BumpType             BumpType                   `json:"bumpType"`
	Packages             []PackageInfo              `json:"packages"`
	RiskHints            string                     `json:"riskHints"`
	SupplyChainFindings  []SupplyChainFindingResult  `json:"supplyChainFindings,omitempty"`
	PRTitle              string                     `json:"prTitle"`
	PRBody               string                     `json:"prBody"`
	Repo                 string                     `json:"repo"`
	PRNumber             int                        `json:"prNumber"`
	Label                string                     `json:"label,omitempty"`
}

// PackageInfo holds extracted package metadata from the PR.
type PackageInfo struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem,omitempty"`
	Changelog string `json:"changelog,omitempty"`
}

// ImportInfo describes a file that imports a dependency.
type ImportInfo struct {
	File    string `json:"file"`
	HasTest bool   `json:"hasTest"`
	Snippet string `json:"snippet,omitempty"`
}

// Advisory holds a GitHub Security Advisory entry.
type Advisory struct {
	GHSAID          string  `json:"ghsaId"`
	CVE             string  `json:"cve,omitempty"`
	Severity        string  `json:"severity"`
	CVSSScore       float64 `json:"cvssScore,omitempty"`
	PatchedVersions string  `json:"patchedVersions,omitempty"`
}

// GovulncheckResult holds govulncheck output for a package.
type GovulncheckResult struct {
	Reachable bool              `json:"reachable"`
	Findings  []VulnFinding     `json:"findings"`
}

// VulnFinding describes a single reachable vulnerability.
type VulnFinding struct {
	ID        string `json:"id"`
	Symbol    string `json:"symbol"`
	CallChain string `json:"callChain,omitempty"`
}

// PackageContext holds the full analysis context for a single package.
type PackageContext struct {
	Name             string              `json:"name"`
	Changelog        string              `json:"changelog,omitempty"`
	NoDirectImports  bool                `json:"noDirectImports"`
	ImportChain      string              `json:"importChain,omitempty"`
	Imports          []ImportInfo        `json:"imports,omitempty"`
	Advisories       []Advisory          `json:"advisories,omitempty"`
	Govulncheck      *GovulncheckResult  `json:"govulncheck,omitempty"`
}

// ContextJSON is the full context assembled for LLM consumption.
type ContextJSON struct {
	PRBody              string                     `json:"prBody"`
	Packages            []PackageContext            `json:"packages"`
	RiskHints           string                     `json:"riskHints,omitempty"`
	SupplyChainFindings []SupplyChainFindingResult  `json:"supplyChainFindings,omitempty"`
}
