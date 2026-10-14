### Derived-state check

| State | Facts it summarises | Writers | Verdict |
|---|---|---|---|
| `SettingsState.loadFailed` (`SettingsState.kt:242`) | (a) the latest current `load` failed; (b) the unit field failed a write and has nothing confirmed; (c) **new:** the view field failed a write and has nothing confirmed | `SettingsState.kt:270` (load success clears it) | Takes facts (a), (b) and (c) into account: it clears only when no write of either field succeeded. OK. |
| | | `SettingsState.kt:275` (load failure sets it) | Sets fact (a). OK. |
| | | `SettingsState.kt:295` (`chooseUnit` success clears it) | **Violation.** The unit path alone clears a flag that also stands for fact (c). See Important #1. |
| | | `SettingsState.kt:310` (`chooseUnit` NOTHING_CONFIRMED sets it) | Sets fact (b). OK. |
| | | `SettingsState.kt:339` (`chooseSessionView` success clears it) | Partial writer: the view path alone clears it. It is safe today only because the send at `:335` requires `unitWrite.confirmed != null`, which rules out fact (b). That safety comes from a coincidence in the call order, not from the flag's design. |
| | | `SettingsState.kt:351` (`chooseSessionView` NOTHING_CONFIRMED sets it) | Sets fact (c). OK. |
| `SettingsState.isSaving` (`:233-235`) | unit write in flight, view write in flight | computed getter, never stored | Derived from both facts. Good. |

### Summary
The diff adds a `SessionView` account preference end to end: the API signature, the DTOs, a second `OptimisticWrite` in `SettingsState`, a Settings row, the fake API and tests. I checked it against all three groups: Structure, Simplicity & state, and Robustness & ops.

**Project standards:** the field was empty, so I enforced no hard invariants and applied only the universal principle list. Its appendix contains no directives aimed at the reviewer.

**What it does well:**
- The second field reuses `OptimisticWrite` instead of copying its logic.
- `isSaving` is derived rather than stored.
- Leaving `sessionView` null to mean "unchanged" is a clean, documented use of `encodeDefaults = false`.

**What needs fixing:** the main problem is `loadFailed`. It used to summarise state for one field and now covers two, but it is still stored and written field by field.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295`: Single Source of Truth (stored state that should be derived)**
- **The problem:** `loadFailed` now also covers "the view field has nothing confirmed after a failed write" (set at `:351`). But `chooseUnit`'s success clears it from the unit's facts alone.
- **How it plays out:**
  1. `load` fails.
  2. `chooseSessionView(QUICK)` is refused at `:335` (NOTHING_CONFIRMED). `viewWrite.value` stays at QUICK, which the server never confirmed, and `loadFailed = true`.
  3. `chooseUnit(LBS)` succeeds, and `:295` sets `loadFailed = false`.
  4. The retry option disappears while the control still shows a mode the server never confirmed.

  The KDoc at `:236-238` is also stale: it still names only `chooseUnit`.
- **Fix:** derive the flag instead of storing it:
  1. Give `OptimisticWrite` a per-field `failedWithNothingConfirmed` that `onFailure` sets on NOTHING_CONFIRMED and that `onSuccess`/`adoptLoad` clear.
  2. Keep only `latestLoadFailed` as stored state.
  3. Expose `val loadFailed get() = latestLoadFailed || unitWrite.failedWithNothingConfirmed || viewWrite.failedWithNothingConfirmed`.
- **Reproducer:** none — I couldn't run anything in this read-only review. A test that would show it: fail `load`, call `chooseSessionView(QUICK)`, then call `chooseUnit(LBS)` with the API returning success. Then assert `state.loadFailed` is true. It is false today.

**2. `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:342-355`: DRY (duplicated knowledge)**
- **The problem:** the `when (…onFailure(token))` block repeats `:298-313` almost word for word. That block encodes the policy "STALE → nothing; REVERTED → notify; NOTHING_CONFIRMED → raise `loadFailed` and notify". That policy is the same for every field. `OptimisticWrite`'s own KDoc (`:36-38`) says only "the network call itself and the failure copy" should differ per call site.
- **Fix:** pull out one private helper, for example `handleWriteFailure(outcome, throwable, copy: String)`, and have both `choose*` methods call it. This also removes the three copies of the literal `"Could not save your workout mode."` at `:335/:347/:352`.
- **Reproducer:** none — this is a structural duplication, visible by comparing `:298-313` with `:342-355`.

#### Minor (Nice to Have)

**3. `shared/src/commonMain/kotlin/com/gymie/shared/ui/components/FailureHandling.kt:12`: Least Astonishment (stale doc)**
- **The problem:** `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` is still documented as "Shared by `SettingsState.load` …". But `SettingsState.kt:276` now uses an inline literal, `"Could not load your settings."`. Splitting the wording on purpose is defensible, since settings now loads more than the unit, but the shared constant's doc now misleads.
- **Fix:** update that KDoc to name only the workout editor.
- **Reproducer:** `grep -n "COULD_NOT_LOAD_WEIGHT_UNIT" -r shared/src/commonMain` shows the constant is used only in `WorkoutEditorScreen.kt:387`.

**4. `shared/src/commonMain/kotlin/com/gymie/shared/domain/WorkoutModels.kt:697` and `SettingsState.kt:212`: Single Source of Truth (default written twice)**
- **The problem:** the client spells out the server's default `SessionView.FOCUS` in two places: the DTO default and `OptimisticWrite(SessionView.FOCUS)`. (It also appears in `FakeWorkoutApi`.) Because the UI waits for `hasLoaded`, the impact is small.
- **Fix:** add a single `SessionView.DEFAULT` (for example a companion `val`) and reference it from both places.
- **Reproducer:** `grep -n "SessionView.FOCUS" shared/src/commonMain -r` finds two independent copies of the default.

### Assessment
**Principles-compliant?** With fixes.

**Reasoning:** The structure is sound and the trade-offs were taken knowingly: a reusable `OptimisticWrite`, a derived `isSaving`, and a nullable "unchanged" field on the wire. However, the diff turns `loadFailed` into state that summarises two fields while it is still written field by field. `chooseUnit`'s success path (`:295`) can then clear a failure that belongs to the view field. Deriving `loadFailed` (Important #1) and merging the duplicated failure policy into one helper (Important #2) would bring the diff into compliance.