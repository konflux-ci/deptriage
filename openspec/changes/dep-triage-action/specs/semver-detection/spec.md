## ADDED Requirements

### Requirement: Detect semver bump type from PR content
The system SHALL parse the PR title and body to determine the semver bump type of a dependency update. It SHALL support three-component versions (`v1.2.3`), two-component versions (`v1.2`), and digest-only changes (`abcdef0 -> 1234abc`). When multiple version pairs are present, the system SHALL return the highest bump level across all pairs (major > minor > patch > digest > unknown).

#### Scenario: Three-component major bump
- **WHEN** the PR body contains `` `v1.5.0` -> `v2.0.0` ``
- **THEN** the system SHALL return bump type `major`

#### Scenario: Three-component minor bump
- **WHEN** the PR body contains `` `v1.5.0` -> `v1.9.0` ``
- **THEN** the system SHALL return bump type `minor`

#### Scenario: Three-component patch bump
- **WHEN** the PR body contains `` `v1.5.0` -> `v1.5.1` ``
- **THEN** the system SHALL return bump type `patch`

#### Scenario: Two-component version bump
- **WHEN** the PR body contains `` `v9.5` -> `v9.7` ``
- **THEN** the system SHALL return bump type `minor`

#### Scenario: Digest-only update
- **WHEN** the PR body contains `` `abcdef0` -> `1234abc` `` with no semver version pairs
- **THEN** the system SHALL return bump type `digest`

#### Scenario: Multiple version pairs with mixed bump types
- **WHEN** the PR body contains both a patch bump (`v1.0.0 -> v1.0.1`) and a minor bump (`v2.3.0 -> v2.4.0`)
- **THEN** the system SHALL return bump type `minor` (the highest)

#### Scenario: No version information found
- **WHEN** the PR title and body contain no recognizable version pairs or digests
- **THEN** the system SHALL return bump type `unknown`

#### Scenario: Optional v prefix
- **WHEN** a version string appears without the `v` prefix (e.g., `1.2.3 -> 1.3.0`)
- **THEN** the system SHALL still detect the bump type correctly

### Requirement: Apply semver labels to PR
The system SHALL apply a color-coded label to the PR based on the detected bump type. Labels SHALL be created if they do not already exist.

#### Scenario: Apply patch label
- **WHEN** the detected bump type is `patch`
- **THEN** the system SHALL apply the label `semver/patch` with color `#0e8a16` (green)

#### Scenario: Apply digest label (Go module)
- **WHEN** the detected bump type is `digest` AND the package ecosystem is `gomod` (detected via Dependency Review API ecosystem field or Go module hosting domain heuristics: `github.com/`, `k8s.io/`, `golang.org/`, `go.uber.org/`, `gopkg.in/`, etc.)
- **THEN** the system SHALL apply the label `semver/minor` with color `#fbca04` (yellow)
- **RATIONALE:** Go pseudo-versions (`v0.0.0-timestamp-hash`) have no semver guarantees; breaking API changes have been observed in k8s.io pseudo-version bumps

#### Scenario: Apply digest label (non-Go module)
- **WHEN** the detected bump type is `digest` AND the package ecosystem is NOT `gomod` (e.g., container images, Tekton task references)
- **THEN** the system SHALL apply the label `semver/patch` with color `#0e8a16` (green)

#### Scenario: Apply minor label
- **WHEN** the detected bump type is `minor`
- **THEN** the system SHALL apply the label `semver/minor` with color `#fbca04` (yellow)

#### Scenario: Apply major label
- **WHEN** the detected bump type is `major`
- **THEN** the system SHALL apply the label `semver/major` with color `#e11d48` (red)

#### Scenario: Skip labeling for unknown bump type
- **WHEN** the detected bump type is `unknown`
- **THEN** the system SHALL NOT apply any semver label

#### Scenario: Existing semver label present
- **WHEN** the PR already has a `semver/*` label
- **THEN** the system SHALL skip labeling (do not replace existing labels)
