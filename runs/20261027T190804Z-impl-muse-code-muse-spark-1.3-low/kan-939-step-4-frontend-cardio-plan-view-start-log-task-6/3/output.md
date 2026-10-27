Done. Committed as [7363705](https://github.com/gymie/frontend/commit/7363705) — the reminder arrived after the commit had already landed, so there was nothing to retry.

What changed (3 new files, 762 insertions, no existing files touched):
- [CardioLogFields.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLogFields.kt) — the `CardioLogField` enum and `CardioActivity.logFields` in sheet order for all six activities (treadmill/indoor-ride built from the drawn tables via `filterNot`).
- [CardioLogDraft.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLogDraft.kt) — immutable `CardioLogDraft` with `effort`/`note` properties, typed texts, left-set, `errors()`, `shownError()`, `canSubmit`, `derived()` via `CardioFormat`, `toRequest()` (typed fields plus effort; never note, never derived), and `fresh()`/`of()` companions.
- [CardioLogDraftTest.kt](/work/shared/src/commonTest/kotlin/com/gymie/shared/cardio/CardioLogDraftTest.kt) — the fourteen named tests, kept with the project as the durable check.

Proof runs:
- RED: `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest*'` failed to compile against the missing draft (`Unresolved reference 'CardioLogDraft'` / `toRequest` / `canSubmit`).
- GREEN: same selector — `BUILD SUCCESSFUL`, report shows `tests="14" skipped="0" failures="0" errors="0"`.
- Gates: `./gradlew :shared:ktlintFormat`, then `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests 'CardioLogDraftTest'` — `BUILD SUCCESSFUL`.
- Drift check: no other file cites the new API, so nothing else needed updating. The extra refusal paths the diff adds (bad distance shape → `use a number`, effort outside 1–10 → `between 1 and 10`) each have an assertion in `theRequestCarriesTypedFieldsOnly`.

Two judgment calls to record: pace bounds read as `between 1:00 and 60:00 /km` (rower `between 1:00 and 10:00 /500 m`) to fit the `between <min> and <max> <unit>` shape, and effort/distance shape errors use `use a whole number` / `use a number` by analogy with the specified duration message.