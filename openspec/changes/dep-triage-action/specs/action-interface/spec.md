## ADDED Requirements

### Requirement: GitHub Action definition with inputs and outputs
The system SHALL provide an `action.yml` file defining a Docker container action with inputs for command selection, PR targeting, LLM configuration, and policy overrides.

#### Scenario: Action inputs defined
- **WHEN** a consumer workflow references the action
- **THEN** the following inputs SHALL be available:
  - `command` (required): one of `classify`, `analyze`, `both`, or `merge`
  - `pr-number` (optional, default: `0`): the pull request number to analyze (not needed for `merge` command when using `head-sha`)
  - `api-key` (optional): LLM provider API key (not needed for `merge` command)
  - `llm-provider` (optional, default: `gemini`): LLM provider name
  - `llm-model` (optional): specific model to use (provider-dependent default)
  - `auto-approve` (optional, default: `false`): enable applying `approved`/`lgtm` labels and formal APPROVE review for eligible low-risk patches
  - `auto-merge` (optional, default: `false`): enable auto-merging eligible PRs after analysis when auto-approve labels are present and CI checks pass
  - `head-sha` (optional): commit SHA to find associated PRs for merge (used with `merge` command in `check_suite` workflows)
  - `github-token` (required): GitHub token for API operations. For the `merge` command, a GitHub App token is recommended to satisfy branch rulesets requiring approval from a different identity than the PR pusher

#### Scenario: Action outputs defined
- **WHEN** the action completes successfully
- **THEN** the following outputs SHALL be available:
  - `bump-type`: detected semver bump type (major/minor/patch/digest/unknown)
  - `risk-level`: assessed risk level (low/medium/high/unknown) — empty if analyze not run
  - `context-json`: path to the assembled context JSON file

### Requirement: Container action packaging
The system SHALL package the Go binary in a Docker container using a multi-stage build: Go build stage producing a static binary, then a minimal runtime image.

#### Scenario: Docker build
- **WHEN** the action is built
- **THEN** the Dockerfile SHALL use a multi-stage build with a Go builder stage and a distroless/static runtime stage

#### Scenario: Binary is statically compiled
- **WHEN** the Go binary is built
- **THEN** it SHALL be compiled with `CGO_ENABLED=0` for compatibility with distroless images

### Requirement: Support classify-only, analyze-only, and combined modes
The system SHALL support running classify alone, analyze alone, or both in sequence based on the `command` input.

#### Scenario: Command is classify
- **WHEN** `command` is set to `classify`
- **THEN** the system SHALL run only the classification pipeline and set the `bump-type` output

#### Scenario: Command is analyze
- **WHEN** `command` is set to `analyze`
- **THEN** the system SHALL run only the analysis pipeline (requires classify output to already exist)

#### Scenario: Command is both
- **WHEN** `command` is set to `both`
- **THEN** the system SHALL run classify first, then analyze, passing the classify output to analyze automatically

#### Scenario: Command is merge
- **WHEN** `command` is set to `merge`
- **THEN** the system SHALL evaluate eligible PRs for merge without running classify or analyze
- **AND** use `head-sha` to find associated PRs if `pr-number` is `0`

#### Scenario: Analyze without prior classify output
- **WHEN** `command` is `analyze` and no classify output file exists
- **THEN** the system SHALL exit with an error explaining that classify must be run first
