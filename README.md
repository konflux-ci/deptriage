# deptriage

Go library and CLI for deterministic dependency PR triage and inspection.

Classifies dependency update PRs by semver bump type, detects risk patterns,
gathers deterministic import and vulnerability evidence, and applies policy
controls for automated dependency updates.

## Features

- **Semver classification** -- detects major, minor, patch, and digest bumps from
  PR content using both ASCII (`->`) and Unicode (`→`) arrows, with support for
  Docker build ID suffixes and ecosystem-aware labeling (Go module pseudo-version
  digests are treated as minor due to lack of semver guarantees)
- **Package extraction** -- parses Renovate/Mintmaker PR bodies (markdown tables,
  linked and bare formats) with fallback to PR title
- **Dependency inspection** -- uses `go mod why`, source scanning, GitHub
  advisories, and `govulncheck` to produce a deterministic JSON report
- **Risk detection** -- pattern-based heuristics for Go toolchain updates, Go
  version bumps, and container image changes
- **Supply-chain hardening** -- validates bot PR author identity against commit
  authors, detects changes to known attack vector paths (`.claude/`, `.vscode/`,
  `.github/workflows/`), and verifies dependency PRs only touch expected files;
  blocks auto-approve, auto-merge, and deferred approval when concerns are found.
  The deferred merge path repeats these checks instead of trusting mutable labels.
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
- `deptriage analyze` -- gathers deterministic import, advisory, and
  `govulncheck` evidence into a local JSON report; performs no GitHub writes
- `deptriage merge` -- evaluates eligible PRs for auto-merge (trusted bot
  provenance, file scope, labels, and CI checks) and merges via the GitHub API;
  always exits 0

```
cmd/deptriage/         CLI entrypoint (cobra)
internal/
  classify/            Semver detection, package extraction, risk hints, supply-chain validation
  analyze/             Deterministic dependency-inspection report assembly
  imports/             Go module usage and source import scanning
  merge/               Auto-merge eligibility, deferred approval, APPROVE + merge
  github/              GitHub API client (labels, comments, reviews, merge, dep review, PR commits/files)
  security/            Advisory lookup and govulncheck integration
  types/               Shared types
```

## Usage

### As a Go binary

```bash
# Build
make build

# Classify a PR
deptriage classify --repo owner/repo --pr-number 42 --github-token $TOKEN

# Classify and apply deterministic auto-approval labels
deptriage classify --repo owner/repo --pr-number 42 --github-token $TOKEN --auto-approve

# Gather deterministic import and vulnerability evidence from a prior classify result
deptriage analyze --classify-output /tmp/deptriage-classify.json \
  --context-output /tmp/deptriage-context.json

# Merge eligible PRs by head SHA (used in check_suite workflows). The PR must
# still point to this SHA when deptriage approves or merges it.
deptriage merge --repo owner/repo --head-sha $SHA --github-token $TOKEN
```

### As a GitHub Action

This repository doubles as a GitHub Action. The `action.yml` at the repo root
defines a Docker container action that pulls the pre-built image from
`quay.io/konflux-ci/deptriage:latest`.

```yaml
# .github/workflows/dep-triage.yaml
name: Dependency Update Triage

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
| `api-key` | | Deprecated and ignored; retained for compatibility |
| `llm-provider` | `gemini` | Deprecated and ignored; retained for compatibility |
| `llm-model` | | Deprecated and ignored; retained for compatibility |
| `context-output` | `$GITHUB_WORKSPACE/deptriage-context.json` | Persistent path for the deterministic inspection report |
| `auto-approve` | `false` | Apply `approved`/`lgtm` labels for eligible patches, minors, and digests without deterministic risk hints |
| `auto-merge` | `false` | Deprecated and ignored; use the separate SHA-bound `merge` command |
| `dry-run` | `false` | Suppress all GitHub API writes; log what would happen |
| `head-sha` | | Head SHA to find PRs for; checks and merge are bound to this SHA |
| `trusted-bots` | | Comma-separated additional trusted bot logins for classification and deferred merge (added to defaults) |
| `suspicious-paths` | | Comma-separated additional suspicious path prefixes for classification and deferred merge |
| `expected-files` | | Comma-separated additional expected file patterns for classification and deferred merge |

The `analyze` command produces deterministic import, advisory, and vulnerability
evidence beneath `GITHUB_WORKSPACE` by default so later workflow steps can read
it. Use `context-output` to choose another path. `context-json` is set only
after that report is written successfully. Analyze never calls an LLM and never
applies labels, reviews, comments, or merges. Its `risk-level` output is always
`unknown`; this output and the former LLM inputs remain only for client
compatibility.

## Supply-Chain Hardening

deptriage includes four supply-chain validators that run automatically during
classification and again before deferred merge. These are always-on with no
flag to disable.

### PR author validation

Verifies that the PR was opened by a recognized dependency bot and that **every
commit** on the PR was authored by the same bot identity. If any commit comes
from a different author, the PR is flagged with a `supply-chain/author-mismatch`
label and blocked from auto-approve and auto-merge.

Default trusted bots: `renovate[bot]`, `red-hat-konflux[bot]`, `dependabot[bot]`.
Add custom bot logins via the `trusted-bots` input. Configure this input on
both the classification and deferred auto-merge action invocations.

### Suspicious file detection

Inspects the list of changed files for known attack vectors:
- `.claude/` -- known malware vector
- `.vscode/` -- known attack vector
- `.github/workflows/` -- CI/CD pipeline tampering
- `.github/actions/` -- CI/CD action tampering
- Executable scripts (`.sh`, `.mjs`, `.js`, `.py`, `.rb`, `.pl`) outside
  vendored directories

Matches trigger the `supply-chain/suspicious-files` label, **except** for pure
GitHub Actions update PRs (where all changed files are under `.github/workflows/`
or `.github/actions/`) from a trusted bot — these are recognized as legitimate
dependency updates. Add custom path prefixes via the `suspicious-paths` input.

### Diff scope validation

Verifies that dependency PRs only touch files expected for a legitimate
dependency update (manifests, lock files, vendored code, Tekton task refs).
Changes outside the expected scope trigger `supply-chain/unexpected-scope`.

Default expected patterns include `go.mod`, `go.sum`, `Dockerfile`,
`Containerfile`, `vendor/`, `.tekton/`, `.gitmodules`, `renovate.json`, and
common package manager manifests. For pure GitHub Actions update PRs (where all
changed files are under `.github/workflows/` or `.github/actions/`), those
prefixes are automatically added to the expected patterns. Add custom patterns
via the `expected-files` input.

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
- The merge phase rejects PRs with any `supply-chain/*` label and independently
  revalidates trusted-bot authorship, commit authors, changed-file scope,
  suspicious paths, and submodule changes before deferred approval or merge
- API errors during this merge-time validation are **fail-closed**. Human PRs
  and untrusted bots are explicitly ineligible for deptriage deferred merge
- For a `head-sha` invocation, the PR is rechecked before approval and merge;
  GitHub receives that SHA in the merge request, preventing a later push from
  being merged using earlier CI results
- If a supply-chain label cannot be applied (API error, permissions), any
  existing `approved`/`lgtm` labels are removed as a fallback to prevent
  stale approval from letting a tampered PR merge
- All checks operate on the PR metadata and file list, not file contents --
  content-based scanning is a non-goal

## Building

```bash
make build      # Static binary (CGO_ENABLED=0)
make test       # Run tests with race detector
make lint       # Run golangci-lint
make image      # Build container image (auto-detects podman/docker)
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
