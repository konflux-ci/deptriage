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

package analyze

import (
	_ "embed"
	"regexp"
	"strings"

	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"
)

//go:embed template.md
var promptTemplate string

var riskLevelRe = regexp.MustCompile(`(?i)Risk Level:\s*(LOW|MEDIUM|HIGH)`)

// RenderPrompt substitutes placeholders in the prompt template.
func RenderPrompt(bumpType types.BumpType, prTitle, packageContext string) string {
	prompt := promptTemplate
	prompt = strings.ReplaceAll(prompt, "{{BUMP_TYPE}}", string(bumpType))
	prompt = strings.ReplaceAll(prompt, "{{PR_TITLE}}", prTitle)
	prompt = strings.ReplaceAll(prompt, "{{PACKAGE_CONTEXT}}", packageContext)
	return prompt
}

// ExtractRiskLevel parses the risk level from an LLM response.
func ExtractRiskLevel(response string) types.RiskLevel {
	matches := riskLevelRe.FindStringSubmatch(response)
	if len(matches) < 2 {
		return types.RiskUnknown
	}
	switch strings.ToUpper(matches[1]) {
	case "LOW":
		return types.RiskLow
	case "MEDIUM":
		return types.RiskMedium
	case "HIGH":
		return types.RiskHigh
	default:
		return types.RiskUnknown
	}
}
