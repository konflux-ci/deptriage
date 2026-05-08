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
	"regexp"
	"strconv"
	"strings"

	"github.com/konflux-ci/deptriage/internal/types"
)

var (
	// Semver version pair: v1.2.3 -> v1.3.0 or v9.5 → v9.7 (third component optional)
	// Handles optional v prefix, backticks, -> or → arrows, and Docker-style suffixes
	// Captures: (1)oldMaj (2)oldMin (3)oldPat? (4)oldSuffix? (5)newMaj (6)newMin (7)newPat? (8)newSuffix?
	versionRe = regexp.MustCompile("[`]?v?([0-9]+)\\.([0-9]+)(?:\\.([0-9]+))?(?:[-.][`]?([^`\\s]*))?[`]?\\s*(?:->|→)\\s*[`]?v?([0-9]+)\\.([0-9]+)(?:\\.([0-9]+))?(?:[-.][`]?([^`\\s]*))?")

	// Digest-only: abcdef0 -> 1234abc
	digestRe = regexp.MustCompile("[`]?([0-9a-f]{7,})[`]?\\s*(?:->|→)\\s*[`]?([0-9a-f]{7,})")

	// pinDigest: Renovate "pinDigest" update type in PR body table
	pinDigestRe = regexp.MustCompile(`(?i)\|\s*pinDigest\s*\|`)
)

// DetectBumpType determines the semver bump type from PR title and body text.
func DetectBumpType(title, body string) types.BumpType {
	text := title + "\n" + body
	highest := types.BumpUnknown

	for _, match := range versionRe.FindAllStringSubmatch(text, -1) {
		bump := compareSemver(match[1], match[2], match[3], match[4], match[5], match[6], match[7], match[8])
		highest = maxBump(highest, bump)
	}

	if highest != types.BumpUnknown {
		return highest
	}

	// Check digest-only updates
	if digestRe.MatchString(text) {
		return types.BumpDigest
	}

	// Check pinDigest updates (first-time SHA pinning, no version transition)
	if pinDigestRe.MatchString(text) {
		return types.BumpPatch
	}

	return types.BumpUnknown
}

func compareSemver(oldMaj, oldMin, oldPat, oldSuffix, newMaj, newMin, newPat, newSuffix string) types.BumpType {
	switch {
	case mustAtoi(newMaj) > mustAtoi(oldMaj):
		return types.BumpMajor
	case mustAtoi(newMin) > mustAtoi(oldMin):
		return types.BumpMinor
	case oldPat != "" && newPat != "" && mustAtoi(newPat) > mustAtoi(oldPat):
		return types.BumpPatch
	case oldSuffix != "" && newSuffix != "" && oldSuffix != newSuffix:
		// Same major.minor(.patch) but different suffix (e.g. build IDs like 10.1-1776071394 → 10.1-1776646707)
		return types.BumpPatch
	default:
		return types.BumpUnknown
	}
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func maxBump(a, b types.BumpType) types.BumpType {
	if b.Priority() > a.Priority() {
		return b
	}
	return a
}
