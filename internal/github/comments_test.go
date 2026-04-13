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

package github

import (
	"strings"
	"testing"
)

func TestTruncateComment(t *testing.T) {
	// Small body should pass through unchanged
	small := "Small body"
	if got := truncateComment(small); got != small {
		t.Errorf("expected unchanged body, got %q", got)
	}

	// Large body should be truncated
	large := strings.Repeat("x", maxCommentBytes+1000)
	result := truncateComment(large)
	if len(result) > maxCommentBytes+100 { // allow margin for truncation message
		t.Errorf("expected truncated body, got length %d", len(result))
	}
}

func TestStripMarkerAndHeader(t *testing.T) {
	body := "<!-- deptriage-analysis -->\n## AI Dependency Impact Analysis\n\nSome analysis content"
	got := stripMarkerAndHeader(body)
	if strings.Contains(got, "<!-- deptriage-analysis -->") {
		t.Error("marker not stripped")
	}
	if strings.Contains(got, "## AI Dependency Impact Analysis") {
		t.Error("header not stripped")
	}
	if !strings.Contains(got, "Some analysis content") {
		t.Error("content should be preserved")
	}
}
