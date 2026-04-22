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
	"testing"

	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"
)

func TestDetectBumpType(t *testing.T) {
	tests := []struct {
		name  string
		title string
		body  string
		want  types.BumpType
	}{
		{
			name:  "three-component major",
			title: "Update foo",
			body:  "| foo | `v1.5.0` -> `v2.0.0` |",
			want:  types.BumpMajor,
		},
		{
			name:  "three-component minor",
			title: "Update foo",
			body:  "| foo | `v1.5.0` -> `v1.9.0` |",
			want:  types.BumpMinor,
		},
		{
			name:  "three-component patch",
			title: "Update foo",
			body:  "| foo | `v1.5.0` -> `v1.5.1` |",
			want:  types.BumpPatch,
		},
		{
			name:  "two-component minor",
			title: "Update foo",
			body:  "| foo | `v9.5` -> `v9.7` |",
			want:  types.BumpMinor,
		},
		{
			name:  "two-component major",
			title: "Update foo",
			body:  "| foo | `v1.5` -> `v2.0` |",
			want:  types.BumpMajor,
		},
		{
			name:  "digest only",
			title: "Update foo digest",
			body:  "`abcdef0` -> `1234abc`",
			want:  types.BumpDigest,
		},
		{
			name:  "no version info",
			title: "Some random PR",
			body:  "No version information here",
			want:  types.BumpUnknown,
		},
		{
			name:  "no v prefix",
			title: "Update foo",
			body:  "| foo | 1.2.3 -> 1.3.0 |",
			want:  types.BumpMinor,
		},
		{
			name:  "mixed bumps highest wins",
			title: "Update deps",
			body:  "| a | `v1.0.0` -> `v1.0.1` |\n| b | `v2.3.0` -> `v2.4.0` |",
			want:  types.BumpMinor,
		},
		{
			name:  "version in title with arrow",
			title: "Update github.com/foo/bar v1.0.0 -> v2.0.0",
			body:  "",
			want:  types.BumpMajor,
		},
		{
			name:  "backtick wrapped versions",
			title: "Update foo",
			body:  "`1.0.0` -> `1.0.1`",
			want:  types.BumpPatch,
		},
		{
			name:  "unicode arrow in body (Renovate/MintMaker style)",
			title: "Update module github.com/google/go-github/v84 to v85",
			body:  "| [github.com/google/go-github/v84] | `v84.0.0` → `v85.0.0` |",
			want:  types.BumpMajor,
		},
		{
			name:  "unicode arrow minor bump",
			title: "Update foo",
			body:  "`v1.2.0` → `v1.3.0`",
			want:  types.BumpMinor,
		},
		// Real-world Renovate/MintMaker patterns from konflux-ci/deptriage PRs
		{
			name:  "go module major bump v68 to v84",
			title: "Update module github.com/google/go-github/v68 to v84",
			body:  "| [github.com/google/go-github/v68] | `v68.0.0` → `v84.0.0` |",
			want:  types.BumpMajor,
		},
		{
			name:  "go module patch bump",
			title: "Update module github.com/spf13/pflag to v1.0.10",
			body:  "| [github.com/spf13/pflag] | `v1.0.9` → `v1.0.10` |",
			want:  types.BumpPatch,
		},
		{
			name:  "go module minor bump",
			title: "Update module github.com/google/go-querystring to v1.2.0",
			body:  "| [github.com/google/go-querystring] | `v1.1.0` → `v1.2.0` |",
			want:  types.BumpMinor,
		},
		{
			name:  "docker tag with build ID (patch)",
			title: "chore(deps): update registry.access.redhat.com/ubi10/ubi-minimal docker tag to v10.1-1776646707",
			body:  "| registry.access.redhat.com/ubi10/ubi-minimal | final | patch | `10.1-1776071394` → `10.1-1776646707` |",
			want:  types.BumpPatch,
		},
		{
			name:  "docker tag go-toolset with build ID (patch)",
			title: "chore(deps): update registry.access.redhat.com/ubi10/go-toolset docker tag to v10.1-1776763700",
			body:  "| registry.access.redhat.com/ubi10/go-toolset | stage | patch | `10.1-1776242024` → `10.1-1776763700` |",
			want:  types.BumpPatch,
		},
		{
			name:  "renovate conventional commit prefix",
			title: "fix(deps): update module github.com/google/go-github/v84 to v85",
			body:  "| [github.com/google/go-github/v84] | `v84.0.0` → `v85.0.0` |",
			want:  types.BumpMajor,
		},
		{
			name:  "title only no body - go module with to",
			title: "Update module github.com/foo/bar to v2.0.0",
			body:  "",
			want:  types.BumpUnknown,
		},
		{
			name:  "docker digest update",
			title: "Update docker.io/library/golang digest to abc1234",
			body:  "`1a2b3c4d5e6f7` → `abc1234def5678`",
			want:  types.BumpDigest,
		},
		{
			name:  "alpine-style suffix version bump",
			title: "Update node Docker tag",
			body:  "| node | `18.5-alpine` → `18.6-alpine` |",
			want:  types.BumpMinor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectBumpType(tt.title, tt.body)
			if got != tt.want {
				t.Errorf("DetectBumpType() = %q, want %q", got, tt.want)
			}
		})
	}
}
