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
	"regexp"
	"strings"
)

var (
	goToolchainRe     = regexp.MustCompile(`(?i)go-toolset|golang.*docker|docker.*golang`)
	goVersionBumpRe   = regexp.MustCompile(`(?i)go\s+1\.\d+.*->.*go\s+1\.\d+|update.*go.*directive`)
	containerImageRe  = regexp.MustCompile(`(?i)docker|container|image|registry\.(access\.)?redhat`)
)

// DetectRiskHints scans the PR title and body for known high-risk patterns
// and returns a newline-separated string of risk hints.
func DetectRiskHints(title, body string) string {
	var hints []string

	if goToolchainRe.MatchString(title) {
		hints = append(hints, "GO_TOOLCHAIN_UPDATE: This PR updates the Go build toolchain image. "+
			"This often requires coordinated changes to the build pipeline and can cause build failures "+
			"if the new Go version is incompatible with the current build infrastructure. "+
			"These updates are historically HIGH risk in this project.")
	}

	if goVersionBumpRe.MatchString(body) {
		hints = append(hints, "GO_VERSION_BUMP: The Go language version directive in go.mod may be changing. "+
			"This can introduce new language features that require a matching Go toolchain version in CI, "+
			"and may break builds if the CI build image uses an older Go version.")
	}

	if containerImageRe.MatchString(title) {
		hints = append(hints, "CONTAINER_IMAGE_UPDATE: This PR updates a container base image. "+
			"Base image changes can affect build behavior, available system libraries, and binary compatibility.")
	}

	return strings.Join(hints, "\n")
}
