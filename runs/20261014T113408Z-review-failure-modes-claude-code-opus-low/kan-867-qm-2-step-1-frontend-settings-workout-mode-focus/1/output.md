### Failure-recovery check

**Failure state: `loadFailed`** (the shared flag that shows the in-place retry link)
- **Set at:** `SettingsState.kt:275` (load fails), `SettingsState.kt:310` (`chooseUnit` fails with no confirmed unit), `SettingsState.kt:351` (`chooseSessionView` fails with no confirmed view, which includes the refusal when no unit is confirmed).
- **Values a failed load leaves unconfirmed:** `unitWrite.confirmed` (null) and `viewWrite.confirmed` (null).
- **Cleared at:**
  - `SettingsState.kt:270`, load success. This only runs when no write of either field has succeeded since the load began.
    - Unit is re-confirmed at `:271` (`unitWrite.adoptLoad`).
    - View is re-confirmed at `:272` (`viewWrite.adoptLoad`).
    - Both are covered, so this is a full recovery.
  - `SettingsState.kt:295`, `chooseUnit` success.
    - Unit is re-confirmed at `:295` (`unitWrite.onSuccess`).
    - View: **none**. The PUT response includes `sessionView`, but the code throws it away. This is a partial recovery, reported below.
  - `SettingsState.kt:339`, `chooseSessionView` success.
    - View is re-confirmed at `:339` (`viewWrite.onSuccess`).
    - Unit was already confirmed before the send, because `:335` refuses to send without `unitWrite.confirmed`. This is a full recovery. The response's `displayUnit` is discarded on purpose, and that is safe.

### Summary
The diff adds a `sessionView` preference: a new field in the API and request, an optimistic `chooseSessionView` write that goes through the shared `sendMutex`, `load()` now adopting the view, and a new Settings row.

Lenses that applied:
- **Error return:** applies to the new write and to the recovery path for `loadFailed`.
- **Concurrent re-entry:** handled. Sends are serialised by the mutex, and the unit is read inside the lock.
- **Timeout:** no new boundary beyond the existing Ktor call path, so nothing new to check.
- **Partial write:** does not apply. Each action is a single PUT.

The design is careful overall. The one real gap is that the new second field was not added to the existing unit-write recovery path.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
**`SettingsState.kt:295`: a successful unit write clears `loadFailed` while the workout mode is still unconfirmed (error-return lens, partial recovery).**
- **Scenario:**
  1. The server holds `QUICK`. `load()` fails, so `loadFailed` is true and the retry link shows. `hasLoaded` is true, so the mode row draws its default, `FOCUS`.
  2. The user picks a unit. The PUT succeeds and its response says `sessionView: QUICK`.
  3. Line 295 clears `loadFailed`, and the retry link disappears.
  4. The mode control keeps showing `FOCUS`, which the server never confirmed, and `viewWrite.confirmed` stays null. Nothing on screen shows that the mode is unknown.
- **The value was available:** the success response carried it, and the code discarded it.
- **Fix sketch:** in the `chooseUnit` success branch, call `viewWrite.adoptLoad(it.sessionView)` when `viewWrite.confirmed == null` (or more generally when no view write has succeeded since this send began). Or clear `loadFailed` only once `viewWrite.confirmed != null`.
- **Reproducer:** none. I can't make changes in this review, so I couldn't add the test. A test would: set `api.preferencesResult` to a failure and call `load()`; set `preferencesResult` to `WorkoutPreferences(KG, QUICK)`; call `chooseUnit(LBS)`; then assert that `!state.loadFailed` and `state.sessionView == FOCUS` both hold, which shows the bug.

#### Minor (Nice to Have)
**`AppScreens.kt:248`: after a failed load, the mode row shows an unconfirmed default (error-return lens).** The KDoc says the row never shows a default Focus, but after a failed load `hasLoaded` is true and the row renders `FOCUS` (`viewWrite` starts at `SessionView.FOCUS`). The unit row already behaves the same way, and the retry link stays visible, so the impact is low.
- **Reproducer:** none — this is a rendering claim that only a UI test would check.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** Serialisation and the refusal to send a guessed unit are both handled correctly. But one success path, the unit write at line 295, declares recovery while the new field is still unknown, and that hides the retry the user would need.