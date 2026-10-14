I found one Important defect and three Minor ones. Nothing is Critical, and nothing in the diff tried to give me instructions. I only read files, using `git`, `grep` and `sed`. The working tree, index and branch are unchanged.

### Shared-state check

| State reused by the new code | Writers (`path:line`) | Verdict |
|---|---|---|
| `loadFailed` (`SettingsState.kt:242`) | `SettingsState.kt:271` (load success: `current && noWriteSucceeded`) | Still correct. A view write that succeeds during the load has already cleared it itself. |
| | `SettingsState.kt:275` (load failure) | Still correct for the cases it handles. It shares the cause of Important #1 below when a unit write succeeds during a retry load. |
| | `SettingsState.kt:295` (`chooseUnit` success: `if (unitWrite.onSuccess(...)) loadFailed = false`) | **Defect, now Important #1.** The flag now also covers the workout mode, but this unchanged line clears it once the unit alone is confirmed. |
| | `SettingsState.kt:~307` (`chooseUnit` NOTHING_CONFIRMED) | Still correct. Raising the flag is right whatever the field. |
| | `SettingsState.kt:~340` (`chooseSessionView` success) | Still correct. It can only run once the unit is confirmed (the guard requires `unitWrite.confirmed`), so both fields are confirmed when it clears. |
| | `SettingsState.kt:~350` (`chooseSessionView` NOTHING_CONFIRMED) | Still correct. |
| `sendMutex` | `chooseUnit:293`, `chooseSessionView:~333` | Still correct. Reading `unitWrite.confirmed` inside the lock gives the server's latest unit, because sends run in tap order and a unit write's success updates the confirmed value even when a newer tap has taken over the control. |
| `unitWrite.confirmed` / `succeededWriteCount` | `OptimisticWrite.onSuccess:~101`, `adoptLoad:~133` | Still correct. A mode write re-sends the unit the server already holds, so not counting it as a unit write is sound. |
| `isSaving` | Derived value, no writers | Still correct. |
| `FakeWorkoutApi.preferencesResult` | `FakeWorkoutApi.kt:~200` (after each update), test setters | Still correct. The fake reads the stored view before the hook runs, so its default answer behaves like the server's "unchanged" rule. |

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. A successful unit save clears `loadFailed` while the workout mode was never loaded, which removes the only way to re-sync it.**
- **Where:** `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295` (an unchanged line). The same gap applies to the load-failure gate at `:274` (`current && noWriteSucceeded`).
- **What happens:**
  1. The first `load()` fails. `hasLoaded` becomes true and `loadFailed` becomes true.
  2. `SettingsScreen` now draws both rows with made-up defaults: KG and FOCUS (`AppScreens.kt:221-247`).
  3. The athlete taps LBS. `chooseUnit` sends a PUT with no `sessionView`, so the server keeps QUICK. The write succeeds, and line 295 sets `loadFailed = false`.
  4. The Retry button (`settings_display_unit_retry`, which only shows while `loadFailed` is true) disappears.
- **Result:** the Workout mode row keeps showing FOCUS, a value the server never sent, while the server holds QUICK. Nothing in the UI offers a re-sync, and `viewWrite.confirmed` stays null for the rest of the screen's life.
- **Why it matters:** before this diff, `loadFailed` only covered the unit, so clearing it on a unit success was right. Now it also covers the mode, and this writer only knows about one of the two fields. This is exactly the "default shown as if it were the truth" outcome the class KDoc forbids. It also contradicts the `WorkoutModeSetting` KDoc ("never a default Focus").
- **Fix:** the PUT answer is the full `WorkoutPreferences`. In `chooseUnit`'s `onSuccess`, adopt `it.sessionView` into `viewWrite` with an adopt that respects in-flight writes, the way `adoptLoad` does. Alternatively, clear `loadFailed` only when both `unitWrite.confirmed` and `viewWrite.confirmed` are non-null. Add a `SettingsStateTest` for this sequence.
- **Reproducer:** none — it needs a new test, and this review is read-only. The test to add to `SettingsStateTest`:
  - Set `api.preferencesResult = failure`, then call `state.load()`.
  - Set `api.preferencesResult = success(WorkoutPreferences(KG, QUICK))`, then call `state.chooseUnit(LBS)`.
  - Assert `state.loadFailed || state.sessionView == QUICK`. This fails today: `loadFailed` is false and `sessionView` is FOCUS.
  - Run it with `./gradlew :shared:desktopTest --tests '*SettingsStateTest*'`.

#### Minor (Nice to Have)

**2. The load-failure message no longer uses the shared constant.**
- **Where:** `SettingsState.kt:276`.
- The `FailureMessages` import was dropped in favour of the inline literal `"Could not load your settings."`. The `"Could not save your workout mode."` literal also appears three times (`:335`, `:~346`, `:~351`).
- `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` is still used by `WorkoutEditorScreen.kt:387`, so the constant isn't dead.
- **Suggestion:** add `COULD_NOT_LOAD_SETTINGS` and `COULD_NOT_SAVE_WORKOUT_MODE` to `FailureMessages` to match the file's existing pattern.
- **Reproducer:** none — wording/consistency only.

**3. Decoding a preferences payload that lacks `sessionView` is only tested indirectly.**
- **Where:** `WorkoutModels.kt:697`.
- The default to FOCUS matters for compatibility with older servers. It is only exercised through the UI test's mock GET (`{"displayUnit":"KG"}`), not in `WorkoutModelsSerializationTest`.
- `GymieApiClientWorkoutTest.kt:555` also doesn't check that a unit-only PUT body leaves `sessionView` out. The new state test checks this with a default `Json`, which happens to match the client's settings (`GymieApiClient.kt:244-248`) but doesn't go through the real client.
- **Suggestion:** add one decode test and one MockEngine body assertion.
- **Reproducer:** none — test-coverage gap.

**4. `theWorkoutModeControlKeepsOneLineAtPhoneWidth` has no `@Feature` tag.**
- **Where:** `SettingsScreenUiTest.kt:~180`. Every other new test has one.
- **Reproducer:** none — traceability only.

### Recommendations
- There are no change artifacts for KAN-865/867 in `openspec/changes` or `spectre/changes`, so I could only check the diff against its own KDocs and test names, not against a plan. Please attach the task or design for KAN-867 for the human gate.
- The interface change to `updatePreferences` is source-compatible: the new parameter is defaulted, and the only implementations are `WorkoutApiImpl` and `FakeWorkoutApi`. Nothing in this repo confirms the backend accepts `sessionView` on PUT or returns it on GET. The client copes with an older server (`ignoreUnknownKeys` and the FOCUS default), but the KDoc's claim that "null means unchanged" depends on the server. Confirm the backend change (KAN-865) is deployed first.
- The class KDoc says "everything below … holds unchanged for `viewWrite`". Issue #1 shows that isn't quite true for `loadFailed`. Update that sentence once #1 is fixed.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The two-field optimistic-write bookkeeping and the mutex-ordered "send the confirmed unit" logic are sound and well tested. However, the unchanged `loadFailed = false` in `chooseUnit` now hides a workout mode that was never loaded behind a vanished Retry button after a failed load. That needs fixing and a test before merge.