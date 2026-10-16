Task 6 is implemented and verified, but not committed: this environment has no git identity configured, so the four files are staged and the commit is left for you.

**Changed**
- `CardioLogField.kt` (new; renamed from `CardioLogFields.kt` because ktlint requires a file holding a single class to take its name). Holds the `CardioLogField` enum and `logFieldsOf(activity)`, the per-activity field table in sheet order.
- `CardioActivity.kt`: adds `val logFields` delegating to that table.
- `CardioLogDraft.kt` (new): the immutable draft data class with `text`, `with`, `left`, `shownError`, `errors`, `canSubmit`, `derived`, `toRequest`, and the companion `fresh` and `of`. Derived pace and speed come from `CardioFormat.derivedPace` and `derivedSpeed`.
- `CardioLogDraftTest.kt` (new): the fourteen named tests.

**Runs**
- RED: `./gradlew --no-configuration-cache :shared:desktopTest --tests '*CardioLogDraftTest'` failed to compile (unresolved `CardioLogDraft`, `CardioLogField`, `logFields`).
- GREEN: same command after the implementation; XML shows `tests="14" failures="0" errors="0"`.
- `:shared:ktlintFormat` ran clean after the file rename. `:shared:ktlintCheck :shared:detekt :shared:desktopTest --rerun --tests '*CardioLogDraftTest'` exited 0. detekt's ReturnCount on `parseClock` was fixed and rerun.
- I did not run the module suite or an old-vs-new comparison, since this task refactors nothing.

**Decisions and gaps to check**
- The backend's `CardioActivity.logFields` and `CardioLogSpecification` are not in this checkout, so the table and ranges are from the task text, not checked against the backend.
- L4's frame figures are not in the repo. The rower test uses 8:24 over 2000 m (derives `2:06`).
- Choices the task did not specify:
  - Duration out of range shows `between 0:01 and 24:00:00`.
  - An unparseable pace shows the pace range message, not a new error text.
  - Unparseable numbers show the range message.
  - Distance bounds are 0.01 to 1000 km (1,000,000 m for a rower), inclusive.
  - Calories use `kcal`.
  - `of` derives the saved distance into the workout's unit (rower stays in metres).
- `toRequest()` uses `checkNotNull` for duration, relying on the caller gating on `canSubmit`.

**Needs you**
- Set a git identity, then commit the staged files with the message from the plan and the `Co-Authored-By: Claude Code <noreply@anthropic.com>` trailer. I did not change git config or invent an author.