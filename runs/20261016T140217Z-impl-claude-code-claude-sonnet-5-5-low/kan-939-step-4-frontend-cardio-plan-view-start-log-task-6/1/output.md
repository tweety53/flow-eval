Task 6 is done and committed as 5517028. All 14 `CardioLogDraftTest` tests pass, and ktlint and detekt are clean.

**What changed**
- `CardioLogFields.kt` has the `CardioLogField` enum and the per-activity field table, in the frames' order. `CardioActivity.logFields` in `CardioActivity.kt` reads from it. I did not check the table against the backend's `CardioActivity.kt`, because that repo isn't in this checkout. It follows the field lists in the task.
- `CardioLogDraft.kt` has the draft and its `fresh` and `of` constructors.
- `CardioLogChecks.kt` is a new file under the allowed `CardioLog*.kt` collateral. It holds the duration and pace parsing and the range checks. The draft's size would otherwise trip detekt.
- `CardioLogDraftTest.kt` has the 14 named tests.

**Decisions to confirm**
- **Filename:** ktlint's filename rule rejected `CardioLogFields.kt`, since the file holds a single class named `CardioLogField`. The path is fixed by the task's `**Files:**` field, so I added `@file:Suppress("ktlint:standard:filename")` rather than rename it.
- **`toRequest()` on an unparseable duration:** it throws instead of sending 0. `canSubmit` is meant to guard against that.
- **Range and parse messages you didn't specify:** a pace of the wrong shape gives `use m:ss`. An out-of-range duration gives `between 0:01 and 24:00:00 h`. The other ranges follow your *between X and Y <unit>* pattern. Rides show cadence in `rpm`, and elevation shows `ft` on a miles workout.

**Commands run**
- RED: `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` failed to compile with `Unresolved reference 'CardioLogDraft'`, as expected.
- `./gradlew :shared:ktlintFormat` ran and finished.
- GREEN: the same test command passed once the draft existed.
- Guard proof, broken state: I changed the rower cadence range from 10..60 to 1..60 and re-ran the tests. `rowerCadenceOutOfRangeNamesTheRange` and `anErrorShowsOnlyOnceTheFieldWasLeft` failed, and the build failed.
- Guard proof, restored: I put the range back to 10..60.
- First `ktlintCheck` run: failed on the filename rule above, then passed after the suppression.
- Final run: `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests '*CardioLogDraftTest'` gave BUILD SUCCESSFUL.

**Pin-before-refactor and old-vs-new diff:** these don't apply, since the task adds new code and refactors nothing.

**Commit:** the repo had no git identity, so I committed with a throwaway author, `claude <a@b.c>`. You may want to amend it.