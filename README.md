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
- **Security advisories** -- queries GitHub Global Security Advisories API and
  optionally runs `govulncheck` for reachability analysis
- **LLM impact analysis** -- assembles structured context and calls Gemini or
  Claude to produce risk assessments with secret redaction and automatic retry
  with exponential backoff on rate limits (429)
- **PR operations** -- applies labels, posts/updates comments with history
  collapse, submits formal review events (APPROVE/REQUEST_CHANGES/COMMENT),
  and applies auto-approve labels for eligible patches
- **Auto-merge** -- merges eligible PRs via the GitHub API after submitting an
  APPROVE review to satisfy branch rulesets, with deferred approval for patch
  bumps with risk hints once CI passes

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
  classify/            Semver detection, package extraction, risk hints
  analyze/             Context assembly, prompt rendering, LLM providers
  merge/               Auto-merge eligibility, deferred approval, APPROVE + merge
  github/              GitHub API client (labels, comments, reviews, merge, dep review)
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
      github.event.pull_request.user.login == 'red-hat-konflux[bot]'
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
      statuses: read
    steps:
      - uses: actions/checkout@v6
      - uses: actions/create-github-app-token@v1
        id: app-token
        with:
          app-id: ${{ secrets.AUTO_MERGER_APP_ID }}
          private-key: ${{ secrets.AUTO_MERGER_APP_PRIVATE_KEY }}
      - uses: konflux-ci/deptriage@main
        with:
          command: merge
          head-sha: ${{ github.event.check_suite.head_sha }}
          github-token: ${{ steps.app-token.outputs.token }}
```

See `.github/workflows/example-dep-triage.yaml` for a ready-to-copy example.

## Building

```bash
make build      # Static binary (CGO_ENABLED=0)
make test       # Run tests with race detector
make lint       # Run golangci-lint
make image      # Build container image (auto-detects podman/docker)
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
