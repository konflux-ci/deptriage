## ADDED Requirements

### Requirement: Merge eligible PRs via GitHub API
The system SHALL merge dependency PRs that have been approved by the classify phase and pass all CI checks, when auto-merge is enabled. The merge decision is driven by deterministic approval (labels) and CI status, not by the AI risk assessment. The AI risk level is informational — only HIGH risk blocks merge (via REQUEST_CHANGES review event).

Merge eligibility requires ALL of the following:
1. The `auto-merge` flag is enabled
2. The `auto-approve` flag is enabled (prerequisite)
3. The `approved` and `lgtm` labels are present on the PR (applied by the classify phase)
4. The AI risk level is NOT `high`
5. All CI checks on the PR head SHA are `success` or `neutral` (excluding the deptriage workflow itself)

#### Scenario: Approved PR with all checks passing and LOW risk
- **WHEN** auto-merge is enabled, auto-approve is enabled, the PR has `approved` and `lgtm` labels, the AI risk level is `low`, and all CI checks pass
- **THEN** the system SHALL merge the PR using the GitHub REST API with squash merge method

#### Scenario: Approved PR with all checks passing and MEDIUM risk
- **WHEN** auto-merge is enabled, auto-approve is enabled, the PR has `approved` and `lgtm` labels, the AI risk level is `medium`, and all CI checks pass
- **THEN** the system SHALL merge the PR
- **RATIONALE:** The AI risk level is informational. Experience shows that dependency updates (e.g., Tekton task digest bumps) flagged as MEDIUM risk are safe when CI checks — especially Red Hat Konflux pipeline checks — pass. The classify phase's deterministic approval and CI status are the real safety gates.

#### Scenario: HIGH risk blocks merge
- **WHEN** the AI risk level is `high`
- **THEN** the system SHALL NOT attempt to merge the PR
- **RATIONALE:** HIGH risk triggers a REQUEST_CHANGES review event, which is a hard block requiring human review.

#### Scenario: CI checks still pending
- **WHEN** auto-merge is enabled and the PR is eligible, but one or more CI checks have status `pending` or `queued`
- **THEN** the system SHALL skip the merge attempt and log that checks are not yet complete
- **AND** the system SHALL NOT fail the action

#### Scenario: CI checks failing
- **WHEN** auto-merge is enabled and the PR is eligible, but one or more CI checks have status `failure` or `error`
- **THEN** the system SHALL NOT merge the PR

#### Scenario: auto-merge disabled (default)
- **WHEN** auto-merge is not enabled (default: `false`)
- **THEN** the system SHALL NOT attempt to merge the PR regardless of risk level, labels, or check status

#### Scenario: auto-merge enabled but auto-approve disabled
- **WHEN** auto-merge is enabled but auto-approve is disabled
- **THEN** the system SHALL NOT attempt to merge the PR
- **RATIONALE:** Auto-approve is a prerequisite — without it, `approved`/`lgtm` labels will not be present, and the classify phase has not made an approval decision.

#### Scenario: auto-approve labels not present
- **WHEN** auto-merge is enabled, auto-approve is enabled, but the `approved` and `lgtm` labels are not present on the PR
- **THEN** the system SHALL NOT merge the PR
- **RATIONALE:** The absence of labels means the classify phase determined the PR is not eligible for auto-approval (e.g., major/minor bump, gomod digest, risk hints detected).

#### Scenario: PR has merge conflict
- **WHEN** the system attempts to merge but the PR has a merge conflict
- **THEN** the system SHALL log a warning and NOT fail the action

#### Scenario: GitHub API merge error
- **WHEN** the system attempts to merge but the GitHub API returns an error
- **THEN** the system SHALL log the error as a warning and NOT fail the action
- **RATIONALE:** Consistent with the analyze phase's "always exit 0" semantics. Merge is best-effort.

### Requirement: Exclude self from CI check status
The system SHALL exclude its own workflow (the deptriage "Dependency Impact Analysis" check) when evaluating CI check status, to avoid a circular dependency where the action waits for its own completion.

#### Scenario: Self-exclusion from check status
- **WHEN** the system queries CI check status for the PR head SHA
- **THEN** the system SHALL exclude any check run whose name matches the deptriage workflow name
- **AND** evaluate only the remaining checks for pass/fail status

### Requirement: Action interface for auto-merge
The `action.yml` SHALL expose an `auto-merge` input, separate from the existing `auto-approve` input.

#### Scenario: auto-merge input
- **WHEN** the action is invoked with `auto-merge: 'true'`
- **THEN** the system SHALL enable auto-merge behavior in the analyze phase
- **AND** the default value SHALL be `'false'`

#### Scenario: Permissions requirement
- **WHEN** auto-merge is enabled
- **THEN** the workflow MUST grant `contents: write` permission to the action (required by the GitHub merge API)

### Requirement: Standalone merge subcommand
The deptriage binary SHALL provide a `merge` subcommand that can be invoked independently from the `analyze` phase. This enables deferred merge via a separate workflow triggered on `check_suite: completed`, solving the timing problem where the analyze phase finishes before other CI checks complete.

#### Scenario: Merge subcommand with PR number
- **WHEN** `deptriage merge --pr-number 28` is invoked
- **THEN** the system SHALL evaluate the PR for merge eligibility (labels, checks, risk) and merge if eligible

#### Scenario: Merge subcommand with head SHA
- **WHEN** `deptriage merge --head-sha <sha>` is invoked
- **THEN** the system SHALL find all open PRs matching that head SHA and evaluate each for merge eligibility

#### Scenario: Deferred merge after all checks complete
- **WHEN** a check suite completes, the auto-merge workflow invokes `deptriage merge` with the check suite's head SHA
- **THEN** the system SHALL find the associated PR, verify labels and check status, and merge if all conditions are met

#### Scenario: Not all checks complete yet
- **WHEN** the merge subcommand runs but other checks on the PR are still pending or failing
- **THEN** the system SHALL skip the merge attempt (a subsequent `check_suite` event will retry)

#### Scenario: Workflow self-exclusion
- **WHEN** evaluating check status for merge eligibility
- **THEN** the system SHALL exclude the auto-merge workflow's own check from the evaluation to avoid circular dependency
