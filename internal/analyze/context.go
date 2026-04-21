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

package analyze

import (
	"context"
	"encoding/json"
	"log/slog"

	ghclient "github.com/konflux-ci/dep-impact-analysis-action/internal/github"
	"github.com/konflux-ci/dep-impact-analysis-action/internal/imports"
	"github.com/konflux-ci/dep-impact-analysis-action/internal/security"
	"github.com/konflux-ci/dep-impact-analysis-action/internal/types"

	gh "github.com/google/go-github/v85/github"
)

// GatherContext assembles the full context JSON for LLM consumption.
func GatherContext(ctx context.Context, result *types.ClassifyResult, ghClient *ghclient.Client, rawGHClient *gh.Client, workDir string) *types.ContextJSON {
	ctxJSON := &types.ContextJSON{
		PRBody:    result.PRBody,
		RiskHints: result.RiskHints,
	}

	goAvailable := imports.GoAvailable(workDir)

	for _, pkg := range result.Packages {
		pkgCtx := types.PackageContext{
			Name:      pkg.Name,
			Changelog: pkg.Changelog,
		}

		// Import chain via go mod why
		if goAvailable {
			chain, err := imports.ModWhy(ctx, pkg.Name, workDir)
			if err != nil {
				slog.Warn("go mod why failed", "package", pkg.Name, "error", err)
			} else if chain == "" {
				pkgCtx.NoDirectImports = true
			} else {
				pkgCtx.ImportChain = chain
			}
		}

		// Source file scanning
		if goAvailable {
			importInfos, err := imports.ScanImports(workDir, pkg.Name)
			if err != nil {
				slog.Warn("import scanning failed", "package", pkg.Name, "error", err)
			}
			if len(importInfos) == 0 {
				pkgCtx.NoDirectImports = true
			} else {
				pkgCtx.Imports = importInfos
			}
		}

		// Security advisories
		if rawGHClient != nil {
			advisories, err := security.FetchAdvisories(ctx, rawGHClient, pkg.Name)
			if err != nil {
				slog.Warn("advisory fetch failed", "package", pkg.Name, "error", err)
			}
			pkgCtx.Advisories = advisories
		}

		ctxJSON.Packages = append(ctxJSON.Packages, pkgCtx)
	}

	// Govulncheck (runs once for the whole repo, not per package)
	if goAvailable {
		vulnResult, err := security.RunGovulncheck(ctx, workDir)
		if err != nil {
			slog.Warn("govulncheck failed", "error", err)
		}
		if vulnResult != nil {
			// Attach to relevant packages
			for i := range ctxJSON.Packages {
				ctxJSON.Packages[i].Govulncheck = vulnResult
			}
		}
	}

	return ctxJSON
}

// ContextToJSON serializes the context to a JSON string.
func ContextToJSON(ctx *types.ContextJSON) (string, error) {
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
