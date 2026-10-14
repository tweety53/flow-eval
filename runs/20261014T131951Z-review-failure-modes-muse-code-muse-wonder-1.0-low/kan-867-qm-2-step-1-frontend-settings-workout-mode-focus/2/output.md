### Failure-recovery check

Failure states set by the changed code, every setter/clearer in the whole file, and what the failure left unconfirmed:

**1. `loadFailed = true` (shared flag for both preference fields)**
- Set on failure at: [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:275) (`load` failure), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:351) (`chooseSessionView` → `NOTHING_CONFIRMED`), and pre-existing [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:310) (`chooseUnit` → `NOTHING_CONFIRMED`).
- Cleared at: [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:270) (load success), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) (`chooseUnit` success), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339) (`chooseSessionView` success).
- Unconfirmed values per failure:
  - `load` failure (line 275): current server `displayUnit` **and** current server `sessionView` — neither read succeeded.
  - `chooseSessionView` `NOTHING_CONFIRMED` (line 351): `sessionView` — `viewWrite.confirmed` is still null and `viewWrite.value` is left holding the optimistic choice ([SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:115)).
- Re-confirmation per clearing line, per unconfirmed value:
  - Line 270 (load success): `displayUnit` re-confirmed at [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:271), `sessionView` at [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:272). Both present — but only because the clear is gated on `noWriteSucceeded` (both fields), so this line is sound.
  - Line 295 (`chooseUnit` success): `displayUnit` re-confirmed at line 295 itself (`it.displayUnit`); `sessionView` — **none**. The success's own response is a `WorkoutPreferences` and *does* carry `sessionView` (see `FakeWorkoutApi` / the PUT echo), but `chooseUnit` passes only `it.displayUnit` to `unitWrite.onSuccess` and discards `it.sessionView`. → partial recovery; this is the issue below.
  - Line 339 (`chooseSessionView` success): `sessionView` re-confirmed at line 339 itself (`it.sessionView`); `displayUnit` — **none**, by explicit design ("The answer's unit is not adopted", [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:320)). The response carries `displayUnit` and the code discards it. In practice this clear can only fire when `unitWrite.confirmed != null` (line 335 refuses otherwise), so the unit is stale-confirmed rather than never-confirmed; noted, not raised separately.

**2. `viewWrite.value` left holding an optimistic, unconfirmed choice**
- Set optimistically at [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:78) (`begin`); on failure reverted to confirmed at [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:119), or left in place when nothing is confirmed (lines 115–124, `NOTHING_CONFIRMED`).
- Overwritten/cleared at: line 105 (`onSuccess`, only when token current), line 119 (revert), line 136 (`adoptLoad`, only when not saving). No other line in the file touches it. Sound on its own; it only becomes a false claim of recovery when combined with the line-295 clear above, which removes the Retry affordance (the only re-sync path, [AppScreens.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/AppScreens.kt:326)) while this value is still unconfirmed.

**3. Failure notification (`notifications.failure(...)`)**
- Set at lines 276, 347, 352 for the new paths. `SettingsState` contains no line that clears `NotificationState.current`; clearing lives in `NotificationState` itself (dismiss / auto-expire). Pre-existing pattern, unchanged by this diff — not raised.

Verified non-issues from the other lenses, so they are not repeated below: timeouts are bounded by the existing client config (`HttpTimeout` 15s request/socket, 10s connect, [GymieApiClient.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/api/GymieApiClient.kt:255)); the new write is a single PUT, so there is no multi-write partial-write surface on the client; local concurrent re-entry is serialised by the shared `sendMutex` (line 249) and ordered by the per-field `Sequencer` tokens, including the queued-unit case covered by the new test.

### Summary

The diff adds a second preference field (`SessionView`) end-to-end: a nullable field on the preferences PUT, a Settings "Workout mode" row, and a second `OptimisticWrite` (`viewWrite`) in `SettingsState` mirroring the existing unit write, with `load()` now adopting both fields per-field. Error-return and concurrent re-entry lenses bore on the shared `loadFailed` flag and the cross-field PUT payload; timeout (bounded, unchanged) and client-side partial write (single PUT) did not produce findings. Overall verdict: one Important partial-recovery defect where a unit write's success silently retires the failure state of an unconfirmed workout-mode value, plus one Important cross-device clobber inherent in the new write having to re-send the unit.

### Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

1. **File:** [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) — **error return (partial recovery)**
   **Scenario:** Load fails (offline) → user taps "Quick" → `chooseSessionView` finds no confirmed unit, sends nothing, ends in `NOTHING_CONFIRMED`: `sessionView` shows the optimistic QUICK, `viewWrite.confirmed` is still null, `loadFailed = true` and the Retry button is offered. The user then taps a weight unit and that PUT succeeds → line 295 clears `loadFailed`. Retry disappears and the screen claims recovery, but the displayed QUICK was never confirmed — the server may still hold FOCUS, and nothing will ever re-read it until the next manual load. The PUT response already contained the true `sessionView`; `chooseUnit` discarded it.
   **Fix sketch:** Either adopt `it.sessionView` into `viewWrite` on `chooseUnit` success (guarded like `adoptLoad`, i.e. only when no view write is in flight / has succeeded since), or split the recovery flag per field so a unit success cannot clear a view failure. Adopting the echoed value is smaller and uses data already on the wire.
   **Reproducer:** none — demonstrating this needs a new `SettingsStateTest` case (failed load → failed `chooseSessionView` → successful `chooseUnit` → assert `loadFailed == false` while `sessionView` is unconfirmed); this is a read-only review and no such test exists in the diff.

2. **File:** [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:336) — **concurrent re-entry (cross-process)**
   **Scenario:** Because the backend requires `displayUnit` on every PUT, a workout-mode tap re-sends the last locally confirmed unit. The `sendMutex` only orders writes from this one client. If the unit was changed on another device (or a second client) after this client's load, tapping Focus/Quick silently writes the stale unit back, reverting the other device's change — last-writer-wins with no version, ETag, or conflict signal, and undetectable locally because the response's unit is deliberately not adopted (line 320 doc), so the client never even sees the value it overwrote.
   **Fix sketch:** Prefer a server-side partial update (make `displayUnit` optional on the PUT, as `sessionView` already is) so a mode write touches only the mode; failing that, re-read preferences inside the mutex before composing the PUT, or adopt-and-compare the echoed `displayUnit` and surface a notification when it differs from what was sent.
   **Reproducer:** none — requires two clients against a real backend; no multi-client harness exists in the repo's tests.

#### Minor (Nice to Have)

None. (The `enabled = true` hardcoding and notification-clearing behaviour are unchanged pre-existing patterns shared with the unit row, not introduced by this diff.)

### Assessment

**Ready for the human gate?** With fixes

**Reasoning:** The core new write is well-guarded locally — mutex-serialised, token-sequenced, refuses to guess a unit after a failed load, and reverts on refusal. But the shared `loadFailed` flag now spans two fields while each write's success clears it after confirming only its own field, and the data needed to confirm the other field is already in the response and thrown away (Issue 1). Combined with the stale-unit re-send clobbering concurrent changes from other clients (Issue 2), the failure paths can present an unconfirmed state as recovered. Both fixes are small and localised.