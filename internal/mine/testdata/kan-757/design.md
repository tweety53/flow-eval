## Context

KAN-757 (epic KAN-751, step 5). Two sources disagreed: the Jira description asks for a per-item
`If-Match` on set, exercise, block, notes and finish answered `412`; the step-5 notes
(`docs/design/offline/notes/step-5.md`, "Backend (decided)") and the operator's Jira comment ask for a
value compare-and-set on the floor set PATCH answered `409 set_changed`, plus session-action
idempotency and coded transition refusals with their actor. The operator chose the notes' mechanism and
the notes' scope (2026-10-07). The consumer is KAN-758 (frames OFF-L5-1 … OFF-L5-7).

Facts the design rests on:

- `workout_set`, `workout_exercise` and `workout_block` carry no `updated_at`; the whole workout
  aggregate is saved through `WorkoutRepositoryAdapter.persist` (a JPA merge), so any per-set column
  must ride on the domain `WorkoutSet` or a merge nulls it.
- `WorkoutFieldPatchServiceImpl.patchSet` reads the workout through `findOwnedForUpdate`, a
  `PESSIMISTIC_WRITE` row lock, so a compare inside it is atomic with the write.
- A whole-workout `PUT` regenerates child ids (KAN-755 review focus), so its sets are new rows.
- KAN-756 answers a stale structure save with `409` and an `etag` property — the project's conflict
  status is `409`.
- `IdempotencyKeyFilter` (`:workout`, KAN-754) matches only `/api/workouts/…` paths; it runs after
  Spring Security and scopes keys to the authenticated user.
- `GroupSessionIllegalTransitionException` carries only a message and maps to a bare `409`;
  `GroupSessionForbiddenException` already sets a `code` property.
- `GroupSessionExceptionHandler` is the advice for both group-session controllers and already maps
  `:workout` exceptions raised on the floor (`WorkoutNotFoundException`, `WorkoutStaleException`).
- `ParticipantDirectory.profilesOf` resolves display names for any account.

## Decisions

### Conflict check on the floor set PATCH

**ID:** value-compare-and-set
**Status:** active
**Chosen:** an optional `expected: {weight, effort, confirmed}` body field, compared with the stored set
under the workout row lock; a mismatch writes nothing and answers `409` `code: set_changed` with
`current` (request shape), `updatedBy` (`{id, displayName}` or `null`) and `updatedAt` (or `null`).
Absent `expected` → last-write-wins. Keep mine resends with `expected` = the 409's `current`.
**Considered:** per-item `If-Match` on the set's `updatedAt` answered `412` (the ticket) — a phone that
queues two edits of one set offline conflicts with itself on the second unless the client rewrites the
version after every landed replay, and `412` departs from KAN-756's `409`; per-item `If-Match` answered
`409` — the same self-conflict; workout-level `If-Match` — false conflicts between drivers on different
sets; last-write-wins — erases live entries.
**Amended by:** unknown-actor-omitted

### Scope: the notes' backend list

**ID:** notes-scope
**Status:** active
**Chosen:** the floor set conflict, set authorship stamps, session-action idempotency and coded transition
refusals with actor — exactly what KAN-758's frames render.
**Considered:** adding the same check on exercise, block and notes PATCH and workout finish (the ticket) —
no client UI consumes those answers, so it is speculative; the ticket's scope alone — leaves OFF-L5-2's
author/time and OFF-L5-4's actor and idempotent session replays unbuilt.

### A floor-only request type

**ID:** floor-set-request-dto
**Status:** active
**Chosen:** a new `PatchFloorSetRequest` (`weight`, `effort`, `confirmed`, optional `expected`) in
`:groupsession` presentation for the floor route; the solo route keeps `PatchWorkoutSetRequest`, whose
unknown-field rejection keeps answering an `expected` with `400`.
**Considered:** adding `expected` to the shared `PatchWorkoutSetRequest` — the solo route would accept the
field and silently ignore it; honouring it on the solo route too — needs its own 409 mapping and
display-name lookup in `:workout`, which nothing asks for.

### Set authorship stamps

**ID:** set-authorship-stamps
**Status:** active
**Chosen:** nullable `workout_set.updated_at` and `workout_set.updated_by`, carried on the domain
`WorkoutSet` and mapped by `WorkoutSetEntity`; every set PATCH (solo and floor, the shared service)
stamps the patched set with the patch time and the authenticated caller — on the floor the driver, not
the workout owner. `patchSet` gains `actorId: UUID = userId` and `expected: SetValues? = null` as
trailing defaulted parameters, so the solo controller and existing callers are unchanged. A created,
copied or `PUT`-saved set has no stamp. The stamps appear only in the `409` body.
**Considered:** stamping in the persistence adapter by diffing stored against saved sets — the adapter has
no actor; stamping on structure saves too — `update` carries no actor and a `PUT` writes new rows; a
version token in every workout response — nothing reads it under value compare-and-set.

### The 409 is mapped in :groupsession

**ID:** set-changed-mapped-in-groupsession
**Status:** active
**Chosen:** `:workout` core throws `WorkoutSetChangedException(current: WorkoutSet)`; the stamps ride on
`current`. `GroupSessionExceptionHandler` maps it, resolving `updatedBy`'s display name through a new
`GroupSessionQueryService.profileOf(userId)` backed by `ParticipantDirectory`.
**Considered:** mapping in `WorkoutExceptionHandler` — it does not advise the floor controller and
`:workout` cannot reference `:groupsession` (the reason `handleWorkoutNotFound` already gives); having
the handler call `ParticipantDirectory` directly — presentation reaching a port past the service layer.

### Coded transition refusals with actor

**ID:** transition-codes
**Status:** active
**Chosen:** `GroupSessionIllegalTransitionException` gains `code`, and for the two "already" codes the
actor id and time. Every throw site gets one:

| Site | Session state | `code` | Extra properties |
|------|---------------|--------|------------------|
| start | `IN_PROGRESS` | `session_already_started` | `startedAt`, `startedBy` |
| start | `FINISHED` | `session_already_finished` | `finishedAt`, `finishedBy` |
| finish | `PLANNED` | `session_not_started` | — |
| finish | `FINISHED` | `session_already_finished` | `finishedAt`, `finishedBy` |
| reopen | not `FINISHED` | `session_not_finished` | — |
| re-date | `IN_PROGRESS` | `session_already_started` | `startedAt`, `startedBy` |
| re-date | `FINISHED` | `session_already_finished` | `finishedAt`, `finishedBy` |
| add participant | `FINISHED` | `session_already_finished` | `finishedAt`, `finishedBy` |
| add participant | already on the roster | `participant_already_added` | — |
| leave | `FINISHED` | `session_already_finished` | `finishedAt`, `finishedBy` |

`startedBy` / `finishedBy` are `{id, displayName}`, or `null` for a session started or finished before
this change. The status stays `409`; the `detail` stays the existing message.
**Considered:** a new exception type per code — ten classes for one mapping; codes without actor (the
ticket) — OFF-L5-4 names whose action arrived first.
**Amended by:** unknown-actor-omitted

### Session actors on the lifecycle

**ID:** session-actor-columns
**Status:** active
**Chosen:** nullable `group_session.started_by` and `group_session.finished_by`, carried on
`GroupSessionLifecycle` as `startedBy` / `finishedBy` (trailing, defaulted `null`). Start stamps
`startedBy` with the viewer, finish stamps `finishedBy`, reopen keeps `startedBy` and clears `finishedBy`.
The lifecycle's status/timestamp `check`s are unchanged; the actors are not checked, since legacy rows
hold none.
**Considered:** reading the actor from an audit log — none exists.

### Idempotency on session actions

**ID:** session-idempotency-routes
**Status:** active
**Chosen:** extend `IdempotencyKeyFilter`'s route match with `POST /api/group-sessions/{id}/(start|leave|finish|participants)`
and `PATCH /api/group-sessions/{id}/participants/{userId}/workout/sets/{setId}`; semantics unchanged from
`offline-write` R3, so a `409 set_changed` releases the key and Keep mine may resend with it.
**Considered:** a second filter in `:groupsession` — duplicates the claim/replay machinery for a path list.

### One migration

**ID:** one-migration-v40
**Status:** active
**Chosen:** `V40__group_session_floor_conflicts.sql` adds the four nullable columns with no backfill;
`idempotency_key` already exists (V37).
**Considered:** a migration per table — two scripts for one change.

### An unknown actor is omitted, not null

**ID:** unknown-actor-omitted
**Status:** superseded by unknown-actor-null
**Chosen:** where `value-compare-and-set` and `transition-codes` answer `null` for an actor or time no one
recorded (`updatedBy`, `updatedAt`, `startedBy`, `finishedBy`), the property is omitted from the problem
body instead. Spring's `ProblemDetail` serializes under `@JsonInclude(NON_EMPTY)`, which drops a `null`
property, so `"startedBy": null` never reaches the wire (measured: `GroupSessionTransitionProblemTest`,
2026-10-07). The phone reads an absent actor as unknown.
**Considered:** a custom serializer or a non-`ProblemDetail` body that keeps explicit `null`s — more code
in every 409 for a distinction the client does not draw.
**Superseded because:** the measurement was misread. `jsonPath(...) { value(null) }` reports "No value at
JSON path" for a JSON `null`, and the review panel (round 0, F2) showed the raw body does carry
`"updatedAt":null` — `ProblemDetail` keeps null properties. The omission rested on a false premise.

### An unknown actor is null

**ID:** unknown-actor-null
**Status:** active
**Chosen:** the contract `value-compare-and-set` and `transition-codes` first stated: an actor or time no one
recorded (`updatedBy`, `updatedAt`, `startedBy`, `finishedBy`) is present with the value `null`. Tests
assert the raw body (`"startedBy":null`), since `jsonPath` cannot tell `null` from absent.
**Considered:** keeping the omission and guarding every property with `?.let` — it departs from the
contract the operator approved, for no client need.

## Open questions

## Live check

Against the local stack (`.flow/project.md` `## run`), after the migration has run: two accounts with
`EDIT` on one participant's workout in an `IN_PROGRESS` session.

1. `GET` the participant's workout; note set S's weight/effort/confirmed.
2. Driver A `PATCH`es S with `expected` = those values → `200`.
3. Driver B `PATCH`es S with the same stale `expected` → `409`, `code: set_changed`, `current` = A's
   values, `updatedBy.displayName` = A's name, `updatedAt` set; `select weight, effort, updated_by from
   workout_set where id = S` still shows A's values and A's id.
4. Driver B `PATCH`es a different set T with a matching `expected` → `200`.
5. `POST …/start` on the already-started session → `409`, `code: session_already_started`,
   `startedBy.displayName` = whoever started it.
6. `POST …/finish` twice with one `Idempotency-Key` → the same `200` body both times, one `finished_at`.

Not working looks like: step 3 answering `200` (last write wins) or a `409` with no `code`; step 5's
`409` with no `code` or no `startedBy` for a session started after the migration; step 6's second
call answering `409`.
