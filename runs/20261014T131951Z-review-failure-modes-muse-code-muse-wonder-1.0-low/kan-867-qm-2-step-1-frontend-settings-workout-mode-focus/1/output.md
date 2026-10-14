### Failure-recovery check

Failure states set by the changed code in [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt):

- **`loadFailed = true`** — set at [SettingsState.kt:275](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:275) (load failure), [SettingsState.kt:310](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:310) (pre-existing, unit write with nothing confirmed), and the new [SettingsState.kt:351](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:351) (session-view write with nothing confirmed). Values left unconfirmed by a failure: `displayUnit` (load failure at 275; unit-write failure at 310) and `sessionView` (load failure at 275; mode-write failure at 351, which also leaves `viewWrite.value` sitting on the optimistic choice via the `NOTHING_CONFIRMED` branch).
- **Transient failure notification** — set at lines 276, 347, 352; it is single-slot UI copy in `NotificationState`, overwritten by the next notification, and carries no confirmed-value semantics, so it is excluded from the re-confirmation trace below.

Every line in the file that clears `loadFailed` (whole-file grep, unchanged functions included): [SettingsState.kt:270](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:270), [SettingsState.kt:295](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295), [SettingsState.kt:339](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339). Per clearing line, per unconfirmed value, the line that re-confirms it:

| Clearing line | `displayUnit` re-confirmed at | `sessionView` re-confirmed at |
|---|---|---|
| 270 (load success) | 271 (`adoptLoad(it.displayUnit)`) | 272 (`adoptLoad(it.sessionView)`) |
| 295 (`chooseUnit` success) | 295 (`unitWrite.onSuccess(token, it.displayUnit)`) | **none** |
| 339 (`chooseSessionView` success) | none on this line | 339 (`viewWrite.onSuccess(token, it.sessionView)`) |

Line 270 is sound: the flag only clears when `noWriteSucceeded` holds for both fields, in which case both `adoptLoad` calls run. Line 339's missing unit re-confirmation is not reachable as a stale recovery: [SettingsState.kt:335](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:335) refuses to send at all while `unitWrite.confirmed` is null, so any successful mode write is preceded by a confirmed unit. Line 295 has no such guard — it is the issue below. In both write paths the success response is a full `WorkoutPreferences` carrying the other field, and the code reads only its own field (`it.displayUnit` at 295, `it.sessionView` at 339), discarding the other.

### Summary

The diff adds an account-level `sessionView` preference end to end: a new `SessionView` enum and fields on `WorkoutPreferences`/`UpdateWorkoutPreferencesRequest`, an optional `sessionView` parameter threaded through `WorkoutCatalogApi.updatePreferences` and its implementation, a second `OptimisticWrite` (`viewWrite`) in `SettingsState` sharing the existing `sendMutex` and the single `loadFailed` flag, and a "Workout mode" row in Settings. Error-return and concurrent re-entry bore on the new write path (single shared retry flag, mutex-serialised sends, token-based staleness); partial write does not (one PUT carries both fields atomically); timeout does not introduce a new gap (the shared Ktor client already installs `HttpTimeout`, confirmed in `GymieApiClient`). One partial-recovery defect found; overall the new write path is otherwise careful — notably the refusal to send an unconfirmed unit at line 335 prevents a worse data-corruption failure.

### Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

- **File:line:** [SettingsState.kt:295](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) — **error return (partial recovery).**
  **Scenario:** Initial `load()` fails (offline): both fields unconfirmed, `loadFailed = true`, Retry shown. The user taps "Quick" — `chooseSessionView` correctly refuses to send (no confirmed unit, line 335) but leaves the control showing the optimistic QUICK with `sessionView` still unconfirmed (line 351). The user then taps a weight unit and that write succeeds: line 295 adopts the returned `displayUnit` and clears the shared `loadFailed`, so the Retry affordance disappears — yet `sessionView` was never re-confirmed. The screen now presents QUICK as the settled workout mode while the server still holds FOCUS (or whatever was stored), with no visible signal that anything is unconfirmed; it only self-heals on the next full `load()`. The success response at line 294's call *did* carry the value — `WorkoutPreferences.sessionView` — and the code discarded it, reading only `it.displayUnit`.
  **Fix sketch:** Either adopt the other field from the write response (`viewWrite.adoptLoad(it.sessionView)` alongside the unit adoption, mirroring lines 271–272), or make `loadFailed` per-field so a unit success cannot clear a failure state the session view still owns. The first is smaller and matches how `load()` already treats the same payload.
  **Reproducer:** none — no existing test drives the load-failure → refused mode-write → successful unit-write sequence; demonstrating it requires adding that case to `SettingsStateTest` (load failure, `chooseSessionView(QUICK)`, then successful `chooseUnit(LBS)`, asserting `loadFailed` remains true or `sessionView` equals the server's stored value), and this review is read-only.

#### Minor (Nice to Have)

None. (Checked and cleared: the new PUT reuses the client's existing `HttpTimeout` and 401 refresh-retry, and a preferences PUT is idempotent, so the retry cannot double-apply; `sendMutex` plus per-field tokens make concurrent unit/mode taps serialise and supersede correctly; cancellation mid-write rethrows via `onFailureNotCancelled` and `viewWrite.end()` still runs in the `finally`.)

### Assessment

**Ready for the human gate?** With fixes
**Reasoning:** The one Important finding is a genuine partial recovery the required check exists to catch: a shared `loadFailed` flag is cleared by a unit-write success while `sessionView` remains unconfirmed, and the response that could have re-confirmed it is in hand and discarded. It is recoverable (next load re-syncs) rather than corrupting — the diff's guard at line 335 correctly prevents the dangerous direction — so it does not block on Critical grounds, but the flag should not claim recovery for a field it never confirmed.