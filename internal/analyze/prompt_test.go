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
	"strings"
	"testing"

	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"
)

func TestRenderPrompt(t *testing.T) {
	result := RenderPrompt(types.BumpMinor, "Update github.com/foo/bar", `{"packages":[]}`)

	if !strings.Contains(result, "**minor**") {
		t.Error("expected bump type in prompt")
	}
	if !strings.Contains(result, "Update github.com/foo/bar") {
		t.Error("expected PR title in prompt")
	}
	if !strings.Contains(result, `{"packages":[]}`) {
		t.Error("expected package context in prompt")
	}
}

func TestExtractRiskLevel(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     types.RiskLevel
	}{
		{
			name:     "low risk",
			response: "### Risk Level: LOW\n\nSome analysis...",
			want:     types.RiskLow,
		},
		{
			name:     "medium risk",
			response: "### Risk Level: MEDIUM\n\nSome analysis...",
			want:     types.RiskMedium,
		},
		{
			name:     "high risk",
			response: "### Risk Level: HIGH\n\nSome analysis...",
			want:     types.RiskHigh,
		},
		{
			name:     "case insensitive",
			response: "### Risk Level: low\n\nSome analysis...",
			want:     types.RiskLow,
		},
		{
			name:     "no risk level found",
			response: "Some analysis without risk level",
			want:     types.RiskUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRiskLevel(tt.response)
			if got != tt.want {
				t.Errorf("ExtractRiskLevel() = %q, want %q", got, tt.want)
			}
		})
	}
}
