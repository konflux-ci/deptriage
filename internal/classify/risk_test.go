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
	"strings"
	"testing"
)

func TestDetectRiskHints(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		body        string
		wantHints   []string
		wantEmpty   bool
	}{
		{
			name:      "go toolset in title",
			title:     "Update go-toolset image to 1.22",
			body:      "",
			wantHints: []string{"GO_TOOLCHAIN_UPDATE"},
		},
		{
			name:      "golang docker in title",
			title:     "Update golang Docker image",
			body:      "",
			wantHints: []string{"GO_TOOLCHAIN_UPDATE", "CONTAINER_IMAGE_UPDATE"},
		},
		{
			name:      "go version bump in body",
			title:     "Update go.mod",
			body:      "go 1.21 -> go 1.22",
			wantHints: []string{"GO_VERSION_BUMP"},
		},
		{
			name:      "container image update",
			title:     "Update registry.access.redhat.com/ubi9 image",
			body:      "",
			wantHints: []string{"CONTAINER_IMAGE_UPDATE"},
		},
		{
			name:      "no risk patterns",
			title:     "Update github.com/stretchr/testify to v1.9.0",
			body:      "Patch bump with bug fixes",
			wantEmpty: true,
		},
		{
			name:      "multiple hints",
			title:     "Update go-toolset Docker image",
			body:      "go 1.21 -> go 1.22",
			wantHints: []string{"GO_TOOLCHAIN_UPDATE", "GO_VERSION_BUMP", "CONTAINER_IMAGE_UPDATE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectRiskHints(tt.title, tt.body)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty hints, got %q", got)
				}
				return
			}
			for _, hint := range tt.wantHints {
				if !strings.Contains(got, hint) {
					t.Errorf("expected hint %q in output, got %q", hint, got)
				}
			}
		})
	}
}
