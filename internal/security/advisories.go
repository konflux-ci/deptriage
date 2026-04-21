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
	"context"
	"fmt"
	"time"

	gh "github.com/google/go-github/v84/github"
	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"
)

const advisoryTimeout = 30 * time.Second

// FetchAdvisories queries GitHub Global Security Advisories for a Go package.
func FetchAdvisories(ctx context.Context, client *gh.Client, pkg string) ([]types.Advisory, error) {
	ctx, cancel := context.WithTimeout(ctx, advisoryTimeout)
	defer cancel()

	ecosystem := "go"
	opts := &gh.ListGlobalSecurityAdvisoriesOptions{
		Ecosystem: &ecosystem,
	}

	advisories, _, err := client.SecurityAdvisories.ListGlobalSecurityAdvisories(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("fetching advisories for %s: %w", pkg, err)
	}

	var results []types.Advisory
	for _, adv := range advisories {
		// Filter for advisories affecting this package
		for _, vuln := range adv.Vulnerabilities {
			if vuln.Package != nil && vuln.Package.Name != nil && *vuln.Package.Name == pkg {
				a := types.Advisory{
					GHSAID:   adv.GetGHSAID(),
					Severity: adv.GetSeverity(),
				}
				if adv.CVEID != nil {
					a.CVE = *adv.CVEID
				}
				if adv.CVSS != nil && adv.CVSS.Score != nil {
					a.CVSSScore = *adv.CVSS.Score
				}
				if vuln.FirstPatchedVersion != nil {
					a.PatchedVersions = *vuln.FirstPatchedVersion
				}
				results = append(results, a)
			}
		}
	}
	return results, nil
}
