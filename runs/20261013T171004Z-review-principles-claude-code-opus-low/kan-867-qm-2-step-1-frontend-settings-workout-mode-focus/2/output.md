### Derived-state check

| State | Facts it summarises | Writers | Verdict |
|---|---|---|---|
| `loadFailed` (`SettingsState.kt:242`) | Two facts since this diff: (a) the unit has no confirmed server value, (b) the session view has no confirmed server value | `:270` clear, `:275` set, `:295` clear (unchanged line), `:310` set, `:339` clear, `:351` set | `:270` and `:275` look at both writes. They check write counts for unit **and** view, so they derive from both facts. `:339` is safe in practice: a view write can only succeed if `unitWrite.confirmed` is non-null (`:335`), so both facts are settled. **`:295` derives from only one fact.** A successful unit write clears the flag even when the view was never confirmed. See Important #1. `:310` and `:351` set the flag from one fact each, which is correct, because either missing fact makes the flag true. |
| `isSaving` (`:234-235`) | Unit write in flight, view write in flight | Computed getter, never stored | Derived from both. OK. |
| `noWriteSucceeded` / `current` (`:265-267`) | Locals derived from the counters | Computed per call | Derived. OK. |

### Summary
The diff adds a `SessionView` preference end to end: the API signature, the DTOs, a new `OptimisticWrite` in `SettingsState`, a new Settings row, and tests.

Principle groups applied:
- **Structure:** Separation of Concerns, Information hiding.
- **Simplicity & state:** Single Source of Truth, DRY/WET, KISS, CQS.
- **Robustness & ops:** Least Astonishment, Testing.

The **Project standards** field was empty, so there are no hard-invariant rules and only the universal principle list applies. I found no new suppressions or lint-config edits in the diff.

The structure holds up. Giving each field its own `OptimisticWrite` and sharing one `sendMutex` between them is a sensible, explicit WET choice, and the class KDoc says so. One SSOT gap remains: `loadFailed` is stored, and one of its writers was not updated for the second field.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. `SettingsState.kt:295`: Single Source of Truth (a stored summary cleared by only one of the facts it covers)**
- **Principle:** "a computed total persisted alongside the rows it is computed from with nothing keeping them consistent."
- **What changed:** `loadFailed` used to mean "the unit is unconfirmed". It now also stands for "the view is unconfirmed": `:270` only clears it when no write of either kind succeeded, and `:351` raises it when the view has nothing confirmed.
- **The gap:** the unchanged writer at `:295` (`if (unitWrite.onSuccess(...)) loadFailed = false`) still clears it based on the unit alone.
- **Scenario:** `load()` fails, then `chooseUnit(LBS)` succeeds. `loadFailed` becomes false, but `viewWrite.confirmed` is still null and the control shows the default FOCUS. The answer's `sessionView` is never adopted. The retry/re-sync path the flag exists to drive is suppressed.
- **Fix:** stop storing the flag and derive it, e.g. `val loadFailed get() = lastLoadFailed && (unitWrite.confirmed == null || viewWrite.confirmed == null)`. Alternatively, have `chooseUnit`'s success also call `viewWrite.adoptLoad(it.sessionView)`, so both facts are settled before the flag is cleared.
- **Reproducer:** `none — needs a new test: preferencesResult = failure; load(); preferencesResult/updatePreferences succeed; chooseUnit(LBS); assert loadFailed is false while state.sessionView was never confirmed`.

#### Minor (Nice to Have)

**2. `SettingsState.kt:335, :347, :352`: DRY**
- The message `"Could not save your workout mode."` is written three times.
- The diff also drops the `FailureMessages` import (`:8`) and inlines `"Could not load your settings."` at `:276`, while `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` (`components/FailureHandling.kt:15`) is still the established home for these strings and is still used by `WorkoutEditorScreen.kt:387`.
- **Fix:** add `COULD_NOT_SAVE_SESSION_VIEW` and `COULD_NOT_LOAD_SETTINGS` constants to `FailureMessages` and reference them.
- **Reproducer:** `grep -n '"Could not save your workout mode."' shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt` returns 3 lines.

**3. `AppScreens.kt` (`WorkoutModeSetting`, `label = { if (it == SessionView.FOCUS) "Focus" else "Quick" }`): Least Astonishment / open-closed**
- The `if/else` silently labels any future enum value "Quick".
- **Fix:** use an exhaustive `when (it) { FOCUS -> "Focus"; QUICK -> "Quick" }` so adding a value fails at compile time.
- **Reproducer:** `none — only shows up when an enum value is added`.

**4. `WorkoutModels.kt` (the `WorkoutPreferences.sessionView` default) and `SettingsState.kt` (`OptimisticWrite(SessionView.FOCUS)`): Single Source of Truth (minor)**
- The server's default FOCUS is written down twice on the client. The comments explain both copies, so this is a knowing choice.
- **Fix:** add a `SessionView.DEFAULT` companion constant and use it in both places.
- **Reproducer:** `none — duplication of a constant, no behavioural divergence today`.

### Assessment
**Principles-compliant?** With fixes

**Reasoning:** The design follows the established single-field pattern (a separate optimistic write per field and a shared send mutex), and it is a deliberate, documented trade. `isSaving` correctly stays derived. However, `loadFailed` now summarises two facts, and one writer (`:295`) still clears it from one of them. That is the SSOT violation this review must flag, and it should be fixed, ideally by deriving the flag, before handoff. The other findings are small.