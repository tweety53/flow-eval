### Derived-state check

| State | Facts it summarises | Writers | Verdict |
|---|---|---|---|
| `loadFailed` (`SettingsState.kt:242`) | Is there a confirmed value for the unit (`unitWrite.confirmed`)? Is there a confirmed value for the mode (`viewWrite.confirmed`)? | `:270` (load success, gated on both fields) | Uses both facts. |
| | | `:275` (load failure, gated on both fields) | Uses both facts. |
| | | `:295` (`chooseUnit` success clears it, unchanged line) | **Uses only the unit fact.** After a failed load, a successful unit write clears `loadFailed`, but `viewWrite.confirmed` is still null and the mode row shows the client default FOCUS, which nothing has confirmed. See Important #1. |
| | | `:310` (unit failure with nothing confirmed raises it) | Uses only the unit fact. Raising it is safe. |
| | | `:339` (`chooseSessionView` success clears it) | Uses only the mode fact. It happens to be safe, because `:335` refuses to send unless the unit is confirmed. That is a coincidence of ordering, not a derivation. |
| | | `:351` (mode failure with nothing confirmed raises it) | Uses only the mode fact. Raising it is safe. |
| `isSaving` (`:234`) | `unitWrite.isSaving`, `viewWrite.isSaving` | Computed getter | Computed from both facts. Fine. |
| `noWriteSucceeded` (`:267`) | The two succeeded-write counters | A local `val` | Computed from both. Fine. |

### Summary
The diff adds a `SessionView` preference through four layers: the API (`WorkoutApi`/`Impl`), the domain models, `SettingsState` (a second `OptimisticWrite`) and the Settings UI row, plus tests. No project standards files resolved, so I enforced no hard invariants and applied only the universal principle list. I checked all three groups:
- **Structure:** the new mode field reuses the existing `OptimisticWrite` (composition) and stays behind `WorkoutCatalogApi` (programming to the interface).
- **Simplicity & state:** see the Single Source of Truth and DRY findings below.
- **Robustness & ops:** failures are surfaced, and the null-unit guard fails fast.

The design is sound overall. The one structural defect is the stored `loadFailed` flag, which now has to cover two fields.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
1. **`SettingsState.kt:295` — Single Source of Truth (stored summary state).**
   - **Problem:** `loadFailed` now means "some field has no confirmed server value", but the `chooseUnit` success path clears it using only the unit field. The diff added the second field and left this writer alone.
   - **Effect:** after a failed load and a successful unit write, the screen treats the settings as synced while the mode row shows FOCUS, a value the server never confirmed.
   - **Fix:** derive the flag instead of storing it, for example `val loadFailed get() = !isLoading && hasLoaded && (unitWrite.confirmed == null || viewWrite.confirmed == null)`, folding in the current load-failure gating. At minimum, gate `:295` and `:339` on both fields being confirmed.
   - **Reproducer:** none — I added no test because this review is read-only. The scenario is: set `preferencesResult` to a failure, call `load()`, then `chooseUnit(LBS)` with success. `loadFailed == false`, yet `viewWrite.confirmed == null`.

#### Minor (Nice to Have)
1. **`SettingsState.kt:276` — DRY / Single Source of Truth.**
   - **Problem:** the diff deletes the use of `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` and inlines a new literal. The constant still exists at `components/FailureHandling.kt:15` with no callers.
   - **Fix:** rename the constant to a `COULD_NOT_LOAD_SETTINGS` entry in `FailureMessages`, or delete the dead constant.
   - **Reproducer:** `grep -rn COULD_NOT_LOAD_WEIGHT_UNIT shared/src` (the only hit is the definition).
2. **`SettingsState.kt:335,347,352` — DRY.**
   - **Problem:** `"Could not save your workout mode."` is written three times in one function.
   - **Fix:** hoist it to a private const, as `CalculatorState.kt:31` does with `SAVE_FAILED`.
   - **Reproducer:** `grep -c "Could not save your workout mode" shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt` (prints 3).
3. **`WorkoutModels.kt` (`WorkoutPreferences.sessionView` default) and `SettingsState.kt` (`OptimisticWrite(SessionView.FOCUS)`) — Single Source of Truth.**
   - **Problem:** "the server's default is FOCUS" is written down in two places.
   - **Fix:** expose one `SessionView.DEFAULT` and reference it from both places.
   - **Reproducer:** none — I didn't run it, but grepping for `SessionView.FOCUS` in `commonMain` should show both sites.

Tradeoffs I'm not counting as violations:
- Sending `displayUnit` on every PUT is forced by the backend contract, and the KDoc explains it.
- Keeping `chooseUnit` and `chooseSessionView` as near-duplicates is a reasonable WET choice for two fields that change for different reasons, and the class KDoc says so.

### Assessment
**Principles-compliant?** With fixes.
**Reasoning:** Adding a second field turned `loadFailed` into a summary of two fields, but its writers still update it from one field at a time. That is a Single Source of Truth violation, and it can leave an unconfirmed mode looking synced. Deriving the flag fixes it. The remaining findings are small DRY cleanups.