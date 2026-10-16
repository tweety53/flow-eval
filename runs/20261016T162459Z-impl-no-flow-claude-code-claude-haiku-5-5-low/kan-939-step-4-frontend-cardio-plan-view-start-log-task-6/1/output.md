The log field table, draft, parsing, checks and derivation are implemented and the 11 tests pass, but the commit is not made. The checkout has no git identity configured, and I didn't set one. The five files are staged; run `git commit` with your identity to finish.

**Changed**
- `CardioLogField.kt` (new): the 12-value `CardioLogField` enum.
- `CardioActivity.kt`: `logFields` in sheet order for all six activities.
- `CardioLogDraft.kt` (new): the immutable draft with `text`, `with`, `left`, `shownError`, `errors`, `canSubmit`, `derived`, `toRequest`, `fresh` and `of`. Range checks use the backend's ranges and the `between X and Y unit` messages. The rower's distance is sent in metres.
- `CardioFormat.kt`: `fixed` changed from private to internal so the draft can reuse it.

**Tests**
- `CardioLogDraftTest.kt` (new, in `commonTest`): 11 tests, one per named family.
- RED: the run failed to compile against the missing API.
- `./gradlew :shared:ktlintFormat`: ran; changed only my files.
- `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests 'CardioLogDraftTest'`: BUILD SUCCESSFUL, 11/11 passed.

**Deviations from the spec**
- The spec says the header holds the fourteen names, but it lists 11. I wrote the 11.
- The spec's `CT/` and `BC/` paths don't exist in this checkout. I put the test in `commonTest`, and I could not check `logFields` against the backend's `CardioActivity.kt`. I used the spec's table, which matches the design notes.
- ktlint requires a file named after its class, so the enum is in `CardioLogField.kt`, not `CardioLogFields.kt`.
- Choices the spec left open: pace and speed derivation as specified; elevation range is 0–65617 ft for mile workouts (0–20000 m otherwise); a typed non-number in Effort clears the field; non-numeric entries show "use a number" or "use a whole number"; the L4 rower figures were not available, so the test uses 2000 m in 8:24, which gives 2:06.