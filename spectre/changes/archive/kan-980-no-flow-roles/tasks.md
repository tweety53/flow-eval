> **Execution:** `/flow` implements this plan. Mark a task's own checkbox when
> `check-task-commit-fields.sh` passes on that task's commit.
> **Relocation:** no
> **Tasks appended:** 2
> **Baseline convention:** every `**Baseline:** before=N after=M` is chain-position absolutes —
> `before` is the count at this task's commit's parent and `after` at the commit, each task
> measured at its own position, never a plan-wide pair — split by source set, counted by a
> word-boundary-safe counter (`@Test` never matches `@Testcontainers`), with any `measured:`
> command naming no ref of its own.

# kan-980-no-flow-roles

KAN-980: `flow-eval run --no-flow` — each role on its existing cases with no flow prompt, rules or tooling, then one haiku-low k=3 run per no-flow role compared with its flow counterpart. `design.md` is canonical for the decisions cited by ID and for the `## No-flow trial` mechanics (prep commands, `plainTask` rules, keys).

**Repository:** single — the worktree `/Users/tweety53/Projects/flow-eval-worktrees/kan-980-no-flow-roles`; every path below is relative to it.

## Global constraints

- Go stdlib only; no new dependency.
- Go skills per `CLAUDE.md`: `modern-go-guidelines:use-modern-go`, and `~/.agents/skills/go-coding-standards`, `go-error-handling`, `go-modernize`, `go-test-quality`, `go-test-table-driven`; `go-cli` for Task 2; `go-documentation` for the README steps.
- **Lint** (worktree root): `gofmt -w .`, `test -z "$(gofmt -l .)"`, `go vet ./...`.
- No test touches the network, a real model or Docker. The host fake sandbox (`hostTrial` in `internal/run/run_test.go`) runs `Exec` for real on this machine, so it must never execute the visual prep's `rm` — Task 1 step 1 makes it skip that exact argv, as it already skips `kill -KILL -1`.
- A flow run (no `--no-flow`) renders, preps and keys exactly as today: every existing test passes unchanged.

## Review focus

- A `task.md` with a field line the strip list does not name (`**Build:**` at column 0 after steps) — `plainTask` must drop by field name wherever the line sits, never by position.
- A review case whose `diff.patch` does not reverse-apply to its snapshot — the prep fails, and the trial must be an infra error naming `no-flow prep`, never a scored recall-0 review of an empty diff.
- `--no-flow` with `--slot principles` — the key is `review-no-flow-principles` and the labels are the principles must-finds; no standards are resolved or copied in (the no-flow prompt names none).
- An existing flow run file without `no_flow` — loads as a flow run, keeps its key and index section, and is still painted against its flow baseline.
- A no-flow and a flow run of the same role in one runs directory — two index sections, and neither painted against the other's baseline.

---

- [x] 1. No-flow trials: own prompts, plain impl spec, container prep, own keys

**Build:** green
**Files:** `internal/run/run.go`, `internal/run/index.go`, `internal/run/run_test.go`, `prompts/no-flow/impl.md`, `prompts/no-flow/review.md`, `prompts/no-flow/visual.md`, `cmd/flow-eval/main.go`, `internal/compare/compare.go`, `internal/compare/compare_test.go`
**Tests:** `TestPlainTaskStripsFlowFields`, `TestNoFlowRendersItsOwnPrompts`, `TestNoFlowPrepRunsBeforeTheAgent`, `TestNoFlowRunsHaveTheirOwnKeyAndSection`
**Regression:** `TestPlainTaskStripsFlowFields` — revert → `plainTask` is undefined; `TestNoFlowRendersItsOwnPrompts` — revert → a no-flow trial renders the vendored flow prompt; `TestNoFlowPrepRunsBeforeTheAgent` — revert → a review trial's agent sees no working-tree diff and a visual trial keeps the flow tool; `TestNoFlowRunsHaveTheirOwnKeyAndSection` — revert → a no-flow run is keyed and painted as a flow run.
**Baseline:** before=41 after=45 <!-- measured: grep -c '^func Test' internal/run/run_test.go -->
**Commit:** `feat(run): run a role with no flow prompt, rules or tooling`
**After:** none

**Decision:** no-flow-flag
**Decision:** no-flow-review-primary
**Decision:** no-flow-review-code-review
**Decision:** no-flow-format-line
**Decision:** no-flow-plain-spec
**Decision:** no-flow-visual-no-tools

  - [x] **Step 1: Make the host fake safe.** In `internal/run/run_test.go` `hostTrial.Exec`, beside the `kill -KILL -1` skip, return `sandbox.Result{}, nil` without running when `slices.Equal(cmd, _noFlowPrep["visual"])` (comment: on the host it would delete this machine's tools). Write this before any no-flow visual test exists.
  - [x] **Step 2: Write the failing tests** in `internal/run/run_test.go`, table-driven where the shape allows, using the existing fixtures (`hostDocker`, `scriptHarness`, `gitRepo`, `implConfig`, `visualConfig`, `readRun`, `sameJudge`):
    - `TestPlainTaskStripsFlowFields` — input: a task with title `- [ ] 3. The PATCH takes expected`, blank line, `**Build:** green`, `**Files:** \`a.kt\``, `**Tests:** \`t\``, `**Regression:** r`, `**Baseline:** before=0 after=4`, `<!-- predicted: x -->`, `**Commit:** \`feat: x\``, `**After:** Task 1, 2`, blank, `**Decision:** d1`, `**Allowed-collateral:** \`x/*\``, `**Squash-with:** Task 4`, blank, `  - [ ] **Step 1: Read first.** \`Dto\` (~line 703)`, `    continued wrapped text`; want exactly `The PATCH takes expected\n\n**Step 1: Read first.** \`Dto\` (~line 703)\n  continued wrapped text\n` (title prefix, checkbox prefixes, field lines and comment lines dropped; the step's continuation keeps its relative indent; runs of blank lines collapse to one; no leading or trailing blank line). A second row: a field name inside step text (`  - [ ] **Step 2:** set **Files:** later`) is kept.
    - `TestNoFlowRendersItsOwnPrompts` — rows impl, review, visual with `cfg.NoFlow = true`; each trial's `prompt.md` artifact (`runs/<id>/<case>/1/prompt.md`) equals the no-flow template rendered: impl contains the plain spec of the case's `task.md` and its `context.md` and no `**Files:**`; review starts `/code-review low` and contains no diff text; visual contains `/case/mockup.png` and not `measure-visual-properties`. A fourth row, flow run, still renders the vendored prompt (`prompts/<target>.md` of the fixture).
    - `TestNoFlowPrepRunsBeforeTheAgent` — review: `gitRepo` with `a.txt` = `new\n` at head, case `diff.patch` = the `a.txt` change `old\n`→`new\n` (`git diff` output), agent script `git diff --stat; git show HEAD:a.txt`; the transcript (`output.md`) shows `a.txt | 2` and `old`; the `hostDocker` log has the prep `exec` after `copyin /` and before the agent's `exec`. Review with a `diff.patch` that does not reverse-apply → the trial is an infra error whose `error` contains `no-flow prep`. Visual: the log holds `exec ` + the visual prep argv before the agent's. Impl: no prep `exec`.
    - `TestNoFlowRunsHaveTheirOwnKeyAndSection` — `BaselineKey` rows: `("impl","",false)`→`impl`, `("impl","",true)`→`impl-no-flow`, `("review","primary",true)`→`review-no-flow-primary`, `("visual","",true)`→`visual-no-flow`; a no-flow run's id contains `-impl-no-flow-` and its header line `"no_flow":true`, a flow run's header has no `no_flow` key; `Repaint` over one flow and one no-flow impl run, with `baselines/impl.json` naming the flow run, writes two index sections (`impl`, `impl-no-flow`) and the no-flow row reads `no baseline`; `Comparable` of the no-flow run against the flow run returns nil.
  - [x] **Step 3: Run them, expect a compile failure.** `go test ./internal/run -race -count=1 -run 'TestPlainTaskStripsFlowFields|TestNoFlowRendersItsOwnPrompts|TestNoFlowPrepRunsBeforeTheAgent|TestNoFlowRunsHaveTheirOwnKeyAndSection'`
  - [x] **Step 4: Write the three prompts**, each a `text/template` (`prompts.Render` fills them; missing keys error):
    - `prompts/no-flow/impl.md`:

      ```markdown
      ## Your task

      {{.Task}}

      ## Context

      {{.Context}}

      ## When you are done

      Implement the task in the current directory, which is the repository. Reply with a short report: what you changed, the tests you added, and each command you ran with its result.
      ```

    - `prompts/no-flow/review.md`:

      ```markdown
      /code-review {{.Effort}}

      End your reply with every finding under the headings `#### Critical`, `#### Important` and `#### Minor`, one list item per finding, each naming its `path:line`.
      ```

      <!-- unverified: that `claude -p` 2.1.286 expands a leading `/code-review <level>` in a clean config home and treats the following paragraph as part of the request rather than a review target — Task 3 step 2 checks it on the real trials -->
    - `prompts/no-flow/visual.md`:

      ```markdown
      Two images of the same screen: the mockup a designer drew, {{.Mockup}}, and a capture of the built app, {{.Capture}}, cut to the same size.

      Read both images and judge whether the capture matches the mockup.

      List each departure on its own line as `- defect: <what differs, and where>`, then end your reply with exactly one line: `verdict: match` or `verdict: mismatch`.
      ```

  - [x] **Step 5: Implement in `internal/run`.**
    - `Config` gains `NoFlow bool`; `Header` gains `NoFlow bool \`json:"no_flow,omitempty"\``, set in `header` from `cfg.NoFlow`.
    - `BaselineKey(role, slot string, noFlow bool) string` per design.md `## No-flow trial` **Keys**; `BaselineID(baselinesDir, role, slot string, noFlow bool)`; `SetBaseline` and the run id pass `h.NoFlow`. In `Repaint`, key `baseAt` and the per-section grouping by `BaselineKey(role, slot, noFlow)`, emitting for each slot in `_indexSlots` order the flow section then the no-flow section. `Comparable` is unchanged.
    - `Run`'s review check: with `cfg.NoFlow`, check `prompts.Vendored(filepath.Join(cfg.PromptsDir, "no-flow"), "review")` instead of the slot's prompt; the must-find check is unchanged.
    - `prepare`: with `cfg.NoFlow`, never resolve standards. `render`: with `cfg.NoFlow`, render from `filepath.Join(cfg.PromptsDir, "no-flow")`, target `cfg.Role`, inputs impl `Task`=`plainTask(task.md)` and `Context`; review `Effort`=`cfg.Effort`; visual `Mockup`, `Capture` as today.
    - `plainTask(task string) string` per design.md's rules: drop a line whose trimmed text starts `**<Field>:**` for each field design.md lists, or `<!--`; strip `- [ ] <n>. ` / `- [x] <n>. ` from the title and `- [ ] ` / `- [x] ` from a step line (the step's indent removed with it; a continuation line loses the same indent); collapse blank runs; trim.
    - `_noFlowPrep = map[string][]string{"review": {"sh", "-c", <design.md review command>}, "visual": {"sh", "-c", <design.md visual command>}}`; in `runTrial`, after the `/case` `CopyIn` and before the agent `Exec`, when `cfg.NoFlow` and the role has a prep: run it; an exec error or non-zero exit returns `infra(fmt.Errorf("no-flow prep: exit %d: %s", …, stderr))`.
    - `report`'s settings line appends `, no-flow` when `h.NoFlow`; `page.go` is not in this task — the run id already names it.
  - [x] **Step 6: Run the tests, expect green.** `go test ./internal/run -race -count=1 -run 'TestPlainTaskStripsFlowFields|TestNoFlowRendersItsOwnPrompts|TestNoFlowPrepRunsBeforeTheAgent|TestNoFlowRunsHaveTheirOwnKeyAndSection|TestEachReviewSlotHasItsOwnBaseline|TestRunWritesIndexOfEveryRun|TestRowsPaintedByOutcome'`
  - [x] **Step 7: Lint and commit** with the subject above.

Correction (2026-10-14): the plan declared `Header.NoFlow` with `json:"no_flow,omitempty"`; it shipped with `omitzero`, because under `encoding/json/v2` (which `internal/run` uses) `omitempty` never omits a `false` bool, so a flow run's header would have carried `"no_flow":false` against the plan's own `no_flow`-absent requirement. `BaselineID`'s new `noFlow` parameter breaks its one caller, so `cmd/flow-eval/main.go`'s default-baseline lookup passing `cand.Header.NoFlow` (Task 2 step 3) moved into this task to keep it `Build: green`. design.md's **Compare** bullet asks the compare report's settings line to name a no-flow side, which no task carried: `internal/compare/compare.go` appends `, no-flow` to a no-flow side's settings, tested by `TestCompareNamesANoFlowSide` in `internal/compare/compare_test.go`.

- [x] 2. `run --no-flow` on the CLI, and the README

**Build:** green
**Files:** `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `README.md`
**Tests:** `TestRunNoFlowReviewNeedsClaudeCode`
**Regression:** `TestRunNoFlowReviewNeedsClaudeCode` — revert → `--no-flow` is an unknown flag (exit 2 with usage), and a codex no-flow review is not refused by name.
**Baseline:** before=11 after=12 <!-- measured: grep -c '^func Test' cmd/flow-eval/main_test.go -->
**Commit:** `feat(cli): add run --no-flow`
**After:** Task 1

**Decision:** no-flow-flag
**Decision:** no-flow-review-code-review

  - [x] **Step 1: Write the failing test.** In `cmd/flow-eval/main_test.go`, `TestRunNoFlowReviewNeedsClaudeCode`, shaped like `TestRunRefusesBeforeAnyContainer` (fake `docker` on PATH that records a call): `run --role review --no-flow --harness codex --model m --effort low` with `OPENAI_API_KEY=k ANTHROPIC_API_KEY=k` exits 2, stderr contains `--no-flow review needs --harness claude-code`, docker never called.
  - [x] **Step 2: Run it, expect a failure.** `go test ./cmd/flow-eval -race -count=1 -run 'TestRunNoFlowReviewNeedsClaudeCode'`
  - [x] **Step 3: Implement.** In `runCmd`: `noFlow := fs.Bool("no-flow", false, "run the role with no flow prompt, rules or tooling: the harness's own behaviour (review: claude-code's /code-review)")`; after the usage check, `if *noFlow && *role == "review" && *harnessName != "claude-code"` print `flow-eval run: --no-flow review needs --harness claude-code: it runs claude-code's /code-review` and return 2; pass `NoFlow: *noFlow` into `run.Config`; add `[--no-flow]` to the usage line. In `compareCmd`, the default-baseline lookup passes `cand.Header.NoFlow` to `run.BaselineID`. `baseline set` needs no change (`SetBaseline` reads the header).
  - [x] **Step 4: Run the tests, expect green.** `go test ./cmd/flow-eval -race -count=1 -run 'TestRunNoFlowReviewNeedsClaudeCode|TestRunRefusesBeforeAnyContainer|TestCompareAndBaselineExitCodes'`
  - [x] **Step 5: README.** Under `### run`, one bullet after the `--slot` bullet: `--no-flow` runs the role with no flow prompt, rules or tooling — impl gets the task as a plain spec plus the change's proposal and design; review runs claude-code's own `/code-review` on the change as the working tree's diff (claude-code only), graded on `--slot`'s must-finds; visual compares the pair with flow's measuring tool removed. Its runs and baselines are keyed `<role>-no-flow[-<slot>]`; compare one with its flow counterpart by `compare <run> --baseline <flow run>`. The prompts are `prompts/no-flow/`, which `prompts sync` never touches.
  - [x] **Step 6: Lint and commit** with the subject above.

Correction (2026-10-14): step 3's `compareCmd` change — the default-baseline lookup passing `cand.Header.NoFlow` to `run.BaselineID` — shipped in Task 1's commit, which needed it to build; this task's commit carries the rest of step 3, and also brings the flags' and README's `baselines/<role>[-<slot>].json` key spellings to `<role>[-no-flow][-<slot>]`.

- [x] 3. Count the tokens of a transcript whose work ran in a forked slash command

**Build:** green
**Files:** `internal/harness/claude.go`, `internal/harness/harness_test.go`
**Tests:** `TestClaudeParseTakesAForkedCommandsTokensFromModelUsage`
**Regression:** `TestClaudeParseTakesAForkedCommandsTokensFromModelUsage` — revert → a no-flow review trial records 0 tokens and $0 cost.
**Baseline:** before=16 after=17 <!-- measured: grep -c '^func Test' internal/harness/harness_test.go @ 5dd1a95 -->
**Commit:** `fix(harness): count a forked slash command's tokens from modelUsage`
**After:** Task 2

**Decision:** no-flow-usage-from-model-usage

  - [x] **Step 1: Write the failing test** in `internal/harness/harness_test.go`: a transcript whose one result event carries an all-zero `usage` and `modelUsage` `{"claude-haiku-5-5":{"inputTokens":8,"outputTokens":3150,"cacheReadInputTokens":52495,"cacheCreationInputTokens":33157,"thinkingTokens":2467}}` (the shape the first committed no-flow review trial's `/code-review` result has) parses to `Input` 8, `Output` 3150, `CacheRead` 52495, `CacheWrite1h` 33157, `Reasoning` 2467. `TestClaudeParseSumsEveryResult` stays as it is: a result with its own usage is never read from `modelUsage`.
  - [x] **Step 2: Run it, expect a failure.** `go test ./internal/harness -race -count=1 -run 'TestClaudeParseTakesAForkedCommandsTokensFromModelUsage'`
  - [x] **Step 3: Implement** per design.md decision `no-flow-usage-from-model-usage`: when every result's `usage` sums to no token, take the tokens from the last result's `modelUsage`, summed over its models, cache creation as `CacheWrite1h`.
  - [x] **Step 4: Run the tests, expect green.** `go test ./internal/harness -race -count=1 -run 'TestClaudeParse'`
  - [x] **Step 5: Lint and commit** with the subject above.

- [x] 4. Haiku-low k=3 no-flow runs and their compares

**Build:** green
**Files:** `runs/index.html`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — eval runs, no code.
**Regression:** none — records measurements.
**Commit:** `chore(runs): add haiku-low no-flow impl, review and visual runs with their compares`
**After:** Task 3

**Decision:** no-flow-runs-no-baseline

  - [x] **Step 1: Run** from the worktree root, one at a time, each as `zsh -ic '<command>'` so the credential comes from the operator's shell and never appears in a command line, commit or report; `--repos /Users/tweety53/Projects` because the default `..` resolves to `flow-eval-worktrees/`: <!-- verified: kan-979 tasks.md Global constraints, the same credential route and --repos, run 2026-10-14 -->
    - `go run ./cmd/flow-eval run --role review --no-flow --harness claude-code --model claude-haiku-5-5 --effort low -k 3 --repos /Users/tweety53/Projects`
    - `go run ./cmd/flow-eval run --role impl --no-flow --harness claude-code --model claude-haiku-5-5 --effort low -k 3 --repos /Users/tweety53/Projects`
    - `go run ./cmd/flow-eval run --role visual --no-flow --harness claude-code --model claude-haiku-5-5 --effort low -k 3 --repos /Users/tweety53/Projects`
  - [x] **Step 2: Check the review setup took** (review run first, before the other two): in `runs/<id>/<case>/1/transcript.jsonl` a `Skill` tool call or the expanded `/code-review` skill text appears, and `output.md` has a `####` severity section. If instead the output says `Unknown skill`/`Unknown command`, reviews an empty diff, or no trial has a `####` section, stop and report it — changing the prompt's form is a plan amendment (design.md decision `no-flow-review-code-review`), not made in this task.
Correction (2026-10-14): the first review run (`20261014T144649Z-review-no-flow-primary-…`) passed step 2's check — `local_command: code_review` in every trial's result, a forked `/code-review` task reviewing the working tree's diff, and `####` sections in each `output.md` — but recorded 0 tokens and $0 cost: the CLI ran the skill in a forked agent (`skip_transcript`), so the result's `usage` is all zero and only `modelUsage` carries the tokens. Task 3 was inserted to count them; that run is deleted and the review run re-made after it. The skill also took the prompt's format sentence as its `Review target:`; at low effort its recipe still read `git diff HEAD`, so the setup holds for this run.

  - [x] **Step 3: A run with any infra error is re-run once whole**; a second infra error is reported, never counted as a fail.
  - [x] **Step 4: Compare** each against the flow haiku-low run of its role (exit 0 non-inferior, 1 regression, 3 improvement are all verdicts; only 2 is a failure):
    - `go run ./cmd/flow-eval compare <review-id> --baseline 20261013T113901Z-review-claude-code-claude-haiku-5-5-low`
    - `go run ./cmd/flow-eval compare <impl-id> --baseline 20261013T112730Z-impl-claude-code-claude-haiku-5-5-low`
    - `go run ./cmd/flow-eval compare <visual-id> --baseline 20261013T114239Z-visual-claude-code-claude-haiku-5-5-low`
    <!-- verified: the three flow runs' headers carry k 3 and the case-set hashes the current cases have (395d8bd4…, 4a69b489…, d3ed92f7…) @ c97af3d — Comparable's case-set and k checks pass -->
  - [x] **Step 5: Lint and commit** each run's `.jsonl`, `.md`, `.html`, its `runs/<id>/` directory, the three `runs/<cand>-vs-<base>.md` and `runs/index.html`, with the subject above; body: per role, passes/k and the compare verdict line. No `baseline set`.

- [x] 5. No-flow review on muse-code: a plain review request

**Build:** green
**Files:** `internal/run/run.go`, `internal/run/run_test.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `prompts/no-flow/review-plain.md`, `README.md`
**Tests:** `TestNoFlowRendersItsOwnPrompts`, `TestRunNoFlowReviewRefusesAHarnessWithoutOne`
**Regression:** `TestNoFlowRendersItsOwnPrompts` — revert → a muse-code no-flow review trial renders `/code-review`, a command muse does not have; `TestRunNoFlowReviewRefusesAHarnessWithoutOne` — revert → `--no-flow --role review --harness muse-code` is refused.
**Baseline:** before=45 after=45 <!-- measured: grep -c '^func Test' internal/run/run_test.go @ e07672e; the new case is a row of an existing table -->
**Commit:** `feat(run): run a no-flow review on muse-code as a plain review request`
**After:** Task 1, 2, 4

**Decision:** no-flow-review-per-harness

  - [x] **Step 1: Write the failing tests.** In `internal/run/run_test.go`, `TestNoFlowRendersItsOwnPrompts`: the existing review row runs on a harness named `claude-code` (a `scriptHarness` wrapper overriding `Name`) and still renders `/code-review low`; a new row runs the same case on the plain `scriptHarness` (any other harness) and its `prompt.md` equals `prompts.Render("../../prompts/no-flow", "review-plain", {})`, carrying no `/code-review` and no diff text. In `cmd/flow-eval/main_test.go`, rename `TestRunNoFlowReviewNeedsClaudeCode` to `TestRunNoFlowReviewRefusesAHarnessWithoutOne`: `--harness codex` still exits 2 before any container, its stderr naming `--no-flow review runs on --harness claude-code or muse-code`.
  - [x] **Step 2: Run them, expect failures.** `go test ./internal/run ./cmd/flow-eval -race -count=1 -run 'TestNoFlowRendersItsOwnPrompts|TestRunNoFlowReviewRefusesAHarnessWithoutOne'`
  - [x] **Step 3: Write `prompts/no-flow/review-plain.md`:**

    ```markdown
    Review the uncommitted changes in the current directory, which is the repository: `git diff` shows them. Report the defects you find.

    End your reply with every finding under the headings `#### Critical`, `#### Important` and `#### Minor`, one list item per finding, each naming its `path:line`.
    ```

  - [x] **Step 4: Implement.** `internal/run`: a no-flow review renders target `review` on `claude-code` and `review-plain` on any other harness, with no inputs for `review-plain`; `Run`'s review check vets the target the run will render. `cmd/flow-eval`: the refusal reads `flow-eval run: --no-flow review runs on --harness claude-code or muse-code: claude-code's /code-review, or a plain review request on muse-code`, for any harness but those two; the `--no-flow` flag help names both. README's `--no-flow` bullet names the muse-code review.
  - [x] **Step 5: Run the tests, expect green**, the step 2 command.
  - [x] **Step 6: Lint and commit** with the subject above.

Correction (2026-10-14): the panel found the muse-code half untested and the supported-harness list held twice (CLI and `noFlowTarget`). A fix commit at the branch tip makes `run.NoFlowReviewTarget` the one source — `Run` refuses a no-flow review on any other harness before a container, and the CLI calls it — with `TestNoFlowReviewNeedsAHarnessWithOne` in `internal/run/run_test.go` and `TestRunNoFlowReviewRefusesAHarnessWithoutOne` table-driven over codex (refused), muse-code and claude-code (not refused); this task's own commit carries neither, so its fields stand as committed.

- [x] 6. Muse-spark-1.3 low k=3 no-flow review run and its compare

**Build:** green
**Files:** `runs/index.html`
**Allowed-collateral:** `runs/*`
**Tests:** **none** — an eval run, no code.
**Regression:** none — records a measurement.
**Commit:** `chore(runs): add the muse-spark-1.3 low no-flow review run with its compare`
**After:** Task 4, 5

**Decision:** no-flow-review-per-harness

  - [x] **Step 1: Run** from the worktree root as `zsh -ic '<command>'`: `go run ./cmd/flow-eval run --role review --no-flow --harness muse-code --model muse-spark-1.3 --effort low -k 3 --repos /Users/tweety53/Projects`.
  - [x] **Step 2: Check the setup took**: each trial's `prompt.md` is `review-plain`, its `output.md` has a `####` severity section and names a path the case's diff touches, and the run has no infra error (a run with one is re-run once whole).
  - [x] **Step 3: Compare** with the flow muse-spark-1.3 low primary review run: `go run ./cmd/flow-eval compare <id> --baseline 20261013T163922Z-review-muse-code-muse-spark-1.3-low` (exit 0, 1 and 3 are verdicts; 2 is a failure). <!-- verified: that run's header carries slot primary, k 3 and case set 4a69b489…, the same as the no-flow claude-code review run's @ e07672e -->
  - [x] **Step 4: Lint and commit** the run's files, its compare and `runs/index.html` with the subject above; body: passes/k, role score and the compare verdict line. No `baseline set`.
