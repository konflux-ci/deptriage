## ADDED Requirements

### Requirement: Detect changes to known attack vector paths
The system SHALL inspect the list of files changed in the PR and flag any file whose path matches a known attack vector. When suspicious files are detected, the system SHALL block auto-approve and apply a `supply-chain/suspicious-files` label.

Default suspicious path prefixes:
- `.claude/` — known malware vector
- `.vscode/` — known attack vector
- `.github/workflows/` — CI/CD pipeline tampering
- `.github/actions/` — CI/CD action tampering

#### Scenario: PR contains .claude/ directory changes
- **WHEN** the PR changes a file at `.claude/settings.json`
- **THEN** the system SHALL apply the `supply-chain/suspicious-files` label with color `e11d48` (red)
- **AND** the system SHALL add a supply-chain risk hint: `SUPPLY_CHAIN_SUSPICIOUS_FILES`
- **AND** the system SHALL block auto-approve
- **AND** the system SHALL log a warning listing the suspicious file paths

#### Scenario: PR contains .vscode/ directory changes
- **WHEN** the PR changes a file at `.vscode/settings.json`
- **THEN** the system SHALL apply the `supply-chain/suspicious-files` label

#### Scenario: PR modifies GitHub Actions workflows
- **WHEN** the PR changes a file at `.github/workflows/ci.yml`
- **THEN** the system SHALL apply the `supply-chain/suspicious-files` label
- **RATIONALE:** Workflow modifications in dependency PRs are unexpected and should require human review, even when the change is a legitimate action version bump.

#### Scenario: PR with only dependency manifest changes
- **WHEN** the PR changes only `go.mod` and `go.sum`
- **THEN** the system SHALL NOT apply the `supply-chain/suspicious-files` label

#### Scenario: Custom suspicious path
- **WHEN** the user specifies `--suspicious-path=.devcontainer/`
- **THEN** the system SHALL add `.devcontainer/` to the suspicious path list alongside the defaults
- **AND** a PR changing `.devcontainer/devcontainer.json` SHALL trigger the label

### Requirement: Detect executable scripts outside vendored directories
The system SHALL flag executable script files (`.sh`, `.mjs`, `.js`, `.py`, `.rb`, `.pl`) that appear outside vendored directories (`vendor/`, `third_party/`, `node_modules/`). Dependency updates should not introduce new executable scripts.

#### Scenario: New shell script in project root
- **WHEN** the PR adds a file `setup.sh` at the project root
- **THEN** the system SHALL include this file in the suspicious files finding
- **AND** apply the `supply-chain/suspicious-files` label

#### Scenario: Script inside vendor directory
- **WHEN** the PR adds a file `vendor/github.com/foo/bar/generate.sh`
- **THEN** the system SHALL NOT flag this file as suspicious
- **RATIONALE:** Vendored directories may legitimately contain scripts from upstream dependencies.

#### Scenario: Script with .mjs extension (known malware pattern)
- **WHEN** the PR adds a file `.claude/setup.mjs`
- **THEN** the system SHALL flag this file (matches both the `.claude/` prefix blocklist AND the executable script check)

### Requirement: Fetch PR changed files via GitHub API
The system SHALL use the GitHub Pull Request Files API (`GET /repos/{owner}/{repo}/pulls/{number}/files`) to retrieve the list of files changed in the PR.

#### Scenario: Fetch changed files
- **WHEN** the system needs to inspect changed files
- **THEN** the system SHALL call the Pull Request Files API with the PR number
- **AND** collect all filenames from the response

#### Scenario: Files API pagination
- **WHEN** the PR changes more than 100 files
- **THEN** the system SHALL paginate through all pages to collect every changed file

#### Scenario: Files API timeout
- **WHEN** the API call does not complete within 30 seconds
- **THEN** the system SHALL abort and, for bot-opened PRs, treat the PR as suspicious (fail-closed)

#### Scenario: Files API error on bot PR
- **WHEN** the API returns an error and the PR was opened by a trusted bot
- **THEN** the system SHALL log a warning and treat the PR as suspicious (fail-closed) by applying a `supply-chain/suspicious-files` label and blocking auto-approve
- **RATIONALE:** When we cannot verify file safety on an automated PR, the safe default is to require human review. Bot PRs are the attack vector — they have auto-approve/auto-merge paths that must be blocked.

#### Scenario: Files API error on human PR
- **WHEN** the API returns an error and the PR was opened by a human (not a trusted bot)
- **THEN** the system SHALL log a warning but SHALL NOT apply supply-chain labels
- **RATIONALE:** Human PRs go through normal code review and have no auto-approve/auto-merge path. Failing closed would add noise without security benefit.
