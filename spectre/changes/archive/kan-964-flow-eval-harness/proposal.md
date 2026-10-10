# kan-964-flow-eval-harness

## Why

The flow pipeline's model and effort per role (implementer, review panel, visual-verify) are chosen by
judgment. Nothing measures whether a cheaper model-effort pair, a new model release or another agent
provider keeps quality. A switch is either avoided and the money spent, or taken blind and any
regression found later in real changes. There is no fixed set of real cases, no isolated way to run a
role headless under different harnesses, and no record of time, tokens, cost or quality per role.

## What changes

- A new repository, `flow-eval`, holds a Go CLI with these commands:
  - `run` executes one role (`impl`, `review`, `visual`) on its case set, under `claude-code` or
    `codex`, at a given model and effort, for `k` trials each.
  - `compare` decides whether a candidate run is non-inferior to the role's stored baseline.
  - `baseline set` records a run as that baseline.
  - `case add` / `case validate` add and check cases.
  - `prompts sync` vendors the flow role prompts from a pinned agents sha.
- Every trial runs in a fresh Docker container. The container has a repository snapshot without
  history, egress limited to an allowlist proxy, and credentials passed in by environment. A trial
  that needs Docker (Testcontainers) gets its own Docker-in-Docker sidecar.
- A repository's Gradle cache is warmed once in a setup container that runs no agent, so the
  trial allowlist never needs GitHub.
- The runner image carries flow's visual measurement script at the prompt pin.
- Cases are real gymie history:
  - 6 implementer cases: per-task commits from archived changes' `ledger.md`. They are graded by the
    reference commit's held-out tests, the agent's own tests, the module suite and lint; a case
    whose module suite fails its own reference in the sandbox drops it, and the agent's own tests
    in that module run by name.
  - 6 reviewer cases: pre-fix diffs, with must-find labels from panel findings that were later
    fixed and known false positives from findings that were withdrawn.
  - 13 visual cases (1 match, 12 mismatch): mockup and capture image pairs with labelled verdicts.
  - Note (2026-10-10, Task 33): the set was cut to one case per role, the one each recorded run
    used (design.md §3, decision `case-set-one-per-role`).
- Each trial records:
  - wall clock, tokens (input, output, cache read, cache write), cost and tool calls;
  - reliability, efficiency and work-quality metrics;
  - role metrics: recall, precision and F1 for review; accuracy and false-pass rate for visual.
- Results are `runs/<id>.jsonl` plus a Markdown report. `compare` exits 0 when the candidate is
  non-inferior, 1 on a regression, 3 when an improvement is flagged for review, and 2 on an infra or
  usage error.
- `docs/cases.md` documents how to add a case of each type.

In-run fix 1 (2026-10-09), from the live check: a claude-code trial runs as root in the runner
container, where Claude Code refuses `--dangerously-skip-permissions`; the trial env marks it
sandboxed (`IS_SANDBOX=1`).

Fix run (2026-10-09), operator request: every `flow-eval run` also writes `runs/<id>.html`, one
self-contained page holding the run header, role score, case table, infra errors and every trial's
metrics, output, verify log and diff.

Fix run (2026-10-10), operator request: every run also rewrites `runs/index.html`, one page
listing every run by role with its key figures and a link to its own page; each run page links
back and lists its cases and trials.

Fix run (2026-10-10): every impl case is checked against its own reference commit, and the kan-755
impl case drops a verify command its snapshot already fails; a model whose rate depends on the
prompt's size (Claude Haiku 5.5) is priced per call.
