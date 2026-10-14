# kan-979-tune-failure-modes-prompt — session narrative

## 2026-10-14 — creating run

- Resumed at implement; inline execution. Before run haiku-low k=6: 0/6. Variant 1 (`0502293c`) 1/6: every trial filled the Failure-recovery check, but most judged `chooseUnit`'s `:295` clear a full recovery ("mode untouched") or never grepped past the diff hunks. Variant 2 (`c0372d03`: whole-file grep, a re-confirming line or `none` per clearing line × unconfirmed value) 4/6 — stop-early bar met, no variant 3.
- The landed agents commit first lacked its `Task-Id: 2` trailer; re-committing it needed a force-push of the change's own agents branch, which the auto-mode classifier blocked until the operator approved it.
- Operator gates: the operator refused the cross-model ask before seeing the haiku results printed, then asked for the prompt diff, then replaced task 4's sonnet/opus k=6 with haiku/sonnet/opus k=3 (2/3, 3/3, 3/3). They then added task 6: delete every k=6 run and every none-passed run across roles (14), and set the variant-2 opus-low run as the failure-modes baseline — `baselines-unchanged` superseded.
- The panel's pre-work rebased the agents branch automatically (5 upstream commits, no overlap), so the vendored PIN `39347524` is the pre-rebase sha; integrate re-syncs PIN to the landed sha (same bytes, verified by the primary reviewer).
- In-run pipeline fix: `render-slot-prompt.sh` could not render an `exp-` slot; the exp-test-sanity brief was typed beside the rendered mutation brief for this panel, and the renderer fix landed on agents main.
- Panel: 7 Minors. Task 6 had left six compare reports naming deleted runs (deleted); the worked example cited the pre-rebase sha (now cites the kept variant commit `c0372d03`); the render test now pins the failure-recovery step (mutation-proved). Three test-gap Minors withdrawn: PIN, baseline and index are only ever written by `prompts sync`, `baseline set` and the run.

## 2026-10-14 — fix run 1

- Operator fix: delete `20261013T121524Z-review-claude-code-opus-low` (the baseline task 6 replaced) and stop ignoring `runs/`. Appended as task 7: the run's three files deleted and the index repainted; `runs/*/` dropped from `.gitignore`; the transcript directories of the 11 still-tracked runs the main checkout held copied in and committed; four wordings that called the directory gitignored reworded. Six older tracked runs have no directory anywhere — the page-copy fallback covers them.
- Panel (small append, `primary` on opus): 2 Minors — a fifth stale "gitignored" comment in `run_test.go` and an overlong README line — both fixed.

## 2026-10-14 — integrate run

- Preflight and the unfinished-work gate passed; route merge and push, from the project default. A stray untracked `.DS_Store` in the flow-eval worktree was removed before the reshape.
