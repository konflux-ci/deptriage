## ADDED Requirements

### Requirement: Apply and remove labels on PRs

The system SHALL apply labels to PRs via the GitHub API. It SHALL create
labels with appropriate colors if they do not exist and remove conflicting
labels before applying new ones.

#### Scenario: Apply semver label

- **WHEN** classification determines bump type `minor`
- **THEN** the system SHALL apply `semver/minor` with color `#fbca04` (yellow)

### Requirement: Apply deterministic auto-approval labels

The system SHALL apply `approved` and `lgtm` labels only to dependency PRs
that meet deterministic auto-approval criteria. Formal approval and merging
are performed later by the authorized merge command after provenance and CI
validation.

#### Scenario: Eligible patch update

- **WHEN** the bump type is `patch`, auto-approve is enabled, and no risk hints
  or supply-chain findings are present
- **THEN** the system SHALL apply `approved` and `lgtm`

#### Scenario: Risk hint blocks immediate approval

- **WHEN** a risk hint is present
- **THEN** the system SHALL NOT apply auto-approval labels during classification

#### Scenario: Major update

- **WHEN** the bump type is `major`
- **THEN** the system SHALL NOT apply auto-approval labels
