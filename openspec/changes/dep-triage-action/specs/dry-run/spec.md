## ADDED Requirements

### Requirement: Dry-run mode for all commands
The system SHALL support a `--dry-run` flag on all commands (classify, analyze, both, merge) that suppresses all GitHub API write operations while preserving all read-only operations. Each suppressed write SHALL be logged as a structured `slog.Info` line with a `[DRY-RUN]` prefix.

#### Scenario: Dry-run suppresses label creation
- **WHEN** dry-run is enabled and the classify command would apply a semver label
- **THEN** the system SHALL log `[DRY-RUN] would apply label` with the label name, color, and PR number
- **AND** the system SHALL NOT call the GitHub Issues API to create or apply the label

#### Scenario: Dry-run suppresses comment posting
- **WHEN** dry-run is enabled and the analyze command would post an analysis comment
- **THEN** the system SHALL log `[DRY-RUN] would upsert analysis comment` with the PR number
- **AND** the system SHALL NOT call the GitHub Issues API to create or edit a comment

#### Scenario: Dry-run suppresses review submission
- **WHEN** dry-run is enabled and the analyze command would submit a review
- **THEN** the system SHALL log `[DRY-RUN] would submit review` with the review event type and PR number
- **AND** the system SHALL NOT call the GitHub Pull Requests API to create a review

#### Scenario: Dry-run suppresses PR merge
- **WHEN** dry-run is enabled and the system would merge a PR
- **THEN** the system SHALL log `[DRY-RUN] would merge PR` with the PR number and merge method
- **AND** the system SHALL NOT call the GitHub Pull Requests API to merge

#### Scenario: Dry-run suppresses PR enqueue
- **WHEN** dry-run is enabled and the system would enqueue a PR into a merge queue
- **THEN** the system SHALL log `[DRY-RUN] would enqueue PR` with the PR node ID
- **AND** the system SHALL NOT call the GitHub GraphQL API

#### Scenario: Dry-run suppresses label removal
- **WHEN** dry-run is enabled and the analyze command would remove a risk label
- **THEN** the system SHALL log `[DRY-RUN] would remove label` with the label name and PR number
- **AND** the system SHALL NOT call the GitHub Issues API to remove the label

#### Scenario: Dry-run preserves read operations
- **WHEN** dry-run is enabled
- **THEN** the system SHALL still execute all read-only GitHub API calls (FetchPR, FetchDependencyReview, ListComments, ChecksAllPassed, HasLabels, FindOpenPRsForSHA)
- **AND** the analysis pipeline SHALL produce accurate results based on real PR data

#### Scenario: Dry-run preserves LLM analysis
- **WHEN** dry-run is enabled and the analyze command runs
- **THEN** the system SHALL still call the LLM provider API and produce an analysis response
- **AND** the response SHALL be logged but NOT posted as a GitHub comment

#### Scenario: Dry-run preserves classify output file
- **WHEN** dry-run is enabled and the classify command runs
- **THEN** the system SHALL still write the classify result JSON to the output file
- **RATIONALE:** The output file is a local artifact, not a GitHub API side effect. The analyze phase depends on it when running `both`.

#### Scenario: Dry-run via CLI flag
- **WHEN** the user invokes `deptriage classify --dry-run`
- **THEN** dry-run mode SHALL be enabled for that command

#### Scenario: Dry-run via GitHub Action input
- **WHEN** the action is invoked with `dry-run: 'true'`
- **THEN** dry-run mode SHALL be enabled via the `INPUT_DRY_RUN` environment variable

#### Scenario: Dry-run default is off
- **WHEN** the `--dry-run` flag is not specified
- **THEN** the system SHALL execute all operations normally (dry-run is disabled by default)

#### Scenario: Dry-run with auto-merge
- **WHEN** dry-run is enabled and auto-merge is enabled and a PR is merge-eligible
- **THEN** the system SHALL log `[DRY-RUN] would submit review` and `[DRY-RUN] would merge PR`
- **AND** SHALL NOT submit the review or merge the PR
