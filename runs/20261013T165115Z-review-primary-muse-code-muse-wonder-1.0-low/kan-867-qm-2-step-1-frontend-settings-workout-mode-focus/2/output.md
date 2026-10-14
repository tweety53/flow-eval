### Shared-state check

- **`loadFailed`** (`shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:242`) — writers: `SettingsState.kt:270` (load success clears), `SettingsState.kt:275` (load failure sets), `SettingsState.kt:295` (chooseUnit success clears — pre-existing, unchanged), `SettingsState.kt:310` (chooseUnit NOTHING_CONFIRMED sets), `SettingsState.kt:339` (chooseSessionView success clears, new), `SettingsState.kt:351` (chooseSessionView NOTHING_CONFIRMED sets, new). **Issue:** the unchanged writer at `:295` now clears a flag that also covers `sessionView`, while confirming only the unit — see Important issue 1.
- **`OptimisticWrite.confirmed`** (`SettingsState.kt:54`) — writers: `SettingsState.kt:102` (`onSuccess`), `SettingsState.kt:135` (`adoptLoad`). New reader: `unitWrite.confirmed` at `SettingsState.kt:335` in `chooseSessionView`. **Issue:** `onSuccess` is invoked outside `sendMutex` — see Important issue 2.
- **`OptimisticWrite.succeededWriteCount`** (`SettingsState.kt:69`) — writer: `SettingsState.kt:102` only; readers at `SettingsState.kt:254-255, 264-265`. Still correct: per-field counts, per-field adoption gates at `:271-272`, shared `loadFailed` gate at `:270` requiring both.
- **`OptimisticWrite.value`** (`SettingsState.kt:46`) — writers: `SettingsState.kt:80` (`begin`), `:105` (`onSuccess`), `:119` (`onFailure` revert), `:136` (`adoptLoad`, suppressed while saving). Still correct for both `unitWrite` and the new `viewWrite`.
- **`sendMutex`** (`SettingsState.kt:249`) — acquisitions: `SettingsState.kt:293` (chooseUnit), `SettingsState.kt:332-337` (chooseSessionView). Serializes sends as claimed, but not the `confirmed` bookkeeping that follows each send — see Important issue 2.
- **`FakeWorkoutApi.preferencesResult`** (`shared/src/commonTest/kotlin/com/gymie/shared/api/FakeWorkoutApi.kt:39`) — writer: `FakeWorkoutApi.kt:200` (every `updatePreferences` stores its answer); new reader at `FakeWorkoutApi.kt:191` snapshots the stored `sessionView` before the hook, mirroring `getPreferences`. Still correct.
- **`WorkoutPreferences.sessionView`** (new field, `WorkoutModels.kt:697`) — default `FOCUS` makes old payloads without the key decode correctly; the client's `Json` at `GymieApiClient.kt:245-248` does not set `encodeDefaults`, so a null `UpdateWorkoutPreferencesRequest.sessionView` is genuinely omitted from the wire, as the new serialization test in `SettingsStateTest.kt` asserts. Still correct.

### Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

1. **A successful unit write clears the retry offer and leaves `sessionView` showing an unconfirmed default.** File: [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295). `chooseUnit`'s success clears the shared `loadFailed`, but adopts only `it.displayUnit` — the full `WorkoutPreferences` answer's `sessionView` is discarded, and `viewWrite` is never touched. After a failed `load()` (nothing confirmed, `sessionView` showing the `FOCUS` default at `SettingsState.kt:212-216`), a successful unit tap clears `loadFailed`, removing the retry path, while the Workout mode row still shows `FOCUS` even if the server holds `QUICK` — and, since the screen only re-runs `load()` on mount, nothing re-syncs it. Before this diff `loadFailed` covered only the unit, so clearing it on unit success was sound; the diff makes the flag cover two fields but left this writer confirming one. Fix: in `chooseUnit`'s `onSuccess`, also adopt the answer's mode (e.g. `viewWrite.adoptLoad(it.sessionView)` — safe under `sendMutex` ordering, since the server processed this PUT after any earlier mode PUT, and `adoptLoad` already refuses to clobber an in-flight optimistic value), or only clear `loadFailed` when both fields are confirmed.
   **Reproducer:** none — demonstrating it requires adding a `SettingsStateTest` case (failed load → successful `chooseUnit` whose answer carries `SessionView.QUICK` → assert `state.sessionView`), and this review is read-only; no existing test covers that sequence.

2. **`chooseSessionView` reads `unitWrite.confirmed` inside `sendMutex`, but `chooseUnit` updates `confirmed` outside it.** Files: [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:293) and [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:335). The KDoc at `SettingsState.kt:320-326` claims the unit is read "once every earlier write has settled," but the earlier write settles at `SettingsState.kt:295` (`unitWrite.onSuccess`), after `withLock` has returned and released the mutex. A queued `chooseSessionView` can therefore acquire the mutex and read the pre-write `confirmed` unit, then PUT that stale unit with the new mode — overwriting the unit the earlier tap just set on the server. This is masked on a single-threaded (Main/test) dispatcher, where the releaser runs `onSuccess` before the waiter resumes, which is why the added ordering test passes; `SettingsState` itself documents no dispatcher confinement, and its callers launch per-tap coroutines. Fix: move each write's `onSuccess`/`confirmed` update inside the `sendMutex` block (record the token outcome under the lock), or have `chooseSessionView` read the unit from a value updated under the same lock.
   **Reproducer:** none — the interleaving needs a multi-threaded dispatcher with the waiter winning the race between mutex release and `onSuccess`; it is not deterministically reproducible with the existing single-threaded test harness.

#### Minor (Nice to Have)

1. **No test gates `load()`'s `sessionView` adoption the way the unit's is gated.** File: [SettingsStateTest.kt](/work/shared/src/commonTest/kotlin/com/gymie/shared/ui/SettingsStateTest.kt:657). The five new tests cover show/write/revert/omit-wire/queued-unit/failed-load, but none holds a mode write succeeding while a load is in flight to prove the stale load does not adopt over it (the unit has exactly such tests). The code at `SettingsState.kt:265, 272` looks correct; the gap is only coverage.
   **Reproducer:** none — this is a coverage gap, not an observed failure.

### Recommendations

- Adopt the whole `WorkoutPreferences` answer in both write paths (each field through its own `OptimisticWrite`), rather than one field per path; the response is already the server's full truth and `sendMutex` ordering makes cross-field adoption safe. That closes Important issue 1 and removes the deliberate asymmetry noted in `chooseSessionView`'s KDoc.
- Consider moving all `OptimisticWrite` outcome bookkeeping inside `sendMutex` so "settled" means one thing for both the wire order and the confirmed values.

### Assessment

**Ready for the human gate?** With fixes
**Reasoning:** The feature wiring, wire format, and backward compatibility are sound and well tested, but the reused `loadFailed` flag and the out-of-lock `confirmed` update each let the new mode field display or send a stale unit/mode under realistic sequences, and both should be fixed before this lands.