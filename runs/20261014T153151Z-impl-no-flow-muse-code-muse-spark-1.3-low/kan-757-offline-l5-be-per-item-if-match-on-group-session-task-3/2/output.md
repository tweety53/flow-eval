Done. The floor set PATCH now takes `expected` and answers `409 set_changed`, committed as `4937bc3`.

What changed:
- [GroupSessionDtos.kt](/work/src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/dto/GroupSessionDtos.kt): new `PatchFloorSetRequest` (weight, effort, confirmed, optional `expected`) and `FloorSetValuesDto`, both with the same `@field:JsonProperty(required = true)` required fields and `@JsonAnySetter` unknown-field rejection the solo `PatchWorkoutSetRequest` uses.
- [GroupSessionWorkoutController.kt](/work/src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionWorkoutController.kt): `patchSet` takes the new DTO and passes `actorId` (the authenticated driver) plus `expected` converted to `SetValues` through the two web converters.
- [GroupSessionExceptionHandler.kt](/work/src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionExceptionHandler.kt): new `WorkoutSetChangedException` handler answering `409` with `detail` `This set was changed on another phone.`, `code: set_changed`, `current` in wire shape via the web converters, `updatedBy` (`{id, displayName}` via `profileOf`) and `updatedAt`, each omitted when the set was never stamped.
- Tests: new [GroupSessionFloorSetConflictTest.kt](/work/src/app/groupsession/src/test/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionFloorSetConflictTest.kt) with the task's four tests, and the existing `patch set delegates…` stub updated to verify `actorId = owner, expected = null`.

Commands run:
- `./gradlew ktlintFormat` — BUILD SUCCESSFUL.
- `./gradlew verifyChange :groupsession:test --tests '*GroupSessionFloorSetConflictTest*' --tests '*GroupSessionWorkoutControllerTest*'` — BUILD SUCCESSFUL; result XMLs confirm 4/4 new tests and 17/17 existing controller tests green, zero failures.