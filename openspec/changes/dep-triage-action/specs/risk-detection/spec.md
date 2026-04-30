## ADDED Requirements

### Requirement: Detect Go toolchain update risk
The system SHALL detect Go toolchain/build image updates in the PR title and flag them as high-risk.

#### Scenario: Go toolset image update
- **WHEN** the PR title matches `go-toolset` (case-insensitive)
- **THEN** the system SHALL add risk hint `GO_TOOLCHAIN_UPDATE` with a message explaining build infrastructure coordination risk

#### Scenario: Golang Docker image update
- **WHEN** the PR title matches `golang.*docker` or `docker.*golang` (case-insensitive)
- **THEN** the system SHALL add risk hint `GO_TOOLCHAIN_UPDATE`

### Requirement: Detect Go version directive change
The system SHALL detect changes to the Go version directive in `go.mod` from the PR body.

#### Scenario: Go directive version bump
- **WHEN** the PR body contains text matching a Go directive version change (e.g., `go 1.21 -> go 1.22` or `update go directive`)
- **THEN** the system SHALL add risk hint `GO_VERSION_BUMP` with a message explaining CI compatibility risk

#### Scenario: No Go directive change
- **WHEN** the PR body does not contain any Go directive version change pattern
- **THEN** the system SHALL NOT add the `GO_VERSION_BUMP` risk hint

### Requirement: Detect container image update risk
The system SHALL detect container/base image updates in the PR title.

#### Scenario: Container image update detected
- **WHEN** the PR title matches `docker`, `container`, `image`, or `registry.redhat` (case-insensitive)
- **THEN** the system SHALL add risk hint `CONTAINER_IMAGE_UPDATE` with a message about build behavior and binary compatibility risk

#### Scenario: Non-container dependency update
- **WHEN** the PR title is `Update github.com/stretchr/testify to v1.9.0`
- **THEN** the system SHALL NOT add any container-related risk hint

### Requirement: Apply risk-hint labels to PRs
The system SHALL apply a GitHub label for each detected risk hint, making risk detection results visible on the PR and available to the merge phase for deferred approval decisions.

Label mapping:
- `GO_TOOLCHAIN_UPDATE` → `risk-hint/go-toolchain` (color: `fbca04`)
- `GO_VERSION_BUMP` → `risk-hint/go-version-bump` (color: `fbca04`)
- `CONTAINER_IMAGE_UPDATE` → `risk-hint/container-image` (color: `fbca04`)

#### Scenario: Risk-hint labels applied for go-toolset update
- **WHEN** the PR title matches `go-toolset` and `docker`
- **THEN** the system SHALL apply labels `risk-hint/go-toolchain` and `risk-hint/container-image`

#### Scenario: No risk-hint labels for non-risky PR
- **WHEN** no high-risk patterns are detected
- **THEN** the system SHALL NOT apply any `risk-hint/*` labels

### Requirement: Aggregate risk hints as structured output
The system SHALL collect all detected risk hints into a single string field, with each hint on its own line, for inclusion in the LLM context.

#### Scenario: Multiple risk hints detected
- **WHEN** both `GO_TOOLCHAIN_UPDATE` and `GO_VERSION_BUMP` patterns are detected
- **THEN** the system SHALL return a risk hints string containing both hints with their explanatory messages

#### Scenario: No risk hints detected
- **WHEN** no high-risk patterns are detected in the PR title or body
- **THEN** the system SHALL return an empty risk hints string
