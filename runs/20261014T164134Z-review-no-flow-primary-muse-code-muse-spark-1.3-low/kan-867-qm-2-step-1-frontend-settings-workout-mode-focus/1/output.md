Reviewed the uncommitted preferences/mode diff. The top finding is that the "null means unchanged and is left off the wire" contract is not actually enforced by the serializer in use, so unit writes risk sending an explicit null mode.

#### Critical
- `shared/src/commonMain/kotlin/com/gymie/shared/domain/WorkoutModels.kt:700`: `UpdateWorkoutPreferencesRequest.sessionView = null` is documented as omitted via `encodeDefaults = false`, but neither the model (no `@EncodeDefault(Mode.NEVER)`) nor the wire `Json` in `shared/src/commonMain/kotlin/com/gymie/shared/api/GymieApiClient.kt:245` (only `ignoreUnknownKeys`/`isLenient`) enables that, so a unit-only write serializes `"sessionView":null` instead of omitting the key.

#### Important
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/AppScreens.kt:248`: `WorkoutModeSetting` (and its divider at line 243) is nested inside `if (profile != null)` at line 233, so a preferences-backed row gated on `settings.hasLoaded` is additionally hidden until the unrelated profile fetch answers.
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339`: `chooseSessionView` adopts only `it.sessionView` and deliberately drops the answer's `displayUnit`, leaving `unitWrite.confirmed` stale if the server ever echoes or normalizes a different unit, which later mode writes then resend.
- `shared/src/commonTest/kotlin/com/gymie/shared/ui/SettingsStateTest.kt:712`: the "unit write omits the mode" check encodes with a bare `Json` instance rather than the wire `Json`, so it asserts nothing about what `WorkoutApiImpl` actually sends and will disagree with the wire config.

#### Minor
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:237`: `loadFailed` KDoc still describes only `chooseUnit` failing with nothing confirmed, omitting the new `chooseSessionView` path that also sets it.
