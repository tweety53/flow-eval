# kan-980-no-flow-roles — session narrative

## 2026-10-14 — creating run

Resumed at `STARTED` with the plan ready; implemented inline (decision `small`, `execution: inline`).

- **json/v2 and `omitempty`.** The plan declared `Header.NoFlow` as `omitempty`; under `encoding/json/v2`, which `internal/run` uses, `omitempty` never omits a `false` bool, so a flow run's header would have carried `"no_flow":false`. Shipped as `omitzero` (Task 1 correction).
- **Build-green across tasks.** `BaselineID`'s new parameter broke `cmd/flow-eval`'s one caller, so Task 2's one-line `compareCmd` change moved into Task 1. design.md's compare-report `no-flow` marker had no task; Task 1 carried it.
- **Zero-cost review trials.** The first no-flow review run recorded 0 tokens and $0: `/code-review` runs as a forked agent (`skip_transcript`), so the result event's `usage` is all zero and only `modelUsage` carries the session's tokens. A task (3) was inserted to read `modelUsage` when no result carries usage, and the run was deleted and re-made. The fallback first priced cache creation as 1-hour writes on the premise "every runner transcript writes 1-hour entries"; the panel measured the CLI's own reported cost against it (flow trials' reported/recorded ratio 20.000, no-flow 15.4–16.6) and the TTL is 5-minute for a forked command. Fixed, decision superseded, and the review run re-made a second time (ratio 20.000 on every trial).
- **The format sentence is the skill's review target.** `claude -p` expands the leading `/code-review low` and hands the rest of the prompt to the skill as `Review target:`. At `low` the skill's recipe reads `git diff HEAD` regardless, so the working-tree diff was reviewed and every trial answered with `####` sections; another effort's recipe may treat the target differently and was not run.
- **Reviewer reproducers bound to text.** Four of the panel's reproducers declared the defective source line as a premise, so the fix itself voided them; they were re-authored as behaviour checks (probe tests run in a copy of HEAD, the run's cost ratio, the marker count) and each proved against the pre-fix commit with `prove-reproducer.sh`. F1's original also ran on the live worktree's HEAD by absolute path, so its pre-fix leg read the fixed tree; re-authored to read the tree it runs in.
- **Main-checkout hook on heredoc writes.** Two Bash calls that wrote files through heredocs carrying relative paths were refused by the main-checkout guard (the shell's cwd is the main checkout); the edits went through scripts in the job's tmp directory and the Write tool instead.
- **Results.** haiku-low k=3: review 0/3 (score 0.0 vs flow 35.0, regression); impl 3/3 (100.0 vs 100.0, non-inferior, cost/pass $0.0452 vs $0.0495); visual 3/3 (100.0 vs 100.0, non-inferior, cost/pass $0.0019 vs $0.0052). The impl and visual runs predate the round-1 fix (no-flow impl `/case` still held `task.md`, headers carry the flow pin); no transcript read `/case/task.md`, so the measurements stand. No baseline set.

## 2026-10-14 — fix run

Operator asked for no-flow review on muse-code after the muse-spark-1.3 low no-flow impl and visual runs (committed on the branch before this fix run).

- **No review command in Muse Code.** `muse skills list` names no review skill, and `muse --help` / `muse exec --help` no review subcommand; the only `/review` strings in the binary are plugin-schema examples (`skills/review/SKILL.md`, `enabledDefault: false`). So muse-code's no-flow review is a plain request to review the working tree's diff (`prompts/no-flow/review-plain.md`), with the same format sentence; decision `no-flow-review-code-review` superseded by `no-flow-review-per-harness`. Installing a review skill or plugin into the trial was rejected: flow-eval's own text in a harness wrapper is no more the harness's behaviour than the plain prompt.
- **Result.** muse-spark-1.3 low k=3 no-flow review: 0/3, score 0.0 against the flow muse-spark-1.3 low primary run's 16.7 (regression). Each trial reported `####` sections naming paths in the case's diff, so the setup held.
- **Panel round 2.** The CLI's acceptance of muse-code was untested, and the supported-harness list lived in the CLI and in `noFlowTarget` both. Fixed with `run.NoFlowReviewTarget` as the one source; `Run` now refuses an unsupported harness itself. The first acceptance test asserted only that the refusal phrase was absent, and the reviewer's mutant — refusing muse-code with its own wording — survived it; the rows now assert the run reaches case selection.
- **Citations.** The re-run's first verdict lines cited lines outside the fix diff's hunks; one send-back re-cited them. The stage marks for the panel close were made before that send-back finished; the send-back's records follow them.

## 2026-10-14 — integrate run

- Preflight clean: nothing foreign staged, no drift on the main checkout, the branch not merged.
- Unfinished-work gate clear: every plan item checked, no finding open; visual verify not configured.
- No rebase: `origin/main` has not moved since the recorded merge base `c97af3d`.
- Route: merge and push, the project's configured default, not asked.
