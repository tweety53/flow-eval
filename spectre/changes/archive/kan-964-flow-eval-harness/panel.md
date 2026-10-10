# Review panel — kan-964-flow-eval-harness

Rendered from the store. Do not edit: the findings are rows, and the next render overwrites this file.

| ID | Slot | Severity | Location | Note | Lineage |
|---|---|---|---|---|---|
| F1 | primary | Critical | internal/sandbox/sandbox.go:181 | Every trial, role and run of a repo shares one writable /root/.gradle; with org.gradle.caching=true a review trial's build puts the compiled held-out answer in the build cache impl trials read — answer leakage. |   |
| F2 | primary | Important | internal/run/run.go:418 | Grading has no deadline: a hung held-out test or judge call hangs the whole run. |   |
| F3 | primary | Important | internal/run/run.go:443 | Impl failures cannot be diagnosed: g.Reason is dropped, verify output discarded, and the agent's diff never saved, though design §8 keeps the agent's output. |   |
| F4 | primary | Important | internal/run/run.go:420 | A grading failure the agent caused (e.g. a leftover .git/index.lock) is recorded infra_error, which every aggregate skips, so an agent can drop its own failing trial. |   |
| F5 | primary | Important | cases/visual | The visual set ships 1 match and 12 mismatch, so an always-mismatch verifier scores 92 with 0 false passes; proposal.md and design.md still promise 10+10. |   |
| F6 | primary | Minor | internal/sandbox/sandbox.go:199 | One fixed warm-up container name per repo: two concurrent runs remove each other's warm-up container. |   |
| F7 | primary | Minor | internal/metrics/metrics.go:204 | An unpriced codex model is silently mispriced by substring match (gpt-5 charged at gpt-5.6-terra rates). |   |
| F8 | primary | Minor | README.md:87 | The README example names a -task-1 case that does not exist; only -task-3 is shipped. |   |
| F9 | principles | Critical | internal/sandbox/sandbox.go:181 | Least Privilege: trials get a shared writable Gradle home across trials, roles and runs. |   |
| F10 | principles | Minor | internal/sandbox/sandbox.go:169 | Visual trials mount the Gradle volume though they never build. |   |
| F11 | principles | Minor | internal/run/run.go | Both Claude credentials enter a trial when both are set. |   |
| F12 | principles | Minor | internal/grade/impl.go:58 | The grader's git flags are written twice (impl.go:58 and :62). |   |
| F13 | principles | Minor | internal/metrics/metrics.go | The token sum is computed at 3 sites and the role score in 2 packages. |   |
| F14 | principles | Minor | internal/sandbox/sandbox.go:325 | CopyOut is unused. |   |
| F15 | principles | Minor | internal/metrics/metrics.go | lint_failures is always written 0 though it is never computed. |   |
| F16 | principles | Minor | internal/cases/case.go | case.json is read with encoding/json v1 and written with v2; errors.As should be errors.AsType. |   |
| F17 | primary+principles | Minor | internal/metrics/metrics.go:207 | matches no longer accepts a date-suffixed served id for a full requested id (claude-sonnet-5-5 vs claude-sonnet-5-5-20260301), so pricedModel falls back to served[0]. |   |
| F18 | primary | Minor | internal/grade/judge.go:84 | The host-side judge reuses Claude.Command and so also gets IS_SANDBOX=1. |   |
| F19 | primary | Minor | cmd/flow-eval/main.go:197 | run's success line names the .jsonl and .md but not the .html page |   |
| F20 | primary | Minor | README.md:103 | README says artifacts are inline whole; the page cuts any over 256 KiB |   |
| F21 | principles | Minor | internal/run/run.go:553 | caseRow sits between report's doc comment and report; the comments merged and report lost its doc |   |
| F22 | primary | Minor | internal/run/index.go:77 | writeIndex globs the runs path unescaped: a runs dir with [ breaks the index or fails the run |   |
| F23 | principles | Minor | internal/run/index.go:21 | Load, the general run-record reader, sits in the HTML index file; its writer is in run.go |   |

findings-total: 23
finding-status: F1 fixed
finding-status: F2 fixed
finding-status: F3 fixed
finding-status: F4 fixed
finding-status: F5 fixed
finding-status: F6 fixed
finding-status: F7 fixed
finding-status: F8 fixed
finding-status: F9 fixed
finding-status: F10 fixed
finding-status: F11 fixed
finding-status: F12 fixed
finding-status: F13 fixed
finding-status: F14 fixed
finding-status: F15 fixed
finding-status: F16 fixed
finding-status: F17 fixed
finding-status: F18 withdrawn the judge runs with --tools "", so IS_SANDBOX=1 only lifts the root refusal of a process that can run no tool; isolation is unchanged
finding-status: F19 fixed
finding-status: F20 fixed
finding-status: F21 fixed
finding-status: F22 fixed
finding-status: F23 fixed

reproducers-total: 23
finding-reproducer: F1 .superpowers/sdd/reproducers/0-primary-1.sh
finding-reproducer: F2 .superpowers/sdd/reproducers/0-primary-2.sh
finding-reproducer: F3 .superpowers/sdd/reproducers/0-primary-3.sh
finding-reproducer: F4 .superpowers/sdd/reproducers/0-primary-4.sh
finding-reproducer: F5 .superpowers/sdd/reproducers/0-primary-6.sh
finding-reproducer: F6 .superpowers/sdd/reproducers/0-primary-5.sh
finding-reproducer: F7 .superpowers/sdd/reproducers/0-primary-8.sh
finding-reproducer: F8 .superpowers/sdd/reproducers/0-primary-7.sh
finding-reproducer: F9 .superpowers/sdd/reproducers/0-principles-1.sh
finding-reproducer: F10 .superpowers/sdd/reproducers/0-principles-2.sh
finding-reproducer: F11 .superpowers/sdd/reproducers/0-principles-3.sh
finding-reproducer: F12 .superpowers/sdd/reproducers/0-principles-4.sh
finding-reproducer: F13 .superpowers/sdd/reproducers/0-principles-5.sh
finding-reproducer: F14 .superpowers/sdd/reproducers/0-principles-6.sh
finding-reproducer: F15 .superpowers/sdd/reproducers/0-principles-7.sh
finding-reproducer: F16 .superpowers/sdd/reproducers/0-principles-8.sh
finding-reproducer: F17 none — a one-line matcher case, pinned by a unit test
finding-reproducer: F18 none — no tool-less behaviour changes
finding-reproducer: F19 .superpowers/sdd/reproducers/4-primary-1.sh
finding-reproducer: F20 .superpowers/sdd/reproducers/4-primary-2.sh
finding-reproducer: F21 .superpowers/sdd/reproducers/4-principles-1.sh
finding-reproducer: F22 .superpowers/sdd/reproducers/5-primary-1.sh
finding-reproducer: F23 none — file placement, no runtime behaviour

## Pass log

### Round 0

- roster: compact — decided (primary, principles)
- base: origin/main moved 2 commits, no overlap; rebased automatically onto 3f234c99
- diff size: 30524 lines, over cap; proceeded unasked (19k lines are case data under cases/)
- docs-only: no — first non-doc path .gitignore; roster dispatched: primary+principles
- no addition this round — the resolved list ran alone
- standards: none declared
- F1/F9 Critical premise confirmed against primary source: Gradle docs place the local build cache under the Gradle user home (caches/build-cache-1); gymie gradle.properties sets org.gradle.caching=true; the volume is mounted read-write at /root/.gradle

### Round 1

- round 1: panel-fix-1 (opus/medium, F1-F6,F9-F11,F14) and panel-fix-1-2 (opus/medium, F7,F8,F12,F13,F15,F16 + compare dual-sha + after-output infra boundary); reproducers re-authored twice for premise drift (repro-reauthor-1.md, -2.md), all 16 flipped; re-review primary+principles on sonnet/low read fix-round-1.diff, sent back once for skipping the hunk walk, raised F17 (Minor)
fix-mutation: internal/sandbox/sandbox.go — `,readonly` dropped from the trial mount — TestTrialGradleCacheIsReadOnlyAndPrivate
fix-mutation: internal/sandbox/sandbox.go — `,readonly` dropped (real Docker) — TestRealDockerGradleCacheIsReadOnlyAndPrivate
fix-mutation: internal/sandbox/sandbox.go — GRADLE_RO_DEP_CACHE env line removed — TestTrialGradleCacheIsReadOnlyAndPrivate
fix-mutation: internal/sandbox/sandbox.go — volume dst back to /root/.gradle (shared home) — TestTrialGradleCacheIsReadOnlyAndPrivate
fix-mutation: internal/sandbox/sandbox.go — wrapper seed exec replaced by `return nil` — TestTrialGradleCacheIsReadOnlyAndPrivate
fix-mutation: internal/sandbox/sandbox.go — warm script copies all of caches/ instead of modules-2 (real Docker) — TestRealDockerGradleCacheIsReadOnlyAndPrivate
fix-mutation: internal/sandbox/sandbox.go — warm-up `./gradlew testClasses` replaced by `true` (real Docker) — TestRealDockerGradleCacheIsReadOnlyAndPrivate
fix-mutation: internal/sandbox/sandbox.go — `if spec.Repo != ""` mount guard → `if true` — TestNoGradleVolumeWithoutRepo
fix-mutation: internal/sandbox/sandbox.go — validate's empty-Repo allowance removed — TestNoGradleVolumeWithoutRepo
fix-mutation: internal/sandbox/sandbox.go — warm-up name suffix emptied (`rand.Text()[:0]`) — TestWarmGradleNamesEachWarmUpApart
fix-mutation: internal/sandbox/sandbox.go — volume name back to flow-eval-gradle-<repo> — TestTrialGradleCacheIsReadOnlyAndPrivate
fix-mutation: internal/sandbox/sandbox.go — CopyOut removal — none — a deletion of an uncalled method; no behaviour
fix-mutation: internal/run/run.go — `if p.snapshot != ""` repo guard → `if true` — TestOnlyBuildingTrialsGetTheGradleVolume
fix-mutation: internal/run/run.go — grading deadline `timeout` → `time.Hour` — TestGradingHasADeadline
fix-mutation: internal/run/run.go — scored fail replaced by `return infra(err)` — TestAgentCausedGradeFailureIsScored
fix-mutation: internal/run/run.go — cancelled-run guard `if ctx.Err() != nil` → `if false` — TestRunCancelledDuringGradingIsInfra
fix-mutation: internal/run/run.go — judge pricing error no longer joined into err — TestJudgePricingFailureIsScored
fix-mutation: internal/run/run.go — `l.Error = g.Reason` dropped — TestImplFailureIsRecorded
fix-mutation: internal/run/run.go — work.patch written empty — TestImplFailureIsRecorded
fix-mutation: internal/run/run.go — verify.log written empty — TestImplFailureIsRecorded
fix-mutation: internal/grade/impl.go — verify output no longer appended to Log — TestImplFailureIsRecorded
fix-mutation: internal/grade/impl.go — diff -U0 → full context — none — tests_added still counted from added lines only; covered green by TestImplOverbuildRatio/TestImplCommittedWorkIsGraded (tests_added == 2)
fix-mutation: internal/metrics/metrics.go — class from visual.label dropped — TestRoleScoreBalancesVisualVerdicts
fix-mutation: internal/grade/visual.go — `v.Metrics.Label = labels.Verdict` dropped — TestVisualRoleScoreIsBalancedAccuracy (run)
fix-mutation: internal/run/run.go — report score back to the plain mean of case means — TestVisualRoleScoreIsBalancedAccuracy (run)
fix-mutation: internal/compare/compare.go — summarize score back to the plain mean of case means — TestVisualRoleScoreIsBalancedAccuracy (compare)
fix-mutation: cmd/flow-eval/main.go — firstCredential back to keep-every-set-name — TestOnlyTheFirstSetCredentialEntersATrial
fix-mutation: internal/metrics/metrics.go — `matches` back to `strings.Contains(served, requested)` — TestUnpricedModelIsNotPricedAsAnIdContainingIt, TestServedModelMismatch
fix-mutation: README.md — none — docs: the run example's case id only; no executable behaviour
fix-mutation: internal/grade/impl.go — `_resetHeldout` prefix `strings.Join(_git, " ")` → bare `git` — TestImplResetOfHeldoutPathsRunsNoAgentHook
fix-mutation: internal/metrics/metrics.go — `LintFailures` field re-added to `Work` — TestImplRecordCarriesNoUnmeasuredLintCount
fix-mutation: internal/metrics/metrics.go — `CacheWrite1h` dropped from `Aggregate.Tokens` sum — TestPassK
fix-mutation: internal/metrics/metrics.go — `Aggregate.WallMS` sum → `+= 0` — TestPassK
fix-mutation: internal/metrics/metrics.go — `Aggregate.Cost` sum → `+= 0` — TestPassK
fix-mutation: internal/metrics/metrics.go — `CostPerPass` no longer divides `a.Cost` — TestPassK
fix-mutation: internal/harness/harness.go — `CacheWrite1h` dropped from `Usage.Tokens` — TestUsageTokensCountsEveryClassOnce
fix-mutation: internal/run/run.go — judge check `judge.Tokens() > 0` → `> 1e9` — TestRunWritesHeaderAndTrials
fix-mutation: internal/run/run.go — report tokens column → 0 — TestRunWritesHeaderAndTrials
fix-mutation: internal/run/run.go — report cost column → 0 — TestRunWritesHeaderAndTrials
fix-mutation: internal/run/run.go — report mean wall → -1 — TestRunWritesHeaderAndTrials
fix-mutation: internal/compare/compare.go — row cost → 0 — TestNonInferiorWithinMargin
fix-mutation: internal/compare/compare.go — row mean wall → 0 — TestNonInferiorWithinMargin
fix-mutation: internal/compare/compare.go — row tokens → 0 — TestNonInferiorWithinMargin
fix-mutation: internal/compare/compare.go — `s.cost += cs.Cost` → `+= 0` — TestNonInferiorWithinMargin
fix-mutation: internal/compare/compare.go — `!t.InfraError` guard on false passes dropped — TestInfraErrorFalsePassIsNotCounted
fix-mutation: internal/compare/compare.go — `report` takes `d Decision` for verdict and sha line — none — parameter plumbing, same verdict value; the sha line is covered by the flow-eval rows below
fix-mutation: internal/cases/case.go — case.json decode + `MatchCaseInsensitiveNames(true)` — TestRejectInvalidFields
fix-mutation: internal/cases/case.go — `RejectUnknownMembers(true)` dropped — TestRejectInvalidFields
fix-mutation: internal/cases/case.go — whole v1→v2 change reverse-patched — TestRejectInvalidFields/key_in_another_case
fix-mutation: internal/cases/case.go — labels.json decode + `MatchCaseInsensitiveNames(true)` — TestRejectInvalidFields/labels_key_in_another_case
fix-mutation: internal/sandbox/sandbox.go — `errors.AsType` result `ok` → `!ok` — TestRunReportsANonZeroExitAsAResult
fix-mutation: cmd/flow-eval/main_test.go — `case isExit` → `case !isExit` — TestUnknownCommandExits2
fix-mutation: internal/mine/mine_test.go — none — test-only decode change (v2 plus explicit names); exercised green by TestMineReview*/TestMineVisualCopiesImages
fix-mutation: internal/run/run.go — `output.md` write failure `infra` → `fail` — TestHarnessSideFailureAfterOutputIsInfra/output.md_cannot_be_written
fix-mutation: internal/run/run.go — trial pricing failure `infra` → `fail` — TestHarnessSideFailureAfterOutputIsInfra/the_trial's_model_is_not_priced
fix-mutation: internal/run/run.go — egress-count failure `infra` → `fail` — TestHarnessSideFailureAfterOutputIsInfra/egress_denials_cannot_be_counted
fix-mutation: internal/run/run.go — `infraFault{werr}` → bare `werr` — TestHarnessSideFailureAfterOutputIsInfra/work.patch_cannot_be_written
fix-mutation: internal/run/run.go — `AsType[infraFault]` check inverted (`!ok && err != nil`) — TestHarnessSideFailureAfterOutputIsInfra/work.patch_cannot_be_written
fix-mutation: internal/run/run.go — every grading error made infra (`ok || err != nil`) — TestJudgePricingFailureIsScored, TestAgentCausedGradeFailureIsScored
fix-mutation: internal/compare/compare.go — DIFFER guard → `if false` — TestFlowEvalShasArePrintedAndADifferenceFlagged/different_sha
fix-mutation: internal/compare/compare.go — DIFFER guard → `if true` — TestFlowEvalShasArePrintedAndADifferenceFlagged/same_sha
fix-mutation: internal/compare/compare.go — report's flow-eval bullet removed — TestFlowEvalShasArePrintedAndADifferenceFlagged
fix-mutation: cmd/flow-eval/main.go — `d.FlowEval` dropped from stdout — TestCompareAndBaselineExitCodes
fix-mutation: README.md — none — docs: infra wording, compare sha behaviour, judge credential
fix-mutation: internal/metrics/metrics.go — undated(served)==requested dropped from matches — TestServedModelMismatch
fix-mutation: internal/metrics/metrics.go — undated price fallback removed — TestServedModelMismatch
fix-mutation: internal/metrics/metrics.go — date suffix regex {8} -> * — TestServedModelMismatch
fix-mutations-total: 67

### Round 2

- round 2: F17 Minor-only, fixed inline by the parent (f7a2691), mutation-proved 3/3 by break-and-prove; no re-run slot (Minor-only inline fix)

### Round 3

- late-fix reduction: 63 changed lines since 389d0b0

### Round 4

- append scope: 563 changed lines since 5b63d11
- round 4 (append scope, primary+principles opus/high): 3 Minors F19-F21, fixed inline by the parent (676f926); all three reproducers flipped
fix-mutation: cmd/flow-eval/main.go — none — the success line's text; no test drives a real run's CLI output — none — 4-primary-1.sh flips
fix-mutations-total: 1

### Round 5

- append scope: 504 changed lines since 2127db6
- round 5 (append scope, primary+principles opus/high): 2 Minors F22-F23 fixed inline (67f15be); 5-primary-1.sh re-anchored (premise drift from the Load move), proof held 0bd0a08 → HEAD
fix-mutation: internal/run/index.go — ReadDir .jsonl suffix filter dropped — TestIndexReadsARunsPathHoldingPatternCharacters, TestRunWritesIndexOfEveryRun
fix-mutation: internal/run/run.go — none — Load moved unchanged from index.go — none — TestCompare*/TestRunWritesIndexOfEveryRun cover it
fix-mutations-total: 2
