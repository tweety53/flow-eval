### Shared-state check

- **`loadFailed`** (`ui/SettingsState.kt`). Lines that write it:
  - `:270` load success clears it only when no write of either field succeeded during the load. **Still correct**, and conservative.
  - `:275` load failure sets it under the same check. **Still correct.**
  - `:295` `chooseUnit` success clears it whenever the unit write is current. **Now wrong** — this is Important #1 below.
  - `:310` `chooseUnit` NOTHING_CONFIRMED sets it. **Still correct.**
  - `:339` `chooseSessionView` success clears it. **Still correct**: this write can only be sent once the unit is confirmed, so both fields are confirmed when it succeeds.
  - `:351` `chooseSessionView` NOTHING_CONFIRMED sets it. **Still correct.**
- **`sendMutex`**: no fields are written. Both write functions now go through it in order, and the mode write reads `unitWrite.confirmed` while holding the lock. **Correct**, and the queued-unit test covers it.
- **`isSaving`** (`:234`) now also covers `viewWrite`. Nothing outside `SettingsState` reads it, so the wider meaning has no effect elsewhere. **Correct.**
- **`unitWrite.confirmed`**: written only at `OptimisticWrite.onSuccess` and `adoptLoad`, as before. The new code only reads it. **Correct.**

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. A unit write clears the retry flag while the workout mode still shows a value the server never confirmed** — `ui/SettingsState.kt:295`
- **What's wrong:** before this diff, `loadFailed` described one field only. Now `viewWrite` can raise it too, at `:351`, and leave the mode control showing a value the server never confirmed. `chooseUnit`'s success at `:295` still clears the flag based on the unit field alone. It also ignores the `sessionView` that comes back in its own response.
- **Failure sequence:**
  1. `load()` fails.
  2. The user taps Quick. No PUT is sent; the view stays QUICK (NOTHING_CONFIRMED) and `loadFailed` is set to true.
  3. The user then picks LBS, and that write succeeds.
  4. `loadFailed` becomes false, so the retry path disappears. The mode control still shows QUICK, but the server holds FOCUS, and `viewWrite.confirmed` stays null.
- **Fix:** on a unit write's success, adopt `it.sessionView` into `viewWrite` when nothing has been confirmed for it yet (the response is authoritative). Alternatively, clear `loadFailed` only when both `unitWrite.confirmed` and `viewWrite.confirmed` are non-null.
- **Reproducer:** none — no test covers this order. It would be a `SettingsStateTest` case: failed `load`, then `chooseSessionView(QUICK)`, then a successful `chooseUnit(LBS)`, then assert `state.sessionView == FOCUS || state.loadFailed`. With the current code that assertion fails.

#### Minor (Nice to Have)

**2. The load-failure fallback message is now a literal instead of the shared constant** — `ui/SettingsState.kt:276`
- The fallback text is now `"Could not load your settings."` written inline, and the `FailureMessages` import was removed.
- A grep finds no remaining use of `COULD_NOT_LOAD_WEIGHT_UNIT`, so the constant is probably dead now.
- Either update the constant (and rename it) or delete it, so the copy stays in one place.
- **Reproducer:** `grep -rn COULD_NOT_LOAD_WEIGHT_UNIT shared/src` finds no remaining use.

**3. "Could not save your workout mode." is repeated three times** — `ui/SettingsState.kt:335-352`
- The same string appears three times, and at `:335` it is the message of an `error(...)` call, so it is also what the fallback shows.
- Pull it into a single constant.
- **Reproducer:** none — duplicated copy, no wrong behaviour.

**4. The new UI test can't detect a dropped unit** — `SettingsScreenUiTest.kt` (`tappingQuickSavesItAtOnce`)
- The mock always answers `"displayUnit":"KG"`, and KG is also the loaded unit.
- So the assertion that the unit was carried would still pass if the code sent a hard-coded default instead of the confirmed unit.
- Loading the unit as LBS would make the test meaningful.
- **Reproducer:** none — this is a gap in the test, not a runtime defect.

### Recommendations
- **Wire format:** check that the backend treats a PUT with no `sessionView` key as "leave it unchanged" and doesn't reset it to the default. The client relies on `encodeDefaults = false`, and the test at `SettingsStateTest` only checks the encoding, not what the server does.
- **Older servers:** a response without `sessionView` silently reads as FOCUS. That is fine for this payload, but say so in the PR description as the backward-compatibility story.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The new field, the wire format and the ordering under `sendMutex` are sound and well tested. The one real problem is that `chooseUnit` still clears `loadFailed` as if the unit were the only field, which can hide the retry path while the mode control shows a value the server never confirmed.