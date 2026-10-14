The floor set PATCH now takes an optional `expected` and answers `409 set_changed` on a mismatch. The change is staged but not committed: git has no author identity configured here, and I didn't set one for you. Run `git commit` with the message from the task, ending with `Co-Authored-By: Claude Code <noreply@anthropic.com>`.

**Changes**
- `GroupSessionDtos.kt`: added `PatchFloorSetRequest` (`weight`, `effort`, `confirmed` required, optional `expected`) and `FloorSetValuesDto`. Both reject unknown fields with `@JsonAnySetter`, like `PatchWorkoutSetRequest`.
- `GroupSessionWorkoutController.kt`: `patchSet` takes `PatchFloorSetRequest`, passes the driver (`authentication.name`) as `actorId`, and converts `expected` through `SetWeightWebConverter`/`SetEffortWebConverter`.
- `GroupSessionExceptionHandler.kt`: maps `WorkoutSetChangedException` to `409` with `detail`, `code: set_changed`, `current`, `updatedBy` (via the existing `actorOf`) and `updatedAt`.
- Tests: new `GroupSessionFloorSetConflictTest` (4 cases) and the `patch set delegates` stub/verify updated to `actorId = owner, expected = null`.

The service method, the `WorkoutSetChangedException`, the set stamps and the transition codes were already in the repo, so I didn't touch them. I also wrote the DTO and controller before the failing tests, not in the order the task lists.

**Commands run**
- `./gradlew ktlintFormat`: BUILD SUCCESSFUL.
- `./gradlew verifyChange :groupsession:test --tests '*GroupSessionFloorSetConflictTest*' --tests '*GroupSessionWorkoutControllerTest*'`: first run had one failure, a test bug where `workout()` built a fresh `Workout` on each call. After switching to a single stub instance, BUILD SUCCESSFUL. `GroupSessionFloorSetConflictTest` 4/4 passed, `GroupSessionWorkoutControllerTest` 17/17 passed.
- `git add` of the five files: succeeded.
- `git commit`: failed with "Author identity unknown".