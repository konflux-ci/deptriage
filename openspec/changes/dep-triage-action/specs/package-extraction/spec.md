## ADDED Requirements

### Requirement: Extract package names from Renovate PR body
The system SHALL extract dependency package names from Renovate/Mintmaker PR body markdown tables. It SHALL support both bare format (`| github.com/foo/bar |`) and linked format (`| [github.com/foo/bar](https://...) |`). Extracted package names SHALL be deduplicated.

#### Scenario: Bare package format
- **WHEN** the PR body contains `| github.com/foo/bar | v1.0.0 -> v1.0.1 |`
- **THEN** the system SHALL extract `github.com/foo/bar`

#### Scenario: Linked package format
- **WHEN** the PR body contains `| [github.com/foo/bar](https://github.com/foo/bar) | v1.0.0 -> v1.0.1 |`
- **THEN** the system SHALL extract `github.com/foo/bar`

#### Scenario: Multiple packages in table
- **WHEN** the PR body contains a markdown table with three different packages
- **THEN** the system SHALL extract all three package names as a deduplicated list

#### Scenario: Duplicate package entries
- **WHEN** the PR body contains the same package name in multiple table rows
- **THEN** the system SHALL return the package name only once

### Requirement: Fallback to PR title for package extraction
The system SHALL fall back to extracting package names from the PR title when no packages are found in the PR body.

#### Scenario: No table in PR body
- **WHEN** the PR body contains no markdown table but the PR title contains `Update github.com/foo/bar to v1.2.0`
- **THEN** the system SHALL extract `github.com/foo/bar` from the title

#### Scenario: No packages found anywhere
- **WHEN** neither the PR body nor the title contains recognizable package names
- **THEN** the system SHALL return an empty package list (no error)

### Requirement: Extract changelog from PR body
The system SHALL extract release notes or changelog content for each package from the PR body. It SHALL strip Renovate boilerplate (Configuration section, renovate-debug comments).

#### Scenario: Package-specific release notes section
- **WHEN** the PR body contains a `### Release Notes` section for a specific package
- **THEN** the system SHALL extract that section's content as the changelog for that package

#### Scenario: No package-specific section
- **WHEN** the PR body has no package-specific release notes section
- **THEN** the system SHALL use the cleaned PR body (minus boilerplate) as the changelog, truncated to 100 lines

#### Scenario: Boilerplate removal
- **WHEN** the PR body contains a `### Configuration` section or `<!--renovate-debug:` comment
- **THEN** the system SHALL strip everything from those markers onward
