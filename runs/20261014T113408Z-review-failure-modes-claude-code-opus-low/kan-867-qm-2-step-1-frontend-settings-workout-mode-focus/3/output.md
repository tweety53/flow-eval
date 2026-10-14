### Failure-recovery check

**The failure state:** `loadFailed`. It is set in three places:
- `SettingsState.kt:275` — a load fails.
- `SettingsState.kt:310` — a unit write fails with nothing confirmed.
- `SettingsState.kt:351` — a mode write fails with nothing confirmed.

**What a failed load leaves unconfirmed:** both `unitWrite.confirmed` and `viewWrite.confirmed` stay `null`. While that is true, the mode control shows the made-up default, `FOCUS`.

**Where it is cleared, and whether each value gets re-confirmed:**
- **`SettingsState.kt:270`** (a load succeeds, gated on no write of either field having succeeded since it began):
  - unit: re-confirmed at `:271`.
  - mode: re-confirmed at `:272`.
  - Full recovery.
- **`SettingsState.kt:295`** (`chooseUnit` succeeds):
  - unit: re-confirmed at `OptimisticWrite.onSuccess` (`:103`).
  - mode: **none**. The PUT's answer is a full `WorkoutPreferences` that includes `sessionView`, but `chooseUnit` only reads `it.displayUnit`, so the mode is thrown away. This is a partial recovery (Important #1).
- **`SettingsState.kt:339`** (`chooseSessionView` succeeds):
  - mode: re-confirmed at `:103` through `viewWrite.onSuccess`.
  - unit: it was already confirmed before this write could go out, because `:335` refuses to send without it. Full recovery.

**A related gap:** the load-failure gate at `:274` uses `noWriteSucceeded`. If a unit write succeeds while a load is in flight and the load then fails, `loadFailed` is never raised, but the mode is still unconfirmed. This is the same defect (Important #1).

### Summary
**What changed:**
- `updatePreferences` now carries an optional `sessionView`.
- There is a new `chooseSessionView` that sends the last confirmed unit, serialised under `sendMutex`.
- `load()` now adopts the mode as well as the unit.
- There is a new Workout mode row in Settings.

**How the four lenses came out:**
- **Error return:** this is where the defect is, in recovery.
- **Concurrent re-entry:** handled well. The unit is read inside the mutex, so a unit choice still queued never leaks out through a mode write, and the mode field is left off unit writes.
- **Timeout:** nothing new. The call uses the existing `requests.apiCall` / HTTP client path.
- **Partial write:** both fields go in a single PUT, so there is no torn write.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
**1. `SettingsState.kt:295` (also `:274`) — error return / partial recovery**

*Scenario:*
1. The server stores `QUICK`.
2. `load()` fails: `loadFailed = true` and the retry is offered.
3. The user taps a unit, and `chooseUnit` succeeds.
4. `:295` clears `loadFailed`, so the retry disappears.
5. The mode row now shows `FOCUS` as if confirmed, while `viewWrite.confirmed` is still `null`.
6. The PUT's answer included `"sessionView":"QUICK"`, and the code discarded it.

The same thing happens when a unit write succeeds while a load is in flight and the load then fails. `:274` stays silent, because `noWriteSucceeded` is false.

*Effect:* the screen claims it has recovered while showing a mode the server never sent. If the user taps the Focus segment that is already shown as selected, the control looks unchanged, but no recovery happens.

*Fix sketch:*
- In `chooseUnit`'s success, also feed the answer's `sessionView` to `viewWrite`, using an adopt that runs only while no view write is in flight and none has succeeded. That is the same gate `load()` uses.
- Alternatively, clear `loadFailed` at `:295` only when `viewWrite.confirmed != null`, and gate `:274` per field instead of on `noWriteSucceeded`.

*Reproducer:* `none — needs a new commonTest:` use `preferencesResult = failure`, then `load()`, then set `preferencesResult = success(KG, QUICK)`, then `chooseUnit(LBS)`. Then check `assertFalse(state.loadFailed)` together with `state.sessionView == FOCUS`, while the server holds `QUICK`.

#### Minor (Nice to Have)
**2. `SettingsState.kt:335` — error return**

When the unit isn't confirmed, the refusal message `"Could not save your workout mode."` doesn't say why: the settings never loaded. The mode stays shown as `QUICK` even though nothing was sent. That is consistent with the `NOTHING_CONFIRMED` design, but a user can't tell from the message that a retry or reload is what's needed.

*Reproducer:* `none — UX wording`

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** Serialising the sends and omitting the unset mode from the wire make the writes safe against concurrent re-entry and torn writes. The one real gap is recovery: a successful unit write clears the failure state while the mode is still unconfirmed, and it throws away the mode the server sent back.