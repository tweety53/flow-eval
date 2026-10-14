### Shared-state check

- `loadFailed` (SettingsState.kt:242)
  - `SettingsState.kt:270` (load success): gated on `current && noWriteSucceeded`. Still correct.
  - `SettingsState.kt:275` (load failure): gated the same way. Still correct.
  - `SettingsState.kt:295` (`chooseUnit` success): unchanged, sets `false` only when `unitWrite.onSuccess` returns true. Still correct.
  - `SettingsState.kt:310` (`chooseUnit` nothing confirmed): unchanged, sets `true`. Still correct.
  - `SettingsState.kt:339` (`chooseSessionView` success): sets `false` only when `viewWrite.onSuccess` returns true. Correct.
  - `SettingsState.kt:351` (`chooseSessionView` nothing confirmed): sets `true`. Correct.
- `unitWrite` state, via `adoptLoad` (`SettingsState.kt:271`): gated on `noUnitWriteSucceededSinceThisLoadBegan`. Still correct. Its writers are the unchanged `chooseUnit` path, which now sends `sessionView = null` and so omits the key. That is correct only if the server treats an absent `sessionView` as "unchanged" (see Important #1).
- `viewWrite` state, via `adoptLoad` (`SettingsState.kt:272`) and `onSuccess`/`onFailure` (lines 339, 351): gated the same way. Still correct.
- `isSaving` and `hasLoaded`: no conflicting writers.

### Issues

#### Critical (Must Fix)
None found in the diff.

#### Important (Should Fix)

1. **`UpdateWorkoutPreferencesRequest` unit writes depend on unseen server semantics.** `WorkoutModels.kt:700` and `SettingsState.kt:290` (`chooseUnit`) send a PUT without `sessionView`. The KDoc says that means "unchanged". If the server treats an absent key as the default `FOCUS`, every unit change silently resets the workout mode. The diff does not show the server, and no test in this diff covers the server side.
   - Fix: confirm the server's PUT handler leaves `session_view` untouched when the key is absent, and add a backend test for it.
   - Reproducer: none — the server handler is not in this repository. Verify with the backend test suite.

#### Minor (Nice to Have)

2. **Unrelated message change.** `SettingsState.kt:276` replaces `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` with a new literal `"Could not load your settings."`, and drops the import. Other screens still use the shared constant (`WorkoutEditorScreen.kt:387`), so this is a silent change to user-visible copy outside the feature. Restore the constant or split it into its own change.
   - Reproducer: `grep -rn "COULD_NOT_LOAD_WEIGHT_UNIT" shared/src` shows the constant still in use elsewhere, so the literal is a divergent copy.

3. **Wire-format test does not use the app's client.** `SettingsStateTest.kt` (`a unit write leaves the workout mode off the wire`) asserts with a bare `Json.encodeToString`. The production client's `Json` configuration is not in the diff, so the test proves kotlinx's default, not the app's config.
   - Fix: assert against the real client's serializer, or against the MockEngine body in `SettingsScreenUiTest`.
   - Reproducer: none — the app's `Json` instance is not in the diff, so the test cannot be checked against it here.

4. **Test `a unit write leaves the workout mode off the wire` also checks `lastUpdatePreferencesView`.** That is a fake-level assertion and does not prove the request body omits the key. The `encodeToString` line is the only wire check, and it is the weaker one from item 3.

### Recommendations
- Keep the server contract for the optional `sessionView` field in one place. Its meaning is currently spread across the KDoc at `WorkoutApi.kt`, `WorkoutModels.kt:700`, and `SettingsState.kt`.
- The `SettingsState` class KDoc is long. Consider moving the per-field rules into a short section per field.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The optimistic-write and load-gating logic for `loadFailed`, `unitWrite` and `viewWrite` is consistent across every writer I checked. The blocking open question is whether the server treats an absent `sessionView` as "unchanged"; that must be verified in the backend before a unit change can be trusted not to reset the mode. The message regression at `SettingsState.kt:276` should be reverted in this change.