## ADDED Requirements

### Requirement: Validate dependency PR diff scope
The system SHALL verify that all files changed in a dependency PR match expected patterns for a legitimate dependency update. Files outside the expected scope block auto-approve and trigger a `supply-chain/unexpected-scope` label. This check only applies to PRs opened by a trusted bot (author validation must pass first).

Default expected file patterns:
- `go.mod`, `go.sum` — Go dependency manifests
- `Dockerfile`, `Containerfile`, `*.Dockerfile` — container image references
- `vendor/**` — vendored dependencies
- `.tekton/**` — Tekton task/pipeline refs
- `renovate.json`, `.renovaterc`, `.renovaterc.json` — Renovate config
- `package.json`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml` — Node.js manifests
- `requirements.txt`, `poetry.lock`, `Pipfile.lock` — Python manifests
- `Gemfile`, `Gemfile.lock` — Ruby manifests
- `Cargo.toml`, `Cargo.lock` — Rust manifests
- `.gitmodules` — git submodule configuration

#### Scenario: PR changes only go.mod and go.sum
- **WHEN** the PR changes only `go.mod` and `go.sum`
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label
- **AND** the PR SHALL remain eligible for auto-approve

#### Scenario: PR changes go.mod, go.sum, and vendor/
- **WHEN** the PR changes `go.mod`, `go.sum`, and files under `vendor/`
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label
- **RATIONALE:** Vendored dependency updates are expected alongside manifest changes.

#### Scenario: PR changes go.mod and a Go source file
- **WHEN** the PR changes `go.mod`, `go.sum`, and `internal/foo/bar.go`
- **THEN** the system SHALL apply the `supply-chain/unexpected-scope` label with color `e11d48` (red)
- **AND** the system SHALL add a supply-chain risk hint: `SUPPLY_CHAIN_UNEXPECTED_SCOPE`
- **AND** the system SHALL block auto-approve
- **AND** the system SHALL log a warning listing the unexpected file paths

#### Scenario: PR changes Tekton task refs
- **WHEN** the PR changes `.tekton/pull-request.yaml` with a digest bump in a `uses:` or `ref:` field
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label
- **RATIONALE:** Tekton task digest bumps via Renovate are expected dependency updates.

#### Scenario: PR changes Dockerfile
- **WHEN** the PR changes `Containerfile` with a base image version bump
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label

#### Scenario: Custom expected file pattern
- **WHEN** the user specifies `--expected-file=Chart.yaml`
- **THEN** the system SHALL add `Chart.yaml` to the expected file list alongside the defaults
- **AND** a PR changing only `Chart.yaml` SHALL NOT trigger the scope validation label

#### Scenario: PR changes only GitHub Actions workflow files
- **WHEN** all changed files in the PR are under `.github/workflows/` and/or `.github/actions/`
- **AND** the PR is opened by a trusted bot
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label
- **RATIONALE:** GitHub Actions are legitimate dependencies managed by renovate/dependabot. Workflow and action files are the expected manifests for these updates.

#### Scenario: Non-bot PR skips scope validation
- **WHEN** the PR is opened by a human (not a trusted bot)
- **THEN** the system SHALL skip diff scope validation entirely
- **RATIONALE:** Scope validation is specific to automated dependency PRs. Human PRs are expected to touch any files.

#### Scenario: PR changes .gitmodules only
- **WHEN** the PR changes only `.gitmodules`
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label
- **RATIONALE:** `.gitmodules` is a dependency manifest for git submodules and is in the default expected patterns.

#### Scenario: PR changes .gitmodules and a submodule pointer
- **WHEN** the PR changes `.gitmodules` and a submodule pointer file (e.g., `oauth2-proxy`)
- **AND** the submodule pointer is identified via the Git Trees API (mode `160000`)
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label for the submodule pointer
- **AND** the system SHALL apply the `supply-chain/submodule-update` label instead
- **RATIONALE:** Submodule pointer changes are expected companions to `.gitmodules` changes but still require human review.

#### Scenario: API error prevents file listing
- **WHEN** the system cannot fetch the PR's changed files
- **THEN** the system SHALL treat the PR as having unexpected scope (fail-closed)
- **AND** block auto-approve

### Requirement: Supply-chain risk hints block deferred approval
The system SHALL NOT grant deferred approval (in the merge subcommand) for PRs that have any `supply-chain/*` label. Unlike `risk-hint/*` labels (Go toolchain, container image) where passing CI proves safety, supply-chain concerns indicate potential tampering that CI cannot validate.

#### Scenario: Patch with supply-chain label and passing CI
- **WHEN** a PR has labels `semver/patch` and `supply-chain/author-mismatch`, and all CI checks pass
- **THEN** the system SHALL NOT grant deferred approval
- **AND** the system SHALL NOT apply `approved`/`lgtm` labels
- **AND** the system SHALL NOT merge the PR
- **RATIONALE:** A tampered PR can pass CI while carrying malicious code. CI success does not prove the PR is safe from a supply-chain perspective.

#### Scenario: Patch with only risk-hint label and passing CI (unchanged behavior)
- **WHEN** a PR has labels `semver/patch` and `risk-hint/go-toolchain` (but no `supply-chain/*` labels), and all CI checks pass
- **THEN** the system SHALL grant deferred approval as before
- **RATIONALE:** Existing deferred approval for Go toolchain updates is correct — CI proves build safety. This behavior is unchanged.
