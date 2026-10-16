I added the log field table and `CardioLogDraft`, and the checks pass. I couldn't compare the field table or the ranges with the backend. `CardioActivity.kt` and `CardioLogSpecification.kt` aren't in this checkout. The table follows your task text, and the ranges follow your task text and the `client-ranges` decision.

**What changed** (all under `shared/src/commonMain/kotlin/com/gymie/shared/cardio/`):
- **`CardioLogFields.kt`:** the `CardioLogField` enum, plus `logFields` in sheet order for all six activities. `logFields` is an extension property on `CardioActivity` rather than a member of the enum. Callers write `activity.logFields` either way.
- **`CardioLogDraft.kt`:** the immutable draft with the API from your spec. A few behaviours you didn't specify:
  - `of` and `fresh` fill in the workout's activity and distance unit, so those are extra constructor parameters.
  - Error messages for ranges your spec didn't spell out:
    - duration out of range reads `between 0:01 and 24:00:00 h:mm:ss`
    - a pace of the wrong shape reads `use m:ss`
    - a non-numeric value in a numeric field gets that field's range message
    - a rower's distance range is `between 0 and 1000000 m`
  - A saved distance is converted into the workout's unit, or metres for the rower, before it becomes text.

**Tests:** `commonTest/.../cardio/CardioLogDraftTest.kt` has 14 tests. Your spec listed 11 names, so I added `anEmptyFieldIsNeverInErrorAndDurationGatesSubmit`, `logFieldsFollowTheSheetOrder` and `durationPrefillsFromTheClockRoundedDown` (an in-progress workout and a planned one).
- The rower derivation case in the spec ("L4's figures") wasn't given, so I used 25:12 over 6000 m, which gives `2:06`.
- I didn't write a test for the duration range message.

**Commands:**
- `./gradlew :shared:ktlintFormat` ran clean.
- `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests '*CardioLogDraftTest'` first failed on a recursive-type compile error, then on two test failures: a null-pointer crash for fields with no range (effort, note) and my own wrong rower arithmetic. After fixing those it succeeds.
- I did not run the RED step on its own before writing the draft, so there is no failing-run record.
- I did not run the full suite.
- The commit needed an author identity, so I passed one inline with `git -c`. Git config is unchanged. The commit is `bff3d5a`.