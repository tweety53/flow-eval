### Shared-state check

The diff reuses the following state:

- **`loadFailed`**:
  - `SettingsState.kt:270`: still correct. `load` clears it only when no write of either field has succeeded since the load began.
  - `SettingsState.kt:275`: still correct. It uses the same gate.
  - `SettingsState.kt:295`: **issue (Important #1).** A unit write that succeeds clears it even when the workout mode has never been confirmed.
  - `SettingsState.kt:310`: still correct. It only raises the flag.
  - `SettingsState.kt:339`: correct in practice. A mode write can only be sent once a unit is confirmed, so by then the unit is known.
  - `SettingsState.kt:351`: in practice this is the guard at line 335 refusing to send. It is correct: nothing is confirmed, so the retry path is right.
- **`sendMutex`**: it is only taken at 293 and 334, and both are send-only critical sections. Still correct.
- **`unitWrite.confirmed`**: it is now read at 335 to build the mode write. Its writers are `OptimisticWrite.onSuccess:103` and `adoptLoad:135`. Both set it only from real server answers, and the read happens inside the mutex, so a queued unit choice can't leak into the mode write. Still correct.
- **`unitWrite.succeededWriteCount` / `viewWrite.succeededWriteCount`**: written only at `:102`. The mode write sends the confirmed unit, so it doesn't change the server's unit, and skipping a bump to the unit counter is sound. Still correct.
- **`isSaving`**: now the OR of both fields. Still correct.
- **`UpdateWorkoutPreferencesRequest` wire shape**: the client's `Json` (`GymieApiClient.kt:245`) leaves `encodeDefaults` at its default of false. A null `sessionView` is therefore omitted from the request, as the KDoc claims. Correct.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. `SettingsState.kt:295` (unchanged writer) and `:297`: after a failed load, a successful unit write clears `loadFailed` but leaves the workout mode on a default the server never sent.**

- **What happens:** `load` fails, so `viewWrite.confirmed` is null and the control shows FOCUS from the constructor. The user picks LBS and the PUT succeeds. Line 295 then sets `loadFailed = false`, which removes the retry.
  - The PUT response carries the stored `sessionView` (say QUICK), but `chooseUnit` throws it away.
  - Settings now shows "Focus" while the server holds Quick, and the screen has no retry left.
- **Why it matters:** this contradicts the class's own rule that it never presents an unconfirmed value as confirmed, and it hides the only way to re-sync. Tapping "Focus" afterwards also does nothing visible, because it is already selected.
- **Fix:** in `chooseUnit`'s `onSuccess`, call `viewWrite.adoptLoad(it.sessionView)` when no mode write has succeeded or is in flight. The answer is true server state. Alternatively, clear `loadFailed` only when `viewWrite.confirmed != null` as well.
- **Reproducer:** none — there is no existing test for this path. This test would fail against the diff: `preferencesResult = failure`; `load()`; `preferencesResult = success(KG, QUICK)`; `chooseUnit(LBS)`; then assert `sessionView == QUICK || loadFailed`.

**2. `WorkoutModels.kt:696`: the backward-compatibility story is one-sided.**

- **What happens:** against a backend that has not yet shipped `session_view`, the PUT's `sessionView` is ignored. The response has no key, so it decodes to FOCUS, and `viewWrite.onSuccess` adopts FOCUS. The user's tap on Quick then silently snaps back with no error.
- **Why it matters:** the project requires a changed interface to carry its compatibility story. This one covers the read side ("a payload without the key means FOCUS") but not deploy order.
- **Fix:** state that the server ships first, or treat a missing key in a PUT answer as "unconfirmed" (for example, make the response field nullable).
- **Reproducer:** none — this needs an old server.

#### Minor (Nice to Have)

- **`SettingsState.kt:276`:** the load-failure fallback is now an inline literal ("Could not load your settings.") instead of a `FailureMessages` constant. "Could not save your workout mode." is also repeated three times. Add constants next to `COULD_NOT_LOAD_WEIGHT_UNIT`.
- **`SettingsState.kt:339`:** the mode write's answer also carries `displayUnit`, which is real server state, and it is dropped. This is harmless because the unit sent was the confirmed one, but adopting it into `unitWrite.confirmed` would be more consistent.
- **`SettingsStateTest.kt` "mode-write-sends-confirmed-unit":** the gate predicate depends on `lastUpdatePreferencesView == null` at hook time. That couples the test to the fake's field-assignment order. A per-call counter would be less brittle.
- **`AppScreens.kt` `WorkoutModeSetting`:**
  - `enabled = true` is hard-coded. Compare whether the unit row disables itself while a load is failing.
  - The label is built with `if FOCUS "Focus" else "Quick"`, which mislabels any future enum entry. Use a `when` to make it exhaustive.

### Recommendations
- Add tests for the cross-field interactions:
  - a unit write after a failed load (Important #1);
  - a mode write overlapping an in-flight load.
- Consider letting each write adopt *both* fields from the full preferences answer, so the two `OptimisticWrite`s can't drift from the server.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The mutex-serialised use of the confirmed unit is sound, and the tests exercise real state transitions. However, the untouched `chooseUnit` writer at line 295 now clears `loadFailed` for a state that also covers a never-confirmed workout mode, so Settings can show the wrong mode with no retry left.