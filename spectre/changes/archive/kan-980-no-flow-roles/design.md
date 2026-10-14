## Context

KAN-980. A trial already runs on an empty harness config home (`internal/harness`: `CLAUDE_CONFIG_DIR` freshly created, no plugins, no CLAUDE.md), so "flow" in a trial is exactly three things: the vendored role prompt (`prompts/<target>.md`, which carries flow's method and rules), the plan-shaped task text of an impl case (`task.md` is a flow `tasks.md` task), and flow's `measure-visual-properties.sh` / `flow-guard` on the runner image's PATH. `--no-flow` removes all three and leaves grading untouched.

## No-flow trial

- **Prompts** live in `prompts/no-flow/{impl,review,visual}.md`, flow-eval's own, rendered with `prompts.Render(<prompts>/no-flow, <role>, inputs)`. `prompts sync` never writes there (it writes the targets, `rules/` and `PIN`). Texts are in `tasks.md` Task 1.
- **impl** inputs: `Task` = `plainTask(task.md)`, `Context` = `context.md`; `/case` receives the rendered prompt alone, never `task.md` or `context.md`, so the flow plan never reaches the agent. `plainTask` keeps the task's title (the `- [ ] <n>. ` prefix dropped) and each step's text (the `  - [ ] ` prefix dropped, the `**Step N: …**` lead kept), and drops every `**<Field>:**` line of `Build`, `Files`, `Allowed-collateral`, `Tests`, `Regression`, `Baseline`, `Commit`, `After`, `Decision`, `Squash-with`, and every `<!-- … -->` comment line.
- **review** input: `Effort` = the run's `--effort`. The diff is not in the prompt; the working tree carries it (below). No `## standards` are resolved or copied in, whatever the slot.
- **visual** inputs: `Mockup`, `Capture` = `/case/mockup.png`, `/case/capture.png`, as today.
- **Prep**: after `/case` is copied in and before the agent starts, a no-flow trial runs its role's prep command with `Exec` (cwd `/work`); a non-zero exit or exec error is an infra error naming the step (`no-flow prep: …`).
  - review: `sh -c 'git apply -R ../case/diff.patch && git add -A -f && git -c user.name=flow-eval -c user.email=flow-eval@localhost commit -q -m before && git apply ../case/diff.patch && git add -N .'` — the snapshot is the change's head, so this leaves `HEAD` at the pre-change tree and `git diff` exactly the change. <!-- measured: on gymie-frontend@b246cc45 snapshot (git archive, init, add -A -f, commit) with cases/review/kan-867…/diff.patch → `git diff --stat` 8 files changed, 341 insertions(+), 32 deletions(-); the patch has 8 `diff --git` headers @ 2026-10-14 -->
  - visual: `rm -f /usr/local/bin/measure-visual-properties.sh /usr/local/bin/flow-guard` (paths per `docker/runner.Dockerfile:53-56`).
  - impl: none.
- **Keys**: `BaselineKey(role, slot, noFlow)` = `<role>[-no-flow][-<slot>]`; it names the run id, the baseline file and the index section, so a no-flow run is never painted against a flow baseline. The header records `"no_flow": true` (omitted when false) and an empty `prompt_pin`: a no-flow run renders no vendored prompt, its prompts versioned by `flow_eval_sha`, so `compare` warns on no prompt pin when either side is no-flow.
- **Compare**: `Comparable` keeps refusing another role or slot and ignores `no_flow`, so `compare <no-flow run> --baseline <flow run>` works; the report's settings line says `no-flow` for a no-flow side.

## Decisions

### No-flow is a flag on the existing roles

**ID:** no-flow-flag
**Status:** active
**Chosen:** `run --no-flow` on `impl|review|visual` — every role switch (timeouts, inputs, grading, pages) stays keyed on the three roles.
**Considered:** three new `--role` values (`impl-no-flow`, …) — every switch would map each back to its base role.

### Review no-flow is graded on the slot's must-finds, primary by default

**ID:** no-flow-review-primary
**Status:** active
**Chosen:** `--slot` (default `primary`) picks the labels; the general correctness reviewer is primary's counterpart, and the run compares with the flow primary run.
**Considered:** every slot's must-finds — F1 and F9 are one defect, and mutation labels are test gaps a general review rarely raises.

### Review no-flow invokes the harness's own /code-review

**ID:** no-flow-review-code-review
**Status:** superseded by no-flow-review-per-harness
**Chosen:** prompt `/code-review <effort>` plus one sentence naming the grader's heading shape; the diff reaches the skill as the working tree's diff (Prep). Claude Code only: `run --no-flow --role review` with another harness exits 2.
**Considered:** the skill alone — the grader reads only `#### Critical|Important|Minor` sections, so a free-form report scores recall 0; a plain "review this diff" prompt — not the harness's skill; codex's `/review` — no codex no-flow run is asked for.
**Superseded because:** the operator asked for no-flow review runs on muse-code, which the claude-code-only refusal blocked.

### Review no-flow is each harness's own review, a plain request where it has none

**ID:** no-flow-review-per-harness
**Status:** active
**Chosen:** on `claude-code`, `/code-review <effort>` plus the format sentence, as decision `no-flow-review-code-review` chose; on `muse-code`, `prompts/no-flow/review-plain.md` — a plain request to review the working tree's diff, plus the same format sentence — since Muse Code ships no review skill or command (`muse skills list` names none; the `/review` strings in its binary are plugin-schema examples). The prep and grading are the same on both. `run --no-flow --role review` on any other harness exits 2.
**Considered:** a muse plugin or skill installed into the trial — flow-eval's own text in a harness wrapper, no more the harness's behaviour than the plain prompt; codex's `/review` — no codex no-flow run is asked for.

### One format sentence, not a judge extraction

**ID:** no-flow-format-line
**Status:** active
**Chosen:** review and visual prompts each carry one sentence giving the output shape the grader parses; grading stays byte-identical to the flow runs.
**Considered:** a judge call extracting findings from a free-form report — an extra step whose errors the flow arm never has.

### Impl no-flow gets a plain spec

**ID:** no-flow-plain-spec
**Status:** active
**Chosen:** `plainTask` strips the flow plan fields from `task.md`; `context.md` (proposal and design) stays as the spec's background.
**Considered:** `task.md` as-is — the agent would still be handed a writing-plans plan.

### Visual no-flow removes the flow measuring tool

**ID:** no-flow-visual-no-tools
**Status:** active
**Chosen:** the prep deletes `measure-visual-properties.sh` and `flow-guard` from the trial container.
**Considered:** leaving them unmentioned on PATH — an agent could still find and use flow tooling.

### Runs committed, baselines left to the operator

**ID:** no-flow-runs-no-baseline
**Status:** active
**Chosen:** this change commits one `claude-haiku-5-5` low k=3 run per no-flow role and its compare against the flow haiku-low run (`20261013T112730Z-impl-…`, `20261013T113901Z-review-…` (primary), `20261013T114239Z-visual-…`); the operator runs other model/effort pairs afterwards and picks each no-flow baseline by hand.
**Considered:** compare against `baselines/<role>.json` (opus-low) — mixes the model change into the flow/no-flow difference.

### A forked slash command's tokens come from modelUsage

**ID:** no-flow-usage-from-model-usage
**Status:** superseded by no-flow-usage-5m-writes
**Chosen:** when every result event of a claude-code transcript carries an all-zero `usage` — the `/code-review` skill ran in a forked agent whose events the transcript skips — `Parse` takes the tokens from the last result's cumulative `modelUsage`, summed over its models, cache creation counted as 1-hour writes (what every runner transcript writes). A transcript with any result usage is parsed exactly as before, so flow runs are unchanged.
**Considered:** always reading `modelUsage` — it also counts any subagent's tokens, which would move every flow run's figures; leaving it — every no-flow review trial would record 0 tokens and $0, and compare's cost columns would favour it falsely.
**Superseded because:** the review panel measured the committed no-flow review run: the CLI's own `total_cost_usd` matches its trials' tokens priced with 5-minute cache writes exactly (reported/recorded 20.000 on every flow trial, 15.4–16.6 on the no-flow trials as 1-hour), so the 1-hour premise overpriced every no-flow review trial by about 30%.

### A forked slash command's cache writes are 5-minute writes

**ID:** no-flow-usage-5m-writes
**Status:** active
**Chosen:** `Parse`'s `modelUsage` fallback (decision `no-flow-usage-from-model-usage`, otherwise unchanged) counts cache creation as 5-minute writes (`CacheWrite`), the TTL the CLI's own reported cost prices them at; the review run is re-made on it.
**Considered:** 1-hour writes — refuted by the reported cost above; re-pricing the committed run's lines by hand — a run's figures are what its flow-eval sha computes, so the run is re-made instead.

## Open questions

## Live check

`flow.verify` re-reads the committed review no-flow run (Task 3) against what the real runner image and model produced: trial 1's `transcript.jsonl` shows a `Skill` call (or the `/code-review` expansion) and its `output.md` at least one finding under a `####` severity heading. Not working looks like: `Unknown skill`/`Unknown command` in the output, a review of an empty diff ("no changes"), or a report with no `####` severity sections — recall 0 for a reason that is the setup, not the model.
