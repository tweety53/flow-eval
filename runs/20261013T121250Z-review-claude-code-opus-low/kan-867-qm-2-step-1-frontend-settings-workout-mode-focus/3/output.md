### Shared-state check

The diff's new code reads or writes these existing pieces of state: `loadFailed`, `sendMutex`, `unitWrite.confirmed`, `unitWrite.succeededWriteCount`, `loadSeq`, `hasLoaded`, `isSaving` and the fake's `preferencesResult`.

- **`loadFailed`** — it now stands for "something about the preferences payload, unit or mode, is unconfirmed". Its writers:
  - `SettingsState.kt:270`: load success, now gated on no write of either field having succeeded. Still correct.
  - `SettingsState.kt:275`: load failure, same gate. Still correct.
  - `SettingsState.kt:295` (unchanged): `chooseUnit` success sets `loadFailed = false` when the unit write is current. Before this diff, a confirmed unit meant everything on the screen was confirmed. Now `viewWrite.confirmed` can still be null. **Defect, see I1.**
  - `SettingsState.kt:310`: unit write with nothing confirmed. Still correct.
  - `SettingsState.kt:339`: view write success. Still correct, because a view write can only be sent once a unit is confirmed (`:335`).
  - `SettingsState.kt:351`: view write with nothing confirmed. Still correct.
- **`unitWrite.confirmed` / `succeededWriteCount`** — written only inside `OptimisticWrite`: `onSuccess` at `:102-103` and `adoptLoad` at `:135`. They are read under the mutex at `:335`, so they reflect the earlier sends once those settle. Still correct. One gap: the unit's own `onSuccess` never feeds the server's `sessionView` into `viewWrite` (part of I1).
- **`sendMutex`** — only locked at `:293` and `:332`. Still correct.
- **`loadSeq`** — written only at `:253` (`next`). Still correct.
- **`hasLoaded`** — written only at `:281`. Still correct.
- **`isSaving`** — derived. Still correct.
- **`FakeWorkoutApi.preferencesResult`** — overwritten at `FakeWorkoutApi.kt:200` by every update. The new default (`sessionView ?: storedView`) keeps the mode the fake already held. Still correct.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**I1 — `SettingsState.kt:295`: a successful unit write clears `loadFailed` even though the workout mode was never confirmed.**
- **What happens:**
  1. `load()` fails, so `loadFailed` is true and `viewWrite.confirmed` is null.
  2. The user taps LBS, and that write succeeds.
  3. Line 295 sets `loadFailed = false`.
  4. The retry option disappears. The Workout mode row keeps showing the placeholder FOCUS, even if the server holds QUICK. Nothing on screen says this is a guess, and the PUT's response carried the real `sessionView`.
- **Why it matters:** the class KDoc promises that nothing unconfirmed is shown as if it were confirmed. That promise now fails for the mode row.
- **How to fix:** in `chooseUnit`'s success path, also adopt `it.sessionView` into `viewWrite` while no view write is in flight. That is safe because the server answers with the stored mode. Alternatively, only clear `loadFailed` when `viewWrite.confirmed != null`. Add a test for this sequence.
- **Reproducer:** `none — no test covers it; the sequence above can be added to SettingsStateTest (fail load, chooseUnit(LBS) succeeds, assert loadFailed or that sessionView equals the server's QUICK)`.

**I2 — `SettingsState.kt:276`: the load-failure message is no longer the shared constant.**
- **What changed:** the message is now the inline string "Could not load your settings." `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` (`FailureHandling.kt:15`) is still used by `WorkoutEditorScreen.kt:387`.
- **Why it matters:** this message change is user-visible and wasn't mentioned anywhere in the change. The other failure strings (`:347`, `:352`) are also inline literals repeated in three places.
- **How to fix:** add a `FailureMessages` constant for the settings-load message and one for the session-view message.
- **Reproducer:** `none — copy/consistency issue, not a runtime failure`.

#### Minor (Nice to Have)

- **`SettingsState.kt:335`** — when no unit is confirmed, the code calls `error(...)` inside `runCatching`. The resulting `IllegalStateException` message becomes the user-facing notification. That works, but a dedicated early return would read more clearly than using an exception for a normal outcome.
- **`SettingsScreenUiTest.kt` (`theWorkoutModeControlKeepsOneLineAtPhoneWidth`)** — this test has no `@Feature` tag, unlike its siblings.
- **`AppScreens.kt` `WorkoutModeSetting`** — `enabled = true` is hard-coded, but the unit rows use their own enabled and saving state. The interaction is safe because sends are serialised, so this is only a consistency point.
- **`WorkoutModels.kt:697`** — `WorkoutPreferences.sessionView` defaults to FOCUS when the key is missing. An older server that doesn't send the key will look exactly like a stored FOCUS. That is acceptable given the comment, but the compatibility assumption belongs in the API KDoc.

### Recommendations
- Add a test where a unit write's response carries `sessionView: QUICK` after a failed load. It covers I1 directly.
- Consider a single "adopt the full server answer" helper used by both write paths and by load. That would stop each field's success path from ignoring the other field's half of the same payload.

### Assessment
**Ready for the human gate?** With fixes
**Reasoning:** The interface change is backward-compatible (the new parameter is nullable with a default, and the request omits it when null). The tests run against real state logic and a MockEngine, and the unit-snapshot-under-mutex design is sound. The one real bug is that the unchanged `chooseUnit` success path clears `loadFailed`, which still assumes the unit is the only field on the screen. After a failed load, that hides retry while the mode row still shows a guessed value.