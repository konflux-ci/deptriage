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

This repository doubles as a GitHub Action. The `action.yml` at the repo root
defines a Docker container action that pulls a version-pinned image from
`quay.io/konflux-ci/deptriage`.

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

See `.github/workflows/example-dep-triage.yaml` for a ready-to-copy example.

## Releasing

Every merge to `main` triggers a Konflux build that pushes a new image to
`quay.io/konflux-ci/deptriage:latest`. The `action.yml` references a
version-tagged image (e.g. `deptriage:v0.1.0`), not `latest`, so code changes
don't take effect until a release is cut.

To create a new release:

1. **Verify** that the `latest` image on quay.io contains the changes you want
   to release.
2. **Tag the image** on quay.io with a semver tag:
   ```bash
   skopeo copy \
     docker://quay.io/konflux-ci/deptriage:latest \
     docker://quay.io/konflux-ci/deptriage:v0.2.0
   ```
   Alternatively, add the tag via the Quay UI.
3. **Update `action.yml`** to reference the new tag:
   ```yaml
   image: "docker://quay.io/konflux-ci/deptriage:v0.2.0"
   ```
4. **Merge** the `action.yml` change. The resulting Konflux build produces a new
   `latest` image, but that image is functionally identical (only `action.yml`
   changed), so no infinite loop occurs.

Consumer workflows that reference `konflux-ci/deptriage@main` will pick up the
new image tag immediately after step 4.

## Building

```bash
make build      # Static binary (CGO_ENABLED=0)
make test       # Run tests with race detector
make lint       # Run golangci-lint
make image      # Build container image (auto-detects podman/docker)
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
