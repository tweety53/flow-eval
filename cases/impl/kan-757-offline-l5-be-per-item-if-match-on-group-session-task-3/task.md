- [ ] 3. The floor set PATCH takes expected and answers set_changed

**Build:** green
**Files:** `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/dto/GroupSessionDtos.kt`, `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionWorkoutController.kt`, `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionExceptionHandler.kt`, `src/app/groupsession/src/test/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionWorkoutControllerTest.kt`, `src/app/groupsession/src/test/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionFloorSetConflictTest.kt`
**Tests:** `expected reaches the service with the driver as actor`, `a set_changed refusal carries current updatedBy and updatedAt`, `a never-stamped set answers null updatedBy and updatedAt`, `an unknown field is still refused`
**Regression:** revert → the floor set PATCH rejects `expected` as an unknown field and stays last-write-wins; OFF-L5-2 never gets a conflict to draw.
**Baseline:** before=0 after=4
<!-- predicted: grep -c '@Test' GroupSessionFloorSetConflictTest.kt after this task -->
**Commit:** `feat(groupsession): the floor set patch refuses a stale expected with set_changed`
**After:** Task 1, 2

**Decision:** value-compare-and-set
**Decision:** floor-set-request-dto
**Decision:** set-changed-mapped-in-groupsession

  - [ ] **Step 1: Read first.** `PatchWorkoutSetRequest` (`:workout` `dto`, ~line 703: required
    nullable fields plus the `@JsonAnySetter` unknown-field rejection), `SetWeightWebConverter` /
    `SetEffortWebConverter` (`toDomain`, `toDto`), and `GroupSessionWorkoutControllerTest`'s
    `patch set delegates with the resolved workout id`.
  - [ ] **Step 2: Failing tests.** `GroupSessionFloorSetConflictTest` (same `@WebMvcTest` config as
    `GroupSessionWorkoutControllerTest`): a body with `expected` → `patchSet(jana, workoutId, setId, …, actorId = owner, expected = SetValues(…))`;
    the service throwing `WorkoutSetChangedException` with a stamped set → `409`, `code: set_changed`,
    `current` in wire shape, `updatedBy: {id, displayName}`, `updatedAt`; an unstamped set → both
    `null`; a body with `foo` → `400`.
  - [ ] **Step 3: DTO.** In `GroupSessionDtos.kt`: `PatchFloorSetRequest(weight, effort, confirmed, expected: FloorSetValuesDto? = null)`
    and `FloorSetValuesDto(weight, effort, confirmed)`, the same `@field:JsonProperty(required = true)`
    and `@JsonAnySetter` rejection `PatchWorkoutSetRequest` uses (on both classes; `expected` itself
    optional).
  - [ ] **Step 4: Controller.** `patchSet` takes `PatchFloorSetRequest` and calls `patchSet` with
    `actorId = UUID.fromString(authentication.name)` and `expected` converted through the two web
    converters. Update the existing `patch set delegates…` stub and verify to the new call (actor =
    `owner`, `expected = null`).
  - [ ] **Step 5: Mapping.** `GroupSessionExceptionHandler` handles `WorkoutSetChangedException`:
    `409`, `detail` `This set was changed on another phone.`, `code: set_changed`, `current` =
    `mapOf("weight" to SetWeightWebConverter.toDto(…), "effort" to SetEffortWebConverter.toDto(…), "confirmed" to (confirmedAt != null))`,
    `updatedBy` via `profileOf` as in Task 2, `updatedAt`.
  - [ ] **Step 6: Verify.** `./gradlew ktlintFormat`, then
    `./gradlew verifyChange :groupsession:test --tests '*GroupSessionFloorSetConflictTest*' --tests '*GroupSessionWorkoutControllerTest*'`.
    Commit.
