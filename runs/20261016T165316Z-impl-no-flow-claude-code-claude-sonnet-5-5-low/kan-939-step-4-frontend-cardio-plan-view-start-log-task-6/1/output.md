The cardio log logic is in and committed (`e8b0ff5`). The new tests pass and ktlint and detekt are clean.

**What I changed** (all under `shared/src`):
- `CardioLogFields.kt` is new. It holds the `CardioLogField` enum and the per-activity field table in the sheet order you gave. It carries a `@file:Suppress("ktlint:standard:filename")`, because the file name you specified doesn't match ktlint's single-class naming rule.
- `CardioActivity.kt` gains `val logFields: List<CardioLogField>`.
- `CardioLogDraft.kt` is new. It is an immutable data class with the API you specified, plus `activity` and `units` constructor parameters.
  - **Errors:** `errors()` skips empty fields and uses the ranges and message texts from the task.
  - **Derived values:** `derived()` returns a value only while the field is blank and its inputs are present.
  - **Request:** `toRequest()` sends typed fields and effort only. A rower's distance goes out in metres.
  - **Entry points:** `fresh()` prefills Duration from the clock when the workout is in progress. `of()` fills the draft from the saved log and `workout.notes`.
- `CardioLogDraftTest.kt` (in `commonTest`) holds the eleven tests you named, plus `fieldTableFollowsTheSheets`, which spot-checks the field table. That makes 12 tests.

**Commands run:**
- `./gradlew :shared:ktlintFormat` initially reported violations, which I fixed by hand. Later runs were clean.
- `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests 'CardioLogDraftTest'` first failed on detekt complexity and function-count limits, and later on a compile error from my refactor. After I split the helpers out, it passed with 12 tests, 0 failures.

**Things to know:**
- **Backend ranges unchecked:** the backend sources (`BC/...`) aren't in this repo, so I couldn't check the field table or ranges against them. They come from your task text only.
- **Copy I chose:** the task didn't give the range messages for duration, distance, pace or speed. I wrote them as `between 0:01 and 24:00:00`, `between 0 and 1000 km` (`1000000 m` for the rower), `between 1:00 and 60:00 /km` (`/500 m` for the rower, 10:00 max) and `between 0.1 and 120.0 km/h`.
- **Rower derived-pace test:** your task gave no figures for it. My test uses 30:00 over 7140 m, which gives `2:06`.
- **Commit identity:** I committed with `-c user.name=flow-eval -c user.email=flow-eval@localhost` because the repo had no git identity configured.