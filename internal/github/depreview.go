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

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/konflux-ci/deptriage/internal/types"
)

const depReviewTimeout = 30 * time.Second

// DepReviewEntry represents a single dependency change from the Dependency Review API.
type DepReviewEntry struct {
	Name             string   `json:"name"`
	Version          string   `json:"version"`
	PreviousVersion  string   `json:"previous_version"`
	Ecosystem        string   `json:"ecosystem"`
	Vulnerabilities  []DepVuln `json:"vulnerabilities"`
	ChangeType       string   `json:"change_type"` // added, removed, updated
}

// DepVuln represents a vulnerability surfaced by the Dependency Review API.
type DepVuln struct {
	Severity        string `json:"severity"`
	AdvisoryGHSAID  string `json:"advisory_ghsa_id"`
	AdvisorySummary string `json:"advisory_summary"`
}

// FetchDependencyReview calls the GitHub Dependency Review API.
func (c *Client) FetchDependencyReview(ctx context.Context, baseRef, headRef string) ([]DepReviewEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, depReviewTimeout)
	defer cancel()

	url := fmt.Sprintf("repos/%s/%s/dependency-graph/compare/%s...%s", c.owner, c.repo, baseRef, headRef)
	req, err := c.inner.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating dependency review request: %w", err)
	}

	var raw json.RawMessage
	_, err = c.inner.Do(ctx, req, &raw)
	if err != nil {
		return nil, fmt.Errorf("dependency review API: %w", err)
	}

	var entries []DepReviewEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parsing dependency review response: %w", err)
	}

	return entries, nil
}

// DepReviewToPackages converts dependency review entries to PackageInfo with vulnerability enrichment.
func DepReviewToPackages(entries []DepReviewEntry) []types.PackageInfo {
	var pkgs []types.PackageInfo
	for _, e := range entries {
		if e.ChangeType == "removed" {
			continue
		}
		pkgs = append(pkgs, types.PackageInfo{
			Name:      e.Name,
			Ecosystem: e.Ecosystem,
		})
	}
	return pkgs
}

// DepReviewVulnerabilities extracts advisories from dependency review entries.
func DepReviewVulnerabilities(entries []DepReviewEntry) map[string][]types.Advisory {
	result := make(map[string][]types.Advisory)
	for _, e := range entries {
		for _, v := range e.Vulnerabilities {
			result[e.Name] = append(result[e.Name], types.Advisory{
				GHSAID:   v.AdvisoryGHSAID,
				Severity: v.Severity,
			})
		}
	}
	return result
}
