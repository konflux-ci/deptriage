## ADDED Requirements

### Requirement: GitHub Action definition with inputs and outputs
The system SHALL provide an `action.yml` file defining a Docker container action with inputs for command selection, PR targeting, and policy overrides.

#### Scenario: Action inputs defined
- **WHEN** a consumer workflow references the action
- **THEN** the following inputs SHALL be available:
  - `command` (required): one of `classify`, `analyze`, `both`, or `merge`
  - `pr-number` (optional, default: `0`): the pull request number to classify (not needed for `merge` command when using `head-sha`)
  - `auto-approve` (optional, default: `false`): enable applying `approved`/`lgtm` labels for eligible dependency updates
  - `dry-run` (optional, default: `false`): suppress all GitHub API writes and log what would happen instead
  - `head-sha` (optional): commit SHA to find associated PRs for merge (used with `merge` command in `check_suite` workflows)
  - `github-token` (required): GitHub token for API operations. For the `merge` command, a GitHub App token is recommended to satisfy branch rulesets requiring approval from a different identity than the PR pusher
  - `api-key`, `llm-provider`, and `llm-model` (optional): deprecated and ignored compatibility inputs
  - `auto-merge` (optional): deprecated and ignored compatibility input
  - `context-output` (optional): persistent path for the deterministic inspection report; defaults to `GITHUB_WORKSPACE/deptriage-context.json` when running as an action

#### Scenario: Action outputs defined
- **WHEN** the action completes successfully
- **THEN** the following outputs SHALL be available:
  - `bump-type`: detected semver bump type (major/minor/patch/digest/unknown)
  - `risk-level`: always `unknown`, retained for compatibility
  - `context-json`: path to the deterministic inspection report, set only after the report is written successfully

### Requirement: Container action packaging
The system SHALL package the Go binary in a Docker container using a multi-stage build. The runtime image SHALL include the Go toolchain required for deterministic import inspection.

#### Scenario: Docker build
- **WHEN** the action is built
- **THEN** the Dockerfile SHALL use a multi-stage build with a Go builder stage and a Go-capable runtime stage

#### Scenario: Binary is statically compiled
- **WHEN** the Go binary is built
- **THEN** it SHALL be compiled with `CGO_ENABLED=0` for compatibility with distroless images

### Requirement: Support deterministic classification, inspection, and merge modes
The system SHALL support deterministic classification, inspection, and deferred merge based on the `command` input.

#### Scenario: Command is classify
- **WHEN** `command` is set to `classify`
- **THEN** the system SHALL run only the classification pipeline and set the `bump-type` output

#### Scenario: Command is merge
- **WHEN** `command` is set to `merge`
- **THEN** the system SHALL evaluate eligible PRs for merge without running classification
- **AND** use `head-sha` to find associated PRs if `pr-number` is `0`

#### Scenario: Command is analyze
- **WHEN** `command` is set to `analyze`
- **THEN** the system SHALL write deterministic import, advisory, and vulnerability evidence to the context JSON output
- **AND** SHALL remove a stale report before starting so a failed inspection cannot expose old evidence
- **AND** SHALL NOT send content to an LLM or perform any GitHub write action

#### Scenario: Command is both
- **WHEN** `command` is set to `both`
- **THEN** the system SHALL run classification followed by deterministic inspection
