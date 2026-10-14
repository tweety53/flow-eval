### Failure-recovery check

Failure states set by the diff's code (all in `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`):

| # | Failure state | Set at | Value left unconfirmed |
|---|---|---|---|
| F1 | `loadFailed = true` + failure toast, load failed | `SettingsState.kt:275`, `SettingsState.kt:276` | `unitWrite.confirmed/value` (server unit unknown), `viewWrite.confirmed/value` (server mode unknown) |
| F2 | `loadFailed = true` + failure toast, `chooseUnit` with nothing confirmed | `SettingsState.kt:310`, `SettingsState.kt:311` | optimistic `unitWrite.value` (never confirmed) |
| F3 | `loadFailed = true` + failure toast, `chooseSessionView` refused/failed with nothing confirmed (includes the `unitWrite.confirmed ?: error(...)` refusal at `SettingsState.kt:335`) | `SettingsState.kt:351`, `SettingsState.kt:352` | optimistic `viewWrite.value` (never confirmed) |
| F4 | `viewWrite.value` reverted, toast only, `loadFailed` untouched | `SettingsState.kt:347` (via `OptimisticWrite.onFailure` revert at `SettingsState.kt:119`) | none — reverted to confirmed |
| F5 | `unitWrite.value` reverted, toast only (unchanged pre-existing path) | `SettingsState.kt:303` | none |

Lines that clear/overwrite `loadFailed` (whole-file grep, unchanged functions included): `SettingsState.kt:270` (load success), `SettingsState.kt:295` (unit-write success), `SettingsState.kt:339` (view-write success). No line in this file clears a `NotificationState` failure — toasts are transient/self-dismissing outside this file, so there is no `loadFailed`-style false-recovery claim there.

Per clearing line × unconfirmed value:

- Clearing `SettingsState.kt:270` (`if (current && noWriteSucceeded) loadFailed = false`): re-confirms unit via `SettingsState.kt:271` (`unitWrite.adoptLoad(it.displayUnit)`) and view via `SettingsState.kt:272` (`viewWrite.adoptLoad(it.sessionView)`). Gating requires *both* `noUnitWriteSucceeded...` and `noViewWriteSucceeded...`, so a success that clears here also re-confirms both values — **not partial**. (`hasLoaded = true` at `SettingsState.kt:281` fires on success or failure and never claims value recovery.)
- Clearing `SettingsState.kt:295` (`if (unitWrite.onSuccess(token, it.displayUnit)) loadFailed = false`): re-confirms unit via `OptimisticWrite.onSuccess` at `SettingsState.kt:102-103`; re-confirmation of the view value: **none** — the PUT response type `WorkoutPreferences` carries `sessionView`, but the code adopts only `it.displayUnit` and discards `it.sessionView`. Clearing `loadFailed` here while F3 (or F1's view half) is outstanding claims recovery while the mode is still unknown — **partial recovery** (issue I1).
- Clearing `SettingsState.kt:339` (`if (viewWrite.onSuccess(token, it.sessionView)) loadFailed = false`): re-confirms view via `OptimisticWrite.onSuccess`; re-confirmation of the unit value: **none** — the response carries `displayUnit`, but the code adopts only `it.sessionView` and explicitly documents "The answer's unit is not adopted" (`SettingsState.kt:325`). Clearing `loadFailed` here while F2 (or F1's unit half) is outstanding claims recovery while the unit is still unknown — **partial recovery** (issue I2).

### Summary

The diff adds a second independently-tracked preferences field (`SessionView` FOCUS/QUICK) alongside `displayUnit`: a `PUT api/workouts/preferences` with an optional `sessionView` key (`WorkoutApiImpl.kt:192-202`, `WorkoutModels.kt:689-705`), an optimistic `chooseSessionView` write serialised with unit writes through `sendMutex` (`SettingsState.kt:328-359`), a dual-gated `load()` (`SettingsState.kt:270-272`), and a settings row that saves on tap (`AppScreens.kt:388-417`). Of the four lenses, **error return** bore (the two single-field `loadFailed = false` clearings discard the other field's value from the same success response). **Timeout** did not: the shared client bounds every call (`GymieApiClient.kt:255-259`, 10–15 s; `ApiRequestSupport.mapException` maps all three Ktor timeout types). **Partial write** did not: each behaviour is one atomic PUT, not a multi-step write. **Concurrent re-entry** did not: `sendMutex` plus per-field sequences preserve tap order (covered by the queued-write test). Verdict: two related partial-recovery gaps, no Critical findings.

### Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

**I1 — `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295`, lens: error return (partial recovery).** Scenario: `load()` fails (F1, mode unknown, `loadFailed = true`), then a unit write succeeds — or a mode write fails with nothing confirmed (F3) and then any unit write succeeds. Line 295 clears `loadFailed` from the unit response alone while `viewWrite.value/confirmed` is still unconfirmed; the response already carried `sessionView` and the code discarded it. The screen then drops the retry affordance while the mode row may still show the default/optimistic value as if synced. Fix sketch: adopt the other field on success too (e.g. `viewWrite.adoptLoad`/confirmed update from `it.sessionView` without disturbing an in-flight optimistic view, mirroring `adoptLoad`'s `isSaving` guard), or gate the `loadFailed = false` clear on the other field being confirmed. Reproducer: `none — requires a scripted coroutine interleaving (failed load → successful chooseUnit) asserting loadFailed against an unconfirmed sessionView; no single shell command demonstrates it`.

**I2 — `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339`, lens: error return (partial recovery).** Mirror scenario: a unit write fails with nothing confirmed (F2, `loadFailed = true`, unit showing an unconfirmed optimistic value), then a mode write succeeds. Line 339 clears `loadFailed` from the mode response alone while the unit is still unknown; the response already carried `displayUnit` and the code deliberately discards it (`SettingsState.kt:325-326`). Same stuck-recovered outcome for the weight-unit row. Fix sketch: same as I1, symmetric — adopt `it.displayUnit` into the unit tracker (respecting in-flight optimism) or do not clear the shared `loadFailed` until both fields hold confirmed values. Reproducer: `none — requires a scripted coroutine interleaving (failed chooseUnit with nothing confirmed → successful chooseSessionView) asserting loadFailed against an unconfirmed displayUnit; no single shell command demonstrates it`.

#### Minor (Nice to Have)

None. The refusal path (`SettingsState.kt:335`, no PUT without a confirmed unit) correctly avoids overwriting a stored unit with the KG default; cancellation is correctly excluded from failure accounting via `onFailureNotCancelled`; UI copy fallbacks (`SettingsState.kt:276,311,352`) report adequately.

### Assessment

**Ready for the human gate?** With fixes

**Reasoning:** No data loss, torn write, double effect, or unbounded wait was found — timeouts are bounded, writes are single atomic PUTs, and concurrent sends are serialised in tap order. The two Important findings are the required-step partial recoveries: each single-field success clears the shared `loadFailed` retry signal while the other field's value is still unknown, even though that same response carried the missing value and the code discarded it.
