### Derived-state check

- **`loadFailed`** (`SettingsState.kt:242`). It stands for three facts together: the newest load failed, the unit has no confirmed value (`unitWrite.confirmed == null`), and the mode has no confirmed value (`viewWrite.confirmed == null`). The class KDoc (`:171-193`) says it tells the screen whether there is "nothing confirmed" yet. Writers:
  - `SettingsState.kt:270` (load success). It clears the flag only when neither field has had a write succeed since the load began. That is a conservative gate, so this writer is OK.
  - `SettingsState.kt:275` (load failure). Same gate. OK.
  - `SettingsState.kt:295` (unit write success, an unchanged line). It clears the flag using only the unit's write. **This is a violation, see Important #1.**
  - `SettingsState.kt:310` (unit write, `NOTHING_CONFIRMED`). It sets the flag from the unit's fact only. A set on failure is acceptable.
  - `SettingsState.kt:339` (mode write success). It clears the flag using only the mode's write. In practice this is safe, because `:335` refuses to send a mode write unless the unit is already confirmed. It is still the same one-fact-decides-all pattern.
  - `SettingsState.kt:351` (mode write, `NOTHING_CONFIRMED`). It sets the flag from the mode's fact. Acceptable.
- **`isSaving`** (`:234-235`). Computed as `unitWrite.isSaving || viewWrite.isSaving`, not stored. Correct.

### Summary
The diff adds a `SessionView` preference end to end: the API signature, the DTOs, `SettingsState` with a second `OptimisticWrite`, the Settings row, and tests. No project standards files were configured, so no hard invariants apply. I applied only the principles in `engineering-principles.md`, across all three groups.

Most of the structure is good:
- `OptimisticWrite` is reused for the new field rather than duplicated (DRY).
- `isSaving` is computed from both writes rather than stored (SSOT).
- The nullable "unchanged" parameter is a deliberate KISS choice and is documented at `WorkoutModels.kt`.

The problem is the shared, stored `loadFailed` flag. It was a correct summary when there was one field. With two fields it is now only a partial summary.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

1. **`SettingsState.kt:295` (made stale by `:226-228`, `:339`). Rule: Single Source of Truth. "Each fact … is stored … in exactly one place; everything else derives from it."**
   - `loadFailed` now stands for "the unit or the mode has nothing confirmed", but each write's success clears it based on its own field alone.
   - Scenario: the load fails, so both fields are unconfirmed. The user then picks a unit, the write succeeds, and `:295` sets `loadFailed = false`.
   - Result: the mode row shows the `FOCUS` placeholder as if it were stored, `viewWrite.confirmed` is still null, and the screen no longer offers the retry/re-sync path.
   - **Fix:** derive it instead of storing it. For example: `val loadFailed get() = latestLoadFailed && (unitWrite.confirmed == null || viewWrite.confirmed == null)`, or simply `unitWrite.confirmed == null || viewWrite.confirmed == null` once a load has finished. Then delete the writers at `:270/:295/:310/:339/:351`.
   - **Reproducer:** none — I'm working read-only and the repo has no such test. The test to add: in `SettingsStateTest`, make `preferencesResult` fail, call `load()`, then `chooseUnit(LBS)`, then `assertTrue(state.loadFailed)`. That assertion fails against this diff.

2. **`SettingsState.kt:276`, `:335`, `:347`, `:352` versus `components/FailureHandling.kt:15`. Rule: DRY. "the same … magic [string] written in two places."**
   - The diff stops using the shared `FailureMessages` object and instead writes the literal `"Could not save your workout mode."` three times, plus a new literal for the load message.
   - It also leaves `COULD_NOT_LOAD_WEIGHT_UNIT` used only by `WorkoutEditorScreen.kt:387`, so the two screens now report the same preferences-load failure with different text.
   - **Fix:** add `COULD_NOT_LOAD_SETTINGS` and `COULD_NOT_SAVE_WORKOUT_MODE` to `FailureMessages`, or at least a private const like `CalculatorState.kt:29-31` uses.
   - **Reproducer:** `grep -n '"Could not save your workout mode."' shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt` prints three lines.

#### Minor (Nice to Have)

1. **`SettingsState.kt:323-358` versus `chooseUnit` (`:283-318`). Rule: WET, which needs any kept duplication to be explained.**
   - The `begin/runCatching/onSuccess/when(onFailure)/end` skeleton is copied almost line for line.
   - The class KDoc says the two "hold unchanged" for each other, which is the case for sharing the code, not for keeping two copies.
   - **Fix:** extract a private `suspend fun <T> write(w: OptimisticWrite<T>, v: T, msg: String, send: suspend () -> T)`, or add a note saying why the copy is kept.
   - **Reproducer:** none — a structural comparison of the two functions, not something a command shows.

2. **`AppScreens.kt:412`. Rule: Least Astonishment.** The labels come from `if (it == FOCUS) "Focus" else "Quick"`, so any new enum value would silently be labelled "Quick". **Fix:** use an exhaustive `when`.
   - **Reproducer:** none — this only goes wrong once a third enum value is added.

### Assessment
**Principles-compliant?** With fixes

**Reasoning:** The design is reasonable: one optimistic-write type per field, a computed `isSaving`, and a documented nullable "unchanged" parameter. But the stored `loadFailed` flag is now cleared by a writer that only knows about one of the two fields it stands for. That is an SSOT violation that can hide the fact that the workout mode was never loaded. The failure messages were also moved out of their single shared home.