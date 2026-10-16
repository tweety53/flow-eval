The log field table, `CardioLogDraft` and the 14 tests are in and committed as `a225d6a`. The 14 tests, ktlint and detekt are green.

**What changed**
- `CardioLogFields.kt` is new: the `CardioLogField` enum and the per-activity field lists in sheet order.
- `CardioActivity.logFields` now delegates to those lists.
- `CardioLogDraft.kt` is new: an immutable data class with the API from the task. Checks use the backend ranges, with rower cadence 10–60.
- Derived pace and speed come from `CardioFormat.derivedPace` and `derivedSpeed`, and `toRequest()` never sends them or the note.
- `CardioLogDraftTest.kt` is new, with the 14 named tests.

**Not checked**
- I did not diff the field lists against the backend's `CardioActivity.logFields` or `CardioLogSpecification`. They come from the task text only.
- The task's "L4's figures" for the rower test were not available. I used 7120 m in 29:54, which gives 2:06 per 500 m.

**Choices the task didn't pin down**
- The range messages for duration, distance, pace and speed are my own wording; only the cadence, max-heart-rate and duration-shape texts were specified:
  - Duration out of range: `between 0:01 and 24:00:00`.
  - Distance: `between 0 and <max> <unit>`.
  - Pace: `between 1:00 and <max> <unit>`.
  - Speed: `between 0.1 and 120 <unit>`.
- Pace is parsed as `m:ss` only.

**Commands**
- `./gradlew :shared:ktlintFormat` failed on the first run with style violations. I fixed them by hand and ran `ktlintFormat` and `ktlintCheck` again, which passed.
- `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` first failed 1 of 14: the rower test, whose fixture I had guessed wrong. After fixing the fixture it passed.
- `./gradlew :shared:detekt` passed.
- I did not run the RED step (tests before the code), so there is no recorded compile failure for the revert case.
- I added no pinning tests or old-vs-new diff because the task refactors nothing.

**Guard check**
- I changed the rower cadence range from `10 to 60` to `1 to 250`.
- Result: `rowerCadenceOutOfRangeNamesTheRange` failed and the other 13 passed (`14 tests completed, 1 failed`).
- I restored the file and the run was `BUILD SUCCESSFUL`.

The commit needed an inline git identity (`-c user.name/user.email`) because the repo has none configured.