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
