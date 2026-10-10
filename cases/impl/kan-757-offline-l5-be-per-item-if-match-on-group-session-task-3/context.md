# proposal.md

# kan-757-offline-l5-be-per-item-if-match-on-group-session

## Why

On the group-session floor two phones drive one workout by design (KAN-424). Once the floor works
offline (KAN-758), a set edit replayed late under last-write-wins would silently erase a live
correction made on the other phone, and a queued session action (start, finish, leave, add someone)
would either be applied twice on a lost answer or refused with a bare `409` the phone cannot word.
KAN-758's frames OFF-L5-2 (Keep mine / Use theirs) and OFF-L5-4 (someone got there first) need the
server to say what changed, who changed it and when.

## What changes

- The floor set `PATCH …/participants/{userId}/workout/sets/{setId}` accepts an optional `expected`
  object (the weight, effort and confirmed values the phone last read). When the stored set no
  longer matches, nothing is written and the answer is `409` with `code: set_changed`, the set's
  `current` values, `updatedBy` (`id`, `displayName`) and `updatedAt`. Without `expected` the PATCH
  is last-write-wins as today; the solo set PATCH is unchanged.
- Every set PATCH records who last changed the set and when (`workout_set.updated_by`,
  `workout_set.updated_at`).
- Every refused group-session transition carries a `code`: `session_already_started` (with
  `startedAt`, `startedBy`), `session_already_finished` (with `finishedAt`, `finishedBy`),
  `session_not_started`, `session_not_finished`, `participant_already_added`. Sessions record who
  started and who finished them (`group_session.started_by`, `group_session.finished_by`).
- `Idempotency-Key` is honoured on `POST /api/group-sessions/{id}/start`, `/leave`, `/finish`,
  `/participants` and on the floor set PATCH, with the semantics `offline-write` R3 already gives
  the solo runner.

Out of scope: per-item version checks on exercises, blocks, notes and workout finish; the phone's
own time on start/finish (KAN-759); the client queue and conflict UI (KAN-758).
# design.md

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
`409` with no `code` or a `null` `startedBy` for a session started after the migration; step 6's second
call answering `409`.
