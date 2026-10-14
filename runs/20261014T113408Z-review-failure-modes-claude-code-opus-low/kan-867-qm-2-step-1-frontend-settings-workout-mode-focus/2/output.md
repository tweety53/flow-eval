### Failure-recovery check

**Failure state: `loadFailed = true`**
- Set at: `SettingsState.kt:275` (load failure), `:310` (unit write with nothing confirmed), `:351` (mode write with nothing confirmed, including the new refusal at `:335` when no unit is confirmed).
- Values a failed load leaves unconfirmed: `unitWrite.confirmed` (null) and `viewWrite.confirmed` (null). Meanwhile `sessionView` shows the built-in FOCUS default, not a value the server sent.

**Lines that clear it:**
- `SettingsState.kt:270` (load success). This only runs when the load is the newest one and no write of either field succeeded during it.
  - Unit is re-confirmed at `:271`.
  - Mode is re-confirmed at `:272`.
  - Nothing is left unconfirmed, so this recovery is complete.
- `SettingsState.kt:295` (unit write success).
  - Unit is re-confirmed at `:103`, through `unitWrite.onSuccess`.
  - Mode is re-confirmed by **none**. `viewWrite.confirmed` stays null and the screen keeps showing the default FOCUS. The PUT's answer (`WorkoutPreferences.sessionView`) contains the server's stored mode, and the code throws it away. **This is a partial recovery: Important #1.**
- `SettingsState.kt:339` (mode write success). This can only happen when `unitWrite.confirmed` is not null, which the guard at `:335` enforces.
  - Mode is re-confirmed at `:103`, through `viewWrite.onSuccess`.
  - Unit was already confirmed before this write started, because the guard requires it.
  - Nothing is left unconfirmed, so this recovery is complete.

### Summary
The diff adds a second field to the preferences payload:
- a new `chooseSessionView` write, sent inside the shared `sendMutex`;
- load gating split per field;
- `loadFailed` shared between both controls.

Three lenses applied:
- **Error return:** shared failure state, and the refusal when no unit is confirmed.
- **Concurrent re-entry:** a mode write queued behind unit writes. The mutex handles this correctly, and reading `unitWrite.confirmed` inside the lock avoids sending a unit that is still waiting in the queue.
- **Timeout:** this goes through the existing `requests.apiCall` and `client` stack. The diff doesn't change how long a call can wait, so I have no new finding here.

Partial write doesn't apply: it's a single PUT.

There's one real gap: recovering through a unit write leaves the mode unconfirmed while the retry affordance has disappeared.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295` (error return / partial recovery)**
- **Scenario:**
  1. `load()` fails, so `loadFailed` is true, both fields are unconfirmed, and the mode shows FOCUS by default.
  2. The server actually holds QUICK.
  3. The user picks LBS and the PUT succeeds. Its response is `{displayUnit: LBS, sessionView: QUICK}`.
  4. Line 295 clears `loadFailed`, which hides the retry affordance.
  5. `viewWrite` is never told about the response, so the Workout mode row confidently shows **Focus** while the server holds Quick. Nothing remains on screen that would prompt a re-sync.
- **Fix sketch:** in `chooseUnit`'s `onSuccess`, also adopt `it.sessionView` into `viewWrite`, but only when `viewWrite.confirmed == null` and no mode write is in flight. Another option is to clear `loadFailed` only once both fields have a non-null `confirmed`.
- **Reproducer:** none — the diff adds no test for it, and I didn't add one because this review is read-only. The test would:
  - set `api.preferencesResult` to a failure, then call `state.load()`;
  - set `api.updatePreferencesResult = Result.success(WorkoutPreferences(LBS, QUICK))`, then call `state.chooseUnit(LBS)`;
  - assert `!state.loadFailed && state.sessionView == FOCUS`. Against the diff, this assertion holds, which demonstrates the bug.

#### Minor (Nice to Have)

**2. `SettingsState.kt:276` (error return)**
- The fallback load-failure message became the generic "Could not load your settings." and `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` was dropped.
- This is acceptable, but the message no longer says which field failed. A shared constant in `FailureMessages` would keep it consistent with the other failure messages.
- **Reproducer:** none — wording only.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The mutex, the refusal when no unit is confirmed, and the per-field load gating all handle failure correctly. The one flaw is that a successful unit write clears the shared `loadFailed` flag even though the mode is still unconfirmed. The screen then shows a guessed mode with no way to retry, even though the server's own response contained the real value.