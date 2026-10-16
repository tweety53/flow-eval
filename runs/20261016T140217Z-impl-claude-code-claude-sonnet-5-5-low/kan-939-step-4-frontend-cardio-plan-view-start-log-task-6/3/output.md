The log field table, `CardioLogDraft` and its 14 tests are in and committed as `1525fb3`. The 14 tests, ktlint and detekt all pass.

**What changed**
- `CardioLogFields.kt` (new) has the `CardioLogField` enum and the per-activity field lists in sheet order. `CardioActivity.logFields` in `CardioActivity.kt` returns them.
- `CardioLogDraft.kt` (new) holds the draft, with every method in your API.
- The checks use the backend ranges, except rower cadence, which is 10–60 per `client-ranges`. Duration accepts `m:ss` and `h:mm:ss`.
- `CardioLogDraftTest.kt` (new) has the 14 named tests.

**Commands**
- `./gradlew :shared:ktlintFormat` ran clean.
- `./gradlew :shared:desktopTest --tests '*CardioLogDraftTest'` passed on its first run. I didn't capture a RED run before writing the code, so the RED step is not recorded.
- The first `ktlintCheck` failed because the file name `CardioLogFields.kt` doesn't match the class `CardioLogField`. I kept the name from your task and added `@file:Suppress("ktlint:standard:filename")`.
- `./gradlew :shared:ktlintCheck :shared:detekt :shared:desktopTest --tests '*CardioLogDraftTest'` then passed.

**Guard check:** I changed the rower cadence range from `10..60` to `1..250` and re-ran the tests. 2 of 14 failed: `rowerCadenceOutOfRangeNamesTheRange` and `anErrorShowsOnlyOnceTheFieldWasLeft`. After I restored the range the full run above passed. I didn't save the failure output beyond that summary.

**Report, don't decide**
- **Error text:** your spec gives exact messages only for duration shape, max heart rate and cadence. I wrote the others myself: `between 1:00 and 60:00 /km` for pace, `between 0.1 and 120 km/h` for speed, `between 0 and 1000 km` for distance, and `between 0:01 and 24:00:00` for duration range. Effort has no check. Please confirm or correct these.
- **Plan figures:** I did not check the field sets against the backend `CardioActivity.kt` or `CardioLogSpecification.kt`; I wrote them from your Step 1 list and Step 3 ranges. That backend repo isn't available here.
- **Rower test figures:** your task says to use "L4's figures" for the rower pace, but I don't have that frame. The test uses 7:30 over 1782 m, which gives 2:06, and 30:00 over 7120 m for the metres distance. Swap in L4's figures if they differ.
- **Test inputs:** the `aSavedLogFillsTheDraft` inputs are mine. I used 4360 s over 5.02 km for the `14:29` case.
- **Committer:** the repo had no git identity, so I committed with a throwaway `-c user.name=claude -c user.email=noreply@anthropic.com`.