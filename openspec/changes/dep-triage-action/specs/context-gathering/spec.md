## ADDED Requirements

### Requirement: Assemble structured context JSON for LLM consumption
The system SHALL assemble a structured JSON object combining all gathered data for each package: package info, changelog, import chains, source snippets, test coverage, security advisories, govulncheck results, and risk hints.

#### Scenario: Full context available
- **WHEN** all data sources return results (changelog, imports, advisories, govulncheck)
- **THEN** the system SHALL produce a JSON object with structure:
  ```json
  {
    "prBody": "cleaned PR body",
    "packages": [
      {
        "name": "github.com/foo/bar",
        "changelog": "release notes text",
        "noDirectImports": false,
        "importChain": "go mod why output",
        "imports": [
          {
            "file": "cmd/main.go",
            "hasTest": true,
            "snippet": "5-line context around usage"
          }
        ],
        "advisories": [
          {
            "ghsaId": "GHSA-xxxx-xxxx-xxxx",
            "cve": "CVE-2024-1234",
            "severity": "HIGH",
            "cvssScore": 8.1,
            "patchedVersions": ">=1.2.3"
          }
        ],
        "govulncheck": {
          "reachable": false,
          "findings": []
        }
      }
    ],
    "riskHints": "detected risk patterns"
  }
  ```

#### Scenario: Partial context (graceful degradation)
- **WHEN** some data sources fail or return empty (e.g., no advisories, govulncheck skipped)
- **THEN** the system SHALL produce valid JSON with empty/null fields for missing data and populated fields for available data

#### Scenario: No packages extracted
- **WHEN** no packages were found in the PR
- **THEN** the system SHALL produce `{"packages":[]}`

### Requirement: Clean PR body before inclusion
The system SHALL strip Renovate/Mintmaker boilerplate from the PR body before including it in the context JSON. This includes `### Configuration` sections and `<!--renovate-debug:` comments.

#### Scenario: PR body with Renovate boilerplate
- **WHEN** the PR body contains a `### Configuration` section followed by schedule/automerge config
- **THEN** the system SHALL strip everything from `### Configuration` onward

#### Scenario: PR body with debug comments
- **WHEN** the PR body contains `<!--renovate-debug:` HTML comments
- **THEN** the system SHALL strip everything from that comment onward
