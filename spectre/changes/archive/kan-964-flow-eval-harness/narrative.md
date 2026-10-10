# kan-964-flow-eval-harness — session narrative

## 2026-10-09 — creating run

- **Plan lookup for mined cases.** The implementation miner first picked the plan revision by
  committer date, then by the change's base date; both picked the wrong `tasks.md` for kan-867,
  whose plan was rewritten after rebases. What held: the latest *author* date at or before the
  reference commit's author date.
- **Answer leakage through the miner.** Task 14's first implementer mined cases with a miner that
  still copied plan artifacts into the review diff; it was stopped and its record closed `stopped`.
  Plan files are now excluded from `diff.patch`, and findings on plan artifacts, other
  repositories or lines outside the diff are routed to `ignored` labels.
- **Mutation proofs in a scratch copy failed** (the copy lacked the module context); they were
  redone in place once no implementer was running.
- **BSD tooling.** `sed 0,/re/` is GNU-only; python was used for first-match edits. In zsh, a loop
  variable named `path` clobbers `PATH` — renamed.
- **Panel round 0** raised 16 findings (2 Critical: a shared, writable Gradle home across trials).
  All reproducers were line-anchored, so every fix moved their premises and the pinned re-runs were
  refused as ambiguous; they were re-authored twice — once after chunk 1, again after chunk 2
  shifted `run_test.go` — with premises on lines identical in the pre-fix and fixed trees. Lesson:
  a premise on a line the fix's own file edits is void by construction; anchor premises to lines
  outside every file the finding's fix will touch.
- **Operator decisions taken mid-run:** a grading failure after the agent's output is a scored
  fail; the visual score is balanced accuracy; `compare` prints both runs' flow-eval shas and flags
  a difference. The dispatcher narrowed the first to grading only: harness-side failures (trial
  model unpriced, egress count, artifact write) stay `infra_error`.
- **Re-review shallowness.** The sonnet/low fix-round re-review first reported "all fixed" without
  reading a hunk; sent back once, it found F17 (a dated served model id no longer matched).
- **Live check.** No credential was set in the agent shell; the operator added
  `CLAUDE_CODE_OAUTH_TOKEN` to `~/.zshrc`, which the already-started shell could not see, so each
  command read it through `zsh -ic`. No `OPENAI_API_KEY` exists, so the review trial ran on
  claude-code and the codex adapter is not live-checked. Round 1 found every claude-code trial
  refused as root (`--dangerously-skip-permissions`); Task 18 sets `IS_SANDBOX=1`, and round 2
  matched on all three roles.

## 2026-10-10 — fix run

- **Request:** one HTML page per run. Operator answers: append to the plan, `run` only, everything
  inline in one file. Implemented inline (Task 19, class regular).
- **The per-task review found two Importants** the first draft missed: a cancelled run returned
  before writing any report, and decoding metrics into `map[string]any` printed `1.5e+06` and
  `<nil>`. Both fixed; a post-fix re-review added a Minor (an artifact cut inside a multi-byte
  character), fixed inline.
- **A mutant survived only by accident of the fixture:** dropping `json.Deterministic` passed with a
  one-key tool-call map; a six-key map makes the unsorted order fail all but 1 in 720 runs.
- **The case-table mean wall time had no test that could tell it from zero** (fake trials run in
  milliseconds); a direct `summarize` test now pins it.
- `close-task.sh` refuses a re-run naming a review-fix commit as the task sha (subject and Baseline
  belong to the task commit); the clean re-review's tick was taken with `flow tasks tick`.

## 2026-10-10 — fix run (runs index)

- **Request:** "see everything together, structured with links". Operator answer: an index page
  of all runs; appended as Task 20.
- **The per-task review's Important:** `os.WriteFile` truncates in place, so two runs sharing a
  runs directory could leave a torn `index.html` (reproduced on APFS); the index is now replaced
  through a temp file and a rename. Its Minors: a role outside impl/review/visual vanished, and
  html/template turned an id with a colon (`qwen:7b`) into `#ZgotmplZ` — `./<id>.html` keeps it a
  relative URL.
- **The panel found a glob over the operator's path:** `--runs …/runs[old]` listed nothing and
  `runs[` failed the run; `os.ReadDir` replaced the glob.
- **Reproducer premise drift again:** moving `Load` out of `index.go` shifted the premise line the
  panel's reproducer anchored on; re-anchored to `page.go:1` and proved against the pre-fix tree.

## Fix run 2026-10-10 — passable impl cases, long-prompt pricing, the agent's own tests (Tasks 21–23)

- **The host passed what the sandbox could not:** the reference check first ran in host worktrees,
  where kan-863's gateway health check found the operator's own Redis on 6379 and the frontend's
  UI tests found a desktop GL stack. Run in the trial's sandbox shape, it refuted four of six impl
  cases: a snapshot gap (kan-755), a missing Redis (kan-863), skiko's missing `libGL`/`libEGL`/
  `libfontconfig` (kan-867, kan-868), and a 3,145-test suite failing a different timing-bound UI
  test every run.
- **The runner's CLI is not the host's:** per-call usage was built from `message_delta` keyed by
  `api_message_id`, measured on the host CLI 2.1.296; the runner's 2.1.286 writes no such field, so
  every delta was dropped and long-prompt Haiku 5.5 calls lost their output surcharge (round 6
  Critical). Deltas now go to the message streaming.
- **The CLI's own cost is no yardstick for Haiku 5.5:** it reports `costBasis: unknown` and a
  figure 20× the published rates.
- **Dropping a suite dropped the agent's tests with it** (round 6): the grader now runs the
  agent's own `@Test` files by name where a case drops its module suite; review then caught a
  helper becoming a selector Gradle fails, an env-prefixed suite run twice, a module name reaching
  `sh -c`, and `@TestConfiguration` counted as `@Test`.

## 2026-10-10 — integrate run

- **Preflight:** staged and drift clean; every guard present.
- **Unfinished-work gate:** clear — every plan item checked, no open finding.
- **Rebase:** none needed — the 2 commits `origin/main` gained were already on the branch; merge base moved to `3f234c9`.
- **Untracked `.DS_Store`** in the worktree root was deleted before the commit rather than landed; the untracked `runs/` reports are committed per decision `results-in-repo-files`.
- **Route:** pull request, from the project's configured default.
