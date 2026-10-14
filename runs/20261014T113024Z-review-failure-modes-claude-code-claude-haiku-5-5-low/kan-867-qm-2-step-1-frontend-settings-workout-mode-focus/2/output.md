### Failure-recovery check

State set on failure, and every line that clears it (all in `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`):

- **`loadFailed`** is set to true at 275 (load failed), 310 (unit write failed with nothing confirmed) and 351 (view write failed with nothing confirmed).
  - Cleared at 270 (load succeeded). Both fields are re-confirmed by `adoptLoad` at 271–272, so nothing is left unconfirmed.
  - Cleared at 295 (unit write succeeded). This leaves `sessionView` unconfirmed: `viewWrite.confirmed` is not touched, and the PUT response's `sessionView` is discarded. Re-confirmed by: **none**.
  - Cleared at 339 (view write succeeded). This leaves `displayUnit` unconfirmed: the PUT response's `displayUnit` is discarded and `unitWrite.confirmed` is not updated. Re-confirmed by: **none**.
- **`viewWrite.value` / `unitWrite.value`** (optimistic) are reverted in `onFailure` (`OptimisticWrite.kt:115–124`) when a confirmed value exists. With nothing confirmed, the optimistic value stays on screen by design, and `loadFailed` is raised.
- **`viewWrite.confirmed`** is written only by `onSuccess` (103). It stays null after a failed load, and the view-write path then refuses to send (335). That refusal is correct.
- **`isSaving`** is derived from `begin`/`end`, and `end()` runs in `finally` for both paths. No issue.

### Summary

The diff changes four behaviours: unit and mode writes sharing one PUT payload, the load gating over two fields, the mode write's send-under-mutex, and the `loadFailed` flag. Lenses that bear on them:

- **Error return:** the discarded PUT answers and the `loadFailed` clear at 295 are the real problem.
- **Timeout:** the PUT is bounded by `REQUEST_TIMEOUT_MILLIS = 15_000` (`GymieApiClient.kt:41, 255–258`), so the `sendMutex` hold is bounded. Queued writes can wait up to about 15 s. Not a finding.
- **Partial write:** each path is a single PUT. Not applicable.
- **Concurrent re-entry:** both writes serialise on `sendMutex` and read `unitWrite.confirmed` inside it. Stale tokens are handled. No finding.

Verdict: one Important defect, in the recovery path, introduced by this diff.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. `SettingsState.kt:295` clears `loadFailed` while `sessionView` is still unconfirmed (error return).**
- Scenario: `load()` fails, so `viewWrite.confirmed` is null and `loadFailed` is true. The user toggles the unit, and the unit PUT succeeds. Line 295 clears `loadFailed`. The PUT response carries the server's `sessionView`, but the code discards it, and `viewWrite` is never confirmed. The control keeps showing the default FOCUS, the retry affordance disappears, and the screen claims recovery. If the server holds QUICK, the screen is wrong and nothing will re-sync it.
- Fix: on unit success, also adopt the response's `sessionView` (e.g. `viewWrite.adoptLoad(it.sessionView)`, or an `onSuccess` path when the answer's value is trusted), or keep `loadFailed` set while any field is unconfirmed. Do the same for line 339: it should adopt `it.displayUnit` for the unit field, or it should not clear `loadFailed` while the unit is unconfirmed.
- Reproducer: none — requires a new `SettingsStateTest` case, not a shell command. Sketch: `api.preferencesResult = Result.failure(...)`; `load()`; set `api.updatePreferencesResult` to succeed with `WorkoutPreferences(LBS, QUICK)` (the fake's `preferencesResult` default is what the PUT returns); `chooseUnit(LBS)`; assert `state.loadFailed == false` and `state.sessionView == QUICK`. Expect the assertion on `sessionView` to fail on the current diff.

**2. Unverified server contract for the omitted `sessionView` (error return).**
- `UpdateWorkoutPreferencesRequest.sessionView` is left off the wire on a unit write (`WorkoutModels.kt`, `encodeDefaults = false`). The comment says null means "unchanged". If the server defaults an absent `sessionView` to FOCUS, every unit save silently resets a QUICK mode to FOCUS, and with fix 1 unaddressed the client would not notice. The unit test only asserts the key is absent from the JSON; it does not prove the server's behaviour.
- Fix: confirm the server's handling of an absent key. If absence means FOCUS, send the confirmed `viewWrite` value on unit writes too.
- Reproducer: none — needs the backend. Check the server's PUT `api/workouts/preferences` handler with a body of `{"displayUnit":"LBS"}` after storing QUICK.

#### Minor (Nice to Have)

**3. `SettingsState.kt:316` hard-codes the load-failure message text (error reporting).**
- `load()` previously used `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT`; it now uses the literal `"Could not load your settings."`, and the `FailureMessages` import was dropped. This changes user-visible copy with no failure-mode reason, and the new text no longer names what failed. Fix: restore the constant or add a settings-level one.
- Reproducer: none — a UI copy change; checked by reading the diff.

### Assessment
**Ready for the human gate?** With fixes
**Reasoning:** The write and load paths are bounded and serialise correctly, and the stale-token handling is sound. The one real defect is that a successful unit save clears `loadFailed` without confirming the mode, and the server's answer for the mode is thrown away, so a failed load can leave the mode display silently wrong. Fix 1 needs a test before merging, and the server contract in item 2 should be confirmed first.