Task 6 is done and committed as `6438306`. All 14 `CardioLogDraftTest` tests pass, and ktlint and detekt are clean. I couldn't check the field table and ranges against the backend, because that repo isn't in this environment.

**What changed**
- `CardioActivity.kt`: added `logFields`, which returns each activity's field list.
- `CardioLogField.kt`: the field enum and the per-activity field table, in the order the plan gives.
- `CardioLogChecks.kt`: reading duration and pace, the range checks with their error messages, and the rule that a rower's distance is in metres.
- `CardioLogDraft.kt`: the draft, built to the plan's API.

**Tests added:** `shared/src/commonTest/kotlin/com/gymie/shared/cardio/CardioLogDraftTest.kt`, with the 14 named tests.

**Commands run**
- **RED:** with my source changes stashed, `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` failed to compile (`Unresolved reference 'logFields'`). The stash left the new untracked files in place, so this is a compile failure, not a run with no draft at all.
- **Verify:** `./gradlew :shared:ktlintFormat`, then `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests '*CardioLogDraftTest'`.
  - First run: the Kotlin daemon crashed, and ktlint rejected the file name (see below).
  - After the rename: build successful, 14 tests, 0 failures.
- **Proof the tests catch a break:** I changed the rower cadence minimum from 10 to 1. Two tests failed (`rowerCadenceOutOfRangeNamesTheRange` and `anErrorShowsOnlyOnceTheFieldWasLeft`). After restoring it, all 14 passed.
  - During this step `git checkout -- .` also threw away my `CardioActivity.kt` edit, and the next run failed to compile. I re-applied the edit, and the final checks above ran after that.
- **Commit:** git has no identity set up here, so I committed with `-c user.name=Claude -c user.email=noreply@anthropic.com`.

**Corrections to the plan's record**
- **`**Files:**`:** replace `CardioLogFields.kt` with `shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLogField.kt`. ktlint requires a file holding one class to be named after it. Also add `shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLogChecks.kt`, the split the allowed-collateral rule permits.
- **Not checked against the backend:** the field table and ranges come from the plan and design text, not from `CardioActivity.kt` or `CardioLogSpecification.kt` in the backend.

**Choices I made where the plan says nothing — please confirm**
- **Text that isn't a number** gives the error `enter a number`.
- **A pace of the wrong shape** gives `use m:ss`.
- **Range message units:**

  | Field | Message |
  |---|---|
  | Duration | `between 0:01 and 24:00:00` |
  | Pace | `between 1:00 and 60:00 /km` (rower: `10:00 /500 m`) |
  | Distance | `between 0 and 1000 km` (or `621 mi`, or `1000000 m` for the rower) |
  | Heart rate | `bpm` |
  | Calories | `kcal` |
  | Elevation | `m` or `ft` by the workout's unit; the 0–20000 range is applied in that unit |
  | Power | `W` |
  | Cadence | `rpm` on rides, `spm` otherwise |
- **The rower test figures** (30:00 over 7120 m → 2:06) are mine, since I didn't have the L4 frame's numbers.