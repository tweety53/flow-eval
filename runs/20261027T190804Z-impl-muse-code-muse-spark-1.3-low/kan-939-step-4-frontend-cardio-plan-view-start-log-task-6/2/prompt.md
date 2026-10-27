**PROVE THE GUARD BITES:** When this task's tests assert on configuration or file content — a
guard script, an embedded config, a fixture file — a passing run alone is not evidence: break
the property the assertion protects, run the guard or the test against the broken state and
capture its failure, then restore and capture the pass. Report both runs. "The pattern is now
stricter" is intent, not evidence — the failing run against the broken state is the evidence.

**PIN BEFORE REFACTOR:** A task that refactors code whose observable behaviour includes
produced output — a guard's warnings, a CLI's printed lines, an emitted file — pins that
output before any code moves: its RED step first writes harness cases asserting the current
outputs exactly as produced today, including the ones nothing asserted yet, and only then
refactors. The refactor lands with two proofs, both recorded before the change closes: the
pinned suite passing, and one end-to-end old-vs-new comparison that runs the whole producer
against the same fixture on both sides of the refactor and diffs the outputs byte-for-byte.
The suite proves only what it asserts; the old-vs-new diff is the proof for every output it
never named.

**PLAN PROVENANCE:** a fenced block tagged `unverified:` is a hypothesis, not code to transcribe.
Establish the real API before writing against it, and report what you found. An assumption tagged
`unverified:` in your task is a guess to confirm before you build on it. When what you
measure contradicts the plan, stop and report the measurement: see **When a measurement
contradicts the plan** (`skills/flow-contracts/plan-amendment.md`). Report a correction your
measurement produced — a tag to retag, a number to replace, a `**Files:**` entry to add — as the
exact field and its corrected value, so it can be transcribed into the record in place
(**The record carries its own corrections**, below).

**FOREGROUND BUILDS:** Never end your turn with a build, test run, or other long-running
command still executing in the background. Run it in the foreground, or poll it to
completion, before you stop.

**TARGETED TESTS:** Run only the tests this task's `**Tests:**` field names, through the build
tool's own selector — `--tests '<class>'` for Gradle, `-run '<name>'` for `go test`, `-t
'<name>'` for vitest — once for RED, once for GREEN, and again only after a source edit. Never
run the module or repository suite mid-task: the full `## test` list runs once per worktree at
the last bundle, and again in `flow.verify`. A plan step that asks for the module or repository
suite is run as this task's targeted tests instead. Pipe a test run's output through `tail` so a
green run costs lines of context, not a build log.

**DRIFT CHECK:** Before you commit, read your own diff once for what it leaves behind. For
every behaviour, name, path, flag or exit code the diff changes, grep for the comments, file
headers, usage lines, docs and cross-file citations that describe it, and bring each one in
line in this same commit. Every refusal or exit path the diff adds gets a test case of its
own.

**PROOF RUNS:** When a check has to be run repeatedly to prove a flake reproduced or fixed, size
the run count to the flake rate you measured, 10–15 runs by default, never a fixed count per
step. Your report states the measured rate and the run count. Sizing the runs never replaces
the proof `~/.claude/rules/fix-determinism-at-the-source.md` requires: a determinism fix still
shows the check failing with the mechanism removed.

**OUTPUT BUDGET:** Every tool result stays in your context, and every later turn re-reads your
whole context — a large output is paid for again on every turn after it. Read a file over 200
lines by line range — `grep -n` for the symbol, then `sed -n '<a>,<b>p'` or Read with
`offset`/`limit` — and never re-read a file already in your context unless you have edited it
since. Cap every search (`| head -40`) and every build, lint or install run (`| tail -30`), and
reproduce a failing block from its log rather than printing the whole log. Never print a
generated file — a lockfile, a snapshot, a bundle, a build artifact.

**REPRODUCE, DON'T READ:** Where a behaviour crosses a boundary — the store, the filesystem, a
guard, a real transcript, a real process — at least one test you write MUST exercise the real
thing. A test backed by a fake or a hand-built value passes while the real integration is broken:
the shape you construct by hand is not the shape the real producer emits. Build the value the way
production builds it, or assert against the real boundary.

**REPORT, DON'T DECIDE:** A question only the operator can settle — a spec point no recorded
decision covers, a recorded decision the plan appears to contradict, a scope the plan does not
name — is reported, never decided in code. Where the plan can proceed, implement it as written
and carry the question in your REPORT FILE; where it cannot proceed without the answer, report
BLOCKED. A decision you make silently is a decision nobody recorded.

## Your task

- [ ] 6. Log logic: the field table, the draft, parsing, checks and derivation

**Build:** green
**Files:** `shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioActivity.kt`, `shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLogFields.kt`, `shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLogDraft.kt`, `shared/src/commonTest/kotlin/com/gymie/shared/cardio/CardioLogDraftTest.kt`
**Allowed-collateral:** `shared/src/commonMain/kotlin/com/gymie/shared/cardio/CardioLog*.kt` — a split for detekt's size rules.
**Tests:** `eachActivityLogsItsOwnFieldsInFrameOrder`, `durationReadsHoursMinutesAndSeconds`, `durationRefusesOtherShapes`, `anEmptyDurationHoldsThePrimary`, `paceIsDerivedWhenBlankAndNeverSent`, `aTypedPaceReplacesTheDerivedOne`, `speedIsDerivedForRides`, `theRowerDerivesPerFiveHundredMetres`, `maxBelowAvgIsAnError`, `rowerCadenceOutOfRangeNamesTheRange`, `anErrorShowsOnlyOnceTheFieldWasLeft`, `durationPrefillsFromTheClockRoundedDown`, `aSavedLogFillsTheDraft`, `theRequestCarriesTypedFieldsOnly`
**Regression:** revert → no log fields, no draft; the fourteen tests fail to compile.
**Baseline:** before=0 after=14
<!-- measured: { cat shared/src/commonTest/kotlin/com/gymie/shared/cardio/CardioLogDraftTest.kt 2>/dev/null || true; } | grep -c -E '@Test\b' @ gymie-frontend origin/feature/cardio 83a2ae97 → 0 -->
**Commit:** `feat(cardio): the log sheet's fields, draft, checks and derived pace and speed`
**After:** Task 5

**Decision:** log-fields-table
**Decision:** duration-input
**Decision:** client-ranges
**Decision:** derived-never-sent
**Decision:** clock-from-started-at

**API the tests use** — exact names and shapes, all in package `com.gymie.shared.cardio`. The tests
also use the existing `CardioWorkout`, `CardioLog`, `CardioLogRequest`, `CardioDistance`,
`DistanceUnit`, `CardioActivity.kinds` and `WorkoutStatus` as they stand.

```kotlin
enum class CardioLogField {
    DURATION, DISTANCE, AVG_PACE, AVG_SPEED, ELEVATION_GAIN, AVG_HEART_RATE,
    MAX_HEART_RATE, AVG_CADENCE, AVG_POWER, CALORIES, EFFORT, NOTE,
}

// on enum class CardioActivity
val logFields: List<CardioLogField>

data class CardioLogDraft(
    // further constructor parameters are free; these two must be properties the data class's
    // copy() takes by name, as in draft.copy(effort = 6, note = "hilly")
    val effort: Int?,
    val note: String,
) {
    fun text(field: CardioLogField): String                    // "" when nothing is typed
    fun with(field: CardioLogField, value: String): CardioLogDraft
    fun left(field: CardioLogField): CardioLogDraft            // marks field as left
    fun shownError(field: CardioLogField): String?             // errors()[field] once left, else null
    fun errors(): Map<CardioLogField, String>                  // an empty field is never in error
    val canSubmit: Boolean                                     // a property, not a function
    fun derived(field: CardioLogField): String?                // AVG_PACE "6:37", AVG_SPEED "31.9"; null when typed
    fun toRequest(): CardioLogRequest                          // typed fields and effort; never note, never a derived value

    companion object {
        fun fresh(workout: CardioWorkout, now: kotlin.time.Instant): CardioLogDraft
        fun of(workout: CardioWorkout): CardioLogDraft          // saved log as text, effort from it, note from workout.notes
    }
}
```

Exact error texts: `use h:mm:ss` (DURATION of another shape), `max heart rate is below average` (on
MAX_HEART_RATE only), and range messages `between <min> and <max> <unit>` (`between 10 and 60 spm`).
Duration text is `m:ss` under an hour and `h:mm:ss` from one up (`41:06`, `1:12:40`). The rower's
distance is metres whatever the workout's unit: `toRequest().distance == CardioDistance(7120.0,
DistanceUnit.M)` for `7120` typed on a km workout. `of` leaves AVG_PACE's text `""` when the saved
log carries only `derivedAvgPaceSeconds`, so `derived(AVG_PACE)` recomputes it (`14:29` for 4360 s
over 5.02 km).

  - [ ] **Step 1: Shape** — `CardioLogFields.kt`:

    ```kotlin
    enum class CardioLogField { DURATION, DISTANCE, AVG_PACE, AVG_SPEED, ELEVATION_GAIN, AVG_HEART_RATE,
        MAX_HEART_RATE, AVG_CADENCE, AVG_POWER, CALORIES, EFFORT, NOTE }
    ```

    and `CardioActivity.logFields: List<CardioLogField>` in sheet order — outdoor run (L1): DURATION, DISTANCE,
    AVG_PACE, ELEVATION_GAIN, AVG_HEART_RATE, MAX_HEART_RATE, AVG_CADENCE, CALORIES, EFFORT, NOTE; outdoor
    ride (L3): DURATION, DISTANCE, AVG_SPEED, ELEVATION_GAIN, AVG_HEART_RATE, MAX_HEART_RATE, AVG_CADENCE,
    AVG_POWER, CALORIES, EFFORT, NOTE; indoor ride: the ride's minus ELEVATION_GAIN; rower (L4): DURATION,
    DISTANCE, AVG_PACE, AVG_POWER, AVG_HEART_RATE, MAX_HEART_RATE, AVG_CADENCE, CALORIES, EFFORT, NOTE;
    treadmill: the outdoor run's minus ELEVATION_GAIN and AVG_CADENCE; elliptical: DURATION, DISTANCE,
    AVG_HEART_RATE, MAX_HEART_RATE, AVG_CADENCE, CALORIES, EFFORT, NOTE. These match the backend's
    `CardioActivity.logFields` sets (`BC/core/domain/cardio/CardioActivity.kt`) — check them against it.
  - [ ] **Step 2: RED** — `CT/cardio/CardioLogDraftTest.kt`, the fourteen names, each one assertion family:
    `durationReadsHoursMinutesAndSeconds` (`33:12` → 1992, `1:12:40` → 4360); `durationRefusesOtherShapes`
    (`45`, `1:75`, `abc` → error *use h:mm:ss*); `paceIsDerivedWhenBlankAndNeverSent` (km run, 33:12 over 5.02
    km → derived 397 s = "6:37", request `avgPaceSeconds == null`); `speedIsDerivedForRides` (1:12:40 over 38.6
    km → 31.9); `theRowerDerivesPerFiveHundredMetres` (L4's figures → "2:06"); `maxBelowAvgIsAnError` (148 /
    139 → *max heart rate is below average* on MAX_HEART_RATE); `rowerCadenceOutOfRangeNamesTheRange` (8 →
    *between 10 and 60 spm*); `anErrorShowsOnlyOnceTheFieldWasLeft` (the error exists but `shownError(field)`
    is null until `left(field)`); `durationPrefillsFromTheClockRoundedDown` (startedAt 18:31:00.000, opened
    at 19:12:06.900 → "41:06"); `aSavedLogFillsTheDraft` (every saved value back as text, derived still
    derived); `theRequestCarriesTypedFieldsOnly`. Run `./gradlew :shared:desktopTest --tests
    '*CardioLogDraftTest'` — fails.
  - [ ] **Step 3: GREEN** — `CardioLogDraft` (immutable data class + pure functions, no Compose): text per
    field, `effort: Int?`, `note: String`, `left: Set<CardioLogField>`; `errors(): Map<CardioLogField,
    String>` from the backend ranges (`BC/core/domain/cardio/specification/CardioLogSpecification.kt`: duration
    1–86400 s, HR 30–250 and max ≥ avg, calories 0–20000, elevation 0–20000, pace 60–3600 s (rower 60–600),
    speed 0.1–120.0, cadence 1–250 (rower 10–60, design `client-ranges`), power 1–2500, distance > 0 and < 1000
    km, rower distance in m), each message *between X and Y <unit>* in the field's unit; `derived(field)` via
    `CardioFormat.derivedPace` / `derivedSpeed`; `canSubmit` = duration parses and `errors()` is empty;
    `toRequest(): CardioLogRequest` with typed fields only; `companion fun fresh(workout, now)` (Duration from
    `startedAt` when IN_PROGRESS, floored to the second) and `fun of(workout)` (from its saved log and notes).
    The 400 ms debounce lives in the sheet (Task 9), not here. Run — pass.
  - [ ] **Step 4: Verify.** `./gradlew :shared:ktlintFormat`, then
    `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests 'CardioLogDraftTest'`.
    Commit.


## Context

# proposal.md

# kan-939-step-4-frontend-cardio-plan-view-start-log

## Why

KAN-939 (epic KAN-935, design steps 3 + 4). A cardio workout can be created (KAN-938) but not
opened, started, logged or finished: the frontend has no plan view and no log form. The backend
has no `last` read and no owner on a cardio read, both of which the frames need.

## What changes

- **Plan view** (`CardioPlanScreen`, frames P1 P2 P4 P5 S2): bar (‹ · title · status · ⋮), activity ·
  kind kicker, date/time meta, PLANNED totals, read-only steps with a foldable repeat, owner line
  when switched in, Last time, and a foot by state — Planned: *Log result* + *Start*; In progress:
  the clock in the bar, *Log result* alone; Finished: *Edit log*; Read: no foot, no ⋮.
- **Start**: a tap posts `/start` and the screen becomes In progress (P2), its clock counted from
  the server's `startedAt`.
- **Log sheet** (`CardioLogSheet`, L1–L6): one field set per activity, Treadmill, Indoor ride and
  Elliptical built from the field table. Duration is required and prefilled from the clock when
  in progress. Pace or speed left blank is derived, shown in italics with *CALCULATED*, and never
  sent. Range errors appear 400 ms after typing, on fields the user has left. The same sheet edits a
  finished log (*Edit log*, *Save*).
- **Finished** (P3): LOGGED figures, a derived one marked *· CALCULATED*, the note as the
  accent-stem quote, PLANNED folded to one line, Last time, *Edit log*.
- **Last time** (S3): its own read, drawn as a static kicker over two shimmer lines while waiting,
  hidden when there is none or the read fails.
- **⋮** (P5): *Edit plan* (the KAN-938 builder) · *Log result* / *Edit log* · *Delete workout*
  (the shipped confirm, back to the day).
- **Backend**: `GET /api/cardio-workouts/{id}/last` replaces the `previous` a read embedded; a read
  carries `owner` when the acting user is not the owner (a switched-in session).
- **No in-app entry yet**: the day's cardio rows that open this screen are KAN-940; this change is
  verified by UI tests against the frames. Playwright P1, P3, L2 are KAN-941.
# design.md

## Context

- Handoff: `docs/design/cardio/` (backend repo, `feature/cardio`) — `tickets/04-frontend-plan-log.md`,
  `notes/step-3.md` (P1 P2 P4 P5 S2), `notes/step-4.md` (L1–L6, P3, S3), `notes/00-overview.md`
  (rules, motion), `screens/<id>.png`. Frames are the specification; magenta is annotation.
- Already on `feature/cardio`: backend get/start/log/delete with per-activity log validation
  (`CardioLogSpecification`, `CardioActivity.logFields`); frontend models, `CardioApi`
  (`cardio`, `startCardio`, `logCardio`, `deleteCardio`), `CardioFormat` (pace, speed, distance,
  elevation, `derivedPace`, `derivedSpeed`), `CardioTotals`, `CardioPlanWords`, the builder.
- Missing before this change: `/last`, an owner on the read, the frontend log field table, log
  validation and draft, any route or screen for one cardio workout.
- One change across two repos (backend + frontend) because the plan view's Last time and owner
  line read fields only this change adds.

## Decisions

### Cut from feature/cardio

**ID:** base-feature-cardio
**Status:** active
**Chosen:** branch `spectre/<name>` from `feature/cardio` in `gymie` and `gymie-frontend` — the epic's branch (handoff README § Branch).
**Considered:** `develop` — the epic merges to develop once, when accepted; KAN-938 had to be redone after being cut from develop.

### Last time is GET /{id}/last

**ID:** last-endpoint-by-id
**Status:** active
**Chosen:** `GET /api/cardio-workouts/{id}/last` — owner, activity, kind, date and the excluded id all come from the workout, under the same ownership check as `GET /{id}`; reuses `findPrevious`. 200 with `{performedOn, log}` (no derived pace/speed), 204 when there is none.
**Considered:** the ticket's `/last?activity&kind&before` — it carries no owner or excluded id, so a read of someone else's workout would answer with the caller's own history.

### The embedded previous is removed

**ID:** previous-moves-to-last
**Status:** active
**Chosen:** drop `previous` from `GET /{id}`'s response and its query; `cardio-workouts` R12 restated for the endpoint.
**Considered:** keep both — one wasted query on every read, and two answers to one question.

### Last time is its own client read

**ID:** last-time-own-read
**Status:** active
**Chosen:** the plan view fetches `/last` after the plan answers; while waiting it draws S3 (static *LAST TIME* kicker, two shimmer lines); 204 or a failure removes the block, the screen stays usable.
**Considered:** the embedded `previous` with no new endpoint — S3 would never draw (operator chose the endpoint).

### The read carries its owner when switched in

**ID:** owner-on-read
**Status:** active
**Chosen:** `CardioWorkoutResponse.owner` = `{userId, displayName, avatarUrl, gender}` when the acting user is not the owner (a switched-in session), else null; through a new `:cardio` port `CardioOwnerLookup` that `:profile` implements, composing the avatar URL on read as `WeekOwnerDirectoryAdapter` does. The plan view draws the owner line (32px face + *Name's workout*, P4) when present.
**Considered:** the route carrying the owner from the row that opened it — rejected by the operator in favour of the server stating it. A grantee's non-switched read (the FRIENDS fold, R1f) is not served by `GET /{id}` today (owner only); opening borrowed rows is KAN-940's.

### No in-app entry in this change

**ID:** no-entry-point
**Status:** active
**Chosen:** add the route and screen with no door to them; KAN-940 wires the day's cardio rows (tap, hold → start). Verified by desktop UI behaviour and look tests against the frames.
**Considered:** pulling a minimal row tap from KAN-940 (splits one ticket's surface across two changes); building KAN-940 first (delays this ticket for its own scope).

### Route.CardioPlan

**ID:** route-cardio-plan
**Status:** active
**Chosen:** `Route.CardioPlan(id: String, viewOnly: Boolean = false)`, a former overlay under `AccessScope.WORKOUTS`, mounted by a `shellRoom<Route.CardioPlan>` beside `CardioBuilder` in `ShellGroupSessionOverlay.kt`; the opener sets `viewOnly = !switchState.canEdit(WORKOUTS)` as `ActiveWorkout` does. The screen re-reads the workout when the builder (Edit plan) returns.
**Considered:** an in-screen state flag on the builder route — mixes two screens behind one route.

### Start is a tap

**ID:** start-tap
**Status:** active
**Chosen:** *Start* posts `/start` (the client's Idempotency-Key) and the screen takes the response — P2. A failure shows the shipped failure bar and leaves P1.
**Considered:** none — the design states a tap (00-overview, "Start on the plan view is a tap").

### The clock

**ID:** clock-from-started-at
**Status:** active
**Chosen:** the bar's status slot shows `WorkoutElapsed.format(startedAt, now)` ticking each second (m:ss, then h:mm:ss); the sheet's Duration prefills with whole seconds `now − startedAt` at the moment it opens (L5).
**Considered:** a client-side stopwatch — would not survive closing the app.

### The log field table lives on the frontend activity

**ID:** log-fields-table
**Status:** active
**Chosen:** frontend `CardioActivity.logFields` mirrors the backend's table (duration, HR, calories, effort, note on every activity, plus — outdoor run: distance, pace, elevation, cadence; treadmill: distance, pace; outdoor ride: distance, speed, elevation, power, cadence; indoor ride: distance, speed, power, cadence; elliptical: distance, cadence; rower: distance, pace, cadence, power). Field order per sheet is the frames' (L1, L3, L4) and step-4's prose for the undrawn three.
**Considered:** deriving fields from the target metrics — a treadmill plan targets speed and incline, which its log does not take, and its log takes no cadence.

### Duration input

**ID:** duration-input
**Status:** active
**Chosen:** accept `h:mm:ss` and `m:ss` (seconds and minutes 00–59 after a colon), whole seconds; anything else is a field error *use h:mm:ss*. Pace inputs take `m:ss`.
**Considered:** bare minutes (`45` = 45:00) — not drawn, and ambiguous beside `m:ss`.

### Client range checks

**ID:** client-ranges
**Status:** active
**Chosen:** the backend's ranges (`CardioLogSpecification`), except where a frame draws a stricter one — rower cadence 10–60. Message *between X and Y <unit>* (L4); max below avg: *max heart rate is below average* (L3). Checked 400 ms after the last key, shown only on a field the user has left; the primary holds at 45% while any error stands or Duration is empty.
**Considered:** the backend's ranges alone — L4's *between 10 and 60 spm* could not be drawn.

### Derived values are shown, never sent

**ID:** derived-never-sent
**Status:** active
**Chosen:** a blank pace/speed with both duration and distance shows `CardioFormat.derivedPace` / `derivedSpeed` in italic neutral-700 with *CALCULATED*; typing replaces it; the request carries typed fields only. P3 reads the response's `derivedAvgPaceSeconds` / `derivedAvgSpeed` and marks them *· CALCULATED*.
**Considered:** none — the backend refuses nothing here, but the spec and ticket forbid storing derived values.

### Log submit

**ID:** log-submit
**Status:** active
**Chosen:** *Log result* / *Save* puts `/log`; when the sheet's Note differs from the workout's `notes` it then patches `notes` (`PATCH /{id}` — the log carries no note; "the workout note is the one note"). Success closes the sheet and the screen takes the latest response (P3). A failure of either call shows the failure bar and keeps the sheet open with every value. Cancel, scrim, drag and back save nothing.
**Considered:** a note field on the log — a backend change the design does not ask for; the note is the workout's.

### Delete

**ID:** delete-confirm
**Status:** active
**Chosen:** ⋮ → *Delete workout* opens the shipped `ConfirmDialog` in the Workout Rows C1 shape, its copy naming the workout; confirm deletes and returns to the day.
**Considered:** none — the design reuses the shipped confirm.

## Live check

Against the worktree's dev stack, as the dev account: create two outdoor-run Intervals workouts,
log the earlier one (2700 s, 9.2 km), then `GET /api/cardio-workouts/{later}/last` — expect 200
with the earlier one's `performedOn` and `durationSeconds` 2700 and no `derivedAvgPaceSeconds`;
`GET /api/cardio-workouts/{earlier}/last` — expect 204; `GET /api/cardio-workouts/{later}` —
expect no `previous` key and `owner` null. Failure looks like: 404/405 on `/last` (route missing),
`previous` still present, or `/last` on the earlier workout answering itself.

## Open questions


## When you are done

Implement the task in the current directory, which is the repository. Files the rules above cite by path (under `~/.claude/` or `skills/`) are not available here; apply each rule as stated. Reply with a short report: what you changed, the tests you added, and each command you ran with its result.
