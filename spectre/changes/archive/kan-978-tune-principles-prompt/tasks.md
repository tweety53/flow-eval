> **Execution:** `/flow` implements this plan. Mark a task's own checkbox when
> `check-task-commit-fields.sh` passes on that task's commit.
> **Relocation:** no
> **Tasks appended:** 8

# kan-978-tune-principles-prompt

KAN-978: tune the principles reviewer prompt on haiku-low at k=6 the way primary was tuned, and write the loop down as `docs/prompt-tuning.md`. `design.md` is canonical for the decisions cited by ID and for the variant-1 wording.

**Repositories:**

- flow-eval — the worktree `/Users/tweety53/Projects/flow-eval-worktrees/kan-978-tune-principles-prompt` (`$FE` below); tasks 1, 3 and 4.
- agents — the worktree `/Users/tweety53/Projects/agents-worktrees/kan-978-tune-principles-prompt` (`$AG` below) on `spectre/kan-978-tune-principles-prompt`, created by task 2 step 1; task 2.

Every path in a task's `**Files:**` is relative to that task's repository.

## Global constraints

- **Run command** (from `$FE`):
  `go run ./cmd/flow-eval run --role review --slot principles --harness claude-code --model claude-haiku-5-5 --effort low -k 6 --repos /Users/tweety53/Projects`
  — `--repos` because the default `..` resolves to `flow-eval-worktrees/`, which holds no `gymie-frontend`. <!-- verified: go run ./cmd/flow-eval run -h @ afee052 — `-repos` default `..` -->
- **Credential** (decision `eval-credential-from-zshrc`): run every run command as `zsh -ic '<run command>'` so it picks up `CLAUDE_CODE_OAUTH_TOKEN` from `~/.zshrc`. <!-- verified: zsh -ic '[ -n "$CLAUDE_CODE_OAUTH_TOKEN" ]' printed SET @ 2026-10-13 --> The value never appears in a command line, a commit, a run file or a report.
- **Runner rebuild** after every `prompts sync` (README `## Setup`), from `$FE`:
  `agents=$(mktemp -d); git -C /Users/tweety53/Projects/agents archive -o "$agents/agents.tar" "$(cat prompts/PIN)" stats; docker build -f docker/runner.Dockerfile --build-context agents="$agents" -t flow-eval-runner .`
- **Finds** for a run = the trials whose `passed` is true in `runs/<id>.jsonl` (the case's only principles must-find is F6, so recall 1 = F6 found).
- No Go code changes; `go vet ./...` stays green.
- **Lint** (from `$FE`, every flow-eval task): `gofmt -w .`, `test -z "$(gofmt -l .)"`, `go vet ./...`. The agents task: the `## lint` lines of `/Users/tweety53/Projects/agents/.flow/project.md` its one Markdown file needs.
- A run that ends with any infra error is re-run once whole; a second infra error is reported, never counted as a miss.
- Each task's tool-call descriptions open with `Task <x>/4` and its round.

---

- [x] 1. Before run: the current principles prompt on haiku-low at k=6

**Build:** green
**Files:** `runs/index.html`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — an eval run, no code.
**Regression:** none — records a measurement.
**Commit:** `chore(runs): haiku-low principles at k=6 on agents 2e053aaa, the tuning's before run`
**After:** none

**Decision:** haiku-low-k6

  - [x] **Step 1: Confirm the pin and image.** `cat prompts/PIN` prints `2e053aaa95ce00195dac3b208e1769dcc45862da`; rebuild the runner per Global constraints so the image matches it.
  - [x] **Step 2: Run.** The run command from Global constraints.
  - [x] **Step 3: Read the result.** Record finds/6 from the run's `.jsonl`; read each trial's output in `runs/<id>.html` and note, one line, what the trials reported instead of F6.
  - [x] **Step 4: Commit** `runs/<id>.jsonl`, `runs/<id>.md`, `runs/<id>.html`, `runs/index.html` with the subject above; body: `finds <n>/6`, the line from step 3.

- [x] 2. The principles prompt's derived-state step, best of at most three variants

**Build:** green
**Files:** `skills/flow/principles-reviewer-prompt.md`
**Tests:** **none** — a prompt; the flow-eval runs are its measurement.
**Regression:** none — revert → the principles reviewer loses the derived-state step; task 3's runs show the effect.
**Commit:** `feat(flow): the principles reviewer derives every stored summary of other state`
**After:** Task 1

**Decision:** agents-canonical-vendored
**Decision:** best-of-three-variants
**Decision:** variant-commits-on-a-pushed-branch

  - [x] **Step 1: Agents worktree.** `<agents-base>` = `git -C /Users/tweety53/Projects/agents symbolic-ref --short refs/remotes/origin/HEAD` without `origin/`. `git -C /Users/tweety53/Projects/agents fetch origin`; `git -C /Users/tweety53/Projects/agents worktree add -b spectre/kan-978-tune-principles-prompt /Users/tweety53/Projects/agents-worktrees/kan-978-tune-principles-prompt origin/<agents-base>`; run agents' `## worktree setup` in it; `flow state add-worktree kan-978-tune-principles-prompt $AG "$(git -C $AG rev-parse HEAD)"`; `git -C $AG push -u origin spectre/kan-978-tune-principles-prompt`. Then the variants worktree: `git -C $AG worktree add -b kan-978-principles-variants $AG-variants HEAD`.
  - [x] **Step 2: Variant 1.** In `$AG-variants`, edit `skills/flow/principles-reviewer-prompt.md`: insert design.md `## The prompt step` — the required step after the `## Scope` section's grep paragraph, and `### Derived-state check` as the first subsection of `## Output Format`, before `### Summary`. Keep the prompt's existing voice; name nothing from the case. Commit `feat(flow): principles derived-state step, variant 1`; `git -C $AG-variants push -u origin kan-978-principles-variants`.
  - [x] **Step 3: Measure variant 1.** From `$FE`: `go run ./cmd/flow-eval prompts sync --agents /Users/tweety53/Projects/agents --sha <variant sha>`; rebuild the runner; the run command. Record finds/6 and, from `runs/<id>.html`, whether trials filled the Derived-state check and what they listed.
  - [x] **Step 4: Variants 2 and 3, only while needed.** Stop after variant 1 if it scored ≥3/6. Otherwise write variant 2 from what step 3 showed the trials missing (they listed no summarising state, or listed it but judged each writer alone) — still generic — commit `variant 2`, push, sync, rebuild, run. Variant 3 likewise only if variant 2 did not reach ≥3/6. Never more than three.
  - [x] **Step 5: Pick and land on the change branch.** The winner is the variant with the most finds (ties → the shorter wording); none found F6 even once → report it, land nothing, and stop the plan for the operator. Cherry-pick the winner onto `spectre/kan-978-tune-principles-prompt` in `$AG` as one commit with the subject above; body: what the step makes the reviewer do, one line per variant `variant <n> <sha>: <one-line wording difference> — finds <n>/6`, and the before run's `finds <n>/6`. `git -C $AG push`.
  - [x] **Step 6: Leave flow-eval synced to the winner.** If the last sync was not the winner, `prompts sync --sha <winner variant sha>` again (no re-run). Leave every variant's run files uncommitted for task 3. `git -C $AG worktree remove $AG-variants`; keep the pushed variants branch.
  - [x] **Step 7: Lint** the agents file per Global constraints.

Correction (2026-10-13): the plan declared one task commit carrying a `Task-Id: 2` trailer; the landed commit `0ff796c0` (agents, `spectre/kan-978-tune-principles-prompt`) carries none — it was pushed before the omission was seen, and `prompts/PIN` in flow-eval names its child `0644d0c8`, so it is recorded here rather than rewritten. Variant 1 scored 3/6, so variants 2 and 3 were not run. `check-verbatim-moves.sh` required a planning commit `0644d0c8` adding `spectre/changes/kan-978-tune-principles-prompt/verbatim-moves.txt` in agents, acknowledging the step's new prompt text.

- [x] 3. Vendor the tuned principles prompt and its variant runs

**Build:** green
**Files:** `prompts/PIN`, `prompts/review-principles.md`, `runs/index.html`
**Allowed-collateral:** `runs/*`, `prompts/*`, `prompts/rules/*`
**Tests:** **none** — vendored prompt and run records.
**Regression:** none — revert → flow-eval measures the old principles prompt again.
**Commit:** `chore(prompts): vendor agents 0644d0c8 — the principles reviewer's derived-state step`
**After:** Task 1, 2

**Decision:** agents-canonical-vendored
**Decision:** baselines-unchanged

  - [x] **Step 1: Pin the change-branch commit.** `go run ./cmd/flow-eval prompts sync --agents /Users/tweety53/Projects/agents --sha "$(git -C $AG rev-parse HEAD)"`. Check `git -C $AG diff <winner variant sha> HEAD -- skills/flow/principles-reviewer-prompt.md` is empty, so the winner run measured exactly these bytes.
  - [x] **Step 2: Verify the vendored diff.** `git diff -- prompts/` touches `prompts/PIN` and `prompts/review-principles.md` only, unless agents moved between `2e053aaa` and the branch base — then every other vendored file's change is agents' own, named in the commit body.
  - [x] **Step 3: Lint** per Global constraints.
  - [x] **Step 4: Commit** the vendored files and every variant run's `.jsonl`, `.md`, `.html` plus `runs/index.html`; body: the agents sha, `before <n>/6 → winner <n>/6 on haiku-low k=6`, one line per variant run id and finds. `baselines/` untouched.

Note: integrate lands agents by merge and push; if that rebase rewrites the agents sha, a fix run re-syncs `prompts/PIN` to the landed sha — the prompt bytes are identical, so no re-run.

- [x] 4. The prompt-tuning guide

**Build:** green
**Files:** `docs/prompt-tuning.md`, `README.md`
**Tests:** **none** — docs.
**Regression:** none — revert → the loop is undocumented again.
**Commit:** `docs(prompts): the versioned prompt-tuning loop, with the principles tuning as its example`
**After:** Task 3

**Decision:** guide-in-docs
**Decision:** variant-commits-on-a-pushed-branch

  - [x] **Step 1: Write `docs/prompt-tuning.md`** (read `~/.agents/skills/go-documentation/SKILL.md` first), in the voice of `docs/cases.md`. Sections, every command copy-pasteable:
    1. **When** — a slot or role whose must-find a cheap model misses; tune the agents prompt, never the vendored copy.
    2. **Read why it missed** — the run page's trial outputs against the must-find; name the reasoning step the trials skipped.
    3. **Write one generic required step and an evidence output section** — the shape of primary (`2e053aaa`) and principles (task 2's sha): a numbered step done before anything else, and an output section listing each item with `path:line` and a verdict; never a case identifier.
    4. **Version every variant** — one commit per variant on a pushed `<key>-<slot>-variants` agents branch, so each run's `prompt pin` resolves.
    5. **Measure** — `prompts sync --sha <variant sha>`, rebuild the runner, `run ... -k <≤6> --repos <dir>` on the cheap model; a same-k before run on the current pin first; finds = passed trials.
    6. **Stop** — at most three variants; most finds wins, ties to the shorter wording; none ever finds → stop and decide, land nothing.
    7. **Land** — the winner cherry-picked as one agents commit whose body lists every variant's sha and finds; flow-eval vendors the change-branch sha and commits the before run and every variant run; baselines change only by an explicit `baseline set`; if landing rewrites the agents sha, re-sync the pin (same bytes, no re-run).
    8. **Worked examples** — primary (`2e053aaa`, 0/33 → 14/33 across models) and principles (this change's before/winner finds and agents sha), read from tasks 1–3's commits.
  - [x] **Step 2: Link it.** README `### prompts sync`: one line after its paragraph — tuning a prompt with flow-eval is [`docs/prompt-tuning.md`](docs/prompt-tuning.md).
  - [x] **Step 3: Lint** per Global constraints.

---

Fix 1 (2026-10-13) — a label at more than one location, and re-scoring the stored runs. Decisions `label-also-lines`, `regrade-from-stored-outputs`. Read `~/.agents/skills/go-coding-standards`, `go-error-handling`, `go-modernize`, `go-test-quality`, `go-test-table-driven`, `go-cli` and `go-documentation` (each `SKILL.md`) before the Go tasks, and invoke `modern-go-guidelines:use-modern-go`.

- [x] 5. A review label may list further lines of its file

**Build:** green
**Files:** `internal/grade/review.go`, `internal/grade/review_test.go`, `cases/review/kan-867-qm-2-step-1-frontend-settings-workout-mode-focus/labels.json`, `docs/cases.md`, `internal/mine/mine_test.go`
**Tests:** `TestCandidatePairsAtAnAlsoLine` (internal/grade) — a finding within 15 lines of an also line is put to the judge and matches; one outside every line's window is not; the judge's labelled text names every line.
**Regression:** revert → a finding at `SettingsState.kt:295` describing F6 is never judged; `TestCandidatePairsAtAnAlsoLine` fails.
**Commit:** `feat(grade): a review label may list further lines where its defect also sits`
**After:** Task 4

**Decision:** label-also-lines

  - [x] **Step 1: RED.** Add `TestCandidatePairsAtAnAlsoLine` beside `TestCandidatePairsWithinWindow`, using `fakeJudge`; run `go test ./internal/grade -run TestCandidatePairsAtAnAlsoLine` and see it fail.
  - [x] **Step 2: GREEN.** `Label` gains `Also []int` (JSON `also`, omitted when empty; doc: further lines of File where the same defect sits); `near` matches when the finding is within `_window` of `Line` or of any `Also` line; `GradeReview`'s labelled text is `file:line[,also…]: summary`.
  - [x] **Step 3: Label F6.** `cases/review/kan-867-…/labels.json`: F6 gains `"also": [295]`. `go run ./cmd/flow-eval case validate` passes.
  - [x] **Step 4: Doc.** `docs/cases.md`'s `mustFind[]` bullet names `also`: added by hand when one defect sits at several writers.
  - [x] **Step 5: Lint** per Global constraints.

Correction (2026-10-13): `Label` now carries a slice, so `internal/mine/mine_test.go`'s `!=` on a `grade.Label` stopped compiling; the task commit landed before `go vet` caught it, and a second commit carrying `Task-Id: 5` compares by `reflect.DeepEqual`. `**Files:**` widened to it.

- [x] 6. `flow-eval regrade` re-grades stored review runs

**Build:** green
**Files:** `internal/run/regrade.go`, `internal/run/run.go`, `internal/run/run_test.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `README.md`
**Tests:** `TestRegradeRescoresFromTrialOutputs` (internal/run) — a recorded review run re-graded against changed labels flips a trial's pass, keeps its trial metrics, replaces its judge usage, takes the new case hash and records the regrading sha; `TestRegradeRestoresOutputFromPage` (internal/run) — with the trial directory gone, the output is read from the run page, for a plain and an escaped case id, nothing is written to the trial directory, and the re-rendered page still carries it; `TestRegradeRefusesANonReviewRun` (internal/run); `TestRegradeRefusesACutPageCopy` (internal/run) — a page copy cut at the artifact cap is refused and the run left as it was; `TestRegradeKeepsASandboxStopNote` (internal/run) — replacing a failed grade keeps the sandbox-stop note; `TestRegradeExitCodes` (cmd/flow-eval) — no run id, an unknown run and missing judge credentials exit 2.
**Regression:** revert → no way to re-score a stored run; the tests above fail to compile.
**Commit:** `feat(run): regrade re-scores a stored review run against the current labels and judge`
**After:** Task 4, 5

**Decision:** regrade-from-stored-outputs

  - [x] **Step 1: RED.** Write the three `internal/run` tests from a run recorded by `Run` with the fakes `run_test.go` already uses, and `TestRegradeExitCodes`; see them fail.
  - [x] **Step 2: GREEN.** `run.Regrade(ctx, cfg Config, id string) error`: `Load` the run; refuse a role other than `review`; resolve each case of `CaseHashes` from `cfg.Cases`; for each line not `infra_error` that was graded (`Review` set, or `Error` opening `grade: `), read its `output.md`, or else extract it from `<id>.html`'s `trial-<case>-<n>` section and write it back, then grade it through `gradeTrial` (no sandbox) and price the judge usage the way `runTrial` does; set `Header.CaseSetHash`, `CaseHashes` and `Judge` to the current ones and a new `Regraded` field to `cfg.FlowEvalSHA`; write the `.jsonl` atomically, then `writeReports`.
  - [x] **Step 3: CLI.** `regrade [flags] <run-id>…` in `commands`, with the `run` flags it needs (`--runs`, `--root`, `--baselines`, `--judge`, `--pricing`); exit 0 when every run is re-graded, 2 on a usage error, missing judge credentials, an unknown run or a failed re-grade, naming the run.
  - [x] **Step 4: Doc.** README `## Commands` gains `### regrade`: what it re-grades, what it keeps, where it reads an output from.
  - [x] **Step 5: Lint** per Global constraints.

- [x] 7. Re-score the kan-867 review runs and record the cross-model eval

**Build:** green
**Files:** `runs/index.html`, `docs/prompt-tuning.md`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — re-graded run records.
**Regression:** none — records a measurement.
**Commit:** `chore(runs): re-score the kan-867 review runs with F6 matched at either writer, and add sonnet-low and opus-low k=6`
**After:** Task 1, 3, 4, 6, 8

**Decision:** regrade-from-stored-outputs
**Decision:** baselines-unchanged

  - [x] **Step 1: Re-grade.** The nine k=3 runs' committed pages lost their trials' output.md to a repaint (task 8); first put back each one's page as `1534195` committed it (`git show 1534195:runs/<id>.html > runs/<id>.html`), which still carries them. Then `zsh -ic 'go run ./cmd/flow-eval regrade <id>…'` over every `runs/*-review-*.jsonl` (the credential per Global constraints); record each run's passes before and after.
  - [x] **Step 2: Guide.** `docs/prompt-tuning.md` `## Worked examples`: the principles row's figures re-read from the re-graded runs, sonnet-low and opus-low k=6 added, and one line on the location trap — a defect at several writers needs every writer in its label's `also`.
  - [x] **Step 3: Lint** per Global constraints.
  - [x] **Step 4: Commit** every re-graded and new run's `.jsonl`, `.md`, `.html` and `runs/index.html` with `docs/prompt-tuning.md`; body: one line per run, `<id> <slot> <model>: passes <before> → <after>`. `baselines/` untouched.

- [x] 8. A run page keeps an artifact whose trial file is gone

**Build:** green
**Files:** `internal/run/page.go`, `internal/run/regrade.go`, `internal/run/run_test.go`
**Tests:** `TestRepaintKeepsArtifactsOfAPageWithoutTrialFiles` (internal/run) — a run page re-rendered after its trial directory is removed still carries the trial's output.md.
**Regression:** revert → every repaint from a checkout without `runs/<id>/` (a fresh clone, another worktree) strips each run page's output.md, as `32b4b5c` and `afee052` did to the nine k=3 runs; the test fails.
**Commit:** `fix(run): a re-rendered run page keeps the artifacts it carried when the trial files are gone`
**After:** Task 6

**Decision:** regrade-from-stored-outputs

Found in task 7 (2026-10-13): `page` reads each artifact from `runs/<id>/<case>/<trial>/`, which is gitignored, and `Repaint` re-renders every run's page on every run, so the nine k=3 kan-867 runs' committed pages lost their outputs the first time another checkout repainted them.

  - [x] **Step 1: RED.** `TestRepaintKeepsArtifactsOfAPageWithoutTrialFiles`: record a run, remove `runs/<id>/`, call `Repaint`, assert the page still carries the output; see it fail.
  - [x] **Step 2: GREEN.** `page` reads the run's existing page once; an artifact whose file does not exist is taken from that page's copy of it (`pageArtifact`, generalising `regrade.go`'s `pageOutput`), never dropped. `trialOutput` uses the same function.
  - [x] **Step 3: Lint** per Global constraints.

Fix 2 (2026-10-13) — a review run's id names its slot. Decision `run-id-names-slot`. Read `~/.agents/skills/go-coding-standards`, `go-test-quality` and `go-test-table-driven` (each `SKILL.md`) and invoke `modern-go-guidelines:use-modern-go` before the task.

- [x] 9. A review run's id names its slot

**Build:** green
**Files:** `internal/run/run.go`, `internal/run/run_test.go`
**Tests:** `TestReviewGradedOnItsSlotsMustFinds` (internal/run), subtest `a review run's id names its slot, so two slots started in one second both run` — with the clock pinned, a primary and a principles run of the same case, harness, model and effort both succeed, each id naming its slot after `review`.
**Regression:** revert → the second run of the pair is refused with `file exists`; the subtest fails.
**Commit:** `fix(run): a review run's id names its slot, so two slots launched together never collide`
**After:** Task 6, 8

**Decision:** run-id-names-slot

  - [x] **Step 1: RED.** Add the subtest to `TestReviewGradedOnItsSlotsMustFinds`, pinning `now` as `TestRunIDsNeverShareSandboxesOrRecords` does; run `go test ./internal/run -run TestReviewGradedOnItsSlotsMustFinds` and see it fail.
  - [x] **Step 2: GREEN.** `Run` builds the id with `review-<slot>` in place of the role on a review run.
  - [x] **Step 3: Lint** per Global constraints.

Fix 3 (2026-10-13) — parsing a bold-led finding whole, rewarding real extras, and re-grading every stored review run. Decisions `parse-bold-led-bullets`, `review-extras-rewarded`. Read `~/.agents/skills/go-coding-standards`, `go-error-handling`, `go-test-quality`, `go-test-table-driven` and `go-documentation` (each `SKILL.md`) and invoke `modern-go-guidelines:use-modern-go` before tasks 10 and 11.

- [x] 10. A bold-led finding keeps its bullets

**Build:** green
**Files:** `internal/grade/review.go`, `internal/grade/review_test.go`
**Tests:** `TestParseReviewKeepsABoldLedFindingsBullets` (internal/grade) — a bold-led finding followed by bullets parses as one finding carrying them; a bold-led numbered finding with no path:line opens a finding; a list-led report still parses one finding per item; a bold-only label ends the finding above it, bold-led or list-led, and its items are no finding.
**Regression:** revert → the opus-low principles trial 2 report of `20261013T171004Z` parses as 18 findings, 7 of them Important; the test fails.
**Commit:** `fix(grade): a bold-led review finding keeps the bullets that follow it`
**After:** Task 5

**Decision:** parse-bold-led-bullets

Correction (2026-10-13): the gated review found a bold-led finding directly under its severity heading still split (a heading did not end a paragraph) and a bold-only label's items folded into the finding above; both fixed in a second commit carrying `Task-Id: 10`, with two more cases in the test, and decision `parse-bold-led-bullets` states both rules.

  - [x] **Step 1: RED.** Add `TestParseReviewKeepsABoldLedFindingsBullets` beside `TestParseReviewReport`; run `go test ./internal/grade -run TestParseReview` and see it fail.
  - [x] **Step 2: GREEN.** `ParseReview` records whether the open finding was bold-led or a sub-heading; while it was, an unindented list item continues it. A bold-led paragraph also opens on a leading item number.
  - [x] **Step 3: Lint** per Global constraints.

- [x] 11. A review trial's score rewards real extras

**Build:** green
**Files:** `internal/grade/review.go`, `internal/grade/review_test.go`, `internal/metrics/metrics.go`, `internal/run/run.go`, `internal/run/run_test.go`, `README.md`
**Tests:** `TestReviewCountsPlausibleExtras` (internal/grade) — unlabelled findings rated plausible count as extras at every severity, rated implausible count as implausible at every severity and as false positives only when Critical or Important, a must-find or known-false match is never an extra, a must-find defect re-reported outside its window is neither an extra nor implausible nor a true positive, a known-false defect outside its window is no known-false hit; `TestUnmatchedFindingIsAskedOncePerLabel` (internal/grade) — an unmatched finding is put to Same once per must-find label, outside the window too; `TestCandidatePairsWithinWindow`, `TestCandidatePairsAtAnAlsoLine`, `TestCandidatePairsByFileTail`, `TestReviewScoresWithFakeJudge`, `TestJudgeCostSeparate` (internal/grade) — only findings within the window are true positives, and the window-free question is counted among the judge's calls; `TestReviewScoreNetsExtras` (internal/run) — a trial with F1 1 scores 100 plus 5 per net extra (extras less implausible, floored at 0), at most 115.
**Regression:** revert → a real finding beyond the labels moves no score; both tests fail.
**Commit:** `feat(grade): a review trial's score rewards real findings beyond the labels`
**After:** Task 4, 5, 6, 8, 9, 10

**Decision:** review-extras-rewarded
**Decision:** review-extras-net

Correction (2026-10-13): Step 1 named `TestReviewScoresWithFakeJudge`'s `plausibleAsked` gaining the Minor finding; that Minor matches must-find L3, which the decision never puts to the judge, so the expectation stays and the test asserts `Extras == 1` instead.

  - [x] **Step 1: RED.** Add both tests; `TestReviewScoresWithFakeJudge` asserts `Extras == 1`; see them fail.
  - [x] **Step 2: GREEN.** `metrics.Review` gains `Extras int` (JSON `extras`); `GradeReview` puts every unmatched finding to `Plausible`, a Minor included, counts a plausible one in `Extras`, and counts an implausible one as a false positive only when it is Critical or Important; `gradeTrial`'s review score is `F1 × 100 + 5 × min(Extras, 3)`.
  - [x] **Step 3: Doc.** README `## Reading results`, case table bullet: a review trial's score is F1 × 100 plus 5 per real extra, at most 15.
  - [x] **Step 4: Lint** per Global constraints.

- [x] 12. Re-grade every stored review run

**Build:** green
**Files:** `runs/index.html`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — re-graded run records.
**Regression:** none — records a measurement.
**Commit:** `chore(runs): re-grade every review run with bold-led findings parsed whole and real extras rewarded`
**After:** Task 1, 3, 7, 11

**Decision:** review-extras-rewarded
**Decision:** baselines-unchanged

  - [x] **Step 1: Re-grade.** `zsh -ic 'go run ./cmd/flow-eval regrade <id>…'` over every `runs/*review*.jsonl` (the credential per Global constraints); record each run's role score before and after.
  - [x] **Step 2: Compare.** Re-run `compare` for every muse run against its baseline, rewriting its `-vs-` report.
  - [x] **Step 3: Commit** every re-graded run's `.jsonl`, `.md`, `.html`, the rewritten `-vs-` reports and `runs/index.html`; body: one line per run, `<id>: score <before> → <after>`. `baselines/` untouched.
