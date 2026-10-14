The floor set PATCH now accepts an optional `expected` and answers a stale one with `409 set_changed`. The tests pass, but the commit is not made: this environment has no git identity configured, so the five files are staged and HEAD is still `6805058 base`.

**Changes**
- `GroupSessionDtos.kt`: added `PatchFloorSetRequest` (weight, effort, confirmed, optional `expected`) and `FloorSetValuesDto`. Both have required fields and the `@JsonAnySetter` rejection, so an unknown field gets a 400.
- `GroupSessionWorkoutController.kt`: `patchSet` takes `PatchFloorSetRequest`, converts `expected` through the two web converters into `SetValues`, and passes `actorId = authentication.name`.
- `GroupSessionExceptionHandler.kt`: maps `WorkoutSetChangedException` to 409 with `code: set_changed`, the detail "This set was changed on another phone.", `current` in wire shape, and `updatedBy` and `updatedAt`.

The `:workout` pieces (`WorkoutSetChangedException`, the `expected` compare in `patchSet`, the transition codes in the handler) were already in the tree, so I didn't touch them.

**Tests**
- New `GroupSessionFloorSetConflictTest` (4 tests): an `expected` body reaches the service with the driver as actor; a stamped set returns the full 409 body; an unstamped set returns no `updatedBy` or `updatedAt`; an unknown field returns 400.
- `GroupSessionWorkoutControllerTest`: the `patch set delegates…` stub and verify now expect actor = owner and `expected = null`.

**Commands run**
- `./gradlew ktlintFormat`: BUILD SUCCESSFUL. Used twice; the first run reformatted my edits.
- `./gradlew verifyChange :groupsession:test --tests '*GroupSessionFloorSetConflictTest*' --tests '*GroupSessionWorkoutControllerTest*'`: first failed on two detekt MaxLineLength findings in the new test, which I fixed. Final run: BUILD SUCCESSFUL.
- JUnit XML: `GroupSessionFloorSetConflictTest` 4 tests, 0 failures; `GroupSessionWorkoutControllerTest` 17 tests, 0 failures.
- `git commit`: failed with "Author identity unknown". I did not set a git identity.

**To finish:** configure a git identity and commit the staged files with the message in the task, ending with the `Co-Authored-By: Claude Code <noreply@anthropic.com>` line.