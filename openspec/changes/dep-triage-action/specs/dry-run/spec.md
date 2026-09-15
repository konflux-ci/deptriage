## ADDED Requirements

### Requirement: Dry-run mode for deterministic commands

The system SHALL support `--dry-run` on `classify` and `merge`. It SHALL
suppress GitHub API writes while preserving the read operations needed to
evaluate deterministic policy. Each suppressed write SHALL be logged with a
`[DRY-RUN]` prefix.

#### Scenario: Dry-run classification

- **WHEN** dry-run is enabled and classification would apply a label
- **THEN** the system SHALL log the intended label write and SHALL NOT call the
  GitHub Issues API

#### Scenario: Dry-run merge

- **WHEN** dry-run is enabled and a trusted PR is eligible to merge
- **THEN** the system SHALL log the intended approval and merge and SHALL NOT
  submit a review, merge, or enqueue the PR

#### Scenario: Dry-run preserves reads

- **WHEN** dry-run is enabled
- **THEN** the system SHALL still fetch PR metadata, dependency-review data,
  commits, files, submodule paths, and CI status needed for its decision
