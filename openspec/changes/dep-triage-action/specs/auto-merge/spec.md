## ADDED Requirements

### Requirement: Merge eligible PRs via GitHub API
The system SHALL merge dependency PRs that have been approved by the classify phase and pass all CI checks, when auto-merge is enabled. The merge decision is driven by deterministic approval (labels) and CI status, not by the AI risk assessment. The AI risk level is informational — HIGH risk is signaled via the `risk/high` label and a COMMENT review, but does not block merge via review state.

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
- **RATIONALE:** HIGH risk is signaled via the `risk/high` label; the merge subcommand skips PRs with this label. The review is a COMMENT, not REQUEST_CHANGES, so a human engineer can still merge manually if they determine the change is safe.

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
- **WHEN** auto-merge is enabled, auto-approve is enabled, but the `approved` and `lgtm` labels are not present on the PR, and the PR is not eligible for deferred approval
- **THEN** the system SHALL NOT merge the PR
- **RATIONALE:** The absence of labels means the classify phase determined the PR is not eligible for auto-approval (e.g., major/minor bump, gomod digest).

### Requirement: Deferred approval for patch bumps with risk hints
The system SHALL grant deferred approval for patch bumps that were not auto-approved during classification due to risk hints, once all CI checks have passed. This enables safe auto-merge of updates like go-toolset rebuilds where the CI pipeline — not the risk hint — is the authoritative safety gate.

Deferred approval eligibility requires ALL of the following:
1. The `semver/patch` label is present (classify determined it's a patch bump)
2. At least one `risk-hint/*` label is present (explains why early approval was skipped)
3. The `risk/high` label is NOT present
4. All CI checks on the PR have passed

#### Scenario: Go-toolset patch bump with passing CI
- **WHEN** a PR has labels `semver/patch` and `risk-hint/go-toolchain`, no `risk/high` label, and all CI checks pass
- **THEN** the system SHALL apply `approved` and `lgtm` labels and merge the PR
- **RATIONALE:** Go-toolset patch bumps (same minor version, different build ID) trigger risk hints because they could theoretically change the Go version. However, when the Konflux CI pipeline passes, the build is proven safe. The risk hint prevented premature approval before CI ran; once CI confirms safety, the merge can proceed.

#### Scenario: Patch with risk hints but CI failing
- **WHEN** a PR has labels `semver/patch` and `risk-hint/go-toolchain`, but CI checks are failing
- **THEN** the system SHALL NOT grant deferred approval and SHALL NOT merge
- **RATIONALE:** CI failure means the risk hint's concern was justified — the update may have broken the build.

#### Scenario: Minor bump with risk hints not eligible for deferred approval
- **WHEN** a PR has labels `semver/minor` and `risk-hint/go-toolchain`, and all CI checks pass
- **THEN** the system SHALL NOT grant deferred approval
- **RATIONALE:** Deferred approval is limited to patch bumps. Minor and major bumps require explicit human review.

#### Scenario: Patch without risk hints uses normal auto-approve path
- **WHEN** a PR has label `semver/patch` but no `risk-hint/*` labels
- **THEN** the system SHALL NOT use deferred approval (the normal auto-approve path in classify should have already applied `approved`/`lgtm` labels)

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
