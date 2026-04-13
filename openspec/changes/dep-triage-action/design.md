## Context

Dependency PRs from Renovate/Mintmaker are the highest-volume PR type across Konflux repos. The current triage system — bash scripts, embedded Python, and GitHub workflows — lives in `tekton-kueue` and cannot be shared. This design covers a standalone Go binary + GitHub Action that any repo can adopt.

The reference implementation consists of:
- `detect-semver-bump.sh` — regex-based version parsing (embedded Python3)
- `dep-imports.sh` — fixed-string grep over Go source for import paths (superseded by `go mod why`/`go mod graph` approach)
- `gather-dep-context.sh` — PR body parsing, changelog extraction, snippet gathering, risk hint detection, JSON assembly via temp files + Python3
- `dep-impact-prompt.md` — LLM prompt template with `{{BUMP_TYPE}}`, `{{PR_TITLE}}`, `{{PACKAGE_CONTEXT}}` placeholders
- Three GitHub workflows orchestrating labels, comments, API calls, and auto-merge

## Goals / Non-Goals

**Goals:**
- Single Go binary (`deptriage`) with `classify` and `analyze` subcommands
- Reusable across any Konflux repo via a single `action.yml` reference
- Feature parity with all existing bash scripts and workflows, plus enhancements from cicaddy-action patterns
- Pluggable LLM provider (Gemini, Claude) via a common interface
- Graceful degradation: `analyze` never blocks CI, missing API keys are handled cleanly
- Multi-source risk data: dependency review API, security advisories, `govulncheck` reachability
- Secret redaction on LLM output before posting to GitHub
- Structured, testable code with clear separation of concerns

**Non-Goals:**
- Supporting non-Go repositories (import analysis is Go-specific)
- Building a general-purpose dependency analysis framework
- Real-time monitoring or dashboards
- Supporting LLM providers beyond Gemini and Claude in the initial release
- Renovate/Mintmaker configuration management (that stays in each consumer repo)

## Decisions

### 1. Single binary with two subcommands (not two binaries or a library)

`deptriage classify` and `deptriage analyze` share types, GitHub client, and config parsing. Two subcommands keep the build simple (one `go build`, one container image) while maintaining distinct failure semantics: `classify` can fail the workflow; `analyze` always exits 0.

**Alternatives considered:**
- Two separate binaries — doubles build/release complexity for no benefit
- Single command with flags — loses the clear semantic separation between "must succeed" and "best effort"

### 2. CLI framework: `cobra`

Cobra is the de facto standard for Go CLIs, provides subcommand routing, flag parsing, help generation, and env-var binding. Minimal overhead.

**Alternatives considered:**
- `urfave/cli` — capable but less ecosystem adoption
- Raw `flag` package — too much boilerplate for two subcommands with shared flags

### 3. GitHub API client: `google/go-github`

Well-maintained, typed Go client for the GitHub REST API. Covers all needed operations: labels, comments, PR merge, file content.

**Alternatives considered:**
- Raw HTTP — too much boilerplate for paginated APIs, error handling, auth
- `gh` CLI — not appropriate for a library; shelling out adds fragility

### 4. LLM provider abstraction via Go interface

```go
type LLMProvider interface {
    Analyze(ctx context.Context, prompt string) (string, error)
}
```

Each provider (Gemini, Claude) implements this interface. The `analyze` command selects the provider based on a `--provider` flag or `LLM_PROVIDER` env var. Prompt is assembled from a Go-embedded template (no external file dependency).

**Alternatives considered:**
- Provider-specific SDKs — Gemini has no official Go SDK; Claude's Go SDK is thin. Raw HTTP with a common interface is simpler and avoids SDK version churn.
- Hardcoded Gemini only — limits adoption and doesn't align with Konflux's multi-cloud approach

### 5. Container action (not composite)

Ship the pre-compiled binary in a Docker image. The `action.yml` uses `runs: using: docker` to guarantee a consistent environment. The Dockerfile uses a multi-stage build: UBI 10 go-toolset builder → UBI 10 ubi-minimal runtime. The binary reads `INPUT_*` env vars directly as flag defaults and writes to `$GITHUB_OUTPUT`, eliminating the need for an entrypoint shell script.

The Makefile auto-detects the container engine (`podman` preferred, `docker` fallback) via `CONTAINER_ENGINE` variable, allowing developers to build with any OCI-compatible tool.

**Alternatives considered:**
- Composite action with `actions/setup-go` + `go run` — slower (compiles on every run), requires Go on the runner
- Pre-built binary with `runs: using: node` wrapper — adds Node.js dependency for no reason
- Shell entrypoint script — adds unnecessary layer; the Go binary handles env var defaults and GITHUB_OUTPUT natively
- Distroless base image — UBI 10 ubi-minimal is preferred for Red Hat ecosystem compatibility and support

### 6. Package discovery: Dependency Review API + PR body fallback

Primary source: GitHub's Dependency Review API (`GET /repos/{owner}/{repo}/dependency-review/{base}...{head}`) returns structured data — package names, old/new versions, ecosystem, and inline vulnerability info. This is more reliable than regex over Renovate markdown tables.

Fallback: when the Dependency Review API returns no results (e.g., non-manifest changes, private repos without dependency graph enabled), fall back to regex extraction from the PR body using the same pattern as the bash scripts: strip markdown links, then match module paths in pipe-delimited table rows. Final fallback to PR title.

**Alternatives considered:**
- PR body regex only — fragile, tightly coupled to Renovate's markdown format
- Renovate API — no stable API for extracting package metadata from a PR

### 7. Data flow: classify outputs JSON, analyze consumes it

`classify` writes a structured JSON result to a file (default: `/tmp/deptriage-classify.json`). When running `both` commands, `analyze` reads this file. In the GitHub Action, the file path is passed between steps via `GITHUB_OUTPUT`.

```
PR event → classify → classify-result.json → analyze → comment + labels
```

This keeps the subcommands independently testable and allows `classify` output to be consumed by other tools.

### 8. Project layout

```
.
├── action.yml                  # GitHub Action definition
├── Dockerfile                  # Multi-stage build
├── cmd/
│   └── deptriage/
│       └── main.go             # Entrypoint, cobra root command
├── internal/
│   ├── classify/
│   │   ├── classify.go         # Orchestrates classification pipeline
│   │   ├── semver.go           # Semver bump detection
│   │   ├── packages.go         # Package extraction from PR body
│   │   └── risk.go             # Risk hint detection
│   ├── analyze/
│   │   ├── analyze.go          # Orchestrates analysis pipeline
│   │   ├── context.go          # Context JSON assembly
│   │   ├── prompt.go           # Prompt template + rendering
│   │   ├── template.md         # LLM prompt template (embedded via go:embed, co-located with prompt.go)
│   │   └── provider/
│   │       ├── provider.go     # LLMProvider interface
│   │       ├── gemini.go       # Gemini implementation
│   │       └── claude.go       # Claude implementation
│   ├── github/
│   │   ├── client.go           # GitHub API wrapper (labels, comments, merge)
│   │   └── pr.go               # PR data fetching
│   ├── imports/
│   │   ├── scanner.go          # Go source file scanning + snippet extraction
│   │   └── modtools.go         # go mod why / go mod graph wrappers
│   ├── security/
│   │   ├── advisories.go       # GitHub Security Advisories API client
│   │   ├── govulncheck.go      # govulncheck subprocess wrapper
│   │   └── redact.go           # Secret redaction from LLM output
│   └── types/
│       └── types.go            # Shared types (ClassifyResult, PackageContext, etc.)
└── test/
    └── testdata/               # Fixture PR bodies, expected outputs
```

### 9. Semver detection: port regex to Go `regexp`

The bash script uses Python3 for regex with named groups. Go's `regexp` (RE2) doesn't support lookaheads, so we use explicit capture groups:

- Three-component: `v?(\d+)\.(\d+)\.(\d+)` matched pairwise with `->` separator
- Two-component: `v?(\d+)\.(\d+)` for Docker-style tags
- Digest: `([0-9a-f]{7,})` pair for commit hash changes

The highest bump across all pairs wins (major > minor > patch > digest > unknown).

**Ecosystem-aware digest labeling:** Digest bumps are labeled differently based on ecosystem. Go module digests (pseudo-versions like `v0.0.0-20250910...`) are labeled `semver/minor` because they have no semver guarantees — breaking API changes have been observed in `k8s.io` pseudo-version bumps. Non-Go-module digests (container images, Tekton task refs) are labeled `semver/patch`. Ecosystem is determined from the Dependency Review API `ecosystem` field, or via domain heuristics (`github.com/`, `k8s.io/`, `golang.org/`, `go.uber.org/`, `gopkg.in/`) when falling back to PR body parsing.

### 10. Comment management: update with history collapse (not delete-and-recreate)

Instead of deleting previous analysis comments, find the existing bot comment (identified by a hidden HTML marker) and update it. The previous analysis is collapsed into a `<details>` block, preserving audit trail without cluttering the PR. Respects GitHub's 65KB comment limit with automatic truncation.

For first-time comments, create with a hidden marker (`<!-- deptriage-analysis -->`) for later identification.

**Alternatives considered:**
- Delete and recreate — loses history, causes notification spam
- Always create new — clutters PR with multiple bot comments

### 11. Import analysis: `go mod why` + `go mod graph` (primary), source grep (supplementary)

Use `go mod why -m <pkg>` to determine if a dependency is actually used (direct or transitive) and `go mod graph` to show the import chain. This is more accurate than grepping for import strings and handles transitive dependencies properly. Source file scanning (`grep -rnF`) remains as a supplementary step to extract usage snippets and detect test files for the LLM context.

**Alternatives considered:**
- Source grep only (current bash approach) — misses transitive deps, false positives on commented-out imports
- `go mod why` only — accurate for reachability but doesn't provide source-level usage context for the LLM

### 12. Security data: GitHub Advisories API + optional `govulncheck`

Query the GitHub Global Security Advisories API for each updated package to surface known CVEs, GHSA IDs, CVSS scores, and patched versions. Feed this into the LLM context alongside changelog data.

Optionally run `govulncheck` (if installed in the container image) for reachability analysis — determines whether vulnerable functions are actually called in the codebase. This dramatically reduces false positives. Gracefully skipped if not available, with a 300-second timeout.

**Alternatives considered:**
- Skip security data entirely — misses a critical risk signal
- `govulncheck` only — doesn't cover advisories for packages outside the Go vulnerability DB
- `osv-scanner` — broader coverage but heavier dependency and slower

### 13. Secret redaction before posting

Scan all LLM output for potential secrets (API keys, tokens, credentials) using regex-based pattern matching before posting to GitHub. Replace findings with `[REDACTED]` markers. This prevents the LLM from accidentally echoing sensitive context (e.g., from code snippets or changelogs) into public PR comments.

**Alternatives considered:**
- `detect-secrets` library — Python-only, would require a heavy dependency in Go
- No redaction — too risky; LLMs can hallucinate or echo sensitive input

### 14. Formal PR review events

When risk is LOW and all checks pass, submit an `APPROVE` review event (not just a comment). When risk is HIGH, submit a `REQUEST_CHANGES` event to block merge until a human reviews. MEDIUM risk posts a comment only. This integrates with GitHub's branch protection rules and required reviews.

**Alternatives considered:**
- Comments only — doesn't integrate with branch protection
- Always approve — defeats the purpose of risk gating

### 15. Systematic subprocess timeouts

All external calls have explicit timeouts enforced via `context.WithTimeout`:
- GitHub API calls: 30 seconds
- `go mod why` / `go mod graph`: 60 seconds
- LLM API calls: 120 seconds
- `govulncheck`: 300 seconds

Timeouts produce structured errors that are logged and result in graceful degradation (partial context rather than failure).

## Risks / Trade-offs

**[Renovate body format changes]** → The regex-based parsing is tightly coupled to Renovate's current markdown table format. Mitigation: the same approach has been stable in tekton-kueue for months; if Renovate changes format, the fallback to PR title still works. Add integration tests with real PR body fixtures.

**[LLM API reliability]** → External API calls can fail, timeout, or return malformed responses. Mitigation: `analyze` always exits 0; failures produce a "analysis unavailable" comment instead of blocking the PR. 120-second timeout on API calls.

**[Go import analysis scope]** → The import scanner only works for Go repos. Mitigation: this is a known non-goal. The scanner gracefully returns empty results if no `.go` files are found, and the LLM analysis still works from changelog alone.

**[Container action cold start]** → Docker-based actions have slower startup than composite actions (~5-10s to pull the image). Mitigation: the image is small (UBI 10 ubi-minimal base, static binary ~15MB). GitHub caches action images within a workflow run.

**[Risk hint false positives]** → Pattern-based heuristics (e.g., matching "docker" in PR title) may flag non-risky changes. Mitigation: risk hints are advisory input to the LLM, not hard gates. The LLM can downgrade risk if the changelog shows a benign change.

**[Dependency Review API availability]** → Requires GitHub dependency graph to be enabled on the repo. May not work on private repos without GitHub Advanced Security. Mitigation: fall back to PR body regex extraction when API returns empty or errors.

**[govulncheck availability and speed]** → Not installed by default; can be slow on large codebases (up to 5 minutes). Mitigation: optional with a 300-second timeout; results are supplementary, not required for analysis. Ship `govulncheck` in the container image so it's available when using the Docker action.

**[Auto-approve via labels]** → Instead of direct auto-merge via the GitHub API, the action applies `approved`/`lgtm` labels for eligible patches. The actual merge is handled by external merge-bot automation (e.g., Konflux merge bot) that acts on these labels after CI passes. This decouples the approval decision from the merge mechanism and ensures PRs with failing tests are never merged. Auto-approve only applies to patches and non-gomod digests (not minor/major), only when no security advisories exist, and is configurable via `--auto-approve` flag (default: off). Gomod digest bumps are excluded because pseudo-versions have no semver guarantees.

**[Formal review auto-approve risk]** → Auto-approving low-risk PRs could let a subtle breaking change through. Mitigation: auto-approve only for patches (not minor/major or gomod digests), and only when no security advisories exist. Configurable via `--auto-approve` flag (default: off).

**[Secret redaction coverage]** → Regex-based redaction may miss novel secret formats. Mitigation: use well-known patterns (AWS keys, GitHub tokens, generic API keys, base64-encoded credentials). Better to over-redact than under-redact; false positives are harmless.
