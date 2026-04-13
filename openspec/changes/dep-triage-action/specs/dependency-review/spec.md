## ADDED Requirements

### Requirement: Fetch structured dependency diff from GitHub API
The system SHALL use the GitHub Dependency Review API (`GET /repos/{owner}/{repo}/dependency-review/{base}...{head}`) to retrieve structured dependency change data including package names, old/new versions, ecosystem, and inline vulnerability information.

#### Scenario: Dependency Review API returns results
- **WHEN** the API returns a list of dependency changes for the PR's base and head refs
- **THEN** the system SHALL parse each entry to extract package name, version change, ecosystem, and any flagged vulnerabilities

#### Scenario: API returns no dependency changes
- **WHEN** the API returns an empty list (e.g., non-manifest file changes)
- **THEN** the system SHALL fall back to PR body regex extraction for package discovery

#### Scenario: API unavailable or errors
- **WHEN** the Dependency Review API returns an error (e.g., dependency graph not enabled, 404)
- **THEN** the system SHALL log a warning and fall back to PR body regex extraction

#### Scenario: API call timeout
- **WHEN** the API call does not complete within 30 seconds
- **THEN** the system SHALL abort and fall back to PR body regex extraction

### Requirement: Enrich package data with API vulnerability info
The system SHALL include any vulnerability data returned by the Dependency Review API in the package context passed to downstream analysis.

#### Scenario: Vulnerability flagged in API response
- **WHEN** the Dependency Review API flags a vulnerability for a package (severity, advisory URL)
- **THEN** the system SHALL include the vulnerability data in that package's context

#### Scenario: No vulnerabilities flagged
- **WHEN** the API response contains no vulnerability data for a package
- **THEN** the system SHALL proceed without vulnerability enrichment for that package
