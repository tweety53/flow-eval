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

  - [ ] **Step 1: Shape** — `CardioLogFields.kt`:

    ```kotlin unverified:authored for this plan; adjust names to the file's idiom
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
  - [ ] **Step 4: Verify** — frontend verify for `CardioLogDraftTest`. Commit.
