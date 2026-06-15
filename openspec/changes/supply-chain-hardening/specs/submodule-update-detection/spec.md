## ADDED Requirements

### Requirement: Detect git submodule updates in dependency PRs
The system SHALL detect when a dependency PR updates git submodules and apply a `supply-chain/submodule-update` label. Submodule updates bring in entire upstream codebases where passing CI alone does not guarantee safety, so they require engineer review. This check prevents the false positive where submodule changes were incorrectly flagged as `supply-chain/unexpected-scope` (which implies tampering).

Detection mechanism:
- For every bot PR, the system fetches the repository tree at the PR's head ref using the Git Trees API
- Tree entries with mode `160000` (gitlink) are identified as submodule paths
- Changed files matching submodule paths are excluded from diff scope validation (no `unexpected-scope` label)
- A `supply-chain/submodule-update` finding is emitted instead, with yellow color (`fbca04`)

Note: submodule pointer bumps do not necessarily modify `.gitmodules` — the gitlink entry changes independently. The system fetches submodule paths for all bot PRs, not only when `.gitmodules` is in the diff.

#### Scenario: Submodule version bump (e.g., oauth2-proxy v7.15.2 to v7.15.3)
- **WHEN** the PR changes only the `oauth2-proxy` submodule pointer (gitlink)
- **AND** `oauth2-proxy` is identified as a submodule via the Git Trees API (mode `160000`)
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label
- **AND** the system SHALL apply the `supply-chain/submodule-update` label with color `fbca04` (yellow)
- **AND** the system SHALL block auto-approve
- **AND** the system SHALL block auto-merge
- **AND** the system SHALL log the submodule paths in the finding details

#### Scenario: Submodule pointer bump without .gitmodules change
- **WHEN** the PR changes only the `oauth2-proxy` submodule pointer (no `.gitmodules` in the diff)
- **AND** `oauth2-proxy` is identified as a submodule via the Git Trees API (mode `160000`)
- **THEN** the system SHALL NOT apply the `supply-chain/unexpected-scope` label
- **AND** the system SHALL apply the `supply-chain/submodule-update` label with color `fbca04` (yellow)
- **AND** the system SHALL block auto-approve and auto-merge
- **RATIONALE:** Submodule version bumps only change the gitlink commit SHA, not `.gitmodules`. The system must detect these independently.

#### Scenario: Submodule update with additional unexpected files
- **WHEN** the PR changes a submodule pointer and `internal/foo/bar.go`
- **THEN** the system SHALL apply `supply-chain/submodule-update` for the submodule pointer
- **AND** the system SHALL apply `supply-chain/unexpected-scope` for `internal/foo/bar.go`
- **RATIONALE:** Submodule detection only exempts actual submodule paths from scope validation. Other unexpected files are still flagged.

#### Scenario: .gitmodules changed but no submodule pointers in diff
- **WHEN** the PR changes `.gitmodules` but no submodule pointer files
- **THEN** the system SHALL NOT apply the `supply-chain/submodule-update` label
- **RATIONALE:** Only actual submodule pointer changes warrant the submodule-update finding.

#### Scenario: Non-bot PR with submodule changes
- **WHEN** a human-opened PR changes `.gitmodules` and a submodule pointer
- **THEN** the system SHALL skip submodule detection (no label applied)
- **RATIONALE:** Submodule detection is only relevant for automated bot PRs. Human PRs go through normal code review.

#### Scenario: Git Trees API error
- **WHEN** the system cannot fetch the repository tree (API error, timeout)
- **THEN** the system SHALL log a warning and skip submodule detection
- **AND** diff scope validation SHALL proceed normally (submodule pointers may trigger `unexpected-scope`)
- **RATIONALE:** Fail-open for submodule detection is acceptable — the worst case is a false `unexpected-scope` label, which is the pre-existing behavior. The diff scope validator itself remains fail-closed.

### Requirement: Fetch submodule paths via Git Trees API
The system SHALL use the GitHub Git Trees API (`GET /repos/{owner}/{repo}/git/trees/{ref}`) to identify submodule entries in the repository tree.

#### Scenario: Fetch submodule paths
- **WHEN** the system needs to identify submodule paths
- **THEN** the system SHALL call the Git Trees API with the PR's head ref (non-recursive)
- **AND** return paths of tree entries where `mode == "160000"` (gitlink)

#### Scenario: Repository has no submodules
- **WHEN** the tree contains no entries with mode `160000`
- **THEN** the system SHALL return an empty list

#### Scenario: Trees API timeout
- **WHEN** the API call does not complete within 30 seconds
- **THEN** the system SHALL abort and log a warning

### Known Limitations

- **Nested submodules are not detected.** The tree fetch uses non-recursive mode, so only top-level gitlinks are discovered. Nested submodules (a submodule inside a submodule) would be missed and flagged as `supply-chain/unexpected-scope`. Acceptable because Konflux repos do not use nested submodules, and recursive fetches introduce truncation risk on large repositories.
- **Submodule removals/renames are not detected.** Only the PR head tree is inspected, so a submodule path removed or renamed by the PR would not be found. Acceptable because Renovate/MintMaker only bumps submodule versions — it does not remove or rename submodules.

### Requirement: Submodule update finding in LLM analysis
The LLM analysis template SHALL include `SUPPLY_CHAIN_SUBMODULE_UPDATE` as a recognized finding type with appropriate guidance: the PR updates git submodules requiring engineer review, and auto-merging should not be recommended.

### Requirement: Supply-chain submodule-update label blocks all merge paths
The `supply-chain/submodule-update` label SHALL block auto-approve, auto-merge, and deferred approval through the same mechanism as other `supply-chain/*` labels (prefix-based check in merge eligibility).
