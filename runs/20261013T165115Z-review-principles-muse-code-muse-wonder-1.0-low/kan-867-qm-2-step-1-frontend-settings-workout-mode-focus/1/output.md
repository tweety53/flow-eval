### Derived-state check

- **`SettingsState.isSaving`** ([SettingsState.kt:234](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:234)) — summarises “any write in flight” from `unitWrite.isSaving` and `viewWrite.isSaving`, each itself derived from its `savingCalls > 0` ([SettingsState.kt:60](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:60)). It is a computed getter, not stored. **Verdict: derives from every fact — no issue.**
- **`SettingsState.loadFailed`** ([SettingsState.kt:242](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:242)) — stored flag summarising “the screen should offer retry”: latest load failed, **or** `chooseUnit` failed with nothing confirmed, **or** (new) `chooseSessionView` failed with nothing confirmed. Every writer, verified by `grep -n "loadFailed ="` over the whole file, unchanged lines included:
  - [SettingsState.kt:270](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:270) clears it only when the load is current **and** neither field has had a succeeding write (`noWriteSucceeded` conjoins both fields’ counts). **Verdict: derives from every fact.**
  - [SettingsState.kt:275](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:275) sets it under the same both-fields gate. **Verdict: derives from every fact.**
  - [SettingsState.kt:295](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) — `chooseUnit` success clears it from the unit field alone. **Verdict: partial-fact writer — see Important issue 1.**
  - [SettingsState.kt:310](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:310) sets it on unit `NOTHING_CONFIRMED`. **Verdict: setting on one fact is correct; the flag is an OR.**
  - [SettingsState.kt:339](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339) — new `chooseSessionView` success clears it from the mode field alone. **Verdict: partial-fact writer — see Important issue 1.**
  - [SettingsState.kt:351](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:351) sets it on mode `NOTHING_CONFIRMED`. **Verdict: correct, as above.**
- **`OptimisticWrite.succeededWriteCount` / `confirmed` / `value`** — event count and last-confirmed server values; not computable from other stored state. No issue.

### Summary

The diff adds a `SessionView` preference end-to-end: API parameter, wire model, a Settings row, a second `OptimisticWrite` in `SettingsState`, and tests. Project standards resolved to the appended principles file only; it states no layer/architecture boundary, suppression policy, or lint rule, so the Hard Invariants section yields no findings, and no new suppression or lint-config weakening appears in the diff. The reuse of `OptimisticWrite` and the mirroring of `chooseUnit` by `chooseSessionView` are a defensible WET tradeoff — the class already documents that per-field network call and failure copy stay at the call site — and the CQS-pragmatic command-that-returns-state in `updatePreferences` is pre-existing shape, knowingly retained. One stored flag was not updated to match the two-field world the diff creates.

### Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

1. **Single Source of Truth — `loadFailed` is cleared by one field’s success while the other field is still unconfirmed.** [SettingsState.kt:339](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339) (new writer) and [SettingsState.kt:295](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) (pre-existing writer the diff makes partial by adding a second summarised fact). The principle: “Each fact … is stored and defined in exactly one place; everything else derives from it,” and the required step’s rule that a writer deciding from part of the facts should derive instead. Concrete failure: load fails (nothing confirmed), `chooseSessionView(QUICK)` fails with `NOTHING_CONFIRMED` so `loadFailed = true` at line 351, then `chooseUnit(LBS)` succeeds and line 295 clears `loadFailed` — yet `viewWrite.confirmed` is still null and the mode control still shows an unconfirmed optimistic value, with the Retry affordance ([AppScreens.kt:326](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/AppScreens.kt:326)) now gone. The mirror image holds at line 339. Notably the load path at line 270 already does this correctly by conjoining both fields. Fix: clear `loadFailed` on a write success only when the other field also has a confirmed value (e.g. `unitWrite.confirmed != null && viewWrite.confirmed != null`), or derive the flag from both fields’ confirmed state.
   **Reproducer:** none — this is a read-only review; demonstrating it behaviourally requires adding a `SettingsStateTest` case (failed load → failed mode write → successful unit write → assert `loadFailed` stays true), which would mutate the tree this review must not touch. The partial writers are directly visible via `grep -n "loadFailed =" shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`.

#### Minor (Nice to Have)

1. **DRY / Single Source of Truth for failure copy — the diff leaves the shared registry.** [SettingsState.kt:276](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:276) drops `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` for a hardcoded `"Could not load your settings."` and removes the import. `FailureMessages` exists, per its own doc at [FailureHandling.kt:10](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/components/FailureHandling.kt:10), so shared wording “cannot drift between the call sites,” and it names `SettingsState.load` as a user; the workout editor still reads the same `GET /preferences` under the old constant ([WorkoutEditorScreen.kt:387](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/workout/WorkoutEditorScreen.kt:387)). The broader “settings” wording is the right call now that the load covers two fields — but it belongs in `FailureMessages` as a new constant, with its KDoc updated, not as a one-off literal (now repeated in spirit by the three `"Could not save your workout mode."` literals at lines 335, 347, 352).
   **Reproducer:** `grep -rn "Could not load your settings" shared/src` — returns exactly one hit, outside the registry that owns this copy.
2. **Principle of Least Astonishment — stale doc on the flag the diff widens.** The `loadFailed` KDoc at [SettingsState.kt:237](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:237) still says only “the latest `chooseUnit` failed with nothing yet confirmed,” though `chooseSessionView` now sets it too; a reader trusting the doc will miss a writer. One-line doc fix.
   **Reproducer:** none — documentation-only; visible by reading lines 237–243 against line 351.

### Assessment

**Principles-compliant?** With fixes

**Reasoning:** The structure is sound — composition via a second `OptimisticWrite`, interface-first API change, wire model defaulting that matches the documented server default, and per-field load gating that correctly conjoin both fields at lines 264–272. The one structural defect is that `loadFailed`, now a summary of two fields’ confirmation state, is still cleared by either field alone, which can hide the retry path while a value on screen was never confirmed; fix that derivation and move the new failure copy into `FailureMessages`, and the diff complies.