# Self-review: kan-980-no-flow-roles

**Deferred:** reasoning pass run on Opus 5.5 from the in-run bundle; the pass covers what the bundle holds and nothing beyond it

## Problems encountered, and what pipeline change would avoid them — `flow-fix`

- **[flow-fix]** `flow record pass` accepted a note opening with a marker label, so round 4's F9 mutation proof became free text (its colon rewritten to U+A789) with no `fix-mutation` row; the CLI now refuses such a note and names `flow record mutation` / `flow record finding` — fixed: 491d7a3f
- **[flow-fix]** `review-panel-late-fix.md` defined the since-close sha only as the last clean close's sha, which integrate's reshape rewrites; the fix run improvised the archive commit. It now names the archive commit (a peer's reshaped branch head) — fixed: b53834b0
- **[flow-fix]** The panel close and verify began before the citation send-back returned; `review-panel-fix-round.md` now holds the close until the `-cite` report passes `check-rerun-citations.sh` — fixed: c8ad5fdf
- **[flow-fix]** F9's reproducer bound to `run.go:974`'s old text (the `pcPremiseAudit` gap) — already filed by the first pass, not re-classified — noted
- **[flow-fix]** Heredoc writes with relative paths from the main checkout were refused by the main-checkout guard, which worked as designed — noted

## Token/time cost, and what would reduce it without quality loss — `flow-cost`

- **[flow-cost]** Inline-execution rows carried high cache reads (panel-fix-1 6.2M, Tasks 1 and 3 3.3M each) — the parent's context re-read per turn, no defect — noted
- **[flow-cost]** `prepare`'s output was tail-truncated by the agent's own read, so ROUTE was re-read through `project-get.sh` — noted

## What went well, and how to reproduce it — `flow-improvement`

- **[flow-improvement]** `check-plan-shape.sh`, the mutation proof (F6's surviving mutant), `prove-reproducer.sh` and `check-rerun-citations` each caught a real defect; the resolved default route skipped the landing ask — noted

## What could be automated or moved to a script — `flow-automation`

None — beyond the since-close fix above.

## What could move to the Go app or its persistent storage — `flow-stats-app`

None — beyond the pass-note refusal above, which keeps mutation proofs as store rows.

## What can be sped up — `flow-speed`

- **[flow-speed]** Task 7's impl and visual runs ran serially, as the plan required — a project plan choice — noted

**Rating:** 4 — good
