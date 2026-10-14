# Review panel — kan-980-no-flow-roles

Rendered from the store. Do not edit: the findings are rows, and the next render overwrites this file.

| ID | Slot | Severity | Location | Note | Lineage |
|---|---|---|---|---|---|
| F1 | primary | Important | internal/run/run.go:378 | The no-flow guard on standards resolution (&& !cfg.NoFlow) has no test: removing it leaves ./internal/run green, yet without it a no-flow principles trial gets flow's project standards copied into /case. |   |
| F2 | primary | Important | internal/harness/claude.go:178 | The forked-command fallback prices every cacheCreationInputTokens as a 1-hour write; the CLI's own total_cost_usd matches 5-minute writes exactly, so every no-flow review trial's cost is overstated ~30%. |   |
| F3 | primary | Minor | internal/run/run.go:391 | A no-flow impl trial's /case still receives the raw flow-plan task.md; only the prompt is plain, so decision no-flow-plain-spec is only half-held. |   |
| F4 | primary | Minor | internal/run/run.go:312 | A no-flow run's header records the flow prompts/PIN it never rendered; compare then prints a prompt-pin DIFFER warning that is noise. |   |
| F5 | principles | Minor | internal/compare/compare.go:189 | DRY: the ', no-flow' settings marker is defined twice, compare.noFlow(Run) and run.noFlowNote(Header), identical logic. |   |
| F6 | primary | Important | cmd/flow-eval/main.go:166 | No test covers the half of the change the CLI makes: --no-flow --role review --harness muse-code being accepted; a binary that still refuses muse-code passes the whole cmd/flow-eval suite. |   |
| F7 | primary | Minor | internal/run/run.go:44 | _noFlowPrep's doc still says the working tree's diff is what /code-review reviews; on muse-code the plain request reviews it. |   |
| F8 | principles | Minor | internal/run/run.go:470 | Which harness has a no-flow review is held twice — the CLI allowlist and noFlowTarget; run.Run accepts a codex no-flow review and silently renders review-plain. |   |
| F9 | primary | Minor | internal/run/run.go:974 | A no-flow run's report renders its empty prompt pin as a bare empty code span (prompt pin ``,). |   |

findings-total: 9
finding-status: F1 fixed
finding-status: F2 fixed
finding-status: F3 fixed
finding-status: F4 fixed
finding-status: F5 fixed
finding-status: F6 fixed
finding-status: F7 fixed
finding-status: F8 fixed
finding-status: F9 fixed

reproducers-total: 9
finding-reproducer: F1 .superpowers/sdd/reproducers/0-primary-2.sh
finding-reproducer: F2 .superpowers/sdd/reproducers/0-primary-1.sh
finding-reproducer: F3 .superpowers/sdd/reproducers/0-primary-3.sh
finding-reproducer: F4 .superpowers/sdd/reproducers/0-primary-4.sh
finding-reproducer: F5 .superpowers/sdd/reproducers/0-principles-1.sh
finding-reproducer: F6 .superpowers/sdd/reproducers/2-primary-1.sh
finding-reproducer: F7 none — doc wording, no behaviour
finding-reproducer: F8 none — a structural duplication; no current caller reaches run.Run with codex past the CLI refusal
finding-reproducer: F9 .superpowers/sdd/reproducers/4-P1-1.sh

## Pass log

### Round 0

- roster: compact — 9
- diff size: 983 lines, under cap — proceed
- docs-only: exit 1 — cmd/flow-eval/main_test.go; roster primary+principles dispatched
- no addition this round — the resolved list ran alone.
- standards: none resolved — no ## standards section
- citation check: no ## review panel citation check declared — skipped
- context bundle: rebuilt — no cached bundle

### Round 1

- fix-round 1: parent fixed F1-F5 inline (execution inline); F2 premise confirmed against the CLI reported cost (ratio 20.000 flow vs 15.4-16.6 no-flow); review run re-made as 20261014T152014Z (ratio 20.000); reproducers F1-F5 re-authored as behaviour checks after their text premises named the fixed lines, each proved by prove-reproducer.sh against 7c8d24c
- re-run: primary+principles on sonnet/low read fix-round-1.diff; F1-F5 fixed, no new finding
fix-mutation: internal/run/run.go — drop && !cfg.NoFlow from the principles standards guard — TestNoFlowPrepRunsBeforeTheAgent
fix-mutation: internal/run/run.go — keep task.md/context.md in a no-flow impl /case (names = names) — TestNoFlowPrepRunsBeforeTheAgent
fix-mutation: internal/run/run.go — read the prompt pin for a no-flow run (if true) — TestNoFlowRunsHaveTheirOwnKeyAndSection
fix-mutation: internal/compare/compare.go — warn on prompt pins against a no-flow side — TestCompareNamesANoFlowSide
fix-mutation: internal/harness/claude.go — count forked cache writes as CacheWrite1h — TestClaudeParseTakesAForkedCommandsTokensFromModelUsage
fix-mutation: internal/compare/compare.go — none — F5 is a pure dedup (NoFlowNote exported, compare calls it): no behaviour changed; TestCompareNamesANoFlowSide pins the output
fix-mutations-total: 6

### Round 2

- append scope: 217 changed lines since 397fc58
- roster: compact — 9
- docs-only: exit 1; roster primary+principles
- no addition this round — the resolved list ran alone.
- standards: none resolved

### Round 3

- fix-round 3: parent fixed F6-F8 inline; F6 reproducer first stayed demonstrated (the muse-code row asserted only the absent refusal phrase), test strengthened to assert case selection is reached; PROOF HELD vs 21d7866
- re-run: primary+principles on sonnet/low read fix-round-3.diff; F6-F8 fixed, no new finding
- citation send-back: F6 and F8 verdicts re-cited inside fix-round-3.diff hunks (main_test.go:205, run.go:185); all CITED
fix-mutation: cmd/flow-eval/main.go — CLI checks NoFlowReviewTarget("codex") for every harness (refuses muse-code and claude-code) — TestRunNoFlowReviewRefusesAHarnessWithoutOne
fix-mutation: cmd/flow-eval/main.go — an init() refusing --no-flow review on muse-code with its own message (reproducer 2-primary-1.sh) — TestRunNoFlowReviewRefusesAHarnessWithoutOne
fix-mutation: internal/run/run.go — Run ignores NoFlowReviewTarget error — TestNoFlowReviewNeedsAHarnessWithOne
fix-mutation: internal/run/run.go — none — F7 is a comment-only change
fix-mutations-total: 4

### Round 4

- append scope: 136 changed lines since 723b8b0 (the reshaped close; pre-reshape close shas no longer on the branch)
- roster: compact — 9
- diff size: 1566 lines, under cap — proceed; docs-only: exit 1 — cmd/flow-eval/main_test.go; roster primary+principles
- no addition this round — the resolved list ran alone.
- citation check: no ## review panel citation check declared — skipped; standards: none resolved
- fix: parent fixed F9 (Minor) inline in 5917f9c under the standing rule; TestReportNamesAnEmptyPromptPinNone fails on revert (RED before the fix); six no-flow run .md files rewritten. The reproducer's premise names run.go:974's old text, so it exits 2 on the fixed tree — the text-bound premise defect the self-review already filed
- fix-mutation꞉ internal/run/run.go — render h.PromptPin in a code span unconditionally — TestReportNamesAnEmptyPromptPinNone
