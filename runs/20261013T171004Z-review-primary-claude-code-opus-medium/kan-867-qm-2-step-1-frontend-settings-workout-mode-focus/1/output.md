## Review of KAN-867: the Workout mode setting

I couldn't find any plan files for KAN-867 under `openspec/`, `spectre/` or `docs/`. So I checked the diff against the claims made in its own comments and docs, not against a plan. I didn't run Gradle; every point below comes from reading the code. The diff contains nothing that tries to give the reviewer instructions.

### Shared-state check

Existing state that the new code reads or writes: `loadFailed`, `sendMutex`, `unitWrite.confirmed`, `unitWrite.succeededWriteCount`, `loadSeq`, `hasLoaded`, `isSaving`.

**`loadFailed`** used to mean "the unit has no confirmed value". It now also has to mean "the workout mode has no confirmed value". Every line that writes it:
- `SettingsState.kt:270` (a load succeeds and no write succeeded meanwhile): still correct. Both fields are adopted together on lines 271–272.
- `SettingsState.kt:275` (a load fails and no write succeeded meanwhile): still correct.
- `SettingsState.kt:295` (a unit write succeeds): **broken by this diff.** It clears the flag just because the unit is now confirmed, even when the workout mode never was. See Important #1.
- `SettingsState.kt:310` (a unit write fails with nothing confirmed): still correct.
- `SettingsState.kt:339` (a mode write succeeds): still correct. A mode write can only be sent once the unit is confirmed (line 335), so both fields are confirmed after it.
- `SettingsState.kt:351` (a mode write fails with nothing confirmed): still correct.

**The other state:**
- **`sendMutex`:** its only users are `:293` and `:332`, and both take the lock. Still correct.
- **`unitWrite.confirmed`:** it is only written inside `OptimisticWrite.onSuccess` (`:103`) and `adoptLoad` (`:135`). The diff reads it inside the lock (`:335`), after every earlier send has settled. Still correct.
- **`unitWrite.succeededWriteCount`:** only written at `:102`. The load gating at `:264–272` now gates each field separately. Still correct.
- **`loadSeq`:** only written at `:253`. Still correct.
- **`hasLoaded`:** only written at `:281`. The new row is drawn inside its gate (`AppScreens.kt:221`). Still correct.
- **`isSaving`:** it is now computed from both writes (`:235`) and nothing in the UI reads `SettingsState.isSaving`. Still correct.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

**1. A successful unit save after a failed load hides Retry while the Workout mode row shows an unconfirmed Focus.**
- **Where:** `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295`
- **What happens:**
  1. The first load fails, so `loadFailed` is true and `viewWrite.confirmed` is null. The screen still draws all rows, because `hasLoaded` becomes true on failure too.
  2. The user taps LBS and the save succeeds.
  3. Line 295 clears `loadFailed`, so the Retry button (`AppScreens.kt:326`) disappears.
  4. The Workout mode row keeps showing the default `FOCUS`, which the server never confirmed. A user whose stored mode is QUICK now sees Focus with no way to re-sync.
- **Why it's a defect:** the row's own doc (`AppScreens.kt:393`) promises it is "never a default Focus". Also, the response to the unit save already contains the server's real `sessionView`, and it is thrown away.
- **How to fix:** in `chooseUnit`'s `onSuccess`, adopt `it.sessionView` into `viewWrite` the same way a load does (only while no mode write is in flight or has succeeded since). Or, at minimum, clear `loadFailed` only when `viewWrite.confirmed != null`. Add a test for this sequence next to `a write that succeeds clears a retry offer…` (`SettingsStateTest.kt:435`), which currently locks in the old behaviour that only considered the unit.
- **Reproducer:** none. It needs a new test in `SettingsStateTest`:
  - set `preferencesResult` to a failure and call `load()`
  - set `preferencesResult` to `success(KG, QUICK)`
  - call `chooseUnit(LBS)`
  - assert `loadFailed || sessionView == QUICK`

  Today that assertion fails: `loadFailed` is false and `sessionView` is `FOCUS`.

#### Minor (Nice to Have)

**2. Some docs are out of date.**
- **Where:** `SettingsState.kt:237-240` and `:143-144`
- **What's wrong:** the `loadFailed` doc still mentions only `chooseUnit`. The class doc says everything about `unitWrite` "holds unchanged" for `viewWrite`, but the mode write has its own refuse-without-a-confirmed-unit path and shares `loadFailed` across both fields.
- **Why it matters:** these are exactly the rules Important #1 got wrong.
- **Reproducer:** none — documentation only.

**3. The Retry button sits under the wrong row after a failed mode write.**
- **Where:** `AppScreens.kt:326`
- **What's wrong:** when a mode write fails with nothing confirmed, the Retry it raises appears under the Weight unit row, with the tag `settings_display_unit_retry`, two rows away from the control the user just tapped.
- **Reproducer:** none — layout and UX.

**4. Older servers will silently undo a Quick choice.**
- **Where:** `WorkoutModels.kt:697`
- **What's wrong:** if the client ships before the backend returns `sessionView`, the response omits the field and it decodes as `FOCUS`. `viewWrite.onSuccess` then adopts `FOCUS`, so choosing Quick snaps back to Focus with no error shown.
- **Why it matters:** the backend isn't in this repo, so I can't check the deploy order. The PR should state which side ships first.
- **Reproducer:** none — depends on the backend.

**5. One new test has no feature tag.** `theWorkoutModeControlKeepsOneLineAtPhoneWidth` (`SettingsScreenUiTest.kt`) has no `@Feature`, unlike the other new tests.
- **Reproducer:** none — traceability only.

### Recommendations
- Treat `loadFailed` as "some field has no confirmed value", for example `unitWrite.confirmed == null || viewWrite.confirmed == null` combined with the load-failed flag. Then no single writer has to think about the other field.
- Have every successful PUT feed both fields from the response, since the server returns the whole preferences object. That fixes #1 and keeps the two fields in step.
- The new tests mostly exercise real behaviour: a fake that keeps state, and a `MockEngine` that inspects the actual JSON body. The rule that sends the confirmed unit and never one still queued is well covered.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The new mode write, the encoding of the field and the ordering under the shared lock are all sound. But the unit-save success path, which this diff didn't touch, now clears the shared retry flag while the workout mode is still unconfirmed. After a failed load that leaves a stale Focus on screen with no Retry.