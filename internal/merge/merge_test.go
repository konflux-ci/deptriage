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

import "testing"

func TestIsMergeEligible(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   bool
	}{
		{
			name:   "approved and lgtm present",
			labels: []string{"approved", "lgtm", "semver/patch"},
			want:   true,
		},
		{
			name:   "approved and lgtm with medium risk",
			labels: []string{"approved", "lgtm", "risk/medium", "semver/patch"},
			want:   true,
		},
		{
			name:   "missing approved",
			labels: []string{"lgtm", "semver/patch"},
			want:   false,
		},
		{
			name:   "missing lgtm",
			labels: []string{"approved", "semver/patch"},
			want:   false,
		},
		{
			name:   "missing both",
			labels: []string{"semver/patch"},
			want:   false,
		},
		{
			name:   "risk/high blocks merge",
			labels: []string{"approved", "lgtm", "risk/high"},
			want:   false,
		},
		{
			name:   "empty labels",
			labels: []string{},
			want:   false,
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   false,
		},
		{
			name:   "supply-chain/author-mismatch blocks merge even with approved+lgtm",
			labels: []string{"approved", "lgtm", "supply-chain/author-mismatch"},
			want:   false,
		},
		{
			name:   "supply-chain/suspicious-files blocks merge even with approved+lgtm",
			labels: []string{"approved", "lgtm", "supply-chain/suspicious-files"},
			want:   false,
		},
		{
			name:   "supply-chain/unexpected-scope blocks merge even with approved+lgtm",
			labels: []string{"approved", "lgtm", "supply-chain/unexpected-scope"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMergeEligible(tt.labels)
			if got != tt.want {
				t.Errorf("isMergeEligible(%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}
}

func TestIsDeferredApprovalEligible(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   bool
	}{
		{
			name:   "patch with go-toolchain risk hint",
			labels: []string{"semver/patch", "risk-hint/go-toolchain"},
			want:   true,
		},
		{
			name:   "patch with multiple risk hints",
			labels: []string{"semver/patch", "risk-hint/go-toolchain", "risk-hint/container-image"},
			want:   true,
		},
		{
			name:   "patch with risk hint and medium risk",
			labels: []string{"semver/patch", "risk-hint/go-toolchain", "risk/medium"},
			want:   true,
		},
		{
			name:   "patch with risk hint but risk/high blocks",
			labels: []string{"semver/patch", "risk-hint/go-toolchain", "risk/high"},
			want:   false,
		},
		{
			name:   "minor bump with risk hint eligible",
			labels: []string{"semver/minor", "risk-hint/go-toolchain"},
			want:   true,
		},
		{
			name:   "patch without risk hint not eligible",
			labels: []string{"semver/patch"},
			want:   false,
		},
		{
			name:   "no labels",
			labels: []string{},
			want:   false,
		},
		{
			name:   "already approved not reached",
			labels: []string{"semver/patch", "risk-hint/go-toolchain", "approved", "lgtm"},
			want:   true,
		},
		{
			name:   "supply-chain author mismatch blocks deferred approval",
			labels: []string{"semver/patch", "risk-hint/go-toolchain", "supply-chain/author-mismatch"},
			want:   false,
		},
		{
			name:   "supply-chain suspicious files blocks deferred approval",
			labels: []string{"semver/patch", "risk-hint/go-toolchain", "supply-chain/suspicious-files"},
			want:   false,
		},
		{
			name:   "supply-chain unexpected scope blocks deferred approval",
			labels: []string{"semver/minor", "risk-hint/go-toolchain", "supply-chain/unexpected-scope"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDeferredApprovalEligible(tt.labels)
			if got != tt.want {
				t.Errorf("isDeferredApprovalEligible(%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}
}
