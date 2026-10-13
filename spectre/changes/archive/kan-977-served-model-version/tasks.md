> **Execution:** `/flow` implements this plan. Mark a task's own checkbox when
> `check-task-commit-fields.sh` passes on that task's commit.
> **Relocation:** no
> **Baseline convention:** every `**Baseline:** before=N after=M` is chain-position absolutes —
> `before` is the count at this task's commit's parent and `after` at the commit, each task
> measured at its own position, never a plan-wide pair — split by source set, counted by a
> word-boundary-safe counter (`@Test` never matches `@Testcontainers`), with any `measured:`
> command naming no ref of its own.

# kan-977-served-model-version

KAN-977: show each run's served model id on the index, the run page, the `.md` report and in `compare`. `design.md` is canonical for the decisions cited by ID.

**Repository:** single — the worktree `/Users/tweety53/Projects/flow-eval-worktrees/kan-977-served-model-version`; every path below is relative to it.

## Global constraints

- Go stdlib only; no new dependency.
- Go skills per `CLAUDE.md`: `modern-go-guidelines:use-modern-go`, and `~/.agents/skills/go-coding-standards`, `go-error-handling`, `go-modernize`, `go-test-quality`, `go-test-table-driven`.
- **Verify** (worktree root): `gofmt -w .`, `test -z "$(gofmt -l .)"`, `go vet ./...`, then the task's `go test ... -run`.
- No test touches the network, a real model or Docker.

---

- [x] 1. Served model on the run report, page and index

**Build:** green
**Files:** `internal/run/run.go`, `internal/run/page.go`, `internal/run/index.go`, `internal/run/run_test.go`, `README.md`
**Tests:** `TestServedModelIsDerivedFromTrials`
**Regression:** `TestServedModelIsDerivedFromTrials` — revert → `Served` is undefined and the `.md`, page and index no longer name the served model.
**Baseline:** before=29 after=30 <!-- measured: grep -c '^func Test' internal/run/run_test.go -->
**Commit:** `feat(run): show each run's served model on its report, page and the index`
**After:** none

**Decision:** served-derived-not-stored
**Decision:** version-is-served-id

  - [x] **Step 1: Write the failing test.** In `internal/run/run_test.go`, `TestServedModelIsDerivedFromTrials`, table-driven over `[]Line` inputs: none naming a model → `""`; two lines `claude-sonnet-5-5` → `claude-sonnet-5-5`; one line `a,b`, one `b` and one infra-error line `c` → `a, b, c`; an empty `served_model` beside `x` → `x`. Then, for one run served `claude-sonnet-5-5`, assert `report(...)` contains `served claude-sonnet-5-5`, the page bytes contain `<dt>served</dt><dd>claude-sonnet-5-5</dd>`, and the index (built the way `TestRunWritesIndexOfEveryRun` builds it) contains `<th>served</th>` and `<td>claude-sonnet-5-5</td>`; a run served none renders `–` in all three.
  - [x] **Step 2: Run it, expect a compile failure.** `go test ./internal/run -race -count=1 -run 'TestServedModelIsDerivedFromTrials'`
  - [x] **Step 3: Implement.** In `run.go` add exported `Served(lines []Line) string` per design.md `## Served model`; add `Served string` to `summary`, set in `summarize`; in `report` append `, served %s` to the settings line (`–` for `""`). In `page.go` add `<dt>served</dt><dd>…</dd>` after `model`. In `index.go` add `<th>served</th>` after `<th>model</th>` and the matching `<td>` from `.S.Served`. Render `""` as `–` with the templates' own `{{or .S.Served "–"}}`, no new helper.
  - [x] **Step 4: Run the tests, expect green.** `go test ./internal/run -race -count=1 -run 'TestServedModelIsDerivedFromTrials|TestRunWritesHTMLPage|TestRunWritesIndexOfEveryRun'`
  - [x] **Step 5: README.** Under `## Reading results`, one bullet after the `runs/<id>.html` bullet: `served` is the model id(s) the run's trials were served, read from their `served_model`, so it names what an alias such as `sonnet` resolved to; `–` when the harness names none (codex).
  - [x] **Step 6: Lint.** `gofmt -w .`, `test -z "$(gofmt -l .)"`, `go vet ./...`

Correction (2026-10-13): Step 1 declared the index built the way `TestRunWritesIndexOfEveryRun` builds it (through `Run`); shipped instead: `Repaint` over two hand-written `.jsonl` files (one served `claude-sonnet-5-5`, one served none), which also writes both pages — `Run`'s fixture harness always serves `m1`, so it cannot produce the no-served run the `–` case needs.

- [x] 2. compare names and warns on the served model

**Build:** green
**Files:** `internal/compare/compare.go`, `internal/compare/compare_test.go`, `README.md`
**Tests:** `TestCompareWarnsOnDifferentServedModels`
**Regression:** `TestCompareWarnsOnDifferentServedModels` — revert → two runs served different models compare with no `served model … DIFFER` warning.
**Baseline:** before=14 after=15 <!-- measured: grep -c '^func Test' internal/compare/compare_test.go -->
**Commit:** `feat(compare): warn when candidate and baseline were served different models`
**After:** Task 1

**Decision:** served-warning-both-known

  - [x] **Step 1: Write the failing test.** In `internal/compare/compare_test.go`, `TestCompareWarnsOnDifferentServedModels`, table-driven and shaped like `TestCompareWarnsOnMixedPromptPins`, setting every fixture line's `ServedModel`: `m1` vs `m1` → `""`; `m2` vs `m1` → `warning: served model m2 vs m1 — DIFFER: the runs were served different models`; `m2` vs `m1` with a regressing candidate → the same warning and `ExitRegression`; `""` vs `m1` → `""`. Assert `d.Served`, `d.Exit`, the warning in `d.Report` exactly when set, and the report's candidate/baseline line naming both served sets.
  - [x] **Step 2: Run it, expect a compile failure.** `go test ./internal/compare -race -count=1 -run 'TestCompareWarnsOnDifferentServedModels'`
  - [x] **Step 3: Implement.** Add `Served string` to `Decision` (doc comment like `Prompts`'); in `Compare` set it from `run.Served(cand.Lines)` and `run.Served(base.Lines)` per design.md; `Warnings()` returns it after `d.Judges`; the report's candidate/baseline line gains `served <ids>` per side (`–` for `""`).
  - [x] **Step 4: Run the tests, expect green.** `go test ./internal/compare -race -count=1 -run 'TestCompareWarnsOnDifferentServedModels|TestCompareWarnsOnMixedPromptPins|TestCompareWarnsOnMixedJudges'`
  - [x] **Step 5: README.** In `### compare`, the sentence listing what still compares with a `DIFFER` warning names served models too.
  - [x] **Step 6: Lint.** `gofmt -w .`, `test -z "$(gofmt -l .)"`, `go vet ./...`

- [x] 3. Price muse-wonder-1.0

**Build:** green
**Files:** `pricing.json`
**Tests:** **none** — data only; `metrics.LoadPricing` validates it on every run.
**Regression:** none — a revert makes every muse-wonder-1.0 trial an infra error (`not in pricing.json`).
**Commit:** `chore(pricing): add muse-wonder-1.0`
**After:** none

  - [x] **Step 1: Add the entry.** In `pricing.json`, after `muse-spark-1.3`: `"muse-wonder-1.0": {"input": 4.00, "output": 20.00, "cache_read": 0.20, "cache_write": 4.00, "source": "https://dev.meta.ai/docs/pricing-rate-limits"}`. <!-- verified: dev.meta.ai/docs/pricing-rate-limits read 2026-10-13 — input $4.00, output $20.00, cached input $0.20, no cache-write price and no long-context premium; cache_write = input follows the muse-spark-1.3 entry's convention -->
  - [x] **Step 2: Validate.** `jq -e '."muse-wonder-1.0".input == 4' pricing.json` exits 0.
