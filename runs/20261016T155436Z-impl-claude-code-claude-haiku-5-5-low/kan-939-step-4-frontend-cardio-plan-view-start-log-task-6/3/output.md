The commit failed because no git identity is configured in this environment. I won't set one in the git config, so the changes stay staged for you to commit.

Summary of the state:

**Changed (staged, not committed):** `CardioActivity.kt` gains `logFields`; new `CardioLogField.kt` (enum), `CardioLogChecks.kt` (range table, parsing, formatting), `CardioLogDraft.kt` (draft, derivation, requests); new test `CardioLogDraftTest.kt`.

**Verification:**
- RED: `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` failed to compile (`Unresolved reference 'CardioLogDraft'`).
- GREEN: same command, `tests="14" failures="0"`.
- Guard-bite proof on the field table: broke the treadmill row (`- AVG_CADENCE` removed) → `eachActivityLogsItsOwnFieldsInFrameOrder` failed at `CardioLogDraftTest.kt:60`; restored → 14/14 pass.
- `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests '*CardioLogDraftTest'` → BUILD SUCCESSFUL, 16/16 pass.

**Deviations to review:**
- `CardioLogFields.kt` is named `CardioLogField.kt`; ktlint requires a file to be named after its single class.
- Added two tests beyond the fourteen (`paceRefusesOtherShapes`, `toRequestRefusesWithoutADuration`) to cover refusal paths the drift check requires.
- `anEmptyDurationHoldsThePrimary` doesn't assert the `use h:mm:ss` text; `durationRefusesOtherShapes` does.
- Rower figures for `theRowerDerivesPerFiveHundredMetres` are my own (25:00 over 5950 m → 2:06), since L4's frame figures aren't in this repo.
- The backend `CardioActivity.logFields` (`BC/...`) isn't available here, so I could not cross-check the field table against it. The table follows the `log-fields-table` decision.
- Open spec points, implemented as written and flagged for you: the distance check is exclusive at 0 and at the max, but its message reads "between 0 and 1000 km", which is slightly misleading; "use m:ss" for an unparseable pace is my wording, since the spec names only the duration message.

**Needs you:** git has no user identity configured here. Run `git commit` with the message in the task (`feat(cardio): the log sheet's fields, draft, checks and derived pace and speed`) plus the Co-Authored-By line, or tell me a identity to use.