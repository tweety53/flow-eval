### Failure-recovery check

Failure states the diff's code sets:

- **`loadFailed = true`** is set at `SettingsState.kt:275` (load failure), `:310` (`chooseUnit`, NOTHING_CONFIRMED) and `:351` (`chooseSessionView`, NOTHING_CONFIRMED).
- **`viewWrite.value` shown as optimistic or default, with `viewWrite.confirmed == null`.** This is the default `FOCUS` after a failed load, or the optimistic choice after a failed view write. `OptimisticWrite.onFailure` (`:~118`) leaves it in place.

Lines that clear `loadFailed`: `:270`, `:295`, `:339`.

A failed `load()` leaves two values unconfirmed: unit (`unitWrite.confirmed`) and view (`viewWrite.confirmed`).

| Clearing line | Unit re-confirmed by | View re-confirmed by |
|---|---|---|
| `:270` (load success, requires no write succeeded since the load began) | `:272` `unitWrite.adoptLoad` | `:273` `viewWrite.adoptLoad` |
| `:295` (`chooseUnit` success) | `:295` `unitWrite.onSuccess` | **none** |
| `:339` (`chooseSessionView` success) | implied: the send requires `unitWrite.confirmed != null` | `:339` `viewWrite.onSuccess` |

`:295` is a partial recovery. The PUT response already carries `sessionView`: the fake and the UI test's mock both answer with it, and `WorkoutPreferences` now has the field. `chooseUnit` discards it, and `:295` isn't in the diff.

### Summary
The diff adds a `sessionView` field to the preferences PUT and GET, and a `chooseSessionView` write beside `chooseUnit`. It also changes how `load` handles the second field. All four lenses were checked. The code adds no timeout or other boundary beyond the existing `api` calls, and I didn't check `apiCall` for timeouts. The new write mutex handling is sound: the confirmed unit is read inside `sendMutex`, so a queued unit choice can't leak through the view write. One recovery gap is Important.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
**`SettingsState.kt:295` (`chooseUnit` success path), error-return lens: partial recovery.**
- **Scenario:** `load()` fails (offline), so `loadFailed = true` and the view control shows the default FOCUS, unconfirmed. The user then taps a unit, and `chooseUnit` succeeds. `:295` clears `loadFailed`, so the UI now claims recovery. But `viewWrite.confirmed` is still null and the control still shows FOCUS, even though the server holds QUICK. The PUT response contained `sessionView=QUICK`, and it is discarded.
- **Fix:** In `chooseUnit`'s success handler, also adopt the answer's view when the view write has no confirmed value or isn't saving, for example `viewWrite.adoptLoad(it.sessionView)`. This needs the same guard as in `load`: no view write succeeded in the meantime. Alternatively, clear `loadFailed` at `:295` only when both fields are confirmed.
- **Reproducer:** none — no test exists for this path. To demonstrate it, add a test in `SettingsStateTest.kt` and run `./gradlew :shared:desktopTest --tests '*SettingsStateTest*'`:
  1. Make `getPreferences` fail on the first call.
  2. Set `api.updatePreferencesResult = Result.success(WorkoutPreferences(KG, QUICK))`.
  3. Call `load()`, then `chooseUnit(LBS)`.
  4. Observe `state.sessionView == FOCUS` and `loadFailed == false`.

#### Minor (Nice to Have)
- **`SettingsState.kt:~330`, concurrent re-entry:** `chooseSessionView` sends the last-confirmed unit. If another device changed the unit since the last load, this PUT silently reverts it. The answer's unit is deliberately not adopted. Consider adopting it when no unit write is in flight.

### Assessment
**Ready for the human gate?** With fixes
**Reasoning:** Adding the second preference field creates a partial-recovery state. A successful unit write clears the failure banner while the workout-mode value remains unconfirmed, even though the response carried it. The fix is small and local. Nothing else rises above Minor.