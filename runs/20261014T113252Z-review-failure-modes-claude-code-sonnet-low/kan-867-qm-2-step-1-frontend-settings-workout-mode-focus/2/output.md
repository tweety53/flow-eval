### Failure-recovery check

**State set on failure: `loadFailed`**, the retry affordance. The unconfirmed fallback values are `unitWrite.value`/`confirmed` and `viewWrite.value`/`confirmed`. `viewWrite` starts at `FOCUS` with `confirmed = null`.

Lines that set it, all in `SettingsState.kt`:
- `:275`, a failed `load`.
- `:310`, `chooseUnit` with nothing confirmed.
- `:351`, `chooseSessionView` with nothing confirmed. This also covers the new "no confirmed unit" refusal, which comes through `onFailure`.

Lines that clear it:
- `:270`, `load` success. It requires no write of either field to have succeeded, and re-confirms both fields through `unitWrite.adoptLoad` (`:271`) and `viewWrite.adoptLoad` (`:272`). This one is fine.
- `:295`, `chooseUnit` success.
- `:339`, `chooseSessionView` success.

After a failed `load`, both the unit and the mode are unconfirmed.

| Clearing line | Unit | Mode |
|---|---|---|
| `:295` | `:295` via `unitWrite.onSuccess` | **none** |
| `:339` | Already confirmed, because `:329` refuses to send otherwise | `:339` via `viewWrite.onSuccess` |

The `:295` path is a partial recovery. The PUT response carries `sessionView`, and `chooseUnit` discards it.

### Summary
The diff adds a `sessionView` field to the preferences payload and a `chooseSessionView` write that shares `sendMutex` and `loadFailed` with `chooseUnit`. I checked it against all four lenses.
- **Error return:** the "unit not confirmed" refusal is sound, but one `loadFailed` flag now covers two fields. That causes the partial recovery above.
- **Concurrent re-entry:** reading `unitWrite.confirmed` inside the mutex is correct, and the diff's own test covers it.
- **Timeout:** the diff adds no new wait, so I raised nothing on that lens.
- **Partial write:** the diff has no multi-write sequence, so I raised nothing on that lens.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
- **`SettingsState.kt:295`** (error return). The initial `load` fails, so `loadFailed` is true and `sessionView` shows the `FOCUS` default with `viewWrite.confirmed == null`. The user taps a unit, and `chooseUnit` succeeds. `loadFailed = false` clears the retry affordance, but the stored mode is still unknown. If the server holds `QUICK`, the screen shows `Focus` as though it were confirmed. The success response already contained the real `sessionView`, and it is discarded.
  - **Fix:** in `chooseUnit`'s success path, call `viewWrite.adoptLoad(it.sessionView)` when `viewWrite.confirmed == null` and `viewWrite.succeededWriteCount` is unchanged. Alternatively, only clear `loadFailed` when both fields are confirmed.
  - **Reproducer:** none — no runnable command was executed. A test would have `getPreferences` fail with `QUICK` stored, call `chooseUnit(LBS)`, and assert `sessionView == QUICK` and `loadFailed == false`. Today `sessionView` stays `FOCUS`.

#### Minor (Nice to Have)
- **`SettingsState.kt:329`** (error return). The refusal throws a bare `error("Could not save your workout mode.")`. It has the same text as the generic fallback, so a debugger can't tell it from a network failure. Use a message that names the cause, for example "unit not loaded". Reproducer: none — wording only.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The new write is serialised correctly and refuses to guess the unit. The one real gap is that a unit-write success clears `loadFailed` while the mode is still unconfirmed, so the Settings screen can show a wrong mode with no retry affordance.