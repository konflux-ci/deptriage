## ADDED Requirements

### Requirement: Apply and remove labels on PRs
The system SHALL apply labels to PRs via the GitHub API. It SHALL create labels with appropriate colors if they do not exist. It SHALL remove conflicting labels before applying new ones.

#### Scenario: Apply risk label
- **WHEN** the analysis determines risk level `high`
- **THEN** the system SHALL remove any existing `risk/*` labels and apply `risk/high` with color `#e11d48` (red)

#### Scenario: Apply semver label
- **WHEN** the classification determines bump type `minor`
- **THEN** the system SHALL apply `semver/minor` with color `#fbca04` (yellow), creating the label if needed

#### Scenario: Label already exists
- **WHEN** the label to be applied already exists on the repository
- **THEN** the system SHALL apply it without attempting to recreate it

### Requirement: Post and update PR comments with history collapse
The system SHALL post analysis results as PR comments. When a previous analysis comment exists (identified by a hidden HTML marker), the system SHALL update the existing comment by collapsing the previous content into a `<details>` block and appending the new analysis.

#### Scenario: First analysis comment
- **WHEN** no previous analysis comment exists on the PR
- **THEN** the system SHALL create a new comment with the header `## AI Dependency Impact Analysis`, a hidden marker `<!-- deptriage-analysis -->`, and the analysis content

#### Scenario: Update existing comment
- **WHEN** a previous analysis comment with the `<!-- deptriage-analysis -->` marker exists
- **THEN** the system SHALL update the comment: collapse the previous analysis into a `<details><summary>Previous analysis</summary>...</details>` block and append the new analysis

#### Scenario: Comment size limit
- **WHEN** the comment body would exceed GitHub's 65KB limit
- **THEN** the system SHALL truncate the oldest collapsed history entries to fit within the limit

#### Scenario: Analysis unavailable fallback
- **WHEN** the LLM analysis fails or is skipped
- **THEN** the system SHALL post a comment stating "Analysis unavailable" with the reason (missing API key, API error, timeout)

#### Scenario: API key not configured
- **WHEN** the LLM API key secret is not set
- **THEN** the system SHALL post a comment explaining the `GEMINI_API_KEY` (or equivalent) secret is not configured

### Requirement: Submit formal PR review events
The system SHALL submit formal GitHub review events (APPROVE or COMMENT) based on the risk level. The AI risk assessment is informational — it signals risk via labels and comments but never blocks merge via REQUEST_CHANGES. Merge eligibility is driven by deterministic signals (labels, CI status), not the AI assessment.

#### Scenario: Low risk auto-approve
- **WHEN** the risk level is `low`, the bump type is `patch`, there are no security advisories, and auto-approve is enabled
- **THEN** the system SHALL submit an `APPROVE` review event

#### Scenario: High risk comment
- **WHEN** the risk level is `high`
- **THEN** the system SHALL submit a `COMMENT` review event with the analysis as the review body
- **AND** the `risk/high` label signals the risk level without blocking merge via review state

#### Scenario: Medium risk comment
- **WHEN** the risk level is `medium`
- **THEN** the system SHALL submit a `COMMENT` review event (does not block merge)

#### Scenario: Auto-approve disabled
- **WHEN** auto-approve is disabled via configuration (default)
- **THEN** the system SHALL NOT submit `APPROVE` review events regardless of risk level

### Requirement: Apply auto-approve labels for eligible PRs
The system SHALL apply `approved` and `lgtm` labels to dependency PRs that meet the auto-approve criteria. This replaces direct auto-merge — the actual merge is handled by external merge-bot automation that acts on these labels.

#### Scenario: Patch bump eligible for auto-approve
- **WHEN** the bump type is `patch`, auto-approve is enabled, no risk hints are detected, and no security advisories exist
- **THEN** the system SHALL apply the `approved` and `lgtm` labels to the PR

#### Scenario: Non-gomod digest eligible for auto-approve
- **WHEN** the bump type is `digest`, the ecosystem is NOT `gomod`, auto-approve is enabled, no risk hints are detected, and no security advisories exist
- **THEN** the system SHALL apply the `approved` and `lgtm` labels to the PR

#### Scenario: Risk hints block auto-approve
- **WHEN** the bump type is `patch` and auto-approve is enabled, but risk hints are detected (e.g., Go toolchain update, Go version bump)
- **THEN** the system SHALL NOT apply auto-approve labels

#### Scenario: Gomod digest NOT eligible for auto-approve
- **WHEN** the bump type is `digest` and the ecosystem is `gomod`
- **THEN** the system SHALL NOT apply auto-approve labels (gomod pseudo-versions have no semver guarantees and require manual review)

#### Scenario: Minor or major bump
- **WHEN** the bump type is `minor` or `major`
- **THEN** the system SHALL NOT apply auto-approve labels

#### Scenario: Auto-approve disabled
- **WHEN** auto-approve is disabled via the `--auto-approve=false` flag or input (default)
- **THEN** the system SHALL NOT apply auto-approve labels regardless of bump type

### Requirement: Auto-merge (separate capability)
Auto-merge — the actual merging of PRs via the GitHub API — is a separate capability from auto-approve label application. See the [auto-merge spec](../auto-merge/spec.md) for full requirements. Auto-approve labels are a prerequisite for auto-merge but do not themselves trigger a merge.
