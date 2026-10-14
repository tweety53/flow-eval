I reviewed the diff against the code in the working tree (HEAD `8ceb8ae` already contains it). The repo has no KAN-865/KAN-867 change artifacts under `openspec/` or `docs/`, so I could only check the diff against its own KDoc and test claims, not against a plan. I did not run Gradle; each reproducer below is a test to add plus the command to run it.

### Shared-state check

**`SettingsState.loadFailed`** — this was already a field, and the diff now uses it for the workout mode too.
- `SettingsState.kt:270` (load success clears it only if no unit or mode write succeeded during the load): still correct.
- `SettingsState.kt:275` (load failure sets it under the same condition): still correct.
- `SettingsState.kt:295` (**not changed by the diff**; a successful unit write clears it): **now wrong.** It clears the flag for the unit alone, but the flag now also means "the workout mode was never confirmed." See Important #1.
- `SettingsState.kt:310` (unit write fails with nothing confirmed, sets it): still correct.
- `SettingsState.kt:339` (new; a successful mode write clears it): correct. A mode write can only be sent when a unit is already confirmed, so after it succeeds both fields are confirmed.
- `SettingsState.kt:351` (new; mode write fails with nothing confirmed, sets it): correct.

**`unitWrite.confirmed`** — the new mode write reads it at `SettingsState.kt:333`.
- `:103` (`onSuccess`): correct.
- `:135` (`adoptLoad`): correct in normal use. The exception is a load that overlaps a mode write: the mode write re-sends a unit, but that send never advances `unitWrite.succeededWriteCount`, so the overlapping load can still adopt an older unit. See Minor #2.

**`unitWrite.succeededWriteCount` / `viewWrite.succeededWriteCount`**
- The only writer is `:102`. Correct, apart from the overlapping-load case above.

**`value`** (`:80`, `:105`, `:119`, `:136`): this is per-field, so it is still correct.

**`sendMutex`**: it only takes locks and writes no state. Both kinds of write now go through it, which keeps sends in tap order. Correct.

**`hasLoaded` / `loadingCalls`** (`:256`, `:280`, `:281`): unchanged and still correct.

**`FakeWorkoutApi.preferencesResult`**
- `FakeWorkoutApi.kt:39` (default): still correct.
- `FakeWorkoutApi.kt:200` (stores what a write answered): correct. The new default answer keeps the stored mode, which matches how the server treats a write that leaves the mode out.

### Issues

#### Critical (Must Fix)

**1. The preferences response no longer decodes when the server holds the old `LIST` value.**
- **Where:** `shared/src/commonMain/kotlin/com/gymie/shared/domain/WorkoutModels.kt:691` and `:697`.
- **What's wrong:** `WorkoutPreferences` now declares `sessionView: SessionView`, and `SessionView` only has `FOCUS` and `QUICK`. Before this diff the key wasn't declared, so `ignoreUnknownKeys` just skipped it.
- **Evidence that `LIST` is a real value:**
  - The archived KAN-71 plan created `session_view as enum ('FOCUS', 'LIST')`: `openspec/changes/archive/2026-08-10-kan-71-design-review-updates-part-2/tasks.md:217`.
  - This repo's own fixtures still have the server answer `LIST`: `GroupSessionLandingUiTest.kt:114` and `GroupSessionFloorUiTest.kt:262/326`.
- **Why it matters:** an unknown enum value throws a `SerializationException`, and `ignoreUnknownKeys` doesn't prevent that (`WorkoutModels.kt:624` explains this). For any account still storing `LIST`:
  - Settings `load()` fails every time, so the retry loops forever.
  - `WorkoutEditorScreen.kt:386` (`api.getPreferences().displayUnit`) fails too, so new workouts lose the user's unit.
- **Missing compatibility story:** nothing here shows the backend migrated `LIST` to something else.
- **Fix:** add a tolerant serializer, as `FinishIssueFieldSerializer` does for `FinishIssueField`. It should map an unknown value (or `LIST` specifically) to the decision KAN-865 made. Also add a decode test for `{"displayUnit":"KG","sessionView":"LIST"}`.
- **Reproducer:** add this to `WorkoutModelsSerializationTest`:
  ```
  @Test fun legacyList() { Json { ignoreUnknownKeys = true }.decodeFromString<WorkoutPreferences>("""{"displayUnit":"KG","sessionView":"LIST"}""") }
  ```
  Then run `./gradlew :shared:desktopTest --tests '*WorkoutModelsSerializationTest*'`. It throws `SerializationException`.

#### Important (Should Fix)

**1. A successful unit write clears the retry while the workout mode is still unconfirmed and possibly wrong on screen.**
- **Where:** `SettingsState.kt:295` (this line is outside the diff; the diff created the problem by widening what `loadFailed` means).
- **Scenario:**
  1. The server holds `{LBS, QUICK}`.
  2. `load()` fails, so `loadFailed = true`, and both `confirmed` values are null.
  3. The user taps KG. The PUT succeeds and its response carries `sessionView = QUICK`.
  4. `:295` sets `loadFailed = false`. The retry link (`AppScreens.kt:326`) disappears.
  5. `chooseUnit` never reads `it.sessionView`, so the Workout mode row keeps showing a default **Focus** that the server never confirmed.
- **Why it matters:** the user is shown the wrong mode, and the screen offers no way to re-sync. This also contradicts the `WorkoutModeSetting` KDoc, which promises the row never shows a default Focus.
- **Fix:** in `chooseUnit`'s `onSuccess`, adopt `it.sessionView` into `viewWrite` when no mode write has succeeded (an `adoptLoad`-style call). Alternatively, clear `loadFailed` only when `viewWrite.confirmed != null`. Add a test for this sequence.
- **Reproducer:**
  ```
  api.preferencesResult = Result.failure(RuntimeException()); state.load()
  api.preferencesResult = Result.success(WorkoutPreferences(WeightUnit.LBS, SessionView.QUICK))
  state.chooseUnit(WeightUnit.KG)
  assertTrue(state.loadFailed || state.sessionView == SessionView.QUICK)
  ```
  Run it with `./gradlew :shared:desktopTest --tests '*SettingsStateTest*'`. It fails: `loadFailed` is false and `sessionView` is `FOCUS`.

**2. The retry link only appears inside the Weight unit row.**
- **Where:** `AppScreens.kt:326`.
- **What's wrong:** when a mode write fails with nothing confirmed (`SettingsState.kt:351`), `loadFailed` is set, but the retry shows up under the Weight unit control rather than next to the row the user just tapped.
- **What's missing:** the new tests don't cover where this retry is drawn.
- **Reproducer:** none — this is a placement/UX gap, and nothing fails.

#### Minor (Nice to Have)

**1. The failure message is hard-coded.**
- **Where:** `SettingsState.kt:276`.
- **What's wrong:** `"Could not load your settings."` replaces `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` as a literal. The repo keeps these strings in `FailureMessages` (`components/FailureHandling.kt`).
- **Fix:** add a `COULD_NOT_LOAD_SETTINGS` constant there. The `"Could not save your workout mode."` literal is repeated four times and should become a constant too.
- **Reproducer:** none — style only.

**2. A mode write re-sends the unit but isn't counted as a unit write.**
- **Where:** `SettingsState.kt:333–339`.
- **What's wrong:** the mode write sends `displayUnit = unitWrite.confirmed`, then ignores the `displayUnit` in the response and never advances `unitWrite.succeededWriteCount`.
- **How it goes wrong:** suppose the unit was changed on another device after this screen loaded. The mode write silently puts the old unit back. A `load()` that overlaps the write can then adopt the GET's pre-PUT unit as `confirmed`, so the screen and the server disagree.
- **Context:** the backend contract (it requires `displayUnit` on every PUT) forces the re-send. Still, the overwrite should at least be documented.
- **Fix:** adopt the response's unit through `unitWrite.onSuccess` with a non-current token.
- **Reproducer:** none — it needs a second device writing at the same time, which the fake can't easily model.

**3. The no-confirmed-unit guard can fire during a retry.**
- **Where:** `SettingsState.kt:333`.
- **What's wrong:** if a retry `load()` is still in flight, a mode tap is refused even though the load is about to supply the unit.
- **Reproducer:** none — a narrow timing window.

### Recommendations
- When this field went from undeclared to a strict enum, it needed the same additive-wire handling the codebase already applies elsewhere (`FinishIssueFieldSerializer`, the `completedAt` test). Add both the serializer and a decode test.
- Update the old fixtures that still serve `"sessionView":"LIST"` (`GroupSessionLandingUiTest.kt:114`, `GroupSessionFloorUiTest.kt:262`), or add one deliberate test that checks a legacy value decodes. As written they would fail at decode and hide why.
- The `SettingsState` class KDoc says "everything below holds unchanged for viewWrite". That's no longer true for the shared `loadFailed` flag (Important #1), so state which write clears it and when.

### Assessment
**Ready for the human gate?** No

**Reasoning:** An account still storing `LIST` can't decode its preferences at all, which breaks Settings and the editor's unit lookup. Separately, a pre-existing unit-write line now clears a retry flag that covers both fields, leaving an unconfirmed mode on screen with no way to recover.