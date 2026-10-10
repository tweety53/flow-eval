# kan-964-flow-eval-harness — live verification

2026-10-09, `flow-eval` at `97ca702`, images `flow-eval-runner` / `flow-eval-proxy` built from the
branch, `CLAUDE_CODE_OAUTH_TOKEN` (subscription), sonnet / low, k=1. No `OPENAI_API_KEY` exists on
this host, so the review trial ran on claude-code; the codex adapter is not live-checked.

## Round 1 — failure look

All three trials ended `infra_error`, zero tokens, `served_model` empty:
`agent produced no output: exit 1: --dangerously-skip-permissions cannot be used with root/sudo
privileges for security reasons`. Fixed by Task 18 (`IS_SANDBOX=1`).

## Round 2 — matched

| Role | Case | Score | wall_ms | Tokens (in / out / cache read / cache write 1h) | cost_usd | Tool calls | served_model | egress_denied |
|---|---|---|---|---|---|---|---|---|
| visual | `kan-870-qm-q6` | 100 | 12450 | 4 / 886 / 33351 / — | 0.0876 | 3 | claude-sonnet-5-5 | 1 |
| review | `kan-961-api-metrics-dashboard` | 0 | 26196 | 2 / 3118 / 12453 / 24969 | 0.1323 | 0 | claude-sonnet-5-5 | 2 |
| impl | `kan-757-…-task-3` | 100 | 180936 | 28 / 12568 / 648035 / 49353 | 0.3880 | 14 | claude-sonnet-5-5 | 7 |

- No figure shows the failure look: tokens, `served_model` and `cost_usd` are non-zero on every run.
- impl: `verify.log` shows the held-out `GroupSessionFloorSetConflictTest` and
  `GroupSessionWorkoutControllerTest` compiled (`:groupsession:compileTestKotlin`) and run
  (`:groupsession:test` executed) after the reset, all three verify commands `[exit 0]`; the agent's
  own copies of those two test files were replaced by the held-out ones, so the pass is not a leak.
- review: score 0 is the model's miss, not the harness — the report lists four Minors and misses the
  one must-find (spec R9's 4xx ratio and p50/p99 panels); the agent made no tool calls.
- Egress: every trial counted denials through the proxy (1, 2, 7); the deliberate
  `git ls-remote https://example.com/x.git` probe was not run separately.

## Fix run 2026-10-10 — HTML page (Task 19)

A real visual run (`kan-870-qm-q6`, sonnet / low, k=1) on the final build printed
`runs/<id>.jsonl, runs/<id>.md, runs/<id>.html`; the page (8.6 KB, valid UTF-8, no `<script`)
carries the header, role score 100.0, the case row and the trial with every metric, its judge usage
and its `output.md`.

## Fix run 2026-10-10 — runs index (Task 20)

A real visual run (`kan-868-qm-q5`, sonnet / low, k=1) on the final build rewrote `runs/index.html`
over this machine's 11 runs: impl, review and visual sections, 4 runs linked to their pages and the
6 from before Task 19 listed `(no page)`; no temp file left. The new run's page links back to
`index.html` and its contents list links its one trial.

## Fix run 2026-10-10 — passable impl cases, long-prompt pricing, own tests, judge verdict (Tasks 21–24)

- **Reference check, sandbox shape** (snapshot as base commit, reference diff on top, rebuilt
  runner image, read-only Gradle cache): every remaining `verify` command of kan-755, kan-863,
  kan-867 and kan-868 exits 0; kan-757 and the other cases were already green.
- **Haiku 5.5 trial** (review `kan-961`, runner CLI 2.1.286, a scratch runs directory outside
  `runs/`): flow-eval priced it $0.0134 from the published rates; the CLI reported $0.2682 with
  `costBasis: unknown`. Its calls' output sums to its result's 9,615.
- **Per-call output** on every Haiku 5.5 runner transcript in `runs/`: the calls' output sums equal
  each result event's output (the impl trials': 23,423, 30,007, 1,970).
- **Runs, one case per role, k=3, effort low** (impl kan-757, review kan-867, visual kan-870). The
  sonnet rows are the 2026-10-09 runs at `f626bb6` (before Task 21; sonnet is priced flat, so
  Task 22 does not move them); Haiku impl and review ran at `6383caa` (kan-757 keeps its module
  suite and the review judge answered every call in one word, so Tasks 23–24's later fixes do not
  move them); Haiku visual ran at `5a38a75`, HEAD's `verdict`:

  | model | impl | review | visual | cost impl / review / visual |
  |-------|------|--------|--------|-----------------------------|
  | sonnet | 100.0 (3/3) | 0.0 (0/3) | 100.0 (3/3) | $0.9645 / $0.2809 / $0.1763 |
  | claude-haiku-5-5 | 66.7 (2/3) | 0.0 (0/3) | 100.0 (3/3) | $0.0889 / $0.0181 / $0.0093 |

  Haiku's failed impl trial failed the held-out `GroupSessionFloorSetConflictTest` /
  `GroupSessionWorkoutControllerTest` run. An earlier Haiku visual run (`20261010T085550Z`,
  replaced) lost a trial to a judge answering "…different defects.\n\n**no**" (Task 24).

## Fix run 2026-10-10 — Muse Code harness and row colours (Tasks 25–26)

- **Row colours:** the index over the real `runs/` paints sonnet impl and both visual runs green,
  Haiku impl yellow and both review runs red, with no blue (no baseline recorded); checked in
  headless Chromium in light and dark schemes.
- **Muse Code setup:** the operator's one-time volume setup printed `Muse Code 1.4.4
  (1.4.4-R5419.1)` and `logged in`.
- **Muse's own shell sandbox:** the first k=3 runs had no working shell — every `bash` call failed
  `bwrap: No permissions to create a new namespace` (panel round 11); those runs were discarded.
  With `--disable-sandbox`, a real impl trial ran 19 `bash` calls with no tool error and no
  permission denial.
- **Runs, muse-code, muse-spark-1.3, effort low, k=3, at `e533a90`** (same three cases as the
  table above), no egress denials, no infra errors:

  | role | score | passes | cost |
  |------|-------|--------|------|
  | impl (kan-757) | 100.0 | 3/3 | $2.2473 |
  | review (kan-867) | 0.0 | 0/3 | $0.5506 |
  | visual (kan-870) | 100.0 | 3/3 | $1.0397 |

## Fix run 2026-10-10 — opus runs; review graded per slot (Task 27)

- **Opus, effort low, k=3**, same three cases, at `33f8d3b` (before Task 27):
  impl kan-757 100.0 (3/3, $2.4208), review kan-867 0.0 (0/3, $0.6423), visual kan-870 100.0
  (3/3, $0.3335).
- **Why every review scored 0:** kan-867's must-finds were raised by the principles and mutation
  slots, and the primary slot's own finding was ignored, so no primary trial could match one. After
  Task 27 the re-mined labels give kan-867 one primary must-find, F1 at `SettingsState.kt:295`;
  `case validate` reports 25 valid. The review runs above were
  not re-run (operator: "fix but do not rerun").
- **Review re-runs on the re-mined labels** (kan-867, `--slot primary`, effort low, k=3): sonnet
  0.0 (0/3, $0.2847), claude-haiku-5-5 0.0 (0/3, $0.0187). No trial raised F1 (a successful unit
  save clearing `loadFailed` while the mode is unconfirmed); one sonnet trial raised the inverse
  path without a `file:line`, judged plausible but matching no label. The earlier review runs of
  every model were deleted. The index now shows each run's valid-trial wall-clock min, mean and
  max.
- **Task 30, judge pin and mixed-judge warning** (at `4d43413`): the real binary compared, on a
  scratch copy of `runs/`, the opus-high review run `20261010T122505Z` against the sonnet-low review
  run `20261010T114457Z`, both from before the header field. It printed
  `warning: judge – vs – — UNRECORDED: scores may not be comparable` on stderr and as a bullet in the
  report, and still exited 3 (improvement to review). The real index was repainted and has a
  `judge` column showing `–` for every existing run; no `runs/*.jsonl` changed.
- **Task 31, principles and failure-modes slots** (prompt pin `b35b350c`, agents HEAD = origin/main
  checked right before the runs; runner image rebuilt at the pin): kan-867, sonnet, effort low, k=3.
  `--slot principles` 0.0 (0/3, $0.2436), `--slot failure-modes` 0.0 (0/3, $0.2293); both run
  headers record the sonnet-medium judge. Every trial wrote a full report in flow's format, but all
  its findings were Minor, which precision and recall do not count: none raised F6 (`loadFailed`
  as a stored copy of "some field is unconfirmed") or F9 (a successful unit save clearing
  `loadFailed` while the mode is unconfirmed); one principles trial named the three-way
  `loadFailed` conditional as a Minor readability nit.
- **Task 32, principles standards** (prompt pin `b35b350c`): rendered through the run's prepare path
  with no trial, kan-755 (gymie snapshot) fills `**Project standards:** /work/CLAUDE.md,
  /work/CONTRIBUTING.md, /agents/rules/kotlin-backend-development-standard.mdc`, all three regular
  files where the container sees them, and kan-753 (gymie-frontend snapshot) fills it empty with the
  no-standards paragraph kept — as flow's principles slot resolves each project. The kan-867
  principles run above (gymie-frontend) already got exactly that: no standards.
- **Clean re-run after Task 33** (one case per role, all earlier runs deleted, prompt pin
  `b35b350c`, judge sonnet/medium, effort low, k=3, 2026-10-10): claude-haiku-5-5 — impl kan-757
  100.0 (3/3, $0.1756), review kan-867 primary / principles / failure-modes 0.0 each (0/3; $0.0188 /
  $0.0236 / $0.0174), visual kan-870-qm-q6 100.0 (3/3, $0.0135); sonnet — impl 100.0 (3/3,
  $1.2433), review 0.0 each (0/3; $0.2731 / $0.2476 / $0.2253), visual 100.0 (3/3, $0.1782). No
  infra errors. No review trial matched a must-find; most raised Important findings matching none.
