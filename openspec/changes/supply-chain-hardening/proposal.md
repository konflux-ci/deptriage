## Why

Supply chain attacks targeting GitHub repositories are a growing threat. Attackers can inject malicious commits into automated dependency PRs by exploiting GitHub's collapsed-diff behavior to hide payloads in files that reviewers are unlikely to expand. Known attack vectors include `.claude/` and `.vscode/` directories containing malicious scripts (e.g., `node .claude/setup.mjs`), and tampered automated PRs where the latest commit comes from an identity other than the bot that opened the PR.

With deptriage's auto-approve and auto-merge capabilities, a tampered Renovate/MintMaker PR could bypass human review entirely if it passes CI checks and semver classification.

## What Changes

- New **PR author validation** step in the classify pipeline that verifies the PR was opened by a known dependency bot and that the most recent commit author matches the PR opener. PRs failing validation are flagged as HIGH risk and blocked from auto-approve/auto-merge.
- New **suspicious file detection** step that inspects the PR's changed files for known attack vectors (`.claude/`, `.vscode/`, CI/CD config, executable scripts) and blocks auto-approve/auto-merge when found.
- New **diff scope validation** step that verifies dependency PRs only touch expected files (dependency manifests like `go.mod`, `go.sum`, `Dockerfile`, `Containerfile`, task refs). Changes outside the expected scope block auto-approve/auto-merge and raise a supply-chain risk hint.
- Configurable blocklist and allowlist patterns so new attack vectors can be added without code changes.
- New `supply-chain/*` risk-hint labels to surface these findings on the PR.

## Capabilities

### New Capabilities

- `pr-author-validation`: Validate that the PR was opened by a recognized dependency bot (`renovate[bot]`, `red-hat-konflux[bot]`, `dependabot[bot]`) and that the most recent commit on the PR branch was authored by the same bot identity. Flag mismatches as HIGH risk with a `supply-chain/author-mismatch` label.
- `suspicious-file-detection`: Inspect the list of files changed in the PR for known malicious patterns (`.claude/`, `.vscode/`, `.github/workflows/`, executable scripts). Block auto-approve/auto-merge and apply `supply-chain/suspicious-files` label when found.
- `diff-scope-validation`: Verify that the files changed in the PR are limited to what a legitimate dependency update would touch. Changes outside the expected scope (dependency manifests, lock files, vendored code) block auto-approve/auto-merge and apply `supply-chain/unexpected-scope` label.

### Modified Capabilities

- `classify` pipeline: Adds the three new validation steps before auto-approve decision. All supply-chain checks run after semver detection and package extraction but before label application and auto-approve.
- `risk-detection`: Extended with supply-chain risk hint types alongside existing Go toolchain/container image hints.
- `auto-merge`: Supply-chain risk hints block auto-merge via the existing `risk-hint/*` mechanism — patches/minors with supply-chain risk hints are NOT eligible for deferred approval (unlike Go toolchain hints where passing CI proves safety).

## Impact

- **Modified files:** `internal/classify/classify.go` (pipeline orchestration), `internal/classify/risk.go` (new risk hint types), `internal/github/pr.go` (new methods to fetch commit authors and changed files), `internal/types/types.go` (new constants)
- **New files:** `internal/classify/supply_chain.go` (author validation, file detection, scope validation logic)
- **External APIs:** GitHub Commits API (list commits on PR), GitHub Pull Request Files API (list changed files)
- **No new dependencies** — uses existing `google/go-github` client
