The commit didn't go through because git has no author identity configured here. I won't set one in git config on your behalf, so the changes stay staged for you to commit.

## Report: Task 6 (log logic)

**Status:** code, tests and checks are done; the commit is blocked on git identity.

**Changed (staged, not committed):**
- `shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLogFields.kt`: the `CardioLogField` enum and `CardioActivity.logFields`. This is an extension property rather than a member of the enum, so it sits in the `CardioLog*.kt` collateral file. It is `internal`.
- `shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLogDraft.kt`: the draft data class with `text`, `with`, `left`, `shownError`, `errors`, `canSubmit`, `derived`, `toRequest`, `fresh` and `of`.
- `shared/src/commonTest/kotlin/com/gymie/shared/cardio/CardioLogDraftTest.kt`: the fourteen tests. Its content changed on disk while I worked; I kept the on-disk version. It uses average 139 and max 138 for the max-below-average case, where the plan's example is 148/139. Both still trigger the error.

**Evidence:**
- RED: `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` failed to compile with unresolved `CardioLogDraft`, which matches the plan's "fails to compile" baseline.
- GREEN: `CardioLogDraftTest.xml` reports `tests=14 skipped=0 failures=0 errors=0`, both before and after the detekt cleanup.
- `ktlintFormat`, `ktlintCheck` and `detekt` are clean after I removed an unused constant. ktlintFormat also reflowed `CardioLogFields.kt`.

**Not verified:**
- The backend's `CardioLogSpecification` and `CardioActivity.kt` are not on this machine. I built the field table and ranges from the plan's lists, so the table is unchecked against the backend.
- The rule that tests exercising the real boundary should not use hand-built values does not apply cleanly here. The saved-log test builds its `CardioLog` by hand.

**Blocked:** `git commit` failed with "Author identity unknown". Set `user.name` and `user.email`, then commit the three staged files with the message from the plan and the Co-Authored-By trailer.