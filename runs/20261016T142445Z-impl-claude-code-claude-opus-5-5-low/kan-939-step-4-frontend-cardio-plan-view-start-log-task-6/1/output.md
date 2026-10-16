I've added the log sheet's logic: the per-activity field table, `CardioLogDraft` (parsing, range checks, derived pace and speed, the request) and the 14 tests. All 14 pass and ktlint and detekt are clean, but **the commit didn't happen**: git has no author name or email set in this environment, and I didn't want to set one for you. The changes are staged under the planned commit message.

**Files**
- `CardioActivity.kt`: adds `logFields`, which reads from the table below.
- **`CardioLogField.kt`** — the plan names this file `CardioLogFields.kt`, but ktlint refuses that name because the file holds the single class `CardioLogField`. Correction for the record: `**Files:**` entry `…/cardio/CardioLogFields.kt` → `…/cardio/CardioLogField.kt`. It holds the enum and the field order per activity, as listed in Step 1.
- `CardioLogDraft.kt`: the data class with the API the plan specifies. It also takes `activity`, `units`, `texts` and `left` as constructor parameters, plus `fresh`, `of` and `durationText`.
- `CardioLogChecks.kt` (allowed collateral, split out for detekt): duration and pace parsing, and the range checks with their messages.
- `CardioLogDraftTest.kt`: the 14 named tests.

**Commands and results**
1. `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` before the code existed: RED, failed to compile (`Unresolved reference 'CardioLogDraft'`).
2. Same command after writing the code: GREEN, 14 tests, 0 failures.
3. Checking that the tests catch breakage: I changed the request to send the derived pace and set the rower's cadence minimum to 1. Result: "14 tests completed, 4 failed", `BUILD FAILED`. I restored both files from backups. I didn't record which four tests failed, because the next build overwrote the XML report.
4. `./gradlew :shared:ktlintFormat`: the first run failed and the second was clean. The run after that reported two problems: the file-name rule above and detekt's "destructuring with 4 entries". I fixed both.
5. `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests 'CardioLogDraftTest'`: `BUILD SUCCESSFUL`, 14 tests, 0 failures.
6. `git commit`: failed with `Author identity unknown`.

**Not checked: the backend repo isn't in this environment.** I couldn't compare `logFields` with the backend's `CardioActivity.kt` or the ranges with `CardioLogSpecification.kt`. The field orders and ranges come from the plan's text only.

**Wording I chose that the plan doesn't give:**
- **Distance:** "between 0 and 1000 km" (621 mi; 1000000 m for the rower).
- **Pace:** "between 1:00 and 60:00 /km" (10:00 /500 m for the rower).
- **Duration out of range:** "between 0:01 and 24:00:00 h:mm:ss".
- **Speed:** uses the same 0.1–120 range whether the workout is in km/h or mph.
- **Non-numeric text in a number field:** shows that field's range message.

**Test values I made up, because the frame figures aren't here:**
- **Rower (L4):** 30:00 over 7120 m, which gives "2:06".
- **Saved log:** the values in `aSavedLogFillsTheDraft`.

Please confirm the wording and these figures against the design.

**Your call:** set a git identity (or tell me which to use) and I'll make the commit.