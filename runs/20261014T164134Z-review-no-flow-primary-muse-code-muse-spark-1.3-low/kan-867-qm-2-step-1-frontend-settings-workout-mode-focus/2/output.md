Review of the uncommitted diff (`git diff`, 8 files):

The feature itself is coherent — separate `OptimisticWrite` per field, mutex-serialized sends, confirmed-unit carried on mode writes — but the "null means unchanged and is omitted from the wire" contract is not enforced, and the fake hides that from the tests.

#### Critical
- `shared/src/commonMain/kotlin/com/gymie/shared/domain/WorkoutModels.kt:700`: the KDoc claims a null `sessionView` "is left off the wire (`encodeDefaults = false`)", but neither the data class nor the app `Json` (`shared/src/commonMain/kotlin/com/gymie/shared/api/GymieApiClient.kt:245`, only `ignoreUnknownKeys`/`isLenient`) configures `encodeDefaults = false` / `explicitNulls = false`, so a real PUT sends an explicit `"sessionView":null`.
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:293`: `chooseUnit` therefore sends that explicit null on every unit write, risking a server-side mode reset; `shared/src/commonTest/kotlin/com/gymie/shared/api/FakeWorkoutApi.kt:198` treats null as "unchanged", so the tests mask the real-wire behavior.
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:270`: `loadFailed` clearing/raising is gated on `noWriteSucceeded` across *both* fields, so a succeeded write on one field vetoes the `loadFailed` update even when the other field's fresh load payload was just adopted — a stale error banner persists (and a fresh failure can be swallowed).

#### Important
- `shared/src/commonTest/kotlin/com/gymie/shared/api/FakeWorkoutApi.kt:124`: `updatePreferencesRequests` records only units, so the ordering assertion in `shared/src/commonTest/kotlin/com/gymie/shared/ui/SettingsStateTest.kt:745` cannot detect a mode value sent on the wrong PUT.
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339`: `chooseSessionView` discards the answer's `displayUnit`, leaving `unitWrite.confirmed` stale if the server ever normalizes or echoes a different unit.
- `shared/src/commonTest/kotlin/com/gymie/shared/ui/SettingsStateTest.kt:712`: the "leaves the mode off the wire" check uses a default `Json` instance that shares production defaults, so it verifies the interface-level null rather than omission on the real wire.

#### Minor
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/AppScreens.kt:413`: `WorkoutModeSetting` hard-codes `enabled = true`, so the control stays interactive while its write is in flight with no saving/loading guard.
- `shared/src/commonMain/kotlin/com/gymie/shared/domain/WorkoutModels.kt:689`: the KDoc names the backend key `session_view` while the wire key is camelCase `sessionView`, an inconsistent contract reference.