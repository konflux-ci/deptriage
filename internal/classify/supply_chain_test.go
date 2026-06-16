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

	ghclient "github.com/konflux-ci/deptriage/internal/github"
	"github.com/konflux-ci/deptriage/internal/types"
)

func TestIsTrustedBot(t *testing.T) {
	tests := []struct {
		name      string
		author    string
		extraBots []string
		want      bool
	}{
		{"renovate bot", "renovate[bot]", nil, true},
		{"mintmaker bot", "red-hat-konflux[bot]", nil, true},
		{"dependabot", "dependabot[bot]", nil, true},
		{"human user", "someuser", nil, false},
		{"custom bot from extra list", "my-renovate[bot]", []string{"my-renovate[bot]"}, true},
		{"unknown bot not in extra", "unknown[bot]", []string{"other[bot]"}, false},
		{"empty author", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTrustedBot(tt.author, tt.extraBots); got != tt.want {
				t.Errorf("IsTrustedBot(%q, %v) = %v, want %v", tt.author, tt.extraBots, got, tt.want)
			}
		})
	}
}

func TestValidateAuthor(t *testing.T) {
	tests := []struct {
		name      string
		prAuthor  string
		commits   []ghclient.CommitInfo
		extraBots []string
		wantNil   bool
		wantKey   string
	}{
		{
			name:     "all commits by same bot",
			prAuthor: "renovate[bot]",
			commits: []ghclient.CommitInfo{
				{SHA: "abc123", Author: "renovate[bot]"},
				{SHA: "def456", Author: "renovate[bot]"},
			},
			wantNil: true,
		},
		{
			name:     "foreign commit detected",
			prAuthor: "renovate[bot]",
			commits: []ghclient.CommitInfo{
				{SHA: "abc123", Author: "renovate[bot]"},
				{SHA: "def456", Author: "attacker"},
			},
			wantKey: "SUPPLY_CHAIN_AUTHOR_MISMATCH",
		},
		{
			name:     "non-bot PR skips validation",
			prAuthor: "human-dev",
			commits: []ghclient.CommitInfo{
				{SHA: "abc123", Author: "someone-else"},
			},
			wantNil: true,
		},
		{
			name:      "custom trusted bot with matching commits",
			prAuthor:  "my-renovate[bot]",
			commits:   []ghclient.CommitInfo{{SHA: "abc123", Author: "my-renovate[bot]"}},
			extraBots: []string{"my-renovate[bot]"},
			wantNil:   true,
		},
		{
			name:      "custom trusted bot with foreign commit",
			prAuthor:  "my-renovate[bot]",
			commits:   []ghclient.CommitInfo{{SHA: "abc123", Author: "evil-user"}},
			extraBots: []string{"my-renovate[bot]"},
			wantKey:   "SUPPLY_CHAIN_AUTHOR_MISMATCH",
		},
		{
			name:     "empty commit list",
			prAuthor: "renovate[bot]",
			commits:  nil,
			wantNil:  true,
		},
		{
			name:     "commit with empty author treated as mismatch",
			prAuthor: "renovate[bot]",
			commits:  []ghclient.CommitInfo{{SHA: "abc123", Author: ""}},
			wantKey:  "SUPPLY_CHAIN_AUTHOR_MISMATCH",
		},
		{
			name:     "multiple foreign commits",
			prAuthor: "renovate[bot]",
			commits: []ghclient.CommitInfo{
				{SHA: "aaa", Author: "renovate[bot]"},
				{SHA: "bbb", Author: "attacker1"},
				{SHA: "ccc", Author: "attacker2"},
			},
			wantKey: "SUPPLY_CHAIN_AUTHOR_MISMATCH",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateAuthor(tt.prAuthor, tt.commits, tt.extraBots)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil finding, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected finding, got nil")
			}
			if got.Key != tt.wantKey {
				t.Errorf("got key %q, want %q", got.Key, tt.wantKey)
			}
			if got.Label != types.LabelSupplyChainAuthorMismatch {
				t.Errorf("got label %q, want %q", got.Label, types.LabelSupplyChainAuthorMismatch)
			}
			if got.Color != types.ColorRed {
				t.Errorf("got color %q, want %q", got.Color, types.ColorRed)
			}
		})
	}
}

func TestDetectSuspiciousFiles(t *testing.T) {
	tests := []struct {
		name       string
		files      []string
		extraPaths []string
		wantNil    bool
		wantCount  int
	}{
		{
			name:    "clean dependency PR",
			files:   []string{"go.mod", "go.sum"},
			wantNil: true,
		},
		{
			name:      "claude directory",
			files:     []string{"go.mod", ".claude/settings.json"},
			wantCount: 1,
		},
		{
			name:      "vscode directory",
			files:     []string{".vscode/settings.json"},
			wantCount: 1,
		},
		{
			name:      "github workflows",
			files:     []string{".github/workflows/ci.yml"},
			wantCount: 1,
		},
		{
			name:      "github actions",
			files:     []string{".github/actions/my-action/action.yml"},
			wantCount: 1,
		},
		{
			name:      "executable script in root",
			files:     []string{"setup.sh"},
			wantCount: 1,
		},
		{
			name:      "mjs script — known malware pattern",
			files:     []string{".claude/setup.mjs"},
			wantCount: 1,
		},
		{
			name:    "script inside vendor directory — not flagged",
			files:   []string{"vendor/github.com/foo/bar/generate.sh"},
			wantNil: true,
		},
		{
			name:    "script inside node_modules — not flagged",
			files:   []string{"node_modules/.bin/prettier.js"},
			wantNil: true,
		},
		{
			name:    "script inside third_party — not flagged",
			files:   []string{"third_party/tool/run.py"},
			wantNil: true,
		},
		{
			name:      "multiple suspicious files",
			files:     []string{".claude/setup.mjs", ".vscode/tasks.json", "hack.sh"},
			wantCount: 3,
		},
		{
			name:       "custom suspicious path",
			files:      []string{".devcontainer/devcontainer.json"},
			extraPaths: []string{".devcontainer/"},
			wantCount:  1,
		},
		{
			name:    "non-script file outside vendor — not flagged",
			files:   []string{"internal/foo/bar.go"},
			wantNil: true,
		},
		{
			name:      "python script outside vendor",
			files:     []string{"scripts/deploy.py"},
			wantCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectSuspiciousFiles(tt.files, tt.extraPaths)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected finding, got nil")
			}
			if got.Label != types.LabelSupplyChainSuspiciousFiles {
				t.Errorf("got label %q, want %q", got.Label, types.LabelSupplyChainSuspiciousFiles)
			}
			if len(got.Details) != tt.wantCount {
				t.Errorf("got %d flagged files, want %d: %v", len(got.Details), tt.wantCount, got.Details)
			}
		})
	}
}

func TestValidateDiffScope(t *testing.T) {
	tests := []struct {
		name          string
		prAuthor      string
		files         []string
		extraBots     []string
		extraPatterns []string
		wantNil       bool
		wantCount     int
	}{
		{
			name:     "manifest-only changes",
			prAuthor: "renovate[bot]",
			files:    []string{"go.mod", "go.sum"},
			wantNil:  true,
		},
		{
			name:     "manifest plus vendor",
			prAuthor: "renovate[bot]",
			files:    []string{"go.mod", "go.sum", "vendor/github.com/foo/bar/baz.go"},
			wantNil:  true,
		},
		{
			name:     "tekton task refs",
			prAuthor: "renovate[bot]",
			files:    []string{".tekton/pull-request.yaml", ".tekton/push.yaml"},
			wantNil:  true,
		},
		{
			name:     "dockerfile changes",
			prAuthor: "renovate[bot]",
			files:    []string{"Containerfile"},
			wantNil:  true,
		},
		{
			name:     "variant dockerfile (e.g. base.Dockerfile)",
			prAuthor: "renovate[bot]",
			files:    []string{"base.Dockerfile"},
			wantNil:  true,
		},
		{
			name:      "unexpected source file",
			prAuthor:  "renovate[bot]",
			files:     []string{"go.mod", "go.sum", "internal/foo/bar.go"},
			wantCount: 1,
		},
		{
			name:      "unexpected file at root",
			prAuthor:  "renovate[bot]",
			files:     []string{"go.mod", "README.md"},
			wantCount: 1,
		},
		{
			name:     "non-bot PR skips validation",
			prAuthor: "human-dev",
			files:    []string{"internal/foo/bar.go", "main.go"},
			wantNil:  true,
		},
		{
			name:          "custom expected pattern",
			prAuthor:      "renovate[bot]",
			files:         []string{"go.mod", "Chart.yaml"},
			extraPatterns: []string{"Chart.yaml"},
			wantNil:       true,
		},
		{
			name:      "multiple unexpected files",
			prAuthor:  "renovate[bot]",
			files:     []string{"go.mod", "cmd/main.go", "internal/util.go", "Makefile"},
			wantCount: 3,
		},
		{
			name:     "renovate config file",
			prAuthor: "renovate[bot]",
			files:    []string{"renovate.json"},
			wantNil:  true,
		},
		{
			name:     "node.js manifests",
			prAuthor: "renovate[bot]",
			files:    []string{"package.json", "package-lock.json"},
			wantNil:  true,
		},
		{
			name:          "directory-aware custom pattern",
			prAuthor:      "renovate[bot]",
			files:         []string{"go.mod", "charts/values.yaml"},
			extraPatterns: []string{"charts/*.yaml"},
			wantNil:       true,
		},
		{
			name:          "directory-aware pattern does not match wrong dir",
			prAuthor:      "renovate[bot]",
			files:         []string{"go.mod", "other/values.yaml"},
			extraPatterns: []string{"charts/*.yaml"},
			wantCount:     1,
		},
		{
			name:     "gitmodules change is expected by default",
			prAuthor: "renovate[bot]",
			files:    []string{".gitmodules"},
			wantNil:  true,
		},
		{
			name:      "submodule pointer without extra pattern is unexpected",
			prAuthor:  "renovate[bot]",
			files:     []string{".gitmodules", "oauth2-proxy"},
			wantCount: 1,
		},
		{
			name:          "submodule pointer with extra pattern is expected",
			prAuthor:      "renovate[bot]",
			files:         []string{".gitmodules", "oauth2-proxy"},
			extraPatterns: []string{"oauth2-proxy"},
			wantNil:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateDiffScope(tt.prAuthor, tt.files, tt.extraBots, tt.extraPatterns)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected finding, got nil")
			}
			if got.Label != types.LabelSupplyChainUnexpectedScope {
				t.Errorf("got label %q, want %q", got.Label, types.LabelSupplyChainUnexpectedScope)
			}
			if len(got.Details) != tt.wantCount {
				t.Errorf("got %d unexpected files, want %d: %v", len(got.Details), tt.wantCount, got.Details)
			}
		})
	}
}
