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
