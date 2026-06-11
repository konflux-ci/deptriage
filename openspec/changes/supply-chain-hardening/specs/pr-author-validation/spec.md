## ADDED Requirements

### Requirement: Validate PR author is a known dependency bot
The system SHALL verify that the pull request was opened by a recognized dependency bot before allowing auto-approve or auto-merge. The default trusted bot list is `renovate[bot]`, `red-hat-konflux[bot]`, and `dependabot[bot]`. Additional bots can be added via the `--trusted-bot` CLI flag or `trusted-bots` action input.

#### Scenario: PR opened by Renovate bot
- **WHEN** the PR author login is `renovate[bot]`
- **THEN** the system SHALL proceed with classification normally (no supply-chain risk hint for author)

#### Scenario: PR opened by MintMaker bot
- **WHEN** the PR author login is `red-hat-konflux[bot]`
- **THEN** the system SHALL proceed with classification normally

#### Scenario: PR opened by unknown user
- **WHEN** the PR author login is not in the trusted bot list
- **THEN** the system SHALL skip supply-chain author validation (the checks are only meaningful for bot-opened PRs)
- **RATIONALE:** Human-opened PRs go through normal code review. Supply-chain checks target the specific risk of tampered automated PRs.

#### Scenario: Custom trusted bot
- **WHEN** the user specifies `--trusted-bot=my-renovate[bot]`
- **THEN** the system SHALL add `my-renovate[bot]` to the trusted list alongside the defaults
- **AND** a PR opened by `my-renovate[bot]` SHALL be treated as a trusted bot PR

### Requirement: Validate all PR commits are authored by the PR opener
The system SHALL fetch all commits on the PR and verify that every commit's author login matches the PR opener's login. If any commit was authored by a different identity, the system SHALL apply a `supply-chain/author-mismatch` label and block auto-approve.

#### Scenario: All commits by the same bot
- **WHEN** the PR is opened by `renovate[bot]` and all commits on the PR have author login `renovate[bot]`
- **THEN** the system SHALL NOT apply the `supply-chain/author-mismatch` label
- **AND** the PR SHALL remain eligible for auto-approve

#### Scenario: Foreign commit detected
- **WHEN** the PR is opened by `renovate[bot]` but one or more commits have a different author login (e.g., `attacker`)
- **THEN** the system SHALL apply the `supply-chain/author-mismatch` label with color `e11d48` (red)
- **AND** the system SHALL add a supply-chain risk hint: `SUPPLY_CHAIN_AUTHOR_MISMATCH`
- **AND** the system SHALL block auto-approve (do NOT apply `approved`/`lgtm` labels)
- **AND** the system SHALL log a warning with the mismatched commit SHA and author

#### Scenario: Bot commit followed by force-push from attacker
- **WHEN** the PR is opened by `renovate[bot]`, has 3 commits, and the last commit is authored by `malicious-user`
- **THEN** the system SHALL detect the mismatch and apply `supply-chain/author-mismatch`
- **RATIONALE:** Even if the first commits are legitimate, a later commit from a different author indicates tampering.

#### Scenario: GitHub API error fetching commits
- **WHEN** the system cannot fetch the PR commits (API error, timeout)
- **THEN** the system SHALL log a warning and treat the PR as having a supply-chain concern (fail-closed)
- **AND** the system SHALL block auto-approve
- **RATIONALE:** When we cannot verify author integrity, the safe default is to require human review.

#### Scenario: Commits API pagination
- **WHEN** the PR has more than 100 commits (unlikely for dependency PRs)
- **THEN** the system SHALL paginate through all commits and check every author

### Requirement: Fetch PR commits via GitHub API
The system SHALL use the GitHub Pull Requests Commits API (`GET /repos/{owner}/{repo}/pulls/{number}/commits`) to retrieve the list of commits on the PR.

#### Scenario: Fetch commits
- **WHEN** the system needs to validate commit authors
- **THEN** the system SHALL call the Pull Requests Commits API with the PR number
- **AND** parse each commit's `author.login` field

#### Scenario: Commits API timeout
- **WHEN** the API call does not complete within 30 seconds
- **THEN** the system SHALL abort and treat the PR as suspicious (fail-closed)
