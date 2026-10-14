### Failure-recovery check

**State set on failure: `loadFailed`**, which shows the retry affordance.
- Set at `SettingsState.kt:275` (load failed), `:310` (`chooseUnit`, NOTHING_CONFIRMED) and `:351` (`chooseSessionView`, NOTHING_CONFIRMED).
- Cleared at `:270` (load success), `:295` (unit write success) and `:339` (view write success).
- A failed load leaves two values unconfirmed: `unitWrite.confirmed` and `viewWrite.confirmed`. Both are null, and `viewWrite.value` shows the FOCUS default.

| Clearing line | Unit re-confirmed by | View re-confirmed by |
|---|---|---|
| `:270` | `:271` (`unitWrite.adoptLoad`) | `:272` (`viewWrite.adoptLoad`). The clear only runs when `noWriteSucceeded`, and each adopt is gated on its own field's count. |
| `:295` (`chooseUnit` success) | `unitWrite.onSuccess` | **none** |
| `:339` (`chooseSessionView` success) | A prior unit confirmation is required before the send (`unitWrite.confirmed ?: error(...)`) | `viewWrite.onSuccess` |

The `:295` row is a partial recovery. It is reported below.

### Summary
The diff adds a second optimistic field, `sessionView`, to the preferences write path. It touches network PUT and GET calls, a shared `sendMutex`, and a `loadFailed` flag now shared by two fields. The error-return lens finds one real gap. Concurrent re-entry is handled well: the mutex serialises sends, and the unit is read inside the lock. I did not check timeouts. The HTTP client's timeout configuration is outside this diff, and the diff adds no new unbounded wait. No partial-write issue applies, because each call is a single PUT.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
1. **`SettingsState.kt:295` (`chooseUnit` success clears `loadFailed`). Lens: error return, partial recovery.**
   - **Scenario:** The initial `load()` fails, so unit and view are both unconfirmed and `loadFailed` is true. The user taps a unit, and that PUT succeeds. Line 295 clears `loadFailed`, and the retry affordance disappears. `viewWrite.confirmed` is still null, and the UI still shows the invented default Focus. The PUT response carried the real `sessionView`, but the code discards it.
   - **Consequence:** If the user then taps a mode and that PUT fails, `onFailure` returns NOTHING_CONFIRMED. The control stays on the optimistic choice, with no revert to the stored mode. The screen claims it is in sync while the displayed mode was never confirmed. If the stored mode was QUICK, Settings shows Focus as though confirmed.
   - **Fix:** In `chooseUnit`'s `onSuccess`, also adopt `it.sessionView` into `viewWrite`. One option is `adoptLoad`, guarded by "no view write in flight or succeeded". Alternatively, only clear `loadFailed` when `viewWrite.confirmed != null`.
   - **Reproducer:** none — no run was done. A unit test in `SettingsStateTest` would show it. Set `api.preferencesResult = failure`, call `load()`, then set `preferencesResult` to `WorkoutPreferences(KG, QUICK)` and call `chooseUnit(LBS)`. Assert `state.loadFailed == false` and `state.sessionView == QUICK`. The second assertion fails, because the view stays FOCUS.

#### Minor (Nice to Have)
- **`SettingsState.kt:262-275`. Lens: error return.** A failed load reports "Could not load your settings." This replaces the `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` fallback. The new string is an inline literal, so it is not shared with the other messages. This is cosmetic.

### Assessment
**Ready for the human gate?** With fixes
**Reasoning:** The write serialisation and the "no confirmed unit, no send" guard are sound. The one real gap is that `loadFailed` is cleared on a unit-write success while the view value stays unconfirmed. This is a recoverable wrong state, not data loss, so it is Important rather than Critical.