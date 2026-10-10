> **Execution:** `/flow` implements this plan. Mark a task's own checkbox when
> `check-task-commit-fields.sh` passes on that task's commit.
> **Relocation:** no

# kan-757-offline-l5-be-per-item-if-match-on-group-session

KAN-757: the group-session floor's set PATCH refuses a stale replay with `409 set_changed` (value
compare-and-set on an optional `expected`), sets record who last changed them, every refused session
transition carries a `code` (the "already" ones with their actor), and session actions plus the floor
set PATCH honour `Idempotency-Key`. `design.md` beside this file is canonical for each decision a task
cites (by `**Decision:**` ID); `proposal.md` for what ships.

**Repository:** `gymie` only — worktree
`/Users/tweety53/Projects/gymie/.worktrees/kan-757-offline-l5-be-per-item-if-match-on-group-session`
(from `origin/feature/offline-mode`). Every path below is relative to that root.

## Global constraints

- **Gradle outruns the default Bash timeout.** Run every Gradle invocation with `timeout: 600000`,
  in the foreground. Never run two at once in this worktree.
- **Verify** (worktree root): `./gradlew ktlintFormat`, then
  `./gradlew verifyChange :<module>:test --tests '<selector>'` for each of the task's own test
  classes, plus every existing test class the task's `**Files:**` modifies. `<module>` is `:workout`,
  `:groupsession` or `:app`.
- **`BaselineLayoutTest` stays green** on every task that adds a main source file:
  `./gradlew :app:test --tests '*BaselineLayoutTest*'`.
- **No new inline lint suppressions** and no config weakening (`CONTRIBUTING.md`).
- **The shared git stash is off limits.** To test a revert, check the previous version of a file out
  over it (`git show HEAD~1:<path> > <path>`), then restore it with `git checkout HEAD -- <path>`.
- **Core purity** (`CLAUDE.md` §6): no Spring, JPA or HTTP type in any `core` package.
- **Wire contract, verbatim.** Floor set PATCH body: `weight`, `effort`, `confirmed` as today, plus
  optional `expected` with the same three fields (all three required inside it). Conflict: HTTP 409
  `ProblemDetail`, `"code": "set_changed"`, properties `current` (`{weight, effort, confirmed}` in the
  request's own wire shape), `updatedBy` (`{"id": "<uuid>", "displayName": "<name>"}` or `null`),
  `updatedAt` (ISO-8601 instant or `null` — decision `unknown-actor-null`). Transition refusals stay HTTP 409 with the existing
  `detail`, plus `"code"` and, for `session_already_started`, `startedAt` and `startedBy`; for
  `session_already_finished`, `finishedAt` and `finishedBy` — each actor `{id, displayName}` or
  `null`. The codes and their sites are the table under decision `transition-codes`.
- **A request without `expected` or `Idempotency-Key` behaves exactly as today** — every existing
  test passes apart from the stub updates the tasks below name.
- **Baselines** count `@Test` statically over each task's own new test files; a file the task
  creates counts 0 before it.

## Review focus

- A weight sent as `60` against a stored `60.00` (the column is `numeric(10,2)`) must compare equal —
  never a `409` for the same value at another scale. Task 1 (`an expected weight at another scale matches`).
- The compare and the write happen under the one workout row lock: two drivers on the same set with
  the same `expected` — exactly one lands, the other answers `409`. Task 4
  (`two drivers on one set with the same expected land once`).
- A set PATCH's stamps survive every later save of the aggregate (another set's PATCH, an exercise
  PATCH, start, finish) — `WorkoutRepositoryAdapter.persist` merges the whole graph, so a stamp the
  entity does not map is nulled. Task 1 (`set stamps survive a save of another set`).
- A `409 set_changed` releases the idempotency key, so Keep mine can resend under the same key.
  Task 4 (`a set_changed answer releases the key`).
- Reopen keeps `startedBy` and clears `finishedBy`. Task 2 (`reopen clears finishedBy`).

---

- [x] 1. Set authorship stamps and the compare-and-set in workout

**Build:** green
**Files:** `src/app/src/main/resources/db/migration/V40__group_session_floor_conflicts.sql`, `src/app/workout/src/main/kotlin/com/gymie/workout/core/domain/workout/WorkoutModels.kt`, `src/app/workout/src/main/kotlin/com/gymie/workout/core/exceptions/WorkoutExceptions.kt`, `src/app/workout/src/main/kotlin/com/gymie/workout/core/services/workout/WorkoutFieldPatchService.kt`, `src/app/workout/src/main/kotlin/com/gymie/workout/core/services/workout/impl/WorkoutFieldPatchServiceImpl.kt`, `src/app/workout/src/main/kotlin/com/gymie/workout/infrastructure/configurations/WorkoutModuleConfig.kt`, `src/app/workout/src/main/kotlin/com/gymie/workout/infrastructure/persistence/WorkoutEntities.kt`, `src/app/workout/src/test/kotlin/com/gymie/workout/core/services/workout/impl/WorkoutFieldPatchServiceImplSetConflictTest.kt`, `src/app/workout/src/test/kotlin/com/gymie/workout/infrastructure/persistence/WorkoutSetAuthorshipIntegrationTest.kt`
**Allowed-collateral:** `src/app/workout/src/test/**`
**Tests:** `a matching expected patches the set and stamps the actor`, `a stale expected throws set changed and writes nothing`, `no expected is last write wins`, `the owner is the actor by default`, `confirmed is compared with confirmedAt`, `an expected weight at another scale matches`, `an unchanged patch keeps the stored stamps`, `set stamps survive a save of another set`, `a structure save writes unstamped sets`
**Regression:** revert → no `updated_at`/`updated_by` on `workout_set`, `patchSet` takes no `expected` and never refuses; a replayed stale set overwrites a live entry and the floor 409 (Task 3) has nothing to report.
**Baseline:** before=0 after=9
<!-- predicted: grep -c '@Test' over the two new test files after this task -->
**Commit:** `feat(workout): a set records who last changed it, and a set patch can be refused when its expected values are stale`
**After:** none

**Decision:** value-compare-and-set
**Decision:** set-authorship-stamps
**Decision:** one-migration-v40

  - [x] **Step 1: Read first.** `WorkoutFieldPatchServiceImpl.patchSet` (lines 139–220), the
    `patchSet` override in `WorkoutModuleConfig` (around line 418, a delegating decorator),
    `WorkoutSetEntity` (`WorkoutEntities.kt` ~403–470), `WorkoutRepositoryAdapter.persist` and
    `WorkoutStructureVersionIntegrationTest` (the DB-backed test pattern in `:workout`). Confirm V40 is
    free: `ls src/app/src/main/resources/db/migration | sort -V | tail -1` prints
    `V39__workout_structure_version.sql`.
    <!-- measured: ls src/app/src/main/resources/db/migration | sort -V | tail -1 → V39__workout_structure_version.sql @ merge-base 5c566151 -->
  - [x] **Step 2: Migration.** `V40__group_session_floor_conflicts.sql`, header comment naming KAN-757
    and the four columns' purpose, `set local lock_timeout = '5s';`, then:

    ```sql unverified:mirror V39's lock-up-front convention for both tables; confirm the users table name for no FK (none is added, matching workout.created_by's style — check V-scripts)
    lock table workout_set in access exclusive mode;
    lock table group_session in access exclusive mode;
    alter table workout_set add column updated_at timestamp with time zone;
    alter table workout_set add column updated_by uuid;
    alter table group_session add column started_by uuid;
    alter table group_session add column finished_by uuid;
    ```

    No backfill: existing rows read `null`.
  - [x] **Step 3: Failing tests.** `WorkoutFieldPatchServiceImplSetConflictTest` (mirror
    `WorkoutFieldPatchServiceImplTest`'s mocked-repository setup) covers the first six names: a matching
    `expected` → the saved set carries the new values, `updatedBy = actorId`, `updatedAt` set; a stale
    one → `WorkoutSetChangedException` whose `current` is the stored set, and `save` never called;
    `expected = null` → patched regardless; calling with only the six original arguments stamps
    `updatedBy = userId`; `expected.confirmed = true` against `confirmedAt = null` is stale;
    `Measured(BigDecimal("60"))` expected against a stored `Measured(BigDecimal("60.00"))` matches.
    `WorkoutSetAuthorshipIntegrationTest` (DB-backed): patch set A, then patch set B → A's stamps
    unchanged on re-read; a `saveStructure` of a workout with new sets → their stamps read `null`.
    Run them; they fail to compile.
  - [x] **Step 4: Domain.** `WorkoutSet` gains trailing `updatedAt: Instant? = null`,
    `updatedBy: UUID? = null`. Add `data class SetValues(val weight: SetWeight?, val effort: SetEffort?, val confirmed: Boolean)`
    beside it, with a `matches(set: WorkoutSet): Boolean` that compares weight and effort by value —
    `SetWeight.Measured` amounts with `compareTo == 0`, never `equals` (scale) — and `confirmed` with
    `set.confirmedAt != null`.
    unverified: read `SetWeight`/`SetEffort` (`WorkoutModels.kt` or their own file) for every variant before writing `matches`; only `Measured`'s `BigDecimal` needs `compareTo`.
  - [x] **Step 5: Exception.** `WorkoutExceptions.kt`: `class WorkoutSetChangedException(val current: WorkoutSet) : RuntimeException("Set ${current.id} changed")`.
  - [x] **Step 6: Service.** `WorkoutFieldPatchService.patchSet` gains trailing
    `actorId: UUID = userId, expected: SetValues? = null` (KDoc: `expected` is the compare-and-set of
    decision `value-compare-and-set`; `actorId` is who `updatedBy` names). The impl, after
    `requireSet`, finds the stored set and throws `WorkoutSetChangedException(stored)` when
    `expected != null && !expected.matches(stored)` — before any validation or write. The patched set
    copies in `updatedAt = nowTruncatedToMicros()`, `updatedBy = actorId`. The `WorkoutModuleConfig`
    decorator passes both through.
  - [x] **Step 7: Entity.** `WorkoutSetEntity` maps `updated_at` / `updated_by` (both nullable) in
    `toDomain` and `fromDomain`.
  - [x] **Step 8: Verify.** `./gradlew ktlintFormat`, then
    `./gradlew verifyChange :workout:test --tests '*WorkoutFieldPatchServiceImpl*' --tests '*WorkoutSetAuthorshipIntegrationTest*' --tests '*WorkoutSetPatchControllerTest*'`
    and `./gradlew :app:test --tests '*BaselineLayoutTest*'`. Commit.

- [x] 2. Session actors and coded transition refusals in groupsession

**Build:** green
**Files:** `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/core/domain/groupsession/GroupSessionModels.kt`, `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/core/exceptions/GroupSessionExceptions.kt`, `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/core/services/groupsession/GroupSessionQueryService.kt`, `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/core/services/groupsession/impl/GroupSessionQueryServiceImpl.kt`, `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/core/services/groupsession/impl/GroupSessionCommandServiceImpl.kt`, `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/persistence/GroupSessionEntities.kt`, `src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionExceptionHandler.kt`, `src/app/groupsession/src/test/kotlin/com/gymie/groupsession/core/services/groupsession/GroupSessionCommandServiceImplTransitionCodeTest.kt`, `src/app/groupsession/src/test/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionTransitionProblemTest.kt`
**Allowed-collateral:** `src/app/groupsession/src/test/**`
**Tests:** `start stamps startedBy`, `finish stamps finishedBy`, `reopen clears finishedBy`, `starting a started session is session_already_started with its actor`, `starting a finished session is session_already_finished`, `finishing a planned session is session_not_started`, `finishing a finished session is session_already_finished`, `reopening an unfinished session is session_not_finished`, `re-dating a started session is session_already_started`, `adding to a finished session is session_already_finished`, `adding someone twice is participant_already_added`, `leaving a finished session is session_already_finished`, `a refusal carries its code and named actor`, `an actor from before the migration is null`, `a code without an actor carries no actor properties`
**Regression:** revert → every refused transition is a bare 409 with no `code`, sessions never record who started or finished them, and OFF-L5-4 cannot say whose action arrived first.
**Baseline:** before=0 after=15
<!-- predicted: grep -c '@Test' over the two new test files after this task -->
**Commit:** `feat(groupsession): a refused session transition names its code, and who started or finished the session`
**After:** Task 1

**Decision:** transition-codes
**Decision:** session-actor-columns

Correction (2026-10-07): the plan declared an unknown actor answered as `"startedBy": null`; what
shipped omits the property, because `ProblemDetail`'s `NON_EMPTY` inclusion drops a `null` property
(measured by `an actor from before the migration is null`, which first failed on `No value at JSON
path "$.startedBy"`). Decision `unknown-actor-omitted`. The test keeps its name and asserts absence.
The `GroupSessionControllerTest` refusal stubs and `GroupSessionMigrationTest`'s column set were
updated for the factory constructor and the two new columns (Allowed-collateral).

Correction (2026-10-07, panel round 0): the correction above is itself withdrawn — `ProblemDetail` keeps a
`null` property; the first failure was `jsonPath`'s `value(null)` reading a JSON `null` as no value.
The unknown actor is `null` again (decision `unknown-actor-null`), asserted on the raw body.

  - [x] **Step 1: Read first.** `GroupSessionCommandServiceImpl` — every
    `GroupSessionIllegalTransitionException(` site (`requireAddable`, `start`, `finish`, `reopen`,
    `patch`, `leave`), `GroupSessionCommandServiceImplLifecycleTest` (the mocked setup to mirror),
    `GroupSessionEntities.kt` (`startedAt`/`finishedAt` mapping), `GroupSessionExceptionHandler`
    (`handleForbidden`'s `code` property) and `GroupSessionControllerTest` (the `@WebMvcTest` setup).
  - [x] **Step 2: Failing tests.** `GroupSessionCommandServiceImplTransitionCodeTest` covers the first
    twelve names — one per row of decision `transition-codes`' table plus the three stamping cases,
    each asserting `code`, and for the "already" codes the actor id and time the exception carries.
    `GroupSessionTransitionProblemTest` (`@WebMvcTest(GroupSessionController)` with the handler) covers
    the last three: a thrown `session_already_started` with `startedBy = A` answers `code`,
    `startedAt`, `startedBy: {id, displayName}` (display name from a stubbed
    `GroupSessionQueryService.profileOf`); `startedBy = null` answers `"startedBy": null`;
    `session_not_started` answers no `startedBy`/`finishedBy` keys at all.
  - [x] **Step 3: Lifecycle.** `GroupSessionLifecycle` gains trailing `startedBy: UUID? = null`,
    `finishedBy: UUID? = null`; the existing `check`s are unchanged and the actors are not checked.
    `GroupSessionEntity` maps `started_by` / `finished_by`.
  - [x] **Step 4: Exception.** `GroupSessionIllegalTransitionException(message, code: String, actorId: UUID? = null, at: Instant? = null)`
    with factories per code (`alreadyStarted(lifecycle)`, `alreadyFinished(lifecycle)`,
    `notStarted()`, `notFinished()`, `participantAlreadyAdded(userId)`), each keeping today's message
    text as `message`. Replace every throw site per the table; `start` on a `FINISHED` session and
    `patch` re-dating a `FINISHED` one pick `alreadyFinished`.
  - [x] **Step 5: Stamps.** `start` writes `startedBy = viewerId`; `finish` keeps `startedBy` and
    writes `finishedBy = viewerId`; `reopen` keeps `startedBy`, `finishedBy = null`.
  - [x] **Step 6: Actor lookup and mapping.** `GroupSessionQueryService.profileOf(userId: UUID): ParticipantProfile?`
    backed by `ParticipantDirectory.profilesOf(setOf(userId))[userId]`. `GroupSessionExceptionHandler`
    takes the query service by constructor and `handleConflict` sets `code`, and for the two "already"
    codes the time property and the actor as `mapOf("id" to id, "displayName" to name)` or `null`
    (`session_already_started` → `startedAt`/`startedBy`, `session_already_finished` →
    `finishedAt`/`finishedBy`). Add the query-service mock to any existing `@WebMvcTest` config the
    constructor change breaks (Allowed-collateral).
  - [x] **Step 7: Verify.** `./gradlew ktlintFormat`, then
    `./gradlew verifyChange :groupsession:test` (the whole module: the handler is shared by every
    group-session web test) and `./gradlew :app:test --tests '*BaselineLayoutTest*' --tests '*GroupSession*'`.
    Commit.

- [x] 3. The floor set PATCH takes expected and answers set_changed

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

  - [x] **Step 1: Read first.** `PatchWorkoutSetRequest` (`:workout` `dto`, ~line 703: required
    nullable fields plus the `@JsonAnySetter` unknown-field rejection), `SetWeightWebConverter` /
    `SetEffortWebConverter` (`toDomain`, `toDto`), and `GroupSessionWorkoutControllerTest`'s
    `patch set delegates with the resolved workout id`.
  - [x] **Step 2: Failing tests.** `GroupSessionFloorSetConflictTest` (same `@WebMvcTest` config as
    `GroupSessionWorkoutControllerTest`): a body with `expected` → `patchSet(jana, workoutId, setId, …, actorId = owner, expected = SetValues(…))`;
    the service throwing `WorkoutSetChangedException` with a stamped set → `409`, `code: set_changed`,
    `current` in wire shape, `updatedBy: {id, displayName}`, `updatedAt`; an unstamped set → both
    `null`; a body with `foo` → `400`.
  - [x] **Step 3: DTO.** In `GroupSessionDtos.kt`: `PatchFloorSetRequest(weight, effort, confirmed, expected: FloorSetValuesDto? = null)`
    and `FloorSetValuesDto(weight, effort, confirmed)`, the same `@field:JsonProperty(required = true)`
    and `@JsonAnySetter` rejection `PatchWorkoutSetRequest` uses (on both classes; `expected` itself
    optional).
  - [x] **Step 4: Controller.** `patchSet` takes `PatchFloorSetRequest` and calls `patchSet` with
    `actorId = UUID.fromString(authentication.name)` and `expected` converted through the two web
    converters. Update the existing `patch set delegates…` stub and verify to the new call (actor =
    `owner`, `expected = null`).
  - [x] **Step 5: Mapping.** `GroupSessionExceptionHandler` handles `WorkoutSetChangedException`:
    `409`, `detail` `This set was changed on another phone.`, `code: set_changed`, `current` =
    `mapOf("weight" to SetWeightWebConverter.toDto(…), "effort" to SetEffortWebConverter.toDto(…), "confirmed" to (confirmedAt != null))`,
    `updatedBy` via `profileOf` as in Task 2, `updatedAt`.
  - [x] **Step 6: Verify.** `./gradlew ktlintFormat`, then
    `./gradlew verifyChange :groupsession:test --tests '*GroupSessionFloorSetConflictTest*' --tests '*GroupSessionWorkoutControllerTest*'`.
    Commit.

- [x] 4. Idempotency on session actions, and the floor end to end

**Build:** green
**Files:** `src/app/workout/src/main/kotlin/com/gymie/workout/infrastructure/presentation/web/IdempotencyKeyFilter.kt`, `src/app/src/test/kotlin/com/gymie/GroupSessionFloorOfflineIntegrationTest.kt`
**Tests:** `two drivers on different sets both land`, `two drivers on one set with the same expected land once`, `a stale expected answers set_changed and changes nothing`, `the solo set patch refuses expected`, `a started session refuses start with its starter`, `a replayed session start returns the first answer`, `a replayed leave returns the first answer`, `a replayed finish returns the first answer`, `a replayed add participant adds once`, `a replayed floor set patch is not applied twice`, `a set_changed answer releases the key`
**Regression:** revert → a replayed session action or floor set PATCH is applied a second time, and nothing proves the floor conflict end to end against the real database.
**Baseline:** before=0 after=11
<!-- predicted: grep -c '@Test' GroupSessionFloorOfflineIntegrationTest.kt after this task -->
**Commit:** `feat(workout): session actions and the floor set patch honour Idempotency-Key`
**After:** Task 3

**Decision:** session-idempotency-routes

  - [x] **Step 1: Read first.** `IdempotencyKeyFilter` (`isKeyedRequest`, the two regexes),
    `WorkoutIdempotencyKeyIntegrationTest` (key helpers) and `GroupSessionFlowIntegrationTest` /
    `GroupSessionConcurrencyIntegrationTest` (seeding a session with two `EDIT` drivers and a
    participant; the concurrent-request pattern).
  - [x] **Step 2: Failing tests.** `GroupSessionFloorOfflineIntegrationTest : AppIntegrationTest()`
    covers the eleven names. The same-set race fires both PATCHes concurrently with the same
    `expected` and asserts one `200`, one `409 set_changed`. "Changes nothing" reads the
    `workout_set` row back. The starter test starts as A, starts again as B, asserts
    `startedBy.id = A` and A's display name. Replays assert identical bodies and one stored effect
    (one roster row, one `finished_at`, one `left_at`). The release test: stale `expected` with key K →
    `409`; resend with key K and the current values as `expected` → `200`.
  - [x] **Step 3: Routes.** Add
    `Regex("/api/group-sessions/$UUID_PATTERN/participants/$UUID_PATTERN/workout/sets/$UUID_PATTERN")`
    to the `PATCH` match and `Regex("/api/group-sessions/$UUID_PATTERN/(start|leave|finish|participants)")`
    to the `POST` match; update the class KDoc's route list (KAN-757, decision
    `session-idempotency-routes`).
  - [x] **Step 4: Verify.** `./gradlew ktlintFormat`, then
    `./gradlew verifyChange :app:test --tests '*GroupSessionFloorOfflineIntegrationTest*' --tests '*WorkoutIdempotencyKeyIntegrationTest*'`.
    Commit.

- [x] 5. Specs: the floor conflict, transition codes and session idempotency

**Build:** green
**Files:** `spectre/specs/group-sessions.md`, `spectre/specs/offline-write.md`
**Tests:** **none** — specification text only.
**Regression:** revert → the specs still describe a last-write-wins floor and bare 409 transitions.
**Baseline:** before=0 after=0
<!-- predicted: no test files touched -->
**Commit:** `docs(specs): group-session floor conflict, transition codes and session-action idempotency`
**After:** Task 4

  - [x] **Step 1:** `spectre/specs/group-sessions.md`: append R19 (the floor set PATCH's `expected`,
    `409 set_changed` body, last-write-wins without it, set stamps), R20 (every refused transition's
    `code` per decision `transition-codes`' table, actor and time on the "already" codes, `null`
    for sessions from before the change; start/finish/reopen stamping), each with one `#### Scenario:`
    taken from the design's acceptance (two phones edit one set; start a started session). Amend R6,
    R8, R13 and R15's `409` wording only to point at R20.
  - [x] **Step 2:** `spectre/specs/offline-write.md` R3: extend the route list with
    `POST /api/group-sessions/{id}/start`, `/leave`, `/finish`, `/participants` and
    `PATCH /api/group-sessions/{id}/participants/{userId}/workout/sets/{setId}`. Leave R1's
    "The group-session floor SHALL never be queued" — that is KAN-758's to change.
  - [x] **Step 3: Verify.** `spectre validate 2>&1 | grep -c 'group-sessions.md\|offline-write.md\|kan-757'` prints `0` (the tree carries unrelated pre-existing findings, so the exit code is not the check). Commit.
    <!-- measured: spectre validate | grep -c … → 0 (1039 findings tree-wide, none in these files) @ merge-base 5c566151 -->
