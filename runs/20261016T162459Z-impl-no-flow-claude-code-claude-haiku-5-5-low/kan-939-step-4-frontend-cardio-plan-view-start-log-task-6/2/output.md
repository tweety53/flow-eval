The log draft logic is in place and the target tests pass, but I did not commit. Git has no author identity configured here, and I didn't set one. The four files are staged; commit them once an identity is set.

**Changed**
- `shared/.../cardio/CardioLogField.kt` (new): the `CardioLogField` enum. The spec named it `CardioLogFields.kt`, but ktlint requires the file to match the class name.
- `shared/.../cardio/CardioActivity.kt`: `logFields` per activity, in sheet order, per the spec's table.
- `shared/.../cardio/CardioLogDraft.kt` (new): immutable data class with `text`, `with`, `left`, `shownError`, `errors`, `canSubmit`, `derived`, `toRequest`, and the `fresh` / `of` companions. Duration parsing, the backend range messages, and derived pace/speed are all here.
- `shared/src/commonTest/.../cardio/CardioLogDraftTest.kt` (new): 11 tests.

**Commands run**
- `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` before implementation: failed to compile (`CardioLogDraft` unresolved). This is the expected red.
- Same command after implementation: 11 tests, 0 failures.
- `./gradlew :shared:ktlintFormat`: passed after fixes (wildcard and import-ordering violations, file rename).
- `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests '*CardioLogDraftTest'`: exit 0.
- `./gradlew :shared:desktopTest --tests 'com.gymie.shared.cardio.*'`: 78 tests, 0 failures.
- `git commit`: failed with "Author identity unknown".

**Things to check**
- The backend `BC/core/domain/cardio/CardioActivity.kt` and `CardioLogSpecification.kt` are not in this environment. I could not verify the field table or ranges against them; I used the spec's tables.
- The spec says "fourteen names" but lists eleven. I wrote the eleven listed.
- The spec's "148 / 139" for max-below-average: I read it as avg 148, max 139.
- Choices the spec didn't cover: "use a number" for non-numeric numeric input, "use m:ss" for a bad pace, and range messages for duration (`between 1 and 86400 s`) and distance (`between 0 and 1000 km`, where the miles case is shown as `621.37 mi`). Effort has no range check.
- Distance is stored as typed in the workout's unit; the rower's is in metres.

To finish, set an identity and run the commit, or tell me which author to use and I'll commit with it.