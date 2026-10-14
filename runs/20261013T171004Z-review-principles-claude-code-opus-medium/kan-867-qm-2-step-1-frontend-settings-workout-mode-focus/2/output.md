### Derived-state check

**`loadFailed`** (`SettingsState.kt:242`). It stands for "there is something the screen cannot vouch for, so offer a retry." It depends on three facts:
- (a) the newest `load` failed;
- (b) `unitWrite.confirmed == null`;
- (c) `viewWrite.confirmed == null`. This one is new in this diff.

Every writer:

| Writer | What it does | Verdict |
|---|---|---|
| `SettingsState.kt:270` (load success) | Clears it when the load is current and no write of either field has succeeded since | Clears (a), and the load adopts both fields. But when a unit write succeeded during the load, it skips the clear even if the view was adopted. Gated on both write counts at once, while each field's adoption is gated on its own count. Mostly sound. |
| `SettingsState.kt:274-275` (load failure) | Sets it | Derives from (a). Fine. |
| `SettingsState.kt:295` (`chooseUnit` success, unchanged line) | Clears it | **Violation.** Only (b) is resolved. `chooseUnit` does not adopt `it.sessionView` from the PUT answer, so (c) can still be null. |
| `SettingsState.kt:310` (`chooseUnit` NOTHING_CONFIRMED) | Sets it | Fine; it only reports (b). |
| `SettingsState.kt:339` (`chooseSessionView` success) | Clears it | Acceptable in practice: line 335 refuses to send without a confirmed unit, so a success means (b) and (c) are both settled. That depends on an invariant elsewhere rather than being derived. |
| `SettingsState.kt:351` (`chooseSessionView` NOTHING_CONFIRMED) | Sets it | Fine. |

**`isSaving`** (`SettingsState.kt:234`) is a getter computed from `unitWrite.isSaving || viewWrite.isSaving`. Nothing stores it. Compliant.

**`hasLoaded`** (`:281`) is written only in `load`'s `finally`. The diff doesn't change it.

### Summary
The diff adds a `SessionView` preference end to end: the domain enum and request/response fields, the `updatePreferences` API parameter, a second `OptimisticWrite` and `chooseSessionView` in `SettingsState`, a Settings row, a fake, and tests.

I checked it against all three principle groups:
- **Structure:** SRP, encapsulation.
- **Simplicity & state:** DRY/WET, KISS, Single Source of Truth, CQS.
- **Robustness & ops:** Least Astonishment, testing principles, robustness.

The **Project standards** field was empty, so no standards file resolved and the hard-invariants section is empty. I applied the universal principle list only.

Reusing `OptimisticWrite` and `sendMutex` was the right call. Reading the confirmed unit inside the mutex (`:335`) is a deliberate, well-documented choice. The main problem is that `loadFailed` is a single stored flag that now covers two fields, and one field's write path still clears it alone.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. `SettingsState.kt:295`: Single Source of Truth (a stored summary cleared from one of its facts).**

`loadFailed` now covers both fields. The unit write's success still clears it on its own, and the server's answer to that same PUT (which carries `sessionView`) is thrown away.

Sequence:
1. `load()` fails, so `loadFailed` is true and `viewWrite.confirmed` is null.
2. `chooseUnit(LBS)` succeeds, so `loadFailed` becomes false.
3. The screen hides the retry and shows `sessionView = FOCUS`, a value the server never confirmed. If the stored value is QUICK, the control is wrong, and nothing offers to re-sync it.

The class KDoc says everything about `unitWrite` "holds unchanged for `viewWrite`", but this is exactly the part where that stops being true.

Fix (either is enough):
- Derive the flag: `val loadFailed get() = latestLoadFailed || unitWrite.confirmed == null && hasLoaded || viewWrite.confirmed == null && hasLoaded`, or something equivalent.
- At minimum, have `chooseUnit`'s success also call `viewWrite.adoptLoad(it.sessionView)` (gated like a load), so the one response confirms both fields it carries.

Reproducer: `none — no existing test covers it`. It is a three-line test in `SettingsStateTest`:
```
preferencesResult = failure
load()
chooseUnit(LBS)
assertTrue(state.loadFailed || <sessionView confirmed>)
```
That assertion fails against this diff.

**2. `SettingsState.kt:328-358`: DRY (knowledge duplication).**

`chooseSessionView` copies `chooseUnit`'s whole failure policy (`:296-312`): the three-way `when` over `WriteFailureOutcome` and the rule "set `loadFailed` on NOTHING_CONFIRMED, clear it on a current success."

`OptimisticWrite` was extracted specifically to stop this kind of repetition (its KDoc cites KAN-71 review finding F4), and that KDoc says only "the network call itself and the failure copy" should differ per field. The policy is now in two places, so finding 1's fix has to be made twice.

Fix: add a private `suspend fun <T> write(w: OptimisticWrite<T>, value: T, failureCopy: String, send: suspend () -> T)` that owns `begin`/`end`, the `onSuccess`→`loadFailed` rule and the outcome `when`. Each `chooseX` then passes only its send lambda and its copy.

Reproducer: `none — structural`. Compare `SettingsState.kt:296-312` with `:340-354`.

#### Minor (Nice to Have)

**3. `FailureHandling.kt:12-15` (stale after the change at `SettingsState.kt:276`): Least Astonishment (a doc that contradicts the code).**

The KDoc on `COULD_NOT_LOAD_WEIGHT_UNIT` still says it is "Shared by `SettingsState.load` and the workout editor". The diff swapped that use for an inline `"Could not load your settings."`. The constant now has one user, so its "shared, kept in one place" reason no longer holds.

Fix: update that KDoc. Either inline the constant into `WorkoutEditorScreen` or keep it with a corrected doc.

Reproducer: `grep -rn COULD_NOT_LOAD_WEIGHT_UNIT shared/src` shows only `FailureHandling.kt:15` and `WorkoutEditorScreen.kt:387`.

**4. `SettingsState.kt:335, 347, 352`: DRY (minor).**

`"Could not save your workout mode."` appears three times in one function. A private const fixes it, and finding 2's refactor would remove the problem anyway.

Reproducer: `grep -c "Could not save your workout mode" shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt` returns 3.

**5. `SettingsStateTest.kt` (`a unit write leaves the workout mode off the wire`): Testing principles (fail for exactly one reason).**

The test checks two unrelated things:
- serialisation, using a default `Json` rather than the app client's configured instance;
- state-holder behaviour, using `lastUpdatePreferencesView` on the fake.

Fix: split it in two, and run the wire assertion against the client's actual `Json` configuration. That configuration is the `encodeDefaults = false` the domain KDoc depends on.

Reproducer: `none — structural`.

**6. `SettingsScreenUiTest.kt` (`theWorkoutModeControlKeepsOneLineAtPhoneWidth`): Testing principles / traceability.**

This is the only new test without a `@Feature` tag; the other new tests in the file all have one.

Fix: add a `workout-session-view.settings.*` tag, or say why it is untagged.

Reproducer: `grep -n -B1 "fun theWorkoutModeControlKeepsOneLineAtPhoneWidth" shared/src/desktopTest/kotlin/com/gymie/shared/ui/SettingsScreenUiTest.kt` shows no `@Feature` line.

**Tradeoffs I'm accepting as defensible:**
- The nullable `sessionView` on `updatePreferences` means "leave unchanged". That's a small, documented three-state value, not a mode flag.
- The server default `FOCUS` appears in both `WorkoutPreferences` and `OptimisticWrite(SessionView.FOCUS)`. The existing `KG` default already follows that pattern, and each copy is commented.

### Assessment
**Principles-compliant?** With fixes

**Reasoning:** The feature reuses the existing optimistic-write machinery, and its concurrency choices are documented and deliberate. But `loadFailed` is a stored value that summarises two fields, and the unit write's success at `:295` still clears it based on one of them. That breaks Single Source of Truth and can hide the retry while an unconfirmed workout mode is on screen. The copied failure policy (finding 2) is what made that drift possible.