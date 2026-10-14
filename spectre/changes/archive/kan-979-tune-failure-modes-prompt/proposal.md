# kan-979-tune-failure-modes-prompt

KAN-979.

## Why

- flow-eval's failure-modes slot misses F9 on `kan-867-qm-2-step-1-frontend-settings-workout-mode-focus` on every model at low effort (agents `2e053aaa`, k=3, re-graded): haiku-low 0/3, sonnet-low 0/3, opus-low 1/3. F9: after a failed load, a successful `chooseUnit` clears `loadFailed` though the workout mode was never confirmed, and it discards the unit PUT's answer that carried the server's mode.
- The haiku trials walk the four lenses call by call and report plausible extras (a stale-unit lost update, the mutex held across the PUT), but none asks what a later success does to the state a failure left behind.
- The failure-modes prompt is unchanged from `2e053aaa` to the current pin `a061813d`. Primary and principles were each lifted by one generic required step plus an evidence output section (`docs/prompt-tuning.md`).

## What changes

- agents `skills/flow/failure-modes-reviewer-prompt.md` gains a required failure-recovery step and a `### Failure-recovery check` output section — the best of at most three variants measured on haiku-low at k=6, per `docs/prompt-tuning.md`.
- flow-eval vendors that prompt (`prompts/PIN`, `prompts/review-failure-modes.md`) and commits the before run and each variant's run.
- Once the operator has read the haiku results and says go, the winner is also run on sonnet-low and opus-low at k=6.
- `docs/prompt-tuning.md`'s worked examples gain a failure-modes row.
- The variant-2 opus-low failure-modes run becomes the slot's baseline, and every k=6 run and every run in which no trial passed is deleted from `runs/`.
