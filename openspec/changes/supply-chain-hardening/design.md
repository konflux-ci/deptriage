## Context

Supply chain attacks targeting GitHub repositories are a growing threat. Attackers can tamper with automated dependency PRs (Renovate/MintMaker) by pushing additional commits that contain malicious payloads hidden in files GitHub collapses by default. With deptriage's auto-merge capability, a tampered PR that passes CI and semver classification could merge without any human ever seeing the malicious code.

The current classify pipeline checks semver bump type, risk hints (Go toolchain, container images), and packages — but it does NOT validate who authored the commits, what files were changed, or whether the diff scope matches a legitimate dependency update.

## Goals / Non-Goals

**Goals:**
- Block auto-approve and auto-merge for any PR where the commit author doesn't match the PR opener bot
- Block auto-approve and auto-merge for PRs that touch known malicious file paths (`.claude/`, `.vscode/`)
- Block auto-approve and auto-merge for PRs that touch files outside the expected dependency update scope
- Surface supply-chain concerns via labels and risk hints so human reviewers are alerted
- Make the detection patterns configurable (CLI flags) so new attack vectors can be added without code changes
- Integrate cleanly into the existing classify pipeline and risk-hint system

**Non-Goals:**
- Malware scanning or static analysis of file contents (that's a separate tool's job)
- Blocking PRs from merging entirely — deptriage signals risk, humans decide
- Validating commit signatures or GPG keys — GitHub has built-in signature verification (GPG/SSH/S/MIME) and repo admins can enforce "Require signed commits" via branch protection rulesets. Signature verification proves cryptographic identity but not intent: an attacker with a valid key produces "Verified" commits under their own identity. Our author validation check is more targeted — it detects commits from *any* identity other than the expected bot, signed or not. The two defenses are complementary and independent.
- Detecting social engineering attacks that don't manifest in the PR metadata

## Decisions

### 1. Four-layer validation: author, files, scope, submodules

The supply-chain checks are split into four independent validators that each produce a risk hint:

1. **Author validation** — Is the PR from a known bot, and did that same bot author all commits?
2. **Suspicious file detection** — Does the PR contain files on a blocklist?
3. **Diff scope validation** — Does the PR only touch files expected for a dependency update?
4. **Submodule update detection** — Does the PR update git submodules (requiring engineer review)?

Each validator runs independently and produces its own risk hint and label. This keeps the logic testable, composable, and easy to extend. A PR can fail multiple validators.

**Alternatives considered:**
- Single monolithic check — harder to test, harder to explain which specific concern was triggered
- Only check author — misses the case where a legitimate bot account is compromised

### 2. Author validation: bot allowlist + commit author comparison

Maintain a default allowlist of known dependency bot logins: `renovate[bot]`, `red-hat-konflux[bot]`, `dependabot[bot]`. The PR author (user who opened the PR) must be on this list. Then, fetch the commits on the PR and verify that every commit's author login matches the PR opener.

The allowlist is configurable via a `--trusted-bot` flag (repeatable) that adds to (not replaces) the defaults, to support teams with custom Renovate instances.

The commit check uses the GitHub Commits API: `GET /repos/{owner}/{repo}/pulls/{number}/commits`. We compare each commit's `author.login` (the GitHub user identity) against the PR opener's login.

**Alternatives considered:**
- Check only the latest commit — a sophisticated attacker could place the malicious commit earlier in the stack and add a clean bot commit on top
- Check committer instead of author — bots typically set both, but author is the more reliable field since GitHub sets committer to `web-flow` for UI merges
- Rely on MintMaker's built-in warning — not all repos use MintMaker; deptriage must be self-sufficient

### 3. Suspicious file detection: path-prefix blocklist

Check the list of changed files (via GitHub Pull Request Files API: `GET /repos/{owner}/{repo}/pulls/{number}/files`) against a blocklist of path prefixes:

Default blocklist:
- `.claude/` — known malware vector (e.g., `.claude/settings.json` with `"command": "node .claude/setup.mjs"`)
- `.vscode/` — known attack vector
- `.github/workflows/` — CI/CD tampering
- `.github/actions/` — CI/CD tampering

Any match triggers the `supply-chain/suspicious-files` risk hint. The blocklist is configurable via a `--suspicious-path` flag (repeatable) that adds to the defaults.

The check also flags executable scripts (`.sh`, `.mjs`, `.js`, `.py`, `.rb`) found outside vendored directories (`vendor/`, `third_party/`, `node_modules/`). Dependency updates should not introduce new executable scripts.

**Alternatives considered:**
- Content-based scanning (grep for `eval`, `exec`, etc.) — too many false positives, and attackers can obfuscate. File path detection is higher signal-to-noise.
- Only block `.claude/` — too narrow; the next attack will use a different directory
- Block ALL non-manifest files — too aggressive for an initial implementation; diff scope validation handles the broader case with an allowlist approach

### 4. Diff scope validation: expected-file allowlist

For dependency PRs (identified by the PR being from a bot on the trusted list), validate that every changed file matches an expected pattern for a dependency update:

Default expected patterns:
- `go.mod`, `go.sum` — Go dependency manifests
- `Dockerfile`, `Containerfile`, `*.Dockerfile` — container image references
- `vendor/` — vendored dependencies (entire directory)
- `.tekton/` — Tekton task/pipeline refs (digest bumps)
- `renovate.json`, `.renovaterc*` — Renovate config (Renovate sometimes updates its own config)
- `package.json`, `package-lock.json`, `yarn.lock` — Node.js manifests (if applicable)
- `requirements.txt`, `poetry.lock`, `Pipfile.lock` — Python manifests (if applicable)
- `.github/workflows/` — **only when the PR is from a trusted bot AND the only workflow changes are version/digest bumps in `uses:` lines** (handled by a targeted check, not the broad blocklist from Decision 3)

Files outside this allowlist trigger `supply-chain/unexpected-scope`. The allowlist is configurable via a `--expected-file` flag (repeatable) that adds to the defaults.

Note: `.github/workflows/` appears in both the suspicious-files blocklist (Decision 3) and the expected-files allowlist (this decision). The blocklist fires unconditionally as a warning signal. The scope validator provides a more nuanced check: workflow files are "expected" only if the changes are limited to version/digest bumps in `uses:` directives. This dual approach means workflow changes are always flagged (suspicious-files label) while also being evaluated in context (scope validation may or may not flag depending on the nature of the change).

**Alternatives considered:**
- Block all non-manifest files — same as above
- Only check file count — a single malicious file in a 50-file dependency update is the exact attack vector
- Use a heuristic ("if >5 non-manifest files, flag") — arbitrary threshold, easy to game

### 5. Submodule update detection

Repositories that track upstream projects as git submodules (e.g., `konflux-ci/oauth2-proxy`) produce legitimate dependency PRs that change `.gitmodules` and a submodule pointer file. Without special handling, these trigger `supply-chain/unexpected-scope` — a false positive with alarming "tampering" language.

However, submodule updates bring in entire upstream codebases. Passing CI alone does not guarantee there are no incompatible or dangerous changes. These PRs must still require human review.

**Solution: detect and re-classify, don't suppress.**

For every bot PR:
1. Fetch the repository tree at the PR's head ref via the Git Trees API (`GET /repos/{owner}/{repo}/git/trees/{ref}`)
2. Identify entries with mode `160000` (gitlink) — these are submodule paths
3. Add those paths to the expected-files allowlist so `ValidateDiffScope` does not flag them as `unexpected-scope`
4. If any changed files match submodule paths, emit a `SUPPLY_CHAIN_SUBMODULE_UPDATE` finding — this blocks auto-approve/auto-merge with accurate messaging

The tree fetch runs for all bot PRs, not only when `.gitmodules` is in the diff, because submodule pointer bumps (the common case) only change the gitlink commit SHA without modifying `.gitmodules`.

`.gitmodules` itself is added to `defaultExpectedPatterns` since it is a standard dependency manifest (analogous to `go.mod`).

The new `supply-chain/submodule-update` label uses **yellow** color (`fbca04`) instead of red — it is a caution requiring human review, not an attack indicator. It still blocks all merge paths via the existing `supply-chain/` prefix check.

**Fail-open on Trees API errors:** If the tree fetch fails, submodule detection is skipped and diff scope validation proceeds normally. The worst case is the pre-existing false `unexpected-scope` label. This is acceptable because the scope validator itself remains fail-closed.

**Known limitations:**
- **Nested submodules are not detected.** The tree fetch uses `recursive=false`, so only top-level gitlinks are discovered. Nested submodules (a submodule inside a submodule) would be missed and flagged as `unexpected-scope`. This is acceptable because Konflux repos do not use nested submodules, and switching to `recursive=true` introduces truncation risk on large repositories.
- **Submodule removals/renames are not detected.** Only the PR head tree is inspected, so a submodule path that was removed or renamed by the PR would not be found. This is acceptable because Renovate/MintMaker only bumps submodule versions — it does not remove or rename submodules. A bot PR that removes a submodule would be unusual enough to warrant the `unexpected-scope` flag.

**Alternatives considered:**
- Only add `.gitmodules` to defaults and let users configure submodule paths via `--expected-file` per repo — requires per-repo configuration for a common pattern
- Suppress the finding entirely and allow auto-merge — unsafe; submodule updates can introduce breaking or malicious upstream changes that CI won't catch
- Parse `.gitmodules` file content to find submodule paths — more complex, requires an additional API call to fetch file contents; the Trees API is simpler and more reliable
- Fetch the tree recursively (`recursive=true`) to discover nested gitlinks — adds truncation handling complexity for a scenario that doesn't occur in Konflux repos
- Union base+head trees to catch submodule removals/renames — adds a second API call for a scenario that doesn't occur with dependency bot PRs

### 6. Integration point: classify pipeline, before auto-approve

The supply-chain checks run in `classify.Run()` after PR fetch, semver detection, and package extraction, but BEFORE auto-approve label application. This means:

```
PR event → FetchPR → DetectBumpType → ExtractPackages → DetectRiskHints
         → ValidateAuthor → DetectSuspiciousFiles → ValidateDiffScope
         → Apply labels → Auto-approve decision → Write result
```

Supply-chain risk hints use the same mechanism as existing risk hints (Go toolchain, container image) — they prevent auto-approve in the classify phase. However, unlike Go toolchain hints, **supply-chain risk hints are NOT eligible for deferred approval** in the merge phase. Passing CI does NOT prove a tampered PR is safe.

This is implemented by checking for the `supply-chain/` label prefix in the merge subcommand's deferred-approval logic and excluding those PRs.

### 7. Supply-chain labels block ALL merge paths

Supply-chain concerns must block merge through every code path, not just deferred approval:

- **`isMergeEligible` (primary path):** Extended to reject PRs with any `supply-chain/*` label, even if `approved`/`lgtm` labels are present. This handles the case where labels were applied before tampering was detected, or were manually added.
- **`isDeferredApprovalEligible` (deferred path):** Extended to reject PRs with any `supply-chain/*` label. Unlike `risk-hint/*` labels where passing CI proves safety, supply-chain concerns indicate potential tampering that CI cannot validate.
- **`submitReview` in analyze:** Skips formal `APPROVE` review when `ClassifyResult.SupplyChainFindings` is non-empty, even if the LLM assesses LOW risk. This prevents the AI from overriding the deterministic supply-chain checks.
- **`tryMerge` in analyze (inline path):** Skips the inline merge attempt when supply-chain findings exist.

- **Label application fallback in classify:** If applying a `supply-chain/*` label fails (API error, permissions), the classify phase removes any existing `approved`/`lgtm` labels as a fallback. This ensures that a tampered PR cannot remain merge-eligible due to a transient label write failure.

**Alternatives considered:**
- Only block deferred approval — insufficient. A PR with stale `approved`/`lgtm` labels (from a run before tampering) would still merge via the primary path.
- Only block in classify — insufficient. The analyze phase has its own APPROVE and merge logic that operates independently.
- Rely solely on labels for merge gating — insufficient. If label application fails and `approved`/`lgtm` remain, the merge phase sees no supply-chain signal. Removing the approval labels closes this gap.

### 8. Supply-chain findings in ClassifyResult and LLM context

Supply-chain findings are serialized as `SupplyChainFindingResult` structs in `ClassifyResult.SupplyChainFindings`. This serves three purposes:

1. **Analyze phase reads them** to gate APPROVE reviews and inline merge attempts
2. **Context JSON includes them** so the LLM sees structured findings (mismatched SHAs, flagged paths) alongside the existing risk hints
3. **Classify JSON file preserves them** for any downstream tooling that reads the output

Each finding includes a `Key` (e.g., `SUPPLY_CHAIN_AUTHOR_MISMATCH`), `Label`, `Message`, and `Details` (specific commit SHAs or file paths). Fail-closed errors use a distinct key `SUPPLY_CHAIN_VERIFICATION_FAILED` to distinguish "could not verify" from "verified and found mismatch" in ops triage.

### 9. New label namespace: `supply-chain/*`

Four labels in the `supply-chain/` namespace:

- `supply-chain/author-mismatch` — PR commit author doesn't match the bot that opened the PR (red `e11d48`)
- `supply-chain/suspicious-files` — PR contains changes to known attack vector paths (red `e11d48`)
- `supply-chain/unexpected-scope` — PR contains changes outside expected dependency update scope (red `e11d48`)
- `supply-chain/submodule-update` — PR updates git submodules requiring engineer review (yellow `fbca04`)

Red color distinguishes attack indicators from yellow `risk-hint/*` labels. The `submodule-update` label uses yellow because it is a caution (legitimate pattern requiring review), not an attack indicator. All four labels block auto-approve and auto-merge via the `supply-chain/` prefix check.

### 10. Project layout for new code

```
internal/
├── classify/
│   ├── supply_chain.go        # ValidateAuthor, DetectSuspiciousFiles, ValidateDiffScope
│   ├── supply_chain_test.go   # Unit tests
│   └── classify.go            # Modified: integrate supply-chain checks, populate findings
├── analyze/
│   ├── analyze.go             # Modified: gate APPROVE and merge on supply-chain findings
│   └── context.go             # Modified: pass supply-chain findings to ContextJSON
├── merge/
│   ├── merge.go               # Modified: block supply-chain/* in both eligibility checks
│   └── merge_test.go          # Modified: supply-chain label tests
├── github/
│   └── pr.go                  # Modified: add FetchPRCommits, FetchPRFiles, FetchSubmodulePaths methods
└── types/
    └── types.go               # Modified: SupplyChainFindingResult, ClassifyResult, ContextJSON
```

### 11. Configuration via CLI flags

New flags on the `classify` and `both` subcommands:

- `--trusted-bot` (repeatable string): additional bot logins to trust (added to defaults)
- `--suspicious-path` (repeatable string): additional path prefixes to block (added to defaults)
- `--expected-file` (repeatable string): additional expected file patterns for scope validation (added to defaults)

These are also exposed as GitHub Action inputs (`trusted-bots`, `suspicious-paths`, `expected-files`) with comma-separated values, mapped to `INPUT_TRUSTED_BOTS`, `INPUT_SUSPICIOUS_PATHS`, `INPUT_EXPECTED_FILES` env vars.

No flag is needed to enable/disable the supply-chain checks — they always run. The risk is too high to make this opt-in.

## Risks / Trade-offs

**[False positives on bot identity]** → Some organizations run custom Renovate instances with non-standard bot names. Mitigation: `--trusted-bot` flag allows adding custom bot logins. Default list covers the standard Konflux bots.

**[Legitimate workflow changes in dependency PRs]** → Renovate sometimes updates GitHub Actions workflow files (e.g., bumping action versions). These will trigger `supply-chain/suspicious-files`. Mitigation: this is the correct behavior — workflow changes in dependency PRs should be reviewed by a human, even when legitimate. The PR is not blocked from merging; it just loses auto-merge eligibility.

**[Performance impact]** → Two additional API calls per PR (commits list, files list). Mitigation: both are lightweight paginated calls; dependency PRs typically have 1-3 commits and 2-10 changed files. Total additional latency: ~200ms.

**[Sophisticated attackers who compromise the bot account itself]** → If the Renovate/MintMaker bot credentials are stolen, the attacker's commits would pass author validation. Mitigation: diff scope validation and suspicious file detection still apply. This defense-in-depth approach means compromising the bot account alone is not sufficient — the attacker must also restrict their changes to expected file patterns, which significantly limits the attack surface.

**[Vendored dependency attacks]** → The `vendor/` directory is on the expected-files allowlist, so a malicious vendored file would pass scope validation. Mitigation: vendored code changes are inherently large and visible in the diff; the existing risk-hint system and LLM analysis provide additional scrutiny. Future work could add content-based scanning for vendored code.

## Future Work: LLM-Assisted Supply-Chain Analysis

The deterministic checks in this change (author validation, suspicious file detection, diff scope validation) address the immediate threat — tampered PRs with clear structural signals. However, a sophisticated attacker who compromises a bot account and restricts changes to expected files would pass all three validators. A subtly malicious `go.mod` change (e.g., swapping a legitimate module for a typosquatted one) would look structurally identical to a normal dependency update.

A future iteration should enrich the LLM analysis prompt with supply-chain context to catch these subtler attacks:

1. **Include validation results in the LLM context** — tell the LLM whether author validation passed, what files changed, and whether any were outside expected scope. This gives the LLM grounding to flag anomalies even when the deterministic checks pass.

2. **Add supply-chain awareness to the prompt template** — instruct the LLM to look for:
   - Typosquatting (module paths that are suspiciously similar to well-known packages)
   - Packages with very low adoption, recent creation dates, or no prior usage in the Konflux ecosystem
   - Unusual version jumps (e.g., jumping from v1.2.3 to v1.2.3-beta.1 or a completely different fork)
   - Module path changes that redirect to a different repository

3. **Include the changed file list in the LLM context** — even when scope validation passes, the LLM can spot oddities like a patch bump touching 30 vendored files or a digest bump that also modifies `go.mod` replace directives.

**Important caveat:** The LLM is the weakest link for security decisions. It is susceptible to prompt injection — the very changelogs and PR bodies it reads could contain adversarial text designed to make it output "LOW risk." LLM-based supply-chain analysis should be treated as a **layered signal** (informational, like the current risk assessment), never as a gate. The deterministic checks from this change remain the authoritative safety mechanism.
