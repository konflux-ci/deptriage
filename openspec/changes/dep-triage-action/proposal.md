## Why

Dependency PRs (from Renovate/Mintmaker) are the highest-volume PR category in Konflux repos, yet most are low-risk patches that consume reviewer time. The current triage system lives as scattered bash/python scripts and GitHub workflows inside `tekton-kueue`, making it impossible to reuse across repos. We need a standalone, reusable GitHub Action backed by a compiled Go binary that any Konflux repo can adopt with a single workflow reference.

## What Changes

- New Go binary `deptriage` with two subcommands:
  - `classify` — detects semver bump type, applies labels, extracts packages, detects risk patterns, and auto-merges eligible patches
  - `analyze` — gathers changelogs and code context, calls an LLM API for impact assessment, posts risk comments and labels
- New GitHub Action definition (`action.yml`) wrapping the binary as a Docker container action using UBI 10 images, with inputs for command selection, PR number, LLM config, and policy overrides
- Replaces bash scripts (`detect-semver-bump.sh`, `dep-imports.sh`, `gather-dep-context.sh`) and their associated workflows from tekton-kueue
- LLM provider abstraction supporting Gemini and Claude (extensible to others)

## Capabilities

### New Capabilities

- `semver-detection`: Parse PR title/body to determine semver bump type (major, minor, patch, digest, unknown) and apply corresponding labels
- `package-extraction`: Extract dependency package names from Renovate/Mintmaker PR body markdown tables (bare and linked formats), with fallback to PR title
- `import-analysis`: Analyze Go dependency usage via `go mod why`/`go mod graph` for import chain verification, plus source file scanning for usage snippets and test file detection
- `risk-detection`: Pattern-based heuristics to flag high-risk changes (Go toolchain updates, Go version bumps, container image changes) as structured risk hints
- `dependency-review`: Use GitHub's Dependency Review API to get structured diff data (package names, versions, ecosystems, inline vulnerabilities) as a reliable alternative to PR body parsing
- `security-advisories`: Query GitHub Global Security Advisories API for GHSA IDs, CVSS scores, severity, and patched versions; optionally run `govulncheck` for vulnerability reachability analysis
- `context-gathering`: Assemble structured JSON context combining package info, changelogs, import chains, security data, usage snippets, and risk hints for LLM consumption
- `llm-analysis`: Call LLM APIs (Gemini, Claude) with a templated prompt to produce risk assessments; parse response for risk level and structured markdown output; redact secrets from LLM output before posting
- `pr-actions`: GitHub PR operations — apply/remove labels, post/update comments with history collapse, submit formal review events (APPROVE/REQUEST_CHANGES), trigger auto-merge — using the GitHub API
- `action-interface`: GitHub Action definition (action.yml) with inputs, outputs, and composite/container run configuration

### Modified Capabilities

_(none — this is a greenfield repo)_

## Impact

- **New repo:** `konflux-ci/dep-impact-analysis-action` — all code is new
- **Dependencies:** Go standard library, `google/go-github` (GitHub API), `cobra` (CLI), LLM provider SDKs or raw HTTP; optional runtime dependency on `govulncheck`
- **Downstream consumers:** Any Konflux repo currently using the tekton-kueue bash scripts will migrate to referencing this action
- **External APIs:** GitHub API (labels, comments, merge, dependency review, security advisories), Gemini API, Anthropic API
- **CI:** GitHub Actions workflows for testing, building, and releasing the binary
