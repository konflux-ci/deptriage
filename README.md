# deptriage

Go library and CLI for dependency PR triage and AI-assisted impact analysis.

Classifies dependency update PRs by semver bump type, detects risk patterns,
gathers code-level usage context, and optionally runs LLM-based impact analysis
to help reviewers prioritize their work.

## Features

- **Semver classification** -- detects major, minor, patch, and digest bumps from
  PR content with ecosystem-aware labeling (Go module pseudo-version digests are
  treated as minor due to lack of semver guarantees)
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
  Claude to produce risk assessments with secret redaction
- **PR operations** -- applies labels, posts/updates comments with history
  collapse, submits formal review events (APPROVE/REQUEST_CHANGES/COMMENT),
  and applies auto-approve labels for eligible patches

## Architecture

The project is a standalone Go module with two subcommands:

- `deptriage classify` -- runs the classification pipeline (semver detection,
  package extraction, risk hints, label application); can fail the workflow
- `deptriage analyze` -- runs the analysis pipeline (context gathering, LLM
  call, comment posting, review submission); always exits 0 to avoid blocking CI

A `both` subcommand runs classify then analyze in sequence.

```
cmd/deptriage/         CLI entrypoint (cobra)
internal/
  classify/            Semver detection, package extraction, risk hints
  analyze/             Context assembly, prompt rendering, LLM providers
  github/              GitHub API client (labels, comments, reviews, dep review)
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

# Run full analysis
deptriage both --repo owner/repo --pr-number 42 --github-token $TOKEN \
  --api-key $GEMINI_API_KEY --provider gemini --auto-approve
```

### As a GitHub Action

This repository contains the Go source code. For GitHub Action packaging, see
[konflux-ci/deptriage-action](https://github.com/konflux-ci/deptriage-action)
(planned), which wraps the binary in a Docker container action.

### As a GitLab component (planned)

The Go binary can also be invoked from GitLab CI pipelines. A GitLab CI
component repository is planned for direct integration.

## Building

```bash
make build      # Static binary (CGO_ENABLED=0)
make test       # Run tests with race detector
make lint       # Run golangci-lint
make image      # Build container image (auto-detects podman/docker)
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
