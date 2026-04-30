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

// RiskHint represents a detected risk pattern with its associated label metadata.
type RiskHint struct {
	Key         string // e.g. "GO_TOOLCHAIN_UPDATE"
	Label       string // GitHub label name, e.g. "risk-hint/go-toolchain"
	Color       string // Label color hex
	Description string // Human-readable explanation for the LLM prompt
}


var (
	goToolchainRe    = regexp.MustCompile(`(?i)go-toolset|golang.*docker|docker.*golang`)
	goVersionBumpRe  = regexp.MustCompile(`(?i)go\s+1\.\d+.*->.*go\s+1\.\d+|update.*go.*directive`)
	containerImageRe = regexp.MustCompile(`(?i)docker|container|image|registry\.(access\.)?redhat`)

	riskHintDefs = []struct {
		re          *regexp.Regexp
		matchField  string // "title" or "body"
		key         string
		label       string
		color       string
		description string
	}{
		{
			re:         goToolchainRe,
			matchField: "title",
			key:        "GO_TOOLCHAIN_UPDATE",
			label:      "risk-hint/go-toolchain",
			color:      types.ColorYellow,
			description: "This PR updates the Go build toolchain image. " +
				"This often requires coordinated changes to the build pipeline and can cause build failures " +
				"if the new Go version is incompatible with the current build infrastructure. " +
				"These updates are historically HIGH risk in this project.",
		},
		{
			re:         goVersionBumpRe,
			matchField: "body",
			key:        "GO_VERSION_BUMP",
			label:      "risk-hint/go-version-bump",
			color:      types.ColorYellow,
			description: "The Go language version directive in go.mod may be changing. " +
				"This can introduce new language features that require a matching Go toolchain version in CI, " +
				"and may break builds if the CI build image uses an older Go version.",
		},
		{
			re:         containerImageRe,
			matchField: "title",
			key:        "CONTAINER_IMAGE_UPDATE",
			label:      "risk-hint/container-image",
			color:      types.ColorYellow,
			description: "This PR updates a container base image. " +
				"Base image changes can affect build behavior, available system libraries, and binary compatibility.",
		},
	}
)

// DetectRiskHints scans the PR title and body for known high-risk patterns
// and returns a newline-separated string of risk hints.
func DetectRiskHints(title, body string) string {
	hints := DetectRiskHintLabels(title, body)
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = h.Key + ": " + h.Description
	}
	return strings.Join(parts, "\n")
}

// DetectRiskHintLabels returns structured risk hints with label metadata.
func DetectRiskHintLabels(title, body string) []RiskHint {
	var hints []RiskHint
	for _, def := range riskHintDefs {
		text := title
		if def.matchField == "body" {
			text = body
		}
		if def.re.MatchString(text) {
			hints = append(hints, RiskHint{
				Key:         def.key,
				Label:       def.label,
				Color:       def.color,
				Description: def.description,
			})
		}
	}
	return hints
}
