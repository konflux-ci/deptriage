## 1. GitHub API Methods

- [x] 1.1 Add `FetchPRCommits` method to `internal/github/pr.go`: call `GET /repos/{owner}/{repo}/pulls/{number}/commits` with pagination, return slice of commit author logins and SHAs
- [x] 1.2 Add `FetchPRFiles` method to `internal/github/pr.go`: call `GET /repos/{owner}/{repo}/pulls/{number}/files` with pagination, return slice of changed file paths
- [x] 1.3 Add unit tests for `FetchPRCommits` and `FetchPRFiles` with mocked API responses (success, error, pagination, nil author, empty results)

## 2. Supply-Chain Types and Constants

- [x] 2.1 Add supply-chain label constants to `internal/types/types.go`: `LabelSupplyChainAuthorMismatch`, `LabelSupplyChainSuspiciousFiles`, `LabelSupplyChainUnexpectedScope`, `SupplyChainLabelPrefix`
- [x] 2.2 Add `SupplyChainFindingResult` type and include in `ClassifyResult` and `ContextJSON`

## 3. PR Author Validation

- [x] 3.1 Create `internal/classify/supply_chain.go` with `ValidateAuthor` function: accepts PR author login, list of commit authors, and trusted bot list; returns a `SupplyChainFinding` with risk hint and label info
- [x] 3.2 Implement default trusted bot list (`renovate[bot]`, `red-hat-konflux[bot]`, `dependabot[bot]`) with `--trusted-bot` flag support for additions
- [x] 3.3 Implement fail-closed behavior: API errors during commit fetch result in a supply-chain finding with distinct `SUPPLY_CHAIN_VERIFICATION_FAILED` key
- [x] 3.4 Add unit tests for author validation: all-same-author, foreign commit, non-bot PR (skip), custom trusted bot, empty commit list, empty author as mismatch, multiple foreign commits

## 4. Suspicious File Detection

- [x] 4.1 Add `DetectSuspiciousFiles` function to `internal/classify/supply_chain.go`: accepts list of changed file paths, returns `SupplyChainFinding` if any match the blocklist
- [x] 4.2 Implement default suspicious path prefix list (`.claude/`, `.vscode/`, `.github/workflows/`, `.github/actions/`) with `--suspicious-path` flag support
- [x] 4.3 Implement executable script detection (`.sh`, `.mjs`, `.js`, `.py`, `.rb`, `.pl`) outside vendored directories
- [x] 4.4 Add unit tests: clean PR, suspicious path match, executable script outside vendor, script inside vendor (no match), custom path, multiple matches

## 5. Diff Scope Validation

- [x] 5.1 Add `ValidateDiffScope` function to `internal/classify/supply_chain.go`: accepts list of changed file paths, returns `SupplyChainFinding` if any file is outside expected patterns
- [x] 5.2 Implement default expected file patterns with `--expected-file` flag support for additions
- [x] 5.3 Add unit tests: manifest-only changes, vendor changes, unexpected source file, custom pattern, non-bot PR (skip)

## 6. Classify Pipeline Integration

- [x] 6.1 Add `--trusted-bot`, `--suspicious-path`, `--expected-file` flags to classify and both subcommands in `cmd/deptriage/main.go`
- [x] 6.2 Add `INPUT_TRUSTED_BOTS`, `INPUT_SUSPICIOUS_PATHS`, `INPUT_EXPECTED_FILES` env var support
- [x] 6.3 Integrate supply-chain checks into `classify.Run()`: call `FetchPRCommits` and `FetchPRFiles`, run all three validators, apply labels, and block auto-approve when findings exist
- [x] 6.4 Update `classify.Options` struct with new fields: `TrustedBots`, `SuspiciousPaths`, `ExpectedFiles`
- [x] 6.5 Update dry-run support: log `[DRY-RUN] would apply supply-chain label` for each finding

## 7. Merge Phase: Block Supply-Chain Labels

- [x] 7.1 Update `isMergeEligible` in `internal/merge/merge.go` to reject PRs with any `supply-chain/*` label (primary merge path)
- [x] 7.2 Update `isDeferredApprovalEligible` to exclude PRs with any `supply-chain/*` label (deferred approval path)
- [x] 7.3 Add unit tests: supply-chain labels block both primary merge and deferred approval

## 8. Analyze Phase: Block APPROVE for Supply-Chain Concerns

- [x] 8.1 Pass `SupplyChainFindings` through `ClassifyResult` JSON to analyze phase
- [x] 8.2 Skip formal `APPROVE` review in `submitReview` when supply-chain findings exist
- [x] 8.3 Skip inline auto-merge in analyze when supply-chain findings exist
- [x] 8.4 Surface supply-chain findings in `ContextJSON` for LLM visibility

## 9. Action Interface

- [x] 9.1 Add `trusted-bots`, `suspicious-paths`, `expected-files` inputs to `action.yml`
- [x] 9.2 Map inputs to `INPUT_TRUSTED_BOTS`, `INPUT_SUSPICIOUS_PATHS`, `INPUT_EXPECTED_FILES` env vars

## 10. Documentation

- [x] 10.1 Update README with supply-chain hardening section
- [x] 10.2 Update README with action inputs table

## 11. Submodule Update Detection

- [x] 11.1 Add `.gitmodules` to `defaultExpectedPatterns` in `internal/classify/supply_chain.go`
- [x] 11.2 Add `LabelSupplyChainSubmoduleUpdate` constant to `internal/types/types.go`
- [x] 11.3 Add `FetchSubmodulePaths` method to `internal/github/pr.go`: call Git Trees API, return paths with mode `160000`
- [x] 11.4 Add submodule detection logic in `internal/classify/classify.go`: when `.gitmodules` is changed, fetch submodule paths, add to expected patterns, emit `SUPPLY_CHAIN_SUBMODULE_UPDATE` finding
- [x] 11.5 Update `internal/analyze/template.md` with `SUPPLY_CHAIN_SUBMODULE_UPDATE` finding type
- [x] 11.6 Add unit tests for `FetchSubmodulePaths` (success, no submodules, API error)
- [x] 11.7 Add unit tests for `ValidateDiffScope` with `.gitmodules` and submodule patterns
- [x] 11.8 Update README with submodule detection section
- [x] 11.9 Create `openspec/changes/supply-chain-hardening/specs/submodule-update-detection/spec.md`
- [x] 11.10 Update design.md, diff-scope-validation spec, proposal.md, and tasks.md

## 12. Integration Tests (follow-up)

- [ ] 12.1 Add integration test for classify pipeline with tampered PR (foreign commit author)
- [ ] 12.2 Add integration test for classify pipeline with suspicious files (`.claude/settings.json`)
- [ ] 12.3 Add integration test for classify pipeline with unexpected scope (Go source file in dependency PR)
- [ ] 12.4 Add integration test for merge phase blocking on supply-chain labels
- [x] 12.5 Add HTTP mock tests for `FetchPRCommits` and `FetchPRFiles` (pagination, errors, empty results, nil author)
