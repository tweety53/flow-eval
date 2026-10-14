> **Execution:** `/flow` implements this plan. Mark a task's own checkbox when
> `check-task-commit-fields.sh` passes on that task's commit.
> **Relocation:** no
> **Tasks appended:** 1

# kan-979-tune-failure-modes-prompt

KAN-979: tune the failure-modes reviewer prompt on haiku-low at k=6 with the loop in `docs/prompt-tuning.md`, then, on the operator's go, measure the winner on sonnet-low and opus-low. `design.md` is canonical for the decisions cited by ID and for the variant-1 wording; `docs/prompt-tuning.md` is canonical for the loop's commands, stop rule and landing steps — the tasks cite its sections rather than restating them.

**Repositories:**

- flow-eval — the worktree `/Users/tweety53/Projects/flow-eval-worktrees/kan-979-tune-failure-modes-prompt` (`$FE` below); tasks 1, 3, 4 and 5.
- agents — the worktree `/Users/tweety53/Projects/agents-worktrees/kan-979-tune-failure-modes-prompt` (`$AG` below) on `spectre/kan-979-tune-failure-modes-prompt`, created by task 2 step 1; task 2.

Every path in a task's `**Files:**` is relative to that task's repository.

## Global constraints

- **Run command** (from `$FE`), `<model>` `claude-haiku-5-5` unless a task says otherwise:
  `go run ./cmd/flow-eval run --role review --slot failure-modes --harness claude-code --model <model> --effort low -k 6 --repos /Users/tweety53/Projects`
  — `--repos` because the default `..` resolves to `flow-eval-worktrees/`, which holds no `gymie-frontend`. <!-- verified: go run ./cmd/flow-eval run -h @ 57f3d36 — `-repos` default `..` -->
- **Credential** (decision `eval-credential-from-zshrc`): every run command runs as `zsh -ic '<run command>'`. <!-- verified: zsh -ic '[ -n "$CLAUDE_CODE_OAUTH_TOKEN" ]' printed SET @ 2026-10-14 --> The value never appears in a command line, a commit, a run file or a report.
- **Runner rebuild** after every `prompts sync`, before any run: the three lines under `docs/prompt-tuning.md` § Measure that archive `$(cat prompts/PIN)` from `/Users/tweety53/Projects/agents` and `docker build` `flow-eval-runner`. Never rebuild or sync while a run is in flight.
- **Finds** for a run = `jq -s 'map(select(.trial)) | map(select(.passed)) | length' runs/<id>.jsonl` (F9 is the slot's only must-find, so recall 1 = F9 found).
- A failed trial that describes F9 but anchors it more than 15 lines from `:295` is handled per `docs/prompt-tuning.md` § Measure (label `also`, `flow-eval regrade`); such a label edit is a plan amendment, reported before it is made.
- A run that ends with any infra error is re-run once whole; a second infra error is reported, never counted as a miss.
- No Go code changes. **Lint** (from `$FE`, every flow-eval task): `gofmt -w .`, `test -z "$(gofmt -l .)"`, `go vet ./...`. The agents task: `scripts/check-verbatim-moves.sh` and `scripts/check-markdown-integrity.py` from `/Users/tweety53/Projects/agents/.flow/project.md` `## lint`, run in `$AG`.
- Each task's tool-call descriptions open with `Task <x>/5`.

---

- [x] 1. Before run: the current failure-modes prompt on haiku-low at k=6

**Build:** green
**Files:** `runs/index.html`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — an eval run, no code.
**Regression:** none — records a measurement.
**Commit:** `chore(runs): haiku-low failure-modes at k=6 on agents a061813d, the tuning's before run`
**After:** none

**Decision:** haiku-low-k6

  - [x] **Step 1: Confirm the pin and image.** `cat prompts/PIN` prints `a061813d9511790d0ef4164a104c0c160dc2bcf0`; rebuild the runner per Global constraints.
  - [x] **Step 2: Run** the run command.
  - [x] **Step 3: Read the result.** Record finds/6; read each trial's output in `runs/<id>.html` and note, one line, what the trials reported instead of F9.
  - [x] **Step 4: Lint and commit** `runs/<id>.jsonl`, `runs/<id>.md`, `runs/<id>.html`, `runs/index.html` with the subject above; body: `finds <n>/6`, the line from step 3.

- [x] 2. The failure-modes prompt's failure-recovery step, best of at most three variants

**Build:** green
**Files:** `skills/flow/failure-modes-reviewer-prompt.md`
**Tests:** **none** — a prompt; the flow-eval runs are its measurement.
**Regression:** none — revert → the failure-modes reviewer loses the failure-recovery step; task 3's runs show the effect.
**Commit:** `feat(flow): the failure-modes reviewer checks what a later success clears after a failure`
**After:** Task 1

**Decision:** agents-canonical-vendored
**Decision:** best-of-three-variants
**Decision:** failure-recovery-step

  - [x] **Step 1: Agents worktree.** `<agents-base>` = `git -C /Users/tweety53/Projects/agents symbolic-ref --short refs/remotes/origin/HEAD` without `origin/`. `git -C /Users/tweety53/Projects/agents fetch origin`; `git -C /Users/tweety53/Projects/agents worktree add -b spectre/kan-979-tune-failure-modes-prompt $AG origin/<agents-base>`; run agents' `## worktree setup` in it; `flow state add-worktree kan-979-tune-failure-modes-prompt $AG "$(git -C $AG rev-parse HEAD)"`; `git -C $AG push -u origin spectre/kan-979-tune-failure-modes-prompt`. Then the variants worktree per `docs/prompt-tuning.md` § Version every variant, as `git -C $AG worktree add -b kan-979-failure-modes-variants $AG-variants HEAD`.
  - [x] **Step 2: Variant 1.** In `$AG-variants`, edit `skills/flow/failure-modes-reviewer-prompt.md`: insert design.md `## The prompt step` — the required step after the `## Scope` section's grep paragraph, and `### Failure-recovery check` as the first subsection of `## Output Format`, before `### Summary`. Keep the prompt's voice; name nothing from the case. Commit `feat(flow): failure-modes failure-recovery step, variant 1`; `git -C $AG-variants push -u origin kan-979-failure-modes-variants`.
  - [x] **Step 3: Measure variant 1.** From `$FE`: `go run ./cmd/flow-eval prompts sync --agents /Users/tweety53/Projects/agents --sha <variant sha>`; rebuild the runner; the run command. Record finds/6 and, from `runs/<id>.html`, whether the trials filled the Failure-recovery check and what they listed.
  - [x] **Step 4: Variants 2 and 3, only while needed** — per `docs/prompt-tuning.md` § Stop. Each next wording answers what step 3 showed missing (no failure state listed, or listed but its clearers not traced, or a clear judged without asking what it left unconfirmed), still generic; commit `variant <n>`, push, sync, rebuild, run.
  - [x] **Step 5: Pick and land on the change branch** per `docs/prompt-tuning.md` § Stop and § Land step 1, with the subject above. None found F9 even once → report it, land nothing, and stop for the operator.
  - [x] **Step 6: Leave flow-eval synced to the winner.** If the last sync was not the winner, `prompts sync --sha <winner variant sha>` again (no re-run). Leave every variant's run files uncommitted for task 3. `git -C $AG worktree remove $AG-variants`; keep the pushed variants branch.
  - [x] **Step 7: Lint** the agents file per Global constraints. A `check-verbatim-moves.sh` `FAIL` is acknowledged per `docs/prompt-tuning.md` § Write one generic required step, in `$AG`'s `spectre/changes/kan-979-tune-failure-modes-prompt/verbatim-moves.txt`, committed as an agents planning commit.

- [x] 3. Vendor the tuned failure-modes prompt and its variant runs

**Build:** green
**Files:** `prompts/PIN`, `prompts/review-failure-modes.md`, `runs/index.html`
**Allowed-collateral:** `runs/*`, `prompts/*`, `prompts/rules/*`
**Tests:** **none** — vendored prompt and run records.
**Regression:** none — revert → flow-eval measures the old failure-modes prompt again.
**Commit:** `chore(prompts): vendor the failure-modes reviewer's failure-recovery step`
**After:** Task 1, 2

**Decision:** agents-canonical-vendored
**Decision:** baselines-unchanged

  - [x] **Step 1: Pin the change-branch tip** per `docs/prompt-tuning.md` § Land step 2, with `$AG` as `<agents>`'s checkout for `rev-parse` and `diff`; the diff against the winner variant sha is empty.
  - [x] **Step 2: Verify the vendored diff.** `git diff -- prompts/` touches `prompts/PIN` and `prompts/review-failure-modes.md` only, unless agents moved between `a061813d` and the branch base — then every other vendored file's change is agents' own, named in the commit body.
  - [x] **Step 3: Lint** per Global constraints.
  - [x] **Step 4: Commit** the vendored files and every variant run's `.jsonl`, `.md`, `.html` plus `runs/index.html`; body: the agents sha, `before <n>/6 → winner <n>/6 on haiku-low k=6`, one line per variant run id and finds. `baselines/` untouched.

- [x] 4. Cross-model runs of the winner, on the operator's go

**Build:** green
**Files:** `runs/index.html`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — eval runs, no code.
**Regression:** none — records a measurement.
**Commit:** `chore(runs): haiku-low, sonnet-low and opus-low failure-modes at k=3 on the tuned prompt`
**After:** Task 1, 3

**Decision:** cross-model-on-operator-go

  - [x] **Step 1: Show the haiku results and ask.** Print the before run's finds/6, each variant's sha, wording difference and finds/6, and what the winner's trials listed in the Failure-recovery check. Then **AskUserQuestion**: **Run the winner on sonnet-low and opus-low at k=6?** — **No — skip the cross-model runs** *(default)* / **Yes — run both**. Silence or anything but an explicit Yes is No.
  - [x] **Step 2: On No**, append under this task `Correction (<date>): the operator declined the cross-model runs after reading the haiku results; no commit.`, tick the task and its steps, and go to task 5.
  - [x] **Step 3: On Yes**, with the runner already on the vendored pin, run the run command with `--model sonnet`, then with `--model opus`. Record finds/6 for each.
  - [x] **Step 4: Lint and commit** both runs' `.jsonl`, `.md`, `.html` and `runs/index.html` with the subject above; body: one line per run id and finds/6.

Correction (2026-10-14): the operator, after reading the haiku results and the prompt diff, asked for haiku-low, sonnet-low and opus-low at k=3 on variant 2 instead of sonnet-low and opus-low at k=6; all three ran on the vendored pin 39347524 (prompt bytes = c0372d03), and the commit carries three runs.

- [x] 5. The failure-modes worked example

**Build:** green
**Files:** `docs/prompt-tuning.md`
**Tests:** **none** — docs.
**Regression:** none — revert → the guide's worked examples omit the failure-modes tuning.
**Commit:** `docs(prompts): the failure-modes tuning as a worked example`
**After:** Task 3, 4

  - [x] **Step 1: Add the row** (read `~/.agents/skills/go-documentation/SKILL.md` first) to `docs/prompt-tuning.md` § Worked examples, in the existing columns: slot `failure-modes`; step `Failure-recovery check: a success that clears a failure's state must re-confirm everything the failure left unconfirmed`; before `<n>/6 haiku-low k=6`; after the winner's haiku-low finds with its variant and sha, plus sonnet-low and opus-low when task 4 ran them; agents the change-branch sha task 3 vendored. Read every figure from tasks 1–4's commits. If no variant found F9, the row says so and the after column reads `none landed`.
  - [x] **Step 2: Fix the sentence under the table** that says both tunings ran on the kan-867 case so it covers all three slots, and add one paragraph on what the haiku trials reported before and after the step, in the voice of the principles paragraph there.
  - [x] **Step 3: Lint** per Global constraints.

- [x] 6. The opus-low failure-modes baseline, and the k=6 and none-passed runs deleted

**Build:** green
**Files:** `baselines/review-failure-modes.json`, `runs/index.html`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — run records and a baseline.
**Regression:** none — revert → the deleted runs return and the baseline is the old opus-low run.
**Commit:** `chore(runs): opus-low failure-modes baseline; drop the k=6 and none-passed runs`
**After:** Task 1, 3, 4, 5

**Decision:** opus-low-baseline-and-run-cleanup

  - [x] **Step 1: Pick the runs.** From `runs/index.html` as task 5 left it: every row whose k column is 6, and every `row-fail` row — 14 runs: `20261013T163946Z-review-muse-code-muse-spark-1.3-low`, `20261013T154257Z-review-claude-code-opus-low`, `20261013T154057Z-review-claude-code-sonnet-low`, `20261013T143107Z-review-claude-code-claude-haiku-5-5-low`, `20261013T142803Z-review-claude-code-claude-haiku-5-5-low`, `20261014T111119Z-review-failure-modes-claude-code-claude-haiku-5-5-low`, `20261014T110715Z-review-failure-modes-claude-code-claude-haiku-5-5-low`, `20261014T110316Z-review-failure-modes-claude-code-claude-haiku-5-5-low`, `20261013T115858Z-review-claude-code-sonnet-low`, `20261013T121433Z-review-claude-code-opus-low`, `20261013T115946Z-review-claude-code-sonnet-low`, `20261013T114007Z-review-claude-code-claude-haiku-5-5-low`, `20261013T120012Z-review-claude-code-sonnet-low`, `20261013T114128Z-review-claude-code-claude-haiku-5-5-low`. None is a baseline.
  - [x] **Step 2: Delete** each run's `.jsonl`, `.md`, `.html` and its gitignored `runs/<id>/` transcript directory.
  - [x] **Step 3: Set the baseline** — `go run ./cmd/flow-eval baseline set 20261014T113408Z-review-failure-modes-claude-code-opus-low`, which repaints `runs/index.html`; confirm no deleted id remains in it.
  - [x] **Step 4: Lint and commit** the deletions, `baselines/review-failure-modes.json` and `runs/index.html`; body: the deleted ids and the new baseline.

Note: integrate lands agents by merge and push; if that rebase rewrites the agents sha, a fix run re-syncs `prompts/PIN` to the landed sha per `docs/prompt-tuning.md` § Land step 5 — same bytes, no re-run.

## Fix round 1

- [x] 7. Drop the old opus-low baseline run, and commit the run transcript directories

**Build:** green
**Files:** `.gitignore`, `README.md`, `internal/run/regrade.go`, `internal/run/page.go`, `runs/index.html`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — run records, an ignore rule and wording; `go test ./... -race -count=1` stays green.
**Regression:** none — revert → the run returns and `runs/<id>/` is ignored again.
**Commit:** `chore(runs): drop 20261013T121524Z-review-claude-code-opus-low, commit run directories`
**After:** Task 6

**Decision:** operator fix instructions — delete the run; `runs/` is no longer gitignored, its directories are kept in the repository.

  - [x] **Step 1: Delete** `runs/20261013T121524Z-review-claude-code-opus-low.{jsonl,md,html}`; repaint `runs/index.html` with `go run ./cmd/flow-eval baseline set 20261014T113408Z-review-failure-modes-claude-code-opus-low` (the baseline already in force) and confirm the id is gone from it.
  - [x] **Step 2: Unignore `runs/`** — drop `runs/*/` from `.gitignore`, and copy into the worktree every main-checkout `runs/<id>/` whose run is still tracked. The main checkout's other `runs/<id>/` directories — this run's and those task 6 deleted — are left for the operator to remove, since no step writes to the main checkout.
  - [x] **Step 3: Reword** the four places that call `runs/<id>/` gitignored — `README.md` (two), the `Regrade` doc comment and the `page` doc comment — to say the directory is committed and may be missing for an older run. The fall-back to the page's copy stays.
  - [x] **Step 4: Lint, test and commit.**
