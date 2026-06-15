# deptriage

Go library and CLI for dependency PR triage and AI-assisted impact analysis.

Classifies dependency update PRs by semver bump type, detects risk patterns,
gathers code-level usage context, and optionally runs LLM-based impact analysis
to help reviewers prioritize their work.

## Features

- **Semver classification** -- detects major, minor, patch, and digest bumps from
  PR content using both ASCII (`->`) and Unicode (`→`) arrows, with support for
  Docker build ID suffixes and ecosystem-aware labeling (Go module pseudo-version
  digests are treated as minor due to lack of semver guarantees)
- **Package extraction** -- parses Renovate/Mintmaker PR bodies (markdown tables,
  linked and bare formats) with fallback to PR title
- **Import analysis** -- uses `go mod why`, `go mod graph`, and source scanning
  to determine how a dependency is used, with snippet extraction and test
  coverage detection
- **Risk detection** -- pattern-based heuristics for Go toolchain updates, Go
  version bumps, and container image changes
- **Supply-chain hardening** -- validates bot PR author identity against commit
  authors, detects changes to known attack vector paths (`.claude/`, `.vscode/`,
  `.github/workflows/`), and verifies dependency PRs only touch expected files;
  blocks auto-approve, auto-merge, and deferred approval when concerns are found
- **Security advisories** -- queries GitHub Global Security Advisories API and
  optionally runs `govulncheck` for reachability analysis
- **LLM impact analysis** -- assembles structured context and calls Gemini or
  Claude to produce risk assessments with secret redaction and automatic retry
  with exponential backoff on rate limits (429)
- **PR operations** -- applies labels, posts/updates comments with history
  collapse, submits formal review events (APPROVE/REQUEST_CHANGES/COMMENT),
  and applies auto-approve labels for eligible patches and minors
- **Auto-merge** -- merges eligible PRs via the GitHub API after submitting an
  APPROVE review to satisfy branch rulesets, with deferred approval for
  patches and minors with risk hints once CI passes, and retry logic for
  in-progress checks
- **Dry-run mode** -- suppresses all GitHub API writes and logs what would
  happen, for testing deptriage on a repo without side effects

## Architecture

The project is a standalone Go module with the following subcommands:

- `deptriage classify` -- runs the classification pipeline (semver detection,
  package extraction, risk hints, label application); can fail the workflow
- `deptriage analyze` -- runs the analysis pipeline (context gathering, LLM
  call, comment posting, review submission); always exits 0 to avoid blocking CI
- `deptriage merge` -- evaluates eligible PRs for auto-merge (labels, CI checks,
  risk level) and merges via the GitHub API; always exits 0

A `both` subcommand runs classify then analyze in sequence, with an optional
inline merge attempt at the end.

```
cmd/deptriage/         CLI entrypoint (cobra)
internal/
  classify/            Semver detection, package extraction, risk hints, supply-chain validation
  analyze/             Context assembly, prompt rendering, LLM providers
  merge/               Auto-merge eligibility, deferred approval, APPROVE + merge
  github/              GitHub API client (labels, comments, reviews, merge, dep review, PR commits/files)
  imports/             go mod tools and source file scanning
  security/            Advisories, govulncheck, secret redaction
  types/               Shared types
```

## Usage

### As a Go binary

```bash
# Build
make build

# Classify a PR
deptriage classify --repo owner/repo --pr-number 42 --github-token $TOKEN

# Run full analysis with auto-merge
deptriage both --repo owner/repo --pr-number 42 --github-token $TOKEN \
  --api-key $GEMINI_API_KEY --provider gemini --auto-approve --auto-merge

# Merge eligible PRs by head SHA (used in check_suite workflows)
deptriage merge --repo owner/repo --head-sha $SHA --github-token $TOKEN
```

### As a GitHub Action

This repository doubles as a GitHub Action. The `action.yml` at the repo root
defines a Docker container action that pulls the pre-built image from
`quay.io/konflux-ci/deptriage:latest`.

```yaml
# .github/workflows/dep-triage.yaml
name: Dependency Impact Analysis

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  triage:
    name: Triage dependency PR
    if: >-
      github.event.pull_request.user.login == 'renovate[bot]' ||
      github.event.pull_request.user.login == 'red-hat-konflux[bot]' ||
      github.event.pull_request.user.login == 'dependabot[bot]'
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
      issues: write
    steps:
      - uses: actions/checkout@v6
      - uses: konflux-ci/deptriage@main
        with:
          command: both
          pr-number: ${{ github.event.pull_request.number }}
          api-key: ${{ secrets.GEMINI_API_KEY }}
          llm-provider: gemini
          auto-approve: 'true'
```

To enable auto-merge after all CI checks pass, add a second workflow triggered
on `check_suite: completed`. This workflow uses a GitHub App token so the
APPROVE review comes from a different identity than the PR pusher, satisfying
branch rulesets that require "approval from someone other than the last pusher."

```yaml
# .github/workflows/auto-merge.yaml
name: Auto-merge approved dependency PRs

on:
  check_suite:
    types: [completed]

jobs:
  auto-merge:
    name: Merge if all checks pass
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
      checks: read
      statuses: read   # optional — if absent, legacy commit status check is skipped gracefully
    steps:
      - uses: actions/checkout@v6
      - uses: actions/create-github-app-token@v3
        id: app-token
        with:
          client-id: ${{ secrets.AUTO_MERGER_APP_ID }}
          private-key: ${{ secrets.AUTO_MERGER_APP_PRIVATE_KEY }}
      - uses: konflux-ci/deptriage@main
        with:
          command: merge
          head-sha: ${{ github.event.check_suite.head_sha }}
          github-token: ${{ steps.app-token.outputs.token }}
```

See `.github/workflows/example-dep-triage-and-auto-merge.yaml` for a ready-to-copy example.

### Action inputs

| Input | Default | Description |
|-------|---------|-------------|
| `command` | `both` | Command to run: `classify`, `analyze`, `both`, or `merge` |
| `pr-number` | `0` | Pull request number |
| `github-token` | `${{ github.token }}` | GitHub token for API operations |
| `api-key` | | LLM provider API key (required for `analyze`) |
| `llm-provider` | `gemini` | LLM provider: `gemini` or `claude` |
| `llm-model` | | LLM model name (provider-dependent default) |
| `auto-approve` | `false` | Apply `approved`/`lgtm` labels for eligible low-risk patches and minors |
| `auto-merge` | `false` | Merge eligible PRs after analysis (requires `auto-approve`) |
| `dry-run` | `false` | Suppress all GitHub API writes; log what would happen |
| `head-sha` | | Head SHA to find PRs for (used by `merge` with `check_suite` trigger) |
| `trusted-bots` | | Comma-separated additional trusted bot logins (added to defaults) |
| `suspicious-paths` | | Comma-separated additional suspicious path prefixes (added to defaults) |
| `expected-files` | | Comma-separated additional expected file patterns for scope validation (added to defaults) |

## Supply-Chain Hardening

deptriage includes four supply-chain validators that run automatically during
classification. These are always-on with no flag to disable.

### PR author validation

Verifies that the PR was opened by a recognized dependency bot and that **every
commit** on the PR was authored by the same bot identity. If any commit comes
from a different author, the PR is flagged with a `supply-chain/author-mismatch`
label and blocked from auto-approve and auto-merge.

Default trusted bots: `renovate[bot]`, `red-hat-konflux[bot]`, `dependabot[bot]`.
Add custom bot logins via the `trusted-bots` input.

### Suspicious file detection

Inspects the list of changed files for known attack vectors:
- `.claude/` -- known malware vector
- `.vscode/` -- known attack vector
- `.github/workflows/` -- CI/CD pipeline tampering
- `.github/actions/` -- CI/CD action tampering
- Executable scripts (`.sh`, `.mjs`, `.js`, `.py`, `.rb`, `.pl`) outside
  vendored directories

Matches trigger the `supply-chain/suspicious-files` label. Add custom path
prefixes via the `suspicious-paths` input.

### Diff scope validation

Verifies that dependency PRs only touch files expected for a legitimate
dependency update (manifests, lock files, vendored code, Tekton task refs).
Changes outside the expected scope trigger `supply-chain/unexpected-scope`.

Default expected patterns include `go.mod`, `go.sum`, `Dockerfile`,
`Containerfile`, `vendor/`, `.tekton/`, `.gitmodules`, `renovate.json`, and
common package manager manifests. Add custom patterns via the `expected-files`
input.

### Submodule update detection

When a dependency PR modifies `.gitmodules`, deptriage fetches the repository
tree to identify git submodule paths (entries with mode `160000`). Changed
submodule pointers are excluded from diff scope validation (they are not
flagged as `supply-chain/unexpected-scope`), but a
`supply-chain/submodule-update` label is applied instead. This label still
blocks auto-approve and auto-merge -- submodule updates bring in entire
upstream codebases where passing CI alone does not guarantee safety, so an
engineer must review the upstream changes.

### Behavior

- Supply-chain labels are **red** (`#e11d48`) to distinguish from yellow
  `risk-hint/*` labels, except `supply-chain/submodule-update` which is
  **yellow** (`#fbca04`) since it is a caution rather than an attack indicator
- Any supply-chain finding blocks auto-approve in the classify phase
- The analyze phase skips formal `APPROVE` reviews when supply-chain findings
  exist, even if the LLM assesses LOW risk
- The merge phase rejects PRs with any `supply-chain/*` label, on both the
  primary merge path and the deferred-approval path
- API errors during commit or file fetch are **fail-closed** for bot PRs --
  the PR is treated as having a supply-chain concern and auto-approve is
  blocked. Human PRs are unaffected since they have no auto-merge path
- If a supply-chain label cannot be applied (API error, permissions), any
  existing `approved`/`lgtm` labels are removed as a fallback to prevent
  stale approval from letting a tampered PR merge
- All checks operate on the PR metadata and file list, not file contents --
  content-based scanning is a non-goal
- Running `deptriage analyze` standalone (without a prior `classify` step)
  skips supply-chain checks -- always use `both` or run `classify` first to
  ensure tamper protection is active

## Building

```bash
make build      # Static binary (CGO_ENABLED=0)
make test       # Run tests with race detector
make lint       # Run golangci-lint
make image      # Build container image (auto-detects podman/docker)
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
