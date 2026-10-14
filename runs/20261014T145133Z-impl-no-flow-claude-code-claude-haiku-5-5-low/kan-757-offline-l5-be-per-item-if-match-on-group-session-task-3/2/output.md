The floor set PATCH now takes an optional `expected` and answers `409 set_changed` on a mismatch. The tests pass, but the commit did not go through: no git identity is configured here, so the five files are staged on top of `6805058`. Run `git config user.name` and `git config user.email`, then commit with the message I drafted (ending in the `Co-Authored-By: Claude Code` line).

**Changes (all under `src/app/groupsession`)**
- `dto/GroupSessionDtos.kt`: added `PatchFloorSetRequest` (`weight`, `effort`, `confirmed`, optional `expected`) and `FloorSetValuesDto`. Both use the required-field and `@JsonAnySetter` rejection pattern from `PatchWorkoutSetRequest`.
- `GroupSessionWorkoutController.kt`: `patchSet` takes `PatchFloorSetRequest` and passes `actorId = UUID.fromString(authentication.name)` and `expected` converted via the two web converters.
- `GroupSessionExceptionHandler.kt`: maps `WorkoutSetChangedException` to `409`, detail "This set was changed on another phone.", `code: set_changed`, `current` in wire shape, and `updatedBy` / `updatedAt`.
- `GroupSessionWorkoutControllerTest.kt`: the `patch set delegates…` stub and verify now expect `actorId = owner, expected = null`.
- `GroupSessionFloorSetConflictTest.kt` (new, 4 tests): `expected` reaches the service as `SetValues` with the driver as actor; a stale set answers 409 with `current`, `updatedBy` and `updatedAt`; an unstamped set answers 409 without those two keys; an unknown field answers 400.

**Commands run**
- `./gradlew ktlintFormat`: passed (first run with `--offline` failed because ktlint wasn't cached; rerun online passed).
- `./gradlew verifyChange :groupsession:test --tests '*GroupSessionFloorSetConflictTest*' --tests '*GroupSessionWorkoutControllerTest*'`: BUILD SUCCESSFUL. `GroupSessionFloorSetConflictTest` 4/4, `GroupSessionWorkoutControllerTest` 17/17.
- `git add` of the five files: succeeded. `git commit`: failed with "Author identity unknown".

**Deviation from the spec:** the spec says an unstamped set gives `updatedBy: null` and `updatedAt: null`. The response omits both keys instead, because `ProblemDetail` drops null properties. The existing `handleConflict` handler relies on the same omission, and the new test asserts absence. Say if you want explicit nulls, which would need a different serialization path.