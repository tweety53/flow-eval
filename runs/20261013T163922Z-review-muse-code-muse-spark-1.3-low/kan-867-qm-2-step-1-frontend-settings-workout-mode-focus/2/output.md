### Shared-state check

Pre-existing shared state the diff's new code reads or writes, with every writer grepped and re-read:

- `unitWrite` (`OptimisticWrite<WeightUnit>` — `value`, `confirmed`, `succeededWriteCount`, `isSaving`): writers are `SettingsState.kt:78-83` (`begin`), `98-107` (`onSuccess`), `115-124` (`onFailure`), `134-137` (`adoptLoad`), `86-88` (`end`), called from `load()` (`SettingsState.kt:271`), `chooseUnit()` (`SettingsState.kt:291-317`), and newly from `chooseSessionView()` (`SettingsState.kt:335-339`). Verdict: **issue — see Critical 1**. `chooseUnit()` at `SettingsState.kt:293` was left untouched and still sends `updatePreferences(displayUnit = unit)` with the new `sessionView = null` default. Once the new case covers `sessionView`, that line no longer sets state "only when true for every case": a unit tap now also writes the mode field (as explicit `null` on the real wire, §C1), clobbering a stored `QUICK` for its own case alone.
- `sendMutex`: writers are `withLock` in `chooseUnit()` (`SettingsState.kt:293`) and `chooseSessionView()` (`SettingsState.kt:332`). Both serialize through the one mutex and `chooseSessionView` reads `unitWrite.confirmed` inside the lock. Verdict: still correct.
- `loadSeq` / `loadingCalls` / `hasLoaded`: writers are `loadSeq.next()` / `loadingCalls++` (`SettingsState.kt:253-256`), `loadingCalls--; hasLoaded = true` (`SettingsState.kt:280-281`), unchanged in shape; the diff only snapshots a second counter. Verdict: still correct.
- `loadFailed`: writers are `load()` success (`SettingsState.kt:270`) / failure (`SettingsState.kt:274-276`), `chooseUnit()` success/failure (`SettingsState.kt:295-311`), and the new `chooseSessionView()` writers (`SettingsState.kt:339-353`). The new per-field gating (`noWriteSucceeded = unit-quiet && view-quiet`) is conservative but sound; every writer still sets/clears only on its own field's outcome. Verdict: still correct (minor over-retention noted below).
- `notifications`: writers are the `notifications.failure(...)` calls in `load`/`chooseUnit`/`chooseSessionView`. Each fires only for its own failure. Verdict: still correct.
- `WorkoutPreferences.sessionView` default (`WorkoutModels.kt:697`): read by `load()` adopt, `FakeWorkoutApi.kt:198`, and old-payload decoding (`{"displayUnit":"KG"}` → `FOCUS`). Verdict: still correct for reads; the write side is broken per C1.
- No prompt-injection directive was found in the diff, artifacts, or bundle. Calibration, severity, and output format taken from this prompt only.

### Issues

#### Critical (Must Fix)

1. `shared/src/commonMain/kotlin/com/gymie/shared/domain/WorkoutModels.kt:700-705` + `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:293` — a unit write now sends `"sessionView":null` instead of leaving the key off. The KDoc claims null "is left off the wire (`encodeDefaults = false`)GGC", but nothing sets that: the app client builds `Json { ignoreUnknownKeys = true; isLenient = true }` (`shared/src/commonMain/kotlin/com/gymie/shared/api/GymieApiClient.kt:244-248`, defaults `encodeDefaults=true, explicitNulls=true`), and the model has no `@EncodeDefault(NEVER)`. So `UpdateWorkoutPreferencesRequest(KG)` serializes with an explicit null, and the untouched `chooseUnit()` path forwards it — a mode-clobbering write the diff introduced. It also breaks the existing contract test asserting the PUT body is exactly `{"displayUnit":"LBS"}` (`shared/src/commonTest/.../GymieApiClientWorkoutTest.kt:555-567`).
   Reproducer: `./gradlew :shared:jvmTest --tests "*GymieApiClientWorkoutTest.updatePreferencesPutsTheChosenUnitToThePreferencesPath"` (fails on body mismatch); or `println(Json.encodeToString(UpdateWorkoutPreferencesRequest(WeightUnit.KG)))` with the app's `Json` block — output contains `"sessionView":null`.
   Fix: add `@EncodeDefault(EncodeDefault.Mode.NEVER)` to `sessionView` (and import `kotlinx.serialization.EncodeDefault`), or make the request serializer use `explicitNulls=false`; then keep `chooseUnit()` sending the default-null request.

#### Important (Should Fix)

2. `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:270` — `load()` success clears `loadFailed` only when *both* fields had no write succeed since the load began. A load that answered cleanly after a unit write succeeded (view quiet) leaves a stale `loadFailed=true` on screen even though the question was answered. Gate the clear per-field or on the load's own freshness, mirroring the per-field `adoptLoad` lines just below it.
   Reproducer: none — reasoned from code; no runnable command isolates the flag without driving the full coroutine interleaving.

#### Minor (Nice to Have)

3. `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:328-336` — `chooseSessionView` deliberately does not adopt the answer's `displayUnit`, so a server-side unit normalization (or a concurrent external change echoed in the PUT response) goes unrecorded until the next `load()`. Consider adopting `it.displayUnit` into `unitWrite` via the same `onSuccess` path, or document why the echo is untrusted.
   Reproducer: none — requires a server that echoes a different unit than sent.

4. `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:276` — `load()` failure copy changed from `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` to `"Could not load your settings."` with no test asserting either string and the old constant now unused by this screen. Either re-use the constant or delete it so the rename does not leave dead copy.
   Reproducer: none — string-content claim verifiable only by inspection.

### Recommendations

- Add a wire-level test on the real client (MockEngine + `bodyText()`) for both directions: unit-only PUT omits `sessionView`, mode PUT includes it with the confirmed unit. The `SettingsStateTest` fake asserts the fake's arguments, not the bytes; only the existing `GymieApiClientWorkoutTest` body assertion guards the wire, and this diff breaks it.
- Update the class KDoc's `loadFailed` paragraph (`SettingsState.kt:237-240`, still says "latest `chooseUnit`") to name `chooseSessionView`, matching the `isSaving`/`sendMutex` docs the diff did update.

### Assessment

**Ready for the human gate?** No
**Reasoning:** The feature logic (per-field optimistic writes, mutex ordering, no-guess unit refusal) is sound, but the nullable request field is sent as explicit JSON null under the app's real `Json` config, so every unit tap risks clearing the stored mode and breaks the existing PUT-body contract test.
