# kan-978-tune-principles-prompt

KAN-978.

## Why

- flow-eval's principles slot scores 0 on every model (haiku-low, sonnet-low, opus-low, k=3 each, agents `2e053aaa`): no trial finds F6 on `kan-867-qm-2-step-1-frontend-settings-workout-mode-focus` — `loadFailed` is a stored copy of "some field is unconfirmed", widened to two fields while each field's write path still clears it alone (Single Source of Truth).
- The primary slot was lifted from 0/33 to 14/33 by one generic required step plus an evidence output section (agents `2e053aaa`); the principles prompt has no such step.
- That tuning loop lives only in a commit message; nothing in flow-eval tells the next tuner how to do it.

## What changes

- agents `skills/flow/principles-reviewer-prompt.md` gains a required derived-state step and a `### Derived-state check` output section — the best of at most three variants measured on haiku-low at k=6.
- flow-eval vendors that prompt (`prompts/PIN`, `prompts/review-principles.md`) and commits the before run and each variant's run.
- flow-eval gains `docs/prompt-tuning.md`, the versioned prompt-tuning loop with this change as its worked example, linked from the README.
- Fix 1 (2026-10-13): a review label may list further lines of its file (`also`), so a multi-writer defect is matched at any of its writers; F6 gains `:295`. A new `flow-eval regrade <run-id>…` re-grades a stored review run's trials against the current labels and judge; every kan-867 review run is re-scored with it. Why: on sonnet-low and opus-low k=6 the tuned prompt scored 1/6 and 0/6, yet opus reported F6 in 6/6 trials — anchored at `:295`, 44 lines from the label's only line, so the ±15-line window never put it to the judge.
- Fix 2 (2026-10-13): a review run's id names its slot, `<started>-review-<slot>-<harness>-<model>-<effort>`. Why: two review runs of one harness, model and effort started in the same second — one per slot, as an eval of every slot is launched — got the same id, and the second was refused (`file exists`) before any trial.
- Fix 3 (2026-10-13): the review report parser keeps a bold-led finding's bullets inside it, and a review trial's score rewards real extras — unlabelled findings the judge rates a plausible defect — with 5 points each, at most 15. Every stored review run is re-graded. Why: opus-low wrote one Important finding as a bold line with `- **Principle:**`, `- **Fix:**`, `- **Reproducer:**` bullets; each bullet was parsed as its own finding, two were rated implausible, and a correct trial scored 50. Real findings beyond the labels counted for nothing, so a higher score meant matching the labels, not finding more real defects.
