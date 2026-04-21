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

package security

import (
	"regexp"
)

var secretPatterns = []*regexp.Regexp{
	// AWS access key IDs
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	// GitHub personal access tokens
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
	// GitHub fine-grained tokens
	regexp.MustCompile(`github_pat_[a-zA-Z0-9_]{82}`),
	// Generic API keys (sk-...)
	regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
	// Anthropic API keys
	regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]{20,}`),
	// Google API keys
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
	// Generic bearer tokens
	regexp.MustCompile(`Bearer\s+[a-zA-Z0-9._-]{20,}`),
	// Generic secrets in key=value format
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password|credential)\s*[:=]\s*["']?[a-zA-Z0-9._/+-]{16,}["']?`),
}

// RedactSecrets scans text for potential secrets and replaces them with [REDACTED].
func RedactSecrets(text string) string {
	for _, re := range secretPatterns {
		text = re.ReplaceAllString(text, "[REDACTED]")
	}
	return text
}
