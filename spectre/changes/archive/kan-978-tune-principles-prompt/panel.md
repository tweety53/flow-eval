# Review panel — kan-978-tune-principles-prompt

Rendered from the store. Do not edit: the findings are rows, and the next render overwrites this file.

| ID | Slot | Severity | Location | Note | Lineage |
|---|---|---|---|---|---|
| F1 | primary | Minor | flow-eval:docs/prompt-tuning.md:155 | The worked example says three trials judged a writer partial; four did, and the judge credited three. |   |
| F2 | principles | Minor | agents:skills/flow/principles-reviewer-prompt.md:31 | The new step says "done before anything else", which conflicts with "Read the principles FIRST … then the diff" just above it (Least Astonishment). |   |
| F3 | principles | Minor | agents:skills/flow/principles-reviewer-prompt.md:42 | The examples "something failed", "any unconfirmed" sit close to the case must-find F6, against the guide no-memorisation rule. |   |
| F4 | primary | Minor | internal/run/regrade.go:70 | A re-graded trial whose Error was "grade: X; stop sandbox: Y" has its whole Error cleared, losing the sandbox-stop failure. |   |
| F5 | primary | Minor | internal/run/page.go:304 | pageArtifact looks for the literal section id, but the template escapes it, so a case id holding + & quote or angle brackets is never found. |   |
| F6 | primary | Minor | internal/run/regrade.go:98 | The page copy may be a body cut at the artifact cap or an unreadable placeholder; trialOutput grades it as the report. |   |
| F7 | primary | Minor | spectre/changes/kan-978-tune-principles-prompt/tasks.md:155 | Task 6 Step 3 names the flag --cases; the code (correctly, matching run) uses --root. |   |
| F8 | principles | Important | internal/run/regrade.go:73 | DRY / Single Source of Truth: Regrade re-implements how header() computes the case hashes and how runTrial prices judge usage. |   |
| F9 | principles | Minor | internal/run/regrade.go:96 | trialOutput answers the report and also writes output.md; since task 8 page() keeps artifacts, so the write-back is redundant and contradicts "Nothing is written unless every trial re-grades". |   |
| F10 | principles | Minor | cmd/flow-eval/main.go:400 | regradeCmd repeats runCmd judge-credential check, pricing and judge loading, git sha and case loading. |   |
| F11 | primary | Minor | internal/run/run.go:208 | O_EXCL comment still defines a colliding id without the review slot task 9 added |   |
| F12 | principles | Minor | internal/run/run.go:185 | role-slot qualification duplicated from BaselineKey; build the header first and use BaselineKey(h.Role, h.Slot) |   |
| F13 | primary | Important | internal/grade/review.go:244 | an unmatched Minor is upside-only: plausible earns +5, implausible costs nothing, so padding Minors is a free +15 |   |
| F14 | primary | Minor | internal/grade/review.go:243 | the labelled defect re-reported outside its window counts as an extra |   |
| F15 | primary | Minor | internal/grade/review.go:102 | a bold-only label drops its items only after a bold-led finding |   |
| F16 | principles | Minor | internal/grade/review.go:102 | Least Astonishment: the same bold-only label block is a label after a bold-led finding and findings after a list-led one |   |
| F17 | principles | Minor | internal/metrics/metrics.go:116 | DRY: the cap value 'up to 15' is restated away from _extraBonus/_maxExtras |   |

findings-total: 17
finding-status: F1 fixed
finding-status: F2 fixed
finding-status: F3 withdrawn the examples are the decided variant-1 wording the 3/6 measures and name generic categories, no case identifier; swapping them voids the measurement this change records
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

reproducers-total: 17
finding-reproducer: F1 none — prose about free-text trial output
finding-reproducer: F2 none — two phrases in prose; see panel-report-0-principles.md
finding-reproducer: F3 none — whether wording overfits is a judgment
finding-reproducer: F4 none — needs a stored trial whose grading and sandbox stop both failed
finding-reproducer: F5 .superpowers/sdd/reproducers/1-primary-1.sh
finding-reproducer: F6 none — needs a review report over 256 KiB or an unreadable output.md at render time
finding-reproducer: F7 none — a wording mismatch in the plan
finding-reproducer: F8 .superpowers/sdd/reproducers/1-principles-1.sh
finding-reproducer: F9 none — a structural redundancy
finding-reproducer: F10 none — duplication visible by grep
finding-reproducer: F11 .superpowers/sdd/reproducers/3-primary-1.sh
finding-reproducer: F12 none — structural duplication, no behaviour differs
finding-reproducer: F13 .superpowers/sdd/reproducers/5-primary-1.sh
finding-reproducer: F14 .superpowers/sdd/reproducers/5-primary-2.sh
finding-reproducer: F15 .superpowers/sdd/reproducers/5-primary-3.sh
finding-reproducer: F16 .superpowers/sdd/reproducers/5-primary-3.sh
finding-reproducer: F17 none — a comment's drift risk

## Pass log

### Round 0

- roster: compact — 88
- no addition this round — the resolved list ran alone.
- diff size: flow-eval 1878 + agents 33 = 1911 lines, under cap
- docs-only: exit 1 — flow-eval prompts/PIN, agents spectre/changes/kan-978-tune-principles-prompt/verbatim-moves.txt; roster primary+principles dispatched
- standards: none resolved (flow-eval declares no ## standards)
- citation check: agents check-references.sh captured; flow-eval declares none

### Round 1

- append scope: 3822 changed lines since 91c3e88 (agents: none since 2441dced)
- no addition this round — the resolved list ran alone.
- roster: compact — 88
- diff size: over cap (runs/*.html and .jsonl re-graded records dominate); proceeded unasked

### Round 2

- re-run: primary (F4–F7) and principles (F8–F10) on fix-round-1.diff, rerun pair sonnet/low

### Round 3

- append scope: 1563 changed lines since 61102f2
- diff size: 7271 changed lines since the merge base, over the cap — proceeded unasked
- docs-only: exit 1 — baselines/review-principles.json; roster primary+principles (opus/medium), one bundled dispatch
fix-mutation: internal/run/run.go — BaselineKey(h.Role, h.Slot) → h.Role in the run id — TestReviewGradedOnItsSlotsMustFinds/a_review_run's_id_names_its_slot
fix-mutations-total: 1

### Round 5

- append scope: 5149 changed lines since 9f7929a (296 outside runs/)
- diff size: over the cap since the merge base — proceeded unasked
- docs-only: exit 1; roster primary+principles (opus/medium), one bundled dispatch; round 5, since round 4 was the prior run's re-run key

### Round 6

- panel-fix 5 finished by the parent after the fix subagent hit a usage limit mid re-grade; reproducers 5-primary-1 and 5-primary-3 re-authored (premises pinned the pre-fix code) and proved both directions against 71c569d
- round 6: primary+principles re-review of fix-round-6.diff on sonnet/low
