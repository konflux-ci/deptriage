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
	"testing"

	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"
)

func TestShouldAttemptMerge(t *testing.T) {
	tests := []struct {
		name        string
		autoMerge   bool
		autoApprove bool
		risk        types.RiskLevel
		want        bool
	}{
		{
			name:        "all conditions met with low risk",
			autoMerge:   true,
			autoApprove: true,
			risk:        types.RiskLow,
			want:        true,
		},
		{
			name:        "medium risk still eligible",
			autoMerge:   true,
			autoApprove: true,
			risk:        types.RiskMedium,
			want:        true,
		},
		{
			name:        "high risk blocks merge",
			autoMerge:   true,
			autoApprove: true,
			risk:        types.RiskHigh,
			want:        false,
		},
		{
			name:        "unknown risk eligible",
			autoMerge:   true,
			autoApprove: true,
			risk:        types.RiskUnknown,
			want:        true,
		},
		{
			name:        "auto-merge disabled",
			autoMerge:   false,
			autoApprove: true,
			risk:        types.RiskLow,
			want:        false,
		},
		{
			name:        "auto-approve disabled",
			autoMerge:   true,
			autoApprove: false,
			risk:        types.RiskLow,
			want:        false,
		},
		{
			name:        "both disabled",
			autoMerge:   false,
			autoApprove: false,
			risk:        types.RiskLow,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAttemptMerge(tt.autoMerge, tt.autoApprove, tt.risk)
			if got != tt.want {
				t.Errorf("shouldAttemptMerge(%v, %v, %v) = %v, want %v",
					tt.autoMerge, tt.autoApprove, tt.risk, got, tt.want)
			}
		})
	}
}
