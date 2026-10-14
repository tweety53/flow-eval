### Failure-recovery check

State the failure path sets or leaves behind, and every line that clears or overwrites it:

- **`loadFailed`**: set true at `SettingsState.kt:275` (load failure) and `:351` (view write with nothing confirmed). Cleared at `:270` (load success, only when no write of either field succeeded since the load began) and `:339` (view write success, only if `viewWrite.onSuccess` reports it is still the current write).
  - `:270` re-confirms both values: `:271` adopts `displayUnit` and `:272` adopts `sessionView`, and `noWriteSucceeded` guarantees both run. No gap.
  - `:339` leaves **`displayUnit` unconfirmed**. The success response carries `it.displayUnit` and the code discards it. Nothing else on that path re-confirms the unit; `unitWrite.onSuccess` (`:295`) is only in `chooseUnit`. **Partial recovery, see Important 1.**
- **`viewWrite.value`** (optimistic QUICK): set at `viewWrite.begin` (`:329`). Reverted in `viewWrite.onFailure` (`:341`) only if the failure callback runs. On cancellation it is never reverted, and nothing else clears it before `end()` (`:357`). **Gap, see Important 2.**
- **`viewWrite.confirmed`**: set by `onSuccess` (`:339`) and `adoptLoad` (`:272`). A missing `sessionView` key in the payload is adopted as `FOCUS` (default in `WorkoutModels.kt`), so the fallback is recorded as if the server said it. **See Minor 1.**
- **`unitWrite.confirmed`** is read at `:335` to build the PUT. If a load failed, it may be stale, not unknown. **See Important 3.**

Timeouts: `GymieApiClient.kt:255-258` installs `HttpTimeout` on request, connect and socket, so the network call is bounded. No timeout finding.

Concurrent re-entry: writes go through `sendMutex` (`:333`), the unit is read inside the lock, and the PUT carries the full preference state, so a double tap sends the same body twice and both succeed. That is acceptable. No finding beyond Important 3.

### Summary

The diff changes the Settings preference write path (`SettingsState.chooseSessionView` and `load`), the `updatePreferences` API signature and the request/response models. The error-return, timeout and concurrent re-entry lenses bear on the write path. The partial-write lens does not apply: each PUT writes a single preference record and there is no multi-file or multi-row write. The main problems are a partial recovery that clears `loadFailed` while the unit is still unconfirmed, a cancelled write that leaves an unconfirmed value on screen, and a stale unit being written back to the server.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

1. **`SettingsState.kt:339`**: error-return lens. A successful view write clears `loadFailed` while `displayUnit` is still unconfirmed.
   - Failure scenario: a load fails, so `unitWrite.confirmed` is an older value. The user taps Quick, the PUT succeeds, and the response's `displayUnit` (the server's real unit) is discarded. `loadFailed` goes false and the screen reads as recovered, but the unit shown is never re-checked against the server.
   - Fix: adopt `it.displayUnit` into `unitWrite.confirmed` on success, or keep `loadFailed` true unless the unit is also confirmed by this response.
   - Reproducer: none. Needs a test where `load()` fails, then `chooseSessionView(QUICK)` succeeds with a response whose `displayUnit` differs from `confirmed`, and assert the unit state. Sketch: `FakeWorkoutApi` with `preferencesResult = failure`, then `updatePreferencesResult = success(WorkoutPreferences(LBS, QUICK))`, then assert `loadFailed == false` and the unit reflects the server.

2. **`SettingsState.kt:329-357`**: timeout / cancellation lens. If the coroutine is cancelled during the PUT (the composable leaves, or its scope is cancelled), `onFailureNotCancelled` is presumed not to call `viewWrite.onFailure`. In that case the optimistic QUICK stays on screen with no revert and no notification, then `end()` runs in `finally`. I inferred the cancellation behaviour from the helper's name and did not read its body.
   - Failure scenario: the user taps Quick and navigates away before the response arrives. The server may or may not have applied it. The control shows Quick, and `confirmed` stays at the last known value.
   - Fix: in `finally`, if the write did not settle, revert via `viewWrite.onFailure(token)` or wrap the PUT in `withContext(NonCancellable)` so the outcome is recorded.
   - Reproducer: none, as a runnable command. Needs a new test: gate `beforeUpdatePreferences`, `launch` the `chooseSessionView(QUICK)` call, cancel the job, then assert `state.sessionView == FOCUS` or that a `load()` restores truth.

3. **`SettingsState.kt:335-336`**: concurrent re-entry (two clients). The PUT sends `unitWrite.confirmed`. If another client changed `displayUnit` after this device's last load, the PUT silently writes the stale unit back to the server.
   - Failure scenario: device A loads KG. Device B sets LBS. Device A taps Focus or Quick and the PUT sends `displayUnit=KG`, overwriting LBS.
   - Fix: send only the changed field if the backend allows it, or re-read the unit before the PUT.
   - Reproducer: none. Needs two clients against a backend. Sketch: a `MockEngine` that returns LBS on a second GET, then assert the PUT body does not carry KG.

#### Minor (Nice to Have)

1. **`WorkoutModels.kt` (`WorkoutPreferences.sessionView = FOCUS`) and `SettingsState.kt:272`**: error-return lens. A payload without `sessionView` is adopted as `FOCUS` and becomes `confirmed`. A later failed write reverts to a value the server never sent.
   - Fix: make the field nullable in the response, or record the absence explicitly.
   - Reproducer: `Json.decodeFromString<WorkoutPreferences>("""{"displayUnit":"KG"}""").sessionView == FOCUS`. This is the fixture the existing `SettingsScreenUiTest` GET already returns, so the fallback is currently untested.

### Assessment
**Ready for the human gate?** With fixes
**Reasoning:** The happy path and the failed-load guard are sound, and the request timeout is bounded. Three failure paths need attention before shipping: the success clear of `loadFailed` that leaves the unit unconfirmed, the cancelled write that leaves an unconfirmed value on screen, and the stale unit written back to the server. None of these corrupts stored workout data, since the unit only affects newly created workouts, but each leaves the screen showing a state the server has not confirmed.