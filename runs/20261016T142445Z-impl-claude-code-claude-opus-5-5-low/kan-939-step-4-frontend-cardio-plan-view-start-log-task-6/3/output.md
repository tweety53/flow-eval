Task 6 is implemented and passes all checks, but **it isn't committed**: git has no author identity set up here, so `git commit` failed. The changes are staged with the planned commit message ready. Tell me which name and email to use (or set `user.name`/`user.email`) and I'll commit.

**What changed** (in `shared/.../cardio/`)
- **`CardioActivity.kt`**: added `logFields`, which lists each activity's log fields in the order the plan gives.
- **`CardioLogField.kt`**: the `CardioLogField` enum and the field table for each activity.
- **`CardioLogChecks.kt`** (extra file, allowed for size): parses durations (`h:mm:ss` or `m:ss`) and paces (`m:ss`), formats them, and checks each field's range. Errors read *use h:mm:ss* or *between X and Y unit*, with rower cadence limited to 10–60.
- **`CardioLogDraft.kt`**: an immutable data class with the API the tests use. A blank pace or speed is calculated from duration and distance and never sent. A rower's distance is always in metres. `fresh` fills Duration from `startedAt` (whole seconds, rounded down), and `of` rebuilds the draft from a saved log.
- **Tests:** the fourteen named tests in `CardioLogDraftTest.kt` (before 0, after 14).

**Commands run**
- RED: `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` failed to compile (`Unresolved reference 'CardioLogField'`).
- GREEN: same command, 14 tests, 0 failures.
- `ktlintFormat`, then `ktlintCheck :shared:detekt :shared:desktopTest --tests 'CardioLogDraftTest'`. The first runs failed on two detekt limits in `fieldError` (fixed by splitting it into helpers) and on ktlint's file-name rule (fixed by the rename below). The final run passed: exit 0, 14/14.
- **Proof the tests catch breakage:** I gave indoor ride an elevation field and widened rower cadence to 1–250, and 2 of 14 tests failed. After restoring both, 14/14 passed.
- The Gradle daemon kept crashing (it lost its connection, probably memory). The runs that counted used `--no-daemon --max-workers=2 -Pkotlin.compiler.execution.strategy=in-process`.

**Corrections to the plan**
- **`**Files:**` entry:** `CardioLogFields.kt` → `CardioLogField.kt`. ktlint requires a file holding one class to be named after it. Also add `CardioLogChecks.kt`; both are covered by the allowed `CardioLog*.kt` files.
- **Backend not checked:** the backend repo isn't here, so I couldn't compare the field table and ranges with `CardioActivity.logFields` / `CardioLogSpecification`. I transcribed them from the plan.

**Questions for you** (I implemented these but didn't treat them as settled)
1. **Messages the plan doesn't give:** I wrote these myself:
   - *use m:ss* for a pace of the wrong shape.
   - *use a number* or *use a whole number* for text that isn't a number.
   - *between 1 and 86400 s* for duration.
   - *between 1:00 and 60:00 /km* for pace.
   - *between 0 and 621.3 mi* for a distance limit converted to miles.
2. **Speed and elevation limits in miles:** I applied the same numbers (0.1–120, 0–20000) to mph and feet, assuming the backend checks the value as typed.
3. **Effort:** an effort typed through `with(EFFORT, …)` isn't range-checked; it's simply ignored if it isn't a number.