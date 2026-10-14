### Shared-state check

The diff's new code reads or writes these existing pieces of state: `loadFailed`, `hasLoaded`, `sendMutex`, `unitWrite.confirmed` and `unitWrite.succeededWriteCount`. `loadFailed` now stands for two fields (unit and mode), not one.

**`loadFailed`** (written in `SettingsState.kt`):
- `SettingsState.kt:270`: load succeeded and is current, and no write of either field has succeeded → clear. **Still correct.**
- `SettingsState.kt:275`: load failed under the same gate → set. **Still correct.**
- `SettingsState.kt:295` (`chooseUnit` success, line not changed by the diff): clears the flag once the **unit** is confirmed. Before this diff that meant every field was confirmed. Now the mode can still be unconfirmed, because `viewWrite.confirmed` is only set by a load or a mode write. **This is a defect** — Important #1.
- `SettingsState.kt:310`: unit write failed with nothing confirmed → set. **Still correct.** Setting the flag is safe for any field.
- `SettingsState.kt:339` (new): mode write success → clear. **Still correct.** A mode write can only be sent once `unitWrite.confirmed` is set, so both fields are confirmed at this point.
- `SettingsState.kt:351` (new): set. **Still correct**, though this branch can only be reached if the mode write ran with a confirmed unit and the mode itself was never confirmed. Error-only.

**`hasLoaded`**: written only at `SettingsState.kt:281`, in `finally`, so it becomes true even when the load **fails**. The new row is drawn behind this gate (`AppScreens.kt:221`). The KDoc relies on the gate to mean "the first frame is the skeleton, never a default Focus", which it does not guarantee. This is Important #2.

**`sendMutex`**: no writes; it's only locked at `:293` and `:334`. **Still correct.** Mode and unit sends are serialised together.

**`unitWrite.confirmed`**: written at `SettingsState.kt:103` (`onSuccess`) and `:135` (`adoptLoad`). Both are true server values. **Still correct.** It is read inside the mutex at `:335`, as the comment says.

**`succeededWriteCount`**: written at `:102` only, and each field keeps its own count. **Still correct.** The load gating now checks the two counts independently.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. `SettingsState.kt:295`: after a failed load, a successful unit write clears `loadFailed` while the mode is still unconfirmed.**
- **What happens:** the load fails, so `loadFailed` is true and `sessionView` shows the default FOCUS. The user then picks a unit, and that write succeeds. The line `if (unitWrite.onSuccess(...)) loadFailed = false` hides the retry. The Workout mode row keeps showing FOCUS as if it were confirmed, even when the server holds QUICK.
- **Why it matters:** `viewWrite.confirmed` is still null. The user sees a wrong mode and has no way to re-sync it.
- **Fix:** the PUT response already carries `sessionView`. In `chooseUnit`'s `onSuccess`, adopt it for the mode when no mode write has succeeded since and none is in flight (as `adoptLoad` does). Alternatively, only clear `loadFailed` when `viewWrite.confirmed != null`.
- **Reproducer:** none — there's no runnable test in the diff. The sequence is: `preferencesResult = failure` → `load()` → set `preferencesResult` to `(KG, QUICK)` → `chooseUnit(LBS)` → result: `loadFailed == false` and `sessionView == FOCUS`.

**2. `AppScreens.kt:249` and `SettingsState.kt:281`: the mode row is drawn with FOCUS selected after a failed load.**
- **What happens:** `hasLoaded` is set in `finally`, so it is true after a failed load too. The row is drawn with FOCUS selected and enabled, which is exactly the "default Focus" the KDoc on `WorkoutModeSetting` promises never to show.
- **Why it matters:** tapping QUICK in this state shows an optimistic QUICK, then throws "Could not save your workout mode.". Showing a value that was never read from the server is misleading.
- **Fix:** while `viewWrite.confirmed == null`, show the row as unselected or disabled. At minimum, correct the KDoc.
- **Reproducer:** none — the UI tests in the diff only use a mock engine where the load succeeds.

**3. `SettingsState.kt:337-354`: the mode-write path has test gaps.**
- **What's missing:** no test covers:
  - a stale mode failure (`STALE`);
  - a load that resolves while a mode write is in flight (`adoptLoad` must not overwrite the optimistic value);
  - Important #1 above.
- **What exists:** the queued-unit test checks the order of `updatePreferencesRequests`, which is good.
- **Reproducer:** none — this is a coverage gap.

#### Minor (Nice to Have)

- **Duplicated literal strings:** `SettingsState.kt:276` and `:335/:344/:349` repeat `"Could not save your workout mode."` and `"Could not load your settings."`. The diff deleted the `FailureMessages` import instead of adding these strings as constants alongside `COULD_NOT_LOAD_WEIGHT_UNIT` (`FailureHandling.kt:15`). Reproducer: none — style only.
- **`SettingsState.kt:335`:** `error(...)` raises an `IllegalStateException` as a stand-in for a refused write. That works, but a dedicated outcome would read more clearly than using an exception for control flow. Reproducer: none — design only.
- **`WorkoutApi.kt:38`:** adding a parameter with a default keeps the Kotlin source compatible. It is binary-breaking for any compiled implementer of `WorkoutCatalogApi` outside this module. Every implementer I found is in-repo, so this is fine as it stands. Reproducer: none.

### Recommendations
- The class KDoc says everything about `unitWrite` "holds unchanged" for `viewWrite`. That's no longer true: `loadFailed` is shared, and `chooseSessionView` depends on `unitWrite.confirmed`. Update the KDoc so it states both couplings.
- Make `chooseUnit` and `chooseSessionView` each adopt the other field from the response under the same gating. That removes the asymmetry behind Important #1.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The wire format, the mutex-serialised use of the confirmed unit, and the per-field load gating are all sound. But `loadFailed` now covers two fields while the unit-write success path still clears it for the unit alone. Together with `hasLoaded` being true after a failed load, the Workout mode row can show an unconfirmed FOCUS with no retry offered.