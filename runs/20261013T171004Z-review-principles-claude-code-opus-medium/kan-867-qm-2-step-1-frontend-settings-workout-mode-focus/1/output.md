### Derived-state check

| State | Facts it summarises | Writers | Verdict |
|---|---|---|---|
| `SettingsState.loadFailed` | (a) the newest `load` failed; (b) `unitWrite` failed with nothing confirmed; (c) since this diff, (c) `viewWrite` failed with nothing confirmed. In short: "some field has no server-confirmed value, so offer retry". | `SettingsState.kt:270` (load success: clears only when no write of *either* field has succeeded) | Partial. A load that is stale for one field still clears the flag for both. Today it is masked by the `noWriteSucceeded` conjunction, so not raised on its own. |
| | | `SettingsState.kt:275` (load failure: sets) | Sets. Fine. |
| | | `SettingsState.kt:295` (unit write success: **clears**) | **Violation.** This is one field's path deciding for both. It clears the flag while `viewWrite.confirmed` can still be `null` (see Critical #1). |
| | | `SettingsState.kt:310` (unit NOTHING_CONFIRMED: sets) | Sets. Fine. |
| | | `SettingsState.kt:339` (view write success: clears) | Holds by accident. The view write only reaches the server once `unitWrite.confirmed != null` (line 335), so both fields are confirmed at that point. Nothing in the code states this coupling. |
| | | `SettingsState.kt:351` (view NOTHING_CONFIRMED: sets) | Sets. Fine. |
| `SettingsState.isSaving` | `unitWrite.isSaving`, `viewWrite.isSaving` | `SettingsState.kt:234-235`, a getter | Derived from every fact. Compliant. |
| `OptimisticWrite.confirmed` / `succeededWriteCount` | Primary state, not a summary | — | n/a |

### Summary
The diff adds a second preference field, `sessionView`, across four places: the shared domain model and API, `SettingsState` (a second `OptimisticWrite` plus `chooseSessionView`), a new Settings row, and tests.

- **Principle groups applied:** all three (Structure, Simplicity & state, Robustness & ops).
- **Project standards:** none resolved, because the **Project standards** field is empty. The Hard Invariants section is therefore empty, and only the universal principle list was applied.
- **Hygiene:** the diff adds no suppressions and touches no lint config.

The API change is clean: a nullable field means "unchanged" and is documented on both the interface and the request DTO. Reading the confirmed unit inside `sendMutex` is a deliberate tradeoff that is explained well. The main problem is that the stored `loadFailed` flag was built for one field and was not reworked when a second field was added.

### Issues

#### Critical (Must Fix)

**1. `SettingsState.kt:295`: a unit write clears `loadFailed` while the workout mode has nothing confirmed**

- **Rule:** Single Source of Truth ("Each fact … is stored … in exactly one place; everything else derives from it").
- **What happens:** `loadFailed` stands for "some field has no confirmed server value, so offer retry". Before this diff there was one field, so `unitWrite.onSuccess(...) → loadFailed = false` was exact. With two fields, the unit's success path decides for the view too.
- **Sequence:**
  1. `load()` fails, so `loadFailed = true` and both `confirmed` values are null.
  2. The user taps LBS, and `chooseUnit` succeeds.
  3. Line 295 sets `loadFailed = false`.
- **Result:** the retry affordance disappears. The Workout mode control keeps showing `FOCUS`, the initial default at `SettingsState.kt:212`, which the server never confirmed. If the stored value is `QUICK`, the screen now shows a guess as fact, which the class KDoc explicitly forbids ("never a guess").
- **Fix:** stop storing the flag and derive it. For example, keep a `lastLoadFailed` fact and expose `val loadFailed get() = unitWrite.confirmed == null || viewWrite.confirmed == null` (gated on `hasLoaded`/`lastLoadFailed` as needed). Alternatively, have every success path clear the flag only when *all* fields' `confirmed` values are non-null.
- **Reproducer:** `none — no test covers it`. Add this to `SettingsStateTest`: set `preferencesResult = Result.failure(...)` and call `load()`; then set `preferencesResult = Result.success(WorkoutPreferences(KG, QUICK))` and call `chooseUnit(LBS)`. Expect `loadFailed == true`, or `sessionView == QUICK`. Against this diff, `loadFailed` is `false` and `sessionView` is `FOCUS`.

#### Important (Should Fix)

**2. `SettingsState.kt:328-358` duplicates the write-outcome handling from `chooseUnit` (`:288-318`)**

- **Rule:** DRY ("Every piece of knowledge has one authoritative representation").
- **Why it matters:** `OptimisticWrite` was extracted specifically to end this repetition (KAN-71 F4). Its KDoc at `SettingsState.kt:37-38` says that only "the network call itself and the failure copy" differ between fields. Yet the whole `runCatching / onSuccess → clear loadFailed / when(STALE, REVERTED_TO_CONFIRMED, NOTHING_CONFIRMED → raise loadFailed)` policy is copied word for word. Finding #1 lives in exactly this duplicated policy: a fix to the flag now has to be applied twice.
- **Fix:** add a private `suspend fun <T> write(field: OptimisticWrite<T>, optimistic: T, failureCopy: String, send: suspend () -> T)` that owns `begin/end`, the mutex, and the outcome `when`. Then `chooseUnit` and `chooseSessionView` each become one call.
- **Reproducer:** `diff <(sed -n 291,317p shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt) <(sed -n 338,357p shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt)`. The only differences are the field name, the message, and one comment.

#### Minor (Nice to Have)

**3. `SettingsState.kt:276`: the shared message constant was replaced with an inline literal, leaving the constant's KDoc false**

- **Rules:** Single Source of Truth, Least Astonishment.
- **What happened:** the diff drops `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` in favour of the literal `"Could not load your settings."`. But `components/FailureHandling.kt:11-15` still documents that constant as "Shared by `SettingsState.load` and the workout editor", which is no longer true.
- **Fix:** add a `COULD_NOT_LOAD_SETTINGS` constant, or update that KDoc.
- **Reproducer:** `grep -n "SettingsState.load" shared/src/commonMain/kotlin/com/gymie/shared/ui/components/FailureHandling.kt; grep -n "COULD_NOT_LOAD_WEIGHT_UNIT" shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`. The first prints the claim; the second prints nothing.

**4. `SettingsState.kt:335, 347, 352`: the same failure copy is written three times in one function**

- **Rule:** DRY. The unit path has the same twofold duplication.
- **Fix:** hoist the string into a `private const`, or let finding #2's helper take it as a parameter.
- **Reproducer:** `grep -c "Could not save your workout mode." shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt` returns `3`.

**5. `SettingsState.kt:212` and `WorkoutModels.kt:697`: the server's default (`FOCUS`) is hardcoded in two places**

- **Rule:** Single Source of Truth ("a default value hardcoded in both code and config").
- **Context:** the KDoc at `SettingsState.kt:214` says it is "matching the server's default". `KG` follows the same existing pattern, so this is defensible.
- **Fix:** have one `WorkoutPreferences.DEFAULT` that both places read.
- **Reproducer:** `grep -n "SessionView.FOCUS" shared/src/commonMain/kotlin/com/gymie/shared/domain/WorkoutModels.kt shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`

**6. `SettingsStateTest.kt` ("a unit write leaves the workout mode off the wire") asserts two separate facts**

- **Rule:** Testing principles ("fail for exactly one reason").
- **What happens:** the test checks both the serializer's `encodeDefaults` behaviour and the fake's recorded `lastUpdatePreferencesView`.
- **Fix:** split it into a serialization test and a state test.
- **Reproducer:** `none — structural; visible in the test body`

**Considered and not raised:**
- The non-exhaustive `if (it == SessionView.FOCUS) "Focus" else "Quick"` label at `AppScreens.kt:412` matches the existing `BodyUnit` precedent at `AppScreens.kt:382`.
- The nullable "unchanged" parameter on `updatePreferences` is documented and is a deliberate wire-compatibility tradeoff.
- Deleting `sessionViewIsGoneFromSettings` is fine: the test was replaced because the behaviour it covered was deliberately reversed, not dropped silently.

### Assessment
**Principles-compliant?** With fixes

**Reasoning:** The API and model changes, and the mutex-ordered "send the confirmed unit" design, are sound and their tradeoffs are written down. But `loadFailed` stays a stored summary that one field's success path clears for both fields, which is a Single Source of Truth violation with visible wrong behaviour (the retry disappears while an unconfirmed mode is on screen). That bug sits inside write-handling logic that was copied rather than shared through the existing `OptimisticWrite` abstraction. Deriving `loadFailed` from each field's `confirmed` state and moving the outcome policy into one helper fixes both.