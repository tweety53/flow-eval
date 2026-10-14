# kan-979-tune-failure-modes-prompt — design

## Context

- The case's failure-modes must-find is F9 (`labels.json`): `SettingsState.kt:295`. Re-graded failure-modes runs on agents `2e053aaa` at k=3: haiku-low 0/3, sonnet-low 0/3, opus-low 1/3.
- What the haiku trials reported instead: a stale-unit lost update on a mode write, the mutex held across the PUT, an unverified server contract for an omitted `sessionView`, a cancellation leaving the optimistic value. Each checked a call's own failure; none traced what a later success does to the state an earlier failure left.
- The loop is `docs/prompt-tuning.md`. This change follows it as written; nothing here restates it.
- `skills/flow/failure-modes-reviewer-prompt.md` is byte-identical at `2e053aaa` and at the current pin `a061813d`, so the old k=3 runs measured today's prompt; the before run is still re-run at k=6 so the counts share k.

## The prompt step

Variant 1 (the starting point; variants 2–3 revise its wording only per `docs/prompt-tuning.md` § Stop):

- **Recovery after a failure — a required step, done before anything else:**
  1. List every piece of state the diff's code sets when a call fails — an error or failed flag, a retry affordance's visibility, a stale or unconfirmed marker, a fallback value shown in place of a real one.
  2. For each, grep its file for every line that clears or overwrites it, unchanged lines included, and name what the failure left unconfirmed.
  3. For each clearing line, ask whether the success that reaches it re-confirms everything the failure left unconfirmed, and whether that success's own response carried the missing value and was discarded. A clear on a partial recovery — the state then claims recovery while some value is still unconfirmed — is Important at least, reported at that clearing line under the error-return lens.
- Output: `### Failure-recovery check` as the first subsection of `## Output Format`, before `### Summary` — each failure state, its setters and clearers as `path:line`, what the failure left unconfirmed, and a verdict per clearer.
- Never names anything from the case (`loadFailed`, settings, units, workout mode).

## Measuring

- Candidate: `--role review --slot failure-modes --harness claude-code --model claude-haiku-5-5 --effort low -k 6`.
- Finds = trials whose `passed` is true (recall 1 = F9 found; F9 is the slot's only must-find).
- After the haiku winner is chosen, its results are shown to the operator; sonnet-low and opus-low at k=6 run only on their go.

## Live check

None: the change edits a prompt and records eval runs; the runs themselves are its measurement against the real models.

## Decisions

### Where the prompt change lives

**ID:** agents-canonical-vendored
**Status:** active
**Chosen:** edit agents `skills/flow/failure-modes-reviewer-prompt.md`, vendor into flow-eval with `prompts sync` — as primary and principles did; agents stays the prompt's single source.
**Considered:** edit `prompts/review-failure-modes.md` in flow-eval — the vendored copy would diverge from its pin and `prompts sync` would overwrite it.

### Measuring model and k

**ID:** haiku-low-k6
**Status:** active
**Chosen:** haiku-low at k=6, with a same-k before run on the current pin — the operator's "k=6, haiku-5.5-low".
**Considered:** reuse the k=3 haiku run as the before figure — mixed k makes the counts incomparable.

### Variants and stopping

**ID:** best-of-three-variants
**Status:** active
**Chosen:** `docs/prompt-tuning.md` § Version every variant and § Stop as written — at most three variants, each a commit on the pushed agents branch `kan-979-failure-modes-variants`; stop early at ≥3/6; most finds wins, a tie to the shorter wording; none ever finds → land nothing. The operator's "same versioned approach as before".
**Considered:** iterate until a target is hit — unbounded cost and overfitting to one case.

### The failure-recovery step

**ID:** failure-recovery-step
**Status:** active
**Chosen:** a step that lists the state a failure sets, greps every line that clears it, and judges each clear as a full or partial recovery, checking whether the success's own response carried the missing value — the reasoning the haiku trials skipped, phrased for any diff.
**Considered:** a fifth lens beside the four — a lens is applied per call and the trials already walked calls one at a time without reaching the later clear; the principles step reworded for this slot — it asks whether stored state should be derived, a principles question, not what a success does after a failure.

### Cross-model runs after the operator reads the haiku results

**ID:** cross-model-on-operator-go
**Status:** active
**Chosen:** once the haiku winner is landed, show the operator the before and per-variant haiku finds and what the trials listed, and run the winner on sonnet-low and opus-low at k=6 only on an explicit go; silence or no → skip, recorded under the task. The operator's answer.
**Considered:** always run them — the operator wants to decide after seeing haiku; never run them — the operator asked for them conditionally.

### Baselines

**ID:** baselines-unchanged
**Status:** superseded by opus-low-baseline-and-run-cleanup
**Chosen:** `baselines/review-failure-modes.json` (opus-low) stays as it is — a cheap-model run is a candidate against it.
**Considered:** set a new run as the baseline — not asked for.
**Superseded because:** the operator asked, after task 4, for the opus-low k=3 run to become the baseline.

### Baseline and run cleanup

**ID:** opus-low-baseline-and-run-cleanup
**Status:** active
**Chosen:** after task 5, set the variant-2 opus-low k=3 failure-modes run `20261014T113408Z-review-failure-modes-claude-code-opus-low` as the failure-modes baseline, and delete every run whose k is 6 and every run whose index row is `row-fail` (none passed), across all roles — the operator's instruction after reading the cross-model results.
**Considered:** keep the baseline unchanged (`baselines-unchanged`) — the operator asked for the new run.

### The eval credential

**ID:** eval-credential-from-zshrc
**Status:** active
**Chosen:** every run command is wrapped in `zsh -ic '…'`, which loads `CLAUDE_CODE_OAUTH_TOKEN` from `~/.zshrc`, as in kan-978; the value never enters a command line or record.
**Considered:** none — the established route.

## Open questions
