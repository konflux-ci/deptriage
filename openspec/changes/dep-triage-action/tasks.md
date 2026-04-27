## 1. Project Scaffold

- [x] 1.1 Initialize Go module (`go mod init github.com/konflux-ci/dep-impact-analysis-action`) and add core dependencies (cobra, google/go-github)
- [x] 1.2 Create project directory structure: `cmd/deptriage/`, `internal/{classify,analyze,github,imports,security,types}/`, `prompt/`, `test/testdata/`
- [x] 1.3 Implement cobra root command with `classify`, `analyze` subcommands and shared flags (--pr-number, --repo, --github-token, --output)
- [x] 1.4 Create `internal/types/types.go` with shared types: `ClassifyResult`, `PackageContext`, `ImportInfo`, `Advisory`, `GovulncheckResult`, `ContextJSON`

## 2. Semver Detection

- [x] 2.1 Implement `internal/classify/semver.go`: port regex patterns for three-component, two-component, and digest-only version detection from bash/Python to Go `regexp`
- [x] 2.2 Implement highest-bump-wins logic across multiple version pairs (major > minor > patch > digest > unknown)
- [x] 2.3 Add unit tests for semver detection with test fixtures covering all scenarios (optional v prefix, backticks, mixed bumps, unknown)

## 3. Package Extraction

- [x] 3.1 Implement `internal/classify/packages.go`: regex extraction of module paths from Renovate markdown tables (bare and linked formats)
- [x] 3.2 Implement PR title fallback when no packages found in body
- [x] 3.3 Implement changelog extraction per package (package-specific section, then fallback to cleaned body) with boilerplate stripping
- [x] 3.4 Add unit tests with real Renovate PR body fixtures (bare, linked, multi-package, empty)

## 4. Dependency Review API

- [x] 4.1 Implement `internal/github/depreview.go`: call GitHub Dependency Review API (`GET /repos/{owner}/{repo}/dependency-graph/compare/{base}...{head}`) and parse structured response
- [x] 4.2 Implement fallback chain: Dependency Review API → PR body regex → PR title
- [x] 4.3 Enrich package data with inline vulnerability info from API response
- [ ] 4.4 Add unit tests with mocked API responses (success, empty, error, timeout)

## 5. Import Analysis

- [x] 5.1 Implement `internal/imports/modtools.go`: `go mod why -m` wrapper with 60s timeout and output parsing
- [x] 5.2 Implement `go mod graph` wrapper for import chain visualization
- [x] 5.3 Implement `internal/imports/scanner.go`: source file scanning with fixed-string grep, excluding test/vendor/hack files
- [x] 5.4 Implement snippet extraction (5 lines of context around each usage) and test file detection (`_test.go` in same directory)
- [x] 5.5 Handle graceful degradation when `go` binary or `go.mod` is not available
- [x] 5.6 Add unit tests for import scanning with test Go source fixtures

## 6. Risk Detection

- [x] 6.1 Implement `internal/classify/risk.go`: pattern matching for GO_TOOLCHAIN_UPDATE, GO_VERSION_BUMP, CONTAINER_IMAGE_UPDATE risk hints
- [x] 6.2 Implement risk hint aggregation into structured string output
- [x] 6.3 Add unit tests covering each risk pattern and the no-match case

## 7. Security Advisories

- [x] 7.1 Implement `internal/security/advisories.go`: query GitHub Global Security Advisories API for each package, parse GHSA ID, CVE, CVSS score, severity, patched versions
- [x] 7.2 Implement `internal/security/govulncheck.go`: subprocess wrapper with 300s timeout, output parsing for reachable vulnerabilities and call chains
- [x] 7.3 Handle graceful degradation when govulncheck is not installed or errors out
- [ ] 7.4 Add unit tests with mocked advisory responses and govulncheck output fixtures

## 8. Classify Orchestrator

- [x] 8.1 Implement `internal/classify/classify.go`: orchestrate the full classification pipeline (semver detection → package extraction → risk detection → label application → auto-merge decision)
- [x] 8.2 Write ClassifyResult JSON output to file (default: `/tmp/deptriage-classify.json`)
- [x] 8.3 Wire classify cobra subcommand to orchestrator with flags (--auto-merge, --output)
- [ ] 8.4 Add integration test for classify pipeline with fixture PR data

## 9. Context Gathering

- [x] 9.1 Implement `internal/analyze/context.go`: assemble structured ContextJSON from classify output + import analysis + security data + risk hints
- [x] 9.2 Implement PR body cleaning (strip Configuration section, renovate-debug comments)
- [x] 9.3 Handle partial context gracefully (empty/null fields for missing data sources)
- [ ] 9.4 Add unit tests for context assembly with various combinations of available/missing data

## 10. LLM Provider Interface

- [x] 10.1 Implement `internal/analyze/provider/provider.go`: LLMProvider interface with `Analyze(ctx, prompt) (string, error)` method
- [x] 10.2 Implement `internal/analyze/provider/gemini.go`: Gemini generateContent API via raw HTTP with 120s timeout
- [x] 10.3 Implement `internal/analyze/provider/claude.go`: Anthropic Messages API via raw HTTP with 120s timeout
- [x] 10.4 Implement provider selection based on `--provider` flag / `LLM_PROVIDER` env var
- [ ] 10.5 Add unit tests with mocked HTTP responses for each provider (success, error, timeout, malformed)

## 11. Prompt & Response Processing

- [x] 11.1 Create `internal/analyze/template.md`: embed the LLM prompt template (ported from tekton-kueue's `dep-impact-prompt.md`) with `{{BUMP_TYPE}}`, `{{PR_TITLE}}`, `{{PACKAGE_CONTEXT}}` placeholders (co-located with prompt.go for go:embed)
- [x] 11.2 Implement `internal/analyze/prompt.go`: template rendering with go:embed, placeholder substitution, context truncation for token limits
- [x] 11.3 Implement risk level extraction from LLM response (`Risk Level: LOW|MEDIUM|HIGH`)
- [x] 11.4 Implement `internal/security/redact.go`: regex-based secret redaction (AWS keys, GitHub tokens, generic API keys, base64 credentials) with `[REDACTED]` replacement
- [x] 11.5 Add unit tests for prompt rendering, risk extraction, and secret redaction

## 12. GitHub PR Operations

- [x] 12.1 Implement `internal/github/client.go`: GitHub API wrapper with authenticated client setup from token
- [x] 12.2 Implement `internal/github/pr.go`: fetch PR metadata (title, body, author, base/head refs, labels)
- [x] 12.3 Implement label operations: create-if-not-exists, apply, remove conflicting labels
- [x] 12.4 Implement comment management: find existing comment by hidden marker, update with history collapse into `<details>` blocks, create new with marker, respect 65KB limit
- [x] 12.5 Implement formal review submission (APPROVE/REQUEST_CHANGES/COMMENT) based on risk level and auto-approve flag
- [ ] 12.6 Implement auto-merge trigger via GitHub API
  - [ ] 12.6a Add `MergePR` method to `internal/github/pr.go`
  - [ ] 12.6b Add `GetCheckStatus` method to `internal/github/pr.go` (combined status + check runs, self-exclusion)
  - [ ] 12.6c Add `auto-merge` input to `action.yml` and wire through CLI flags in `cmd/deptriage/main.go`
  - [ ] 12.6d Integrate merge step at end of `analyze.Run()` (eligibility: labels present + CI green + risk != HIGH)
  - [ ] 12.6e Add `merge` subcommand to `cmd/deptriage/main.go` with `--head-sha` and `--pr-number` flags
  - [ ] 12.6f Create `internal/merge/merge.go` with standalone merge orchestration (reuses github.Client methods)
  - [ ] 12.6g Add `auto-merge.yaml` workflow triggered on `check_suite: completed` using the `merge` subcommand
  - [ ] 12.6h Add unit tests for merge eligibility logic
- [x] 12.7 Add unit tests for comment management (first post, update with collapse, truncation)

## 13. Analyze Orchestrator

- [x] 13.1 Implement `internal/analyze/analyze.go`: orchestrate analysis pipeline (read classify output → gather context → render prompt → call LLM → parse response → redact secrets → post comment → apply labels → submit review)
- [x] 13.2 Implement graceful failure semantics: always exit 0, post fallback comment on any error
- [x] 13.3 Wire analyze cobra subcommand with flags (--provider, --api-key, --model, --auto-approve, --classify-output)
- [x] 13.4 Implement `both` command mode: run classify then analyze in sequence
- [ ] 13.5 Add integration test for analyze pipeline with mocked LLM and GitHub API

## 14. GitHub Action Packaging

- [x] 14.1 Create `action.yml` with inputs (command, pr-number, api-key, llm-provider, llm-model, auto-merge, auto-approve, github-token) and outputs (bump-type, risk-level, context-json)
- [x] 14.2 Create multi-stage `Dockerfile`: UBI 10 go-toolset builder (CGO_ENABLED=0, static binary) → UBI 10 ubi-minimal runtime with govulncheck pre-installed
- [x] 14.3 Binary reads `INPUT_*` env vars as flag defaults and writes to `$GITHUB_OUTPUT` directly (no entrypoint shell script)
- [x] 14.4 Add a sample consumer workflow (`.github/workflows/example-deptriage.yaml`) showing recommended usage

## 15. CI & Release

- [x] 15.1 Create `.github/workflows/test.yaml`: run `go test ./...` on PR and push
- [x] 15.2 Create `.github/workflows/lint.yaml`: run `golangci-lint` on PR
- [x] 15.3 Create `.github/workflows/build.yaml`: build Docker image and verify action runs
- [x] 15.4 Add `Makefile` with targets: build, test, lint, image (auto-detects podman/docker via `CONTAINER_ENGINE`)
