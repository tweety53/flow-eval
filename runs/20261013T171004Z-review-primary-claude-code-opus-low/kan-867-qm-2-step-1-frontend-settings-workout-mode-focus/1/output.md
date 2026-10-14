### Shared-state check

The diff reads or writes these existing pieces of state: `loadFailed`, `unitWrite.confirmed`, `unitWrite.succeededWriteCount`, `sendMutex`, `hasLoaded`, `loadSeq`, and the fake's `preferencesResult`.

- **`loadFailed`** — this now covers two fields: unit and mode.
  - `SettingsState.kt:270` (load success): still correct. It only clears when no write of either field has succeeded during the load.
  - `SettingsState.kt:275` (load failure): still correct.
  - **`SettingsState.kt:295` (`chooseUnit` success): not correct any more.** It clears `loadFailed` because the unit is now confirmed, but it doesn't check that `viewWrite.confirmed` is set. This is Important issue 1 below.
  - `SettingsState.kt:310` (`chooseUnit` with nothing confirmed): still correct.
  - `SettingsState.kt:339` (mode write success): still correct. A mode write can only be sent once the unit is confirmed, so both fields are confirmed when it clears the flag.
  - `SettingsState.kt:351` (mode write with nothing confirmed): still correct.
- **`unitWrite.confirmed`** — written only at `SettingsState.kt:103` (`onSuccess`) and `SettingsState.kt:135` (`adoptLoad`). The new code only reads it, inside `sendMutex`. Still correct.
- **`unitWrite.succeededWriteCount`** — written only at `SettingsState.kt:102`. The new code reads it the same way it reads the mode's count. Still correct.
- **`sendMutex`** — now shared by both kinds of write, so sends stay in order. Still correct.
- **`hasLoaded`** and **`loadSeq`** — the diff doesn't change who writes them. Still correct.
- **`FakeWorkoutApi.preferencesResult`** — written by `updatePreferences` when it returns. The fake now reads it before the hook runs and uses it to fill in a missing `sessionView`. Still correct.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. `SettingsState.kt:295`: after a failed load, a successful unit write clears `loadFailed` even though the mode was never loaded.**
- **What's wrong:** Say `load()` fails, and the server actually stores `QUICK`. The user picks a unit and that write succeeds, so `loadFailed` becomes false and the retry button disappears. But `viewWrite.confirmed` is still null, and the Workout mode control keeps showing the default `FOCUS`, which is wrong.
- **Why it matters:** The user is left with no retry and a screen showing a value the server doesn't hold. The PUT's answer did include the real `sessionView` (`WorkoutApi` returns the whole preferences object), but `chooseUnit` throws it away.
- **How to fix:** Pick one:
  - In `chooseUnit`'s success branch, also adopt the mode from the answer when `viewWrite.confirmed == null` and no mode write is in flight.
  - Or only clear `loadFailed` there when `viewWrite.confirmed != null`.

  The mode write has the same gap in the other direction: it never adopts `it.displayUnit`. That's harmless today, because the unit has to be confirmed before a mode write is sent.
- **Reproducer:** none — there's no test for this sequence. To show it: set `preferencesResult` to a failure, call `load()`, set `preferencesResult` to `success(KG, QUICK)`, call `chooseUnit(LBS)`, then assert `!loadFailed && sessionView == FOCUS`. This holds today, which is the bug.

**2. `SettingsState.kt:328-351`: a mode write that was refused without being sent still leaves the control showing the new choice.**
- **What's wrong:** After a failed load, `chooseSessionView(QUICK)` never sends a PUT. It ends up in `NOTHING_CONFIRMED`, which leaves `value = QUICK` on screen.
- **Why it matters:** The unit rule this copies ("keep the optimistic choice") was written for a PUT that really was sent and whose result is unknown. Here we know nothing was sent, so showing `QUICK` is wrong. The new test only checks `loadFailed` and the notification, not what the control shows.
- **How to fix:** Bail out before calling `begin()` when `unitWrite.confirmed == null`. The catch is that the check has to happen inside the mutex, so the alternative is to put the control back to its previous value in this case.
- **Reproducer:** none — no test covers it. Add `assertEquals(SessionView.FOCUS, state.sessionView)` to `a workout mode write after a failed load does not overwrite the stored unit`; that assertion fails today.

#### Minor (Nice to Have)

- **`SettingsState.kt:276`, `:345`, `:350`, `:336` — hard-coded messages.** `"Could not load your settings."` and `"Could not save your workout mode."` are written out inline. The diff also drops the `FailureMessages` import, while `COULD_NOT_LOAD_WEIGHT_UNIT` is still used in `WorkoutEditorScreen.kt:387`. Add new constants to `FailureMessages` instead.
- **`SettingsState.kt:336` — the refusal message is used twice.** `error("Could not save your workout mode.")` produces the same text as the fallback message. It works, but it's an exception standing in for a control-flow signal.
- **Public API change — `WorkoutApi.updatePreferences`.** The new parameter defaults to `null`, so existing callers still compile. However, the `= null` default isn't repeated on the overrides in `WorkoutApiImpl` or `FakeWorkoutApi`. That's fine in Kotlin, which takes the default from the interface. Separately, `WorkoutPreferences.sessionView` defaulting to `FOCUS` handles an older server that doesn't send the field. No migration is needed on the client side.
- **`SettingsScreenUiTest`.** `theWorkoutModeControlKeepsOneLineAtPhoneWidth` has no `@Feature` tag, unlike its neighbouring tests.

### Recommendations
- Make "is everything loaded?" depend on each field's own confirmed state, e.g. `loadFailed = unitWrite.confirmed == null || viewWrite.confirmed == null`, instead of a shared flag that each writer sets for its own case only.
- Have both write paths adopt the field they don't own from the PUT's answer, but only while that field has no confirmed value and no write in flight.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The model change, API change and UI wiring are sound, and the mode write correctly sends the confirmed unit from inside the mutex. But now that `loadFailed` covers two fields, the unit write's existing line that clears it is wrong: after a failed load, the retry disappears while the mode control shows an unconfirmed `FOCUS`.