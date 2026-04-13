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

package security

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantRedacted bool
		pattern  string
	}{
		{
			name:         "AWS key",
			input:        "Found key AKIAIOSFODNN7EXAMPLE in config",
			wantRedacted: true,
			pattern:      "AKIA",
		},
		{
			name:         "GitHub PAT",
			input:        "Token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh",
			wantRedacted: true,
			pattern:      "ghp_",
		},
		{
			name:         "sk- API key",
			input:        "api_key: sk-1234567890abcdefghijklmn",
			wantRedacted: true,
			pattern:      "sk-",
		},
		{
			name:         "generic key=value",
			input:        "api_key=supersecretvalue1234567890",
			wantRedacted: true,
			pattern:      "api_key",
		},
		{
			name:         "no secrets",
			input:        "This is a normal analysis with no secrets",
			wantRedacted: false,
		},
		{
			name:         "Google API key",
			input:        "key: AIzaSyA1234567890_abcdefghijklmnopqrstuv",
			wantRedacted: true,
			pattern:      "AIza",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSecrets(tt.input)
			if tt.wantRedacted {
				if !strings.Contains(result, "[REDACTED]") {
					t.Errorf("expected redaction, got: %s", result)
				}
				if tt.pattern != "" && strings.Contains(result, tt.pattern) {
					t.Errorf("expected %s to be redacted, got: %s", tt.pattern, result)
				}
			} else {
				if strings.Contains(result, "[REDACTED]") {
					t.Errorf("unexpected redaction in: %s", result)
				}
			}
		})
	}
}
