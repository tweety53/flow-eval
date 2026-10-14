### Shared-state check
- **`loadFailed`** (it now covers both fields; before this diff it covered only the unit):
  - `SettingsState.kt:270` (a load succeeds): still correct. It only clears when no write of either field has succeeded since the load began.
  - `SettingsState.kt:275` (a load fails): still correct. The same gate applies.
  - `SettingsState.kt:295` (a `chooseUnit` write succeeds): **wrong now.** It clears the flag because the *unit* is confirmed, but it never checks whether the workout mode is confirmed. This is Important #1 below.
  - `SettingsState.kt:310` (a `chooseUnit` write fails with nothing confirmed): still correct.
  - `SettingsState.kt:339` (a `chooseSessionView` write succeeds): correct. This write is only sent when the unit is already confirmed, so after it succeeds both fields are confirmed.
  - `SettingsState.kt:351` (a `chooseSessionView` write fails with nothing confirmed): correct.
- **`sendMutex`**: both send paths take it at `:293` and `:334`. Correct.
- **`unitWrite.confirmed`**: read at `:335`, set only inside `OptimisticWrite` (`:103`, `:135`). Correct.
- **`isSaving`**: now combines both writes. Correct.
- **`hasLoaded`**: written only at `:281`. Unchanged and correct.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
1. **`SettingsState.kt:295`: a successful unit write clears the retry flag while the workout mode is still unknown.**
   - **What happens:** `getPreferences` fails, so `loadFailed` is true and `viewWrite.confirmed` is null. The user then picks a unit, and the PUT succeeds. Line 295 sets `loadFailed = false`, but `chooseUnit` ignores `it.sessionView` in the answer.
   - **Why it matters:** the Workout mode row keeps showing its built-in default, Focus, even if the server stores Quick, and the retry offer is gone. The user is shown a mode the server never confirmed, which the class KDoc says must not happen.
   - **Fix:** either have `chooseUnit` adopt the answer's `sessionView` into `viewWrite` when it has nothing confirmed yet, or clear `loadFailed` only when both fields are confirmed (`viewWrite.confirmed != null`). Add a test for this sequence.
   - **Reproducer:** none — I couldn't run tests here. Test sequence: `preferencesResult = failure`, `load()`, then `preferencesResult = success(KG, QUICK)`, then `chooseUnit(LBS)`. Expected: `!loadFailed` and `sessionView == FOCUS`, while the server holds QUICK.

2. **`SettingsStateTest.kt` ("unit-write-omits-the-mode"): the wire check uses the wrong JSON configuration.**
   - **What's wrong:** it asserts against the default `Json`, not the client's real `Json` instance. The only `encodeDefaults` mention in `commonMain` is a doc comment, so nothing shows the real client config leaves `sessionView` off the request.
   - **Why it matters:** if the client is configured with `encodeDefaults = true`, every unit PUT sends `"sessionView":null`. A server that treats an explicit null differently from a missing key would then break.
   - **Fix:** assert on the body the real `GymieApiClient` sends, for example in `GymieApiClientWorkoutTest` around `:559`.
   - **Reproducer:** none — it depends on the client's `Json` configuration, which I didn't inspect.

#### Minor (Nice to Have)
3. **`SettingsState.kt:276`: the load-failure fallback copy is an inline literal.** "Could not load your settings." replaces `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT`, and the constant is still used by `WorkoutEditorScreen.kt:387`. Consider adding a `FailureMessages` entry for the new text. Reproducer: none — wording only.
4. **`SettingsState.kt:335`: the error message is a generic `error(...)`.** The refusal after a failed load doesn't tell the user to retry loading first. Reproducer: none — UX wording only.
5. **Backward compatibility:** `WorkoutPreferences.sessionView` defaults to FOCUS when an older server leaves the key out. That's acceptable, but a client could then send `sessionView` to a server that doesn't support it yet. Confirm the backend is deployed first. Reproducer: none — deployment order.

### Recommendations
- Make `loadFailed` mean "a field has nothing confirmed" by deriving it from both fields' confirmed state, rather than having each writer clear it for its own field. That removes the whole class of issue #1.
- The fake now echoes the stored mode back, which is good. Use it to cover the unit-write-after-failed-load case.

### Assessment
**Ready for the human gate?** With fixes
**Reasoning:** The design is sound: sends are serialized, the mode write carries the last confirmed unit, and it refuses to guess a unit after a failed load. But `loadFailed` now covers two fields, and the unit-write success path clears it for the unit alone. That can leave an unconfirmed mode on screen with no retry offered.