The log field table, draft and tests are in place and green, but I did not commit. Git has no author identity configured here, and I didn't set one. The five files are staged on `master`; run `git commit` once `user.name` and `user.email` are set, using the message in the task's commit step plus the `Co-Authored-By` line.

**What changed**
- `CardioLogField.kt`: the `CardioLogField` enum. It lives in its own file because ktlint requires a single-class file to be named after the class.
- `CardioLogFields.kt`: `cardioLogFields(activity)`, the per-activity field table in sheet order.
- `CardioActivity.kt`: a `logFields` member that delegates to that table.
- `CardioLogDraft.kt`: the immutable draft. It covers text per field, `effort`, `note`, `left`, `errors()` against the backend ranges, `derived()` via `CardioFormat.derivedPace`/`derivedSpeed`, `canSubmit`, `toRequest()`, and the `fresh`/`of` factories.
- `CardioLogDraftTest.kt`: 12 tests.

**Tests**
- 11 of the spec's named tests, plus `theRowerLogsDistanceInMetres`. The spec said "fourteen names" but listed eleven, so I added the one it requires separately.
- RED: `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` failed to compile (`Unresolved reference 'CardioLogDraft'`).
- GREEN: the 12 tests pass.
- Ran `:shared:ktlintFormat`, then `:shared:ktlintCheck :shared:detekt :shared:desktopTest --tests 'CardioLogDraftTest'`. All pass.
- The whole `com.gymie.shared.cardio.*` package also passes: 80 tests across 9 suites.
- `--offline` fails on missing cached dependencies, so I ran the Gradle commands online.

**Gaps and judgement calls**
- The backend `CardioActivity.kt` is not on this machine, so I could not check the field table against it. The table follows the spec and the design decision.
- The rower example uses 29:54 over 7120 m. I picked it because it yields the stated 2:06; the frame's actual figures were not available.
- The spec gives no exact message text for pace, speed and distance ranges. I formatted them as `between 1:00 and 60:00 /km`, `between 0.1 and 120.0 km/h` and `between 0.01 and 1000 km`.
- Work is on `master`, not the `spectre/<name>` branch the design doc asks for, because the task didn't say to branch.