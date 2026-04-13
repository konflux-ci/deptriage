## ADDED Requirements

### Requirement: Query GitHub Security Advisories for updated packages
The system SHALL query the GitHub Global Security Advisories API for each updated package to surface known CVEs, GHSA IDs, CVSS scores, severity levels, and patched version ranges.

#### Scenario: Advisory exists for package
- **WHEN** a security advisory exists for `github.com/foo/bar` affecting versions before the new version
- **THEN** the system SHALL include the advisory data (GHSA ID, CVE, CVSS score, severity, patched versions) in the package context

#### Scenario: No advisories for package
- **WHEN** no security advisories exist for the package
- **THEN** the system SHALL proceed without advisory data for that package

#### Scenario: API rate limit or error
- **WHEN** the Security Advisories API returns an error or rate limit
- **THEN** the system SHALL log a warning and proceed without advisory data (graceful degradation)

#### Scenario: Advisory API timeout
- **WHEN** the API call does not complete within 30 seconds
- **THEN** the system SHALL abort and proceed without advisory data

### Requirement: Run govulncheck for reachability analysis
The system SHALL optionally run `govulncheck` to determine if vulnerable functions in updated dependencies are actually called in the codebase.

#### Scenario: govulncheck available and finds reachable vulnerability
- **WHEN** `govulncheck` is installed and reports a vulnerable function is called in the codebase
- **THEN** the system SHALL include the reachability finding (vulnerable function, call chain) in the security context

#### Scenario: govulncheck available and finds no reachable vulnerabilities
- **WHEN** `govulncheck` completes and reports no reachable vulnerabilities
- **THEN** the system SHALL include a clean reachability status in the security context

#### Scenario: govulncheck not installed
- **WHEN** the `govulncheck` binary is not found in PATH
- **THEN** the system SHALL skip reachability analysis with a log message (not an error)

#### Scenario: govulncheck timeout
- **WHEN** `govulncheck` does not complete within 300 seconds
- **THEN** the system SHALL kill the process and report a timeout (proceed without reachability data)

#### Scenario: govulncheck execution error
- **WHEN** `govulncheck` exits with a non-zero code (e.g., build errors)
- **THEN** the system SHALL log the error and proceed without reachability data
