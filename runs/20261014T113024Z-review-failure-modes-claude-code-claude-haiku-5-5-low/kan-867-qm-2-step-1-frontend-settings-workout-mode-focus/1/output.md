## Failure-recovery check

**Failure state set and cleared (`shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`):**

- `loadFailed` is set at lines 275 (load failure), 310 (unit write failed, nothing confirmed) and 351 (mode write failed, nothing confirmed).
- `loadFailed` is cleared at lines 270 (load success), 295 (unit write success) and 339 (mode write success).
- `unitWrite.confirmed` and `viewWrite.confirmed` (`OptimisticWrite`, lines 54, 103, 135) stay `null` after a failed load. They are set only by `onSuccess` (103) and `adoptLoad` (135).
- `viewWrite.value` shows the FOCUS fallback (initial value 212, `begin` line 80, `adoptLoad` 136, revert 119).

**Clearing lines, per unconfirmed value:**

- **270 (load success):** the mode value is re-confirmed by 272, and the unit value by 271. Both run on the same success path, so there is no gap.
- **295 (unit write success):** the unit value is re-confirmed by 103. The mode value is **not** re-confirmed on this path (`none`). The response's `sessionView` is discarded at 295. This is a partial recovery, reported as the first issue below.
- **339 (mode write success):** the mode value is re-confirmed by 103 (via `viewWrite.onSuccess`). The unit value is not re-confirmed here, but the write cannot send without a confirmed unit (line 335), so the unit was already confirmed earlier, and that earlier path cleared `loadFailed` itself. No gap.

## Summary

The diff adds a workout-mode field to the preferences PUT. It changes the `updatePreferences` signature, the `WorkoutPreferences` and `UpdateWorkoutPreferencesRequest` models, and `SettingsState.load`, `chooseUnit` and the new `chooseSessionView`. Lenses that apply: error return (the recovery flag above), timeout (the 15s HTTP bounds in `GymieApiClient.kt:41,43`, plus the serialising `sendMutex`), and concurrent re-entry (the `confirmed` read inside the mutex). Partial write is not meaningful here, since each PUT is one request. The verdict is "with fixes": the recovery gap and the wire-format claim both look wrong.

## Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

**1. `SettingsState.kt:295`. Error return / partial recovery.** A successful unit write clears `loadFailed` while the mode value is still unconfirmed.
- Failure scenario: `load()` fails, so both `confirmed` values are `null` and the mode control shows FOCUS as a fallback. The user taps LBS, and `updatePreferences` succeeds. The response's `sessionView` (the stored mode) is discarded, `loadFailed` is cleared, and the retry affordance disappears. The screen now shows FOCUS, which the server may not hold, with no way to re-sync.
- Fix sketch: at line 295 also adopt the answer's mode when it is unconfirmed, e.g. `viewWrite.adoptLoad(it.sessionView)` when `viewWrite.confirmed == null`. Alternatively, clear `loadFailed` only when both fields are confirmed.
- Reproducer: none — no existing test covers "failed load, then successful unit write". The test would need to be added to `SettingsStateTest`: `preferencesResult` fails, `updatePreferences` returns a `QUICK` mode, then assert `loadFailed == false` and that the mode shown is `QUICK`. Against the diff, `FakeWorkoutApi` returns FOCUS, so the assertion fails.

**2. `WorkoutModels.kt:700` / `Models.kt:33`. Wire contract. Lens: error return.** The claim that a `null` `sessionView` is omitted from the wire does not hold for the client's JSON instance.
- The client's `Json` (`GymieApiClient.kt:245-248`) sets only `ignoreUnknownKeys` and `isLenient`. kotlinx.serialization's default is `encodeDefaults = true`, so `sessionView: null` is serialised as `"sessionView":null`. The comment's "`encodeDefaults = false`" is wrong; the existing `Models.kt:33` comment has the same mistake.
- The new test `unit write leaves the workout mode off the wire` (`SettingsStateTest.kt`) uses the top-level `Json` with the same default, so it should fail for the same reason.
- Failure scenario: every unit PUT sends `"sessionView":null`. If the server's DTO declares a non-null `SessionView`, every unit write is rejected, which breaks the existing unit path. If the server treats a missing or null mode as FOCUS, every unit change silently resets a stored QUICK. The diff alone cannot show which server behaviour applies.
- Fix sketch: annotate `UpdateWorkoutPreferencesRequest` with `@EncodeDefaults(false)` (kotlinx `@OptIn(ExperimentalSerializationApi)`), or use a `Json` with `encodeDefaults = false`, and confirm the server's handling of an absent `sessionView`.
- Reproducer: `./gradlew :shared:jvmTest --tests 'com.gymie.shared.ui.SettingsStateTest'` (task name assumed; I did not run it). I expect `a unit write leaves the workout mode off the wire` to fail with `"sessionView"` present in the encoded string.

**3. `SettingsState.kt:293-295` and `328-336`. Concurrent re-entry.** `chooseUnit` updates `unitWrite.confirmed` in `onSuccess`, which runs after `sendMutex.withLock` releases. `chooseSessionView` reads `unitWrite.confirmed` inside the lock.
- Failure scenario: on a multi-threaded dispatcher, a unit write (LBS) finishes, the lock is released, and a waiting mode write acquires it before `onSuccess` runs. The mode write then sends the previous confirmed unit (KG) and overwrites the server's LBS. The single-threaded `runTest` cannot show this, and the comment at 322-324 assumes an ordering the code does not enforce.
- Fix sketch: move the `confirmed` update inside `withLock`, or derive the unit from a value that is written under the same lock.
- Reproducer: none — needs a test on `Dispatchers.Default` with real concurrency; `runTest` is single-threaded.

#### Minor (Nice to Have)

**4. `SettingsState.kt:328-359`. Timeout.** Each PUT is bounded at 15s (`GymieApiClient.kt:41,43`), but `sendMutex` serialises every write with no coalescing. Repeated taps queue sequential 15s PUTs, and `isSaving` stays true for the whole queue. Failure scenario: a slow server turns a few taps into a long stall. Fix sketch: coalesce to the latest pending value per field. Reproducer: none — needs a delayed `MockEngine` and a timing assertion.

## Assessment

**Ready for the human gate?** With fixes

**Reasoning:** Issues 1 and 2 are concrete and fixable. Issue 1 leaves the mode showing an unconfirmed FOCUS fallback with no retry, and issue 2 may break every unit write, which the existing tests should catch. Issue 3 needs a multi-threaded test to confirm, and issue 4 is a usability concern rather than a correctness one. I did not run the test suite, so the wire-format conclusion is based on reading the code and the kotlinx default.