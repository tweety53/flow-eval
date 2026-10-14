# kan-980-no-flow-roles

KAN-980.

## Why

- flow-eval measures each role only through flow's own prompts and rules, so nothing says how much flow adds over the harness used plainly: a role's score cannot be read against a no-flow control.

## What changes

- `flow-eval run --no-flow` runs a role on its existing cases with flow taken out of the trial: no vendored flow prompt, no flow rules, no flow tooling — the harness's own behaviour on a clean config home.
  - **impl** — the agent gets the task as a plain spec (the flow plan fields stripped) and the change's proposal and design, and implements it.
  - **review** — Claude Code's built-in `/code-review` skill reviews the case's change as the working tree's diff; graded on the slot's must-finds (primary by default).
  - **visual** — the agent compares the mockup and the capture with no flow verifier prompt, and the flow measuring tool is removed from the container.
- Grading, pricing and the judge are unchanged; each prompt names only the output shape the grader reads.
- A claude-code transcript whose work ran in a forked slash command — the review's `/code-review` — has its tokens counted from the result's `modelUsage`, so a no-flow review trial's cost is not recorded as $0.
- No-flow runs are their own index sections and baselines (`impl-no-flow`, `review-no-flow-<slot>`, `visual-no-flow`), and `compare` takes a flow run as a no-flow run's baseline.
- One `claude-haiku-5-5` low k=3 run per no-flow role is committed with its compare against the flow haiku-low run of the same role; no baseline is set.

## Fix 1 (2026-10-14)

- `--no-flow --role review` also runs on `muse-code`. Muse Code ships no review skill or command, so its no-flow review is a plain request to review the working tree's diff, with the same one format sentence the grader reads; codex stays refused. One `muse-spark-1.3` low k=3 run is committed with its compare against the flow muse-spark-1.3 low primary review run.
