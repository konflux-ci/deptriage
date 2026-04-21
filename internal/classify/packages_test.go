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
)

func TestExtractPackages(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		title string
		want  []string
	}{
		{
			name:  "bare package format",
			body:  "| github.com/foo/bar | `v1.0.0` -> `v1.0.1` |",
			title: "",
			want:  []string{"github.com/foo/bar"},
		},
		{
			name:  "linked package format",
			body:  "| [github.com/foo/bar](https://github.com/foo/bar) | `v1.0.0` -> `v1.0.1` |",
			title: "",
			want:  []string{"github.com/foo/bar"},
		},
		{
			name: "multiple packages",
			body: "| github.com/foo/bar | `v1.0.0` -> `v1.0.1` |\n| github.com/baz/qux | `v2.0.0` -> `v2.1.0` |\n| go.uber.org/zap | `v1.0.0` -> `v1.1.0` |",
			want: []string{"github.com/foo/bar", "github.com/baz/qux", "go.uber.org/zap"},
		},
		{
			name:  "duplicate packages",
			body:  "| github.com/foo/bar | `v1.0.0` -> `v1.0.1` |\n| github.com/foo/bar | `v1.0.0` -> `v1.0.1` |",
			title: "",
			want:  []string{"github.com/foo/bar"},
		},
		{
			name:  "fallback to title",
			body:  "No packages here",
			title: "Update github.com/foo/bar to v1.2.0",
			want:  []string{"github.com/foo/bar"},
		},
		{
			name:  "no packages found",
			body:  "Nothing here",
			title: "Some PR",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPackages(tt.body, tt.title)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractPackages() returned %d packages, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("package[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetectEcosystem(t *testing.T) {
	tests := []struct {
		pkg  string
		want string
	}{
		{"github.com/foo/bar", "gomod"},
		{"k8s.io/api", "gomod"},
		{"sigs.k8s.io/controller-runtime", "gomod"},
		{"golang.org/x/net", "gomod"},
		{"google.golang.org/grpc", "gomod"},
		{"go.uber.org/zap", "gomod"},
		{"gopkg.in/yaml.v3", "gomod"},
		{"registry.access.redhat.com/ubi9", ""},
		{"quay.io/konflux-ci/some-task", ""},
		{"some-random-string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			got := detectEcosystem(tt.pkg)
			if got != tt.want {
				t.Errorf("detectEcosystem(%q) = %q, want %q", tt.pkg, got, tt.want)
			}
		})
	}
}

func TestExtractPackagesWithChangelogSetsEcosystem(t *testing.T) {
	body := "| github.com/foo/bar | `v1.0.0` -> `v1.0.1` |"
	pkgs := ExtractPackagesWithChangelog(body, "")
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].Ecosystem != "gomod" {
		t.Errorf("expected ecosystem %q, got %q", "gomod", pkgs[0].Ecosystem)
	}
}

func TestCleanPRBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "strip configuration section",
			body: "## Release Notes\nSome notes\n\n### Configuration\nschedule: automerge\nrebasing: true",
			want: "## Release Notes\nSome notes",
		},
		{
			name: "strip renovate debug",
			body: "## Changes\nStuff\n\n<!--renovate-debug:abc123-->",
			want: "## Changes\nStuff",
		},
		{
			name: "no boilerplate",
			body: "Just a clean body",
			want: "Just a clean body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanPRBody(tt.body)
			if got != tt.want {
				t.Errorf("CleanPRBody() = %q, want %q", got, tt.want)
			}
		})
	}
}
