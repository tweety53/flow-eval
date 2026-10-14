### Failure-recovery check

Failure states set by this diff's code when a call fails (whole-file grep of [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt), not just the hunks):

| Failure state | Set at | Cleared / overwritten at | Values the failure left unconfirmed |
|---|---|---|---|
| `loadFailed = true` (retry affordance shown) | [SettingsState.kt:275](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:275) (load failure), [SettingsState.kt:310](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:310) (unit write, `NOTHING_CONFIRMED`), [SettingsState.kt:351](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:351) (mode write, `NOTHING_CONFIRMED`, including the synthetic `error(...)` at :335) | `loadFailed = false` at [SettingsState.kt:270](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:270), [SettingsState.kt:295](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295), [SettingsState.kt:339](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339) | `displayUnit` and `sessionView` — a failed `load()` confirms neither; a failed write with nothing confirmed leaves its own field unconfirmed |
| Optimistic `viewWrite.value` left showing the tapped mode | [SettingsState.kt:80](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:80) via `begin()` at [SettingsState.kt:329](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:329), left in place on `NOTHING_CONFIRMED` at [SettingsState.kt:122](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:122) | Reverted to confirmed at [SettingsState.kt:119](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:119), adopted from load at [SettingsState.kt:136](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:136), overwritten by a later `begin()` | `sessionView` itself |
| Failure notification | `notifications.failure(...)` at [SettingsState.kt:276](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:276), :347, :352 (and unit equivalents :303, :311) | No line in `SettingsState.kt` clears it; only `NotificationState.dismiss()` ([NotificationState.kt:56](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/components/NotificationState.kt:56)) or replacement by a newer notification | n/a — not a recovery claim |

Per clearing line, per unconfirmed value — the line that re-confirms it, or `none`:

- [SettingsState.kt:270](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:270) (load success): `displayUnit` re-confirmed at [SettingsState.kt:271](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:271), `sessionView` at [SettingsState.kt:272](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:272). The clear is gated on `noWriteSucceeded` (both fields), so both adoptions run whenever the clear does. **Full recovery.**
- [SettingsState.kt:295](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) (unit write success): `displayUnit` re-confirmed inside `unitWrite.onSuccess(token, it.displayUnit)`. `sessionView` re-confirmed at **`none`** — the success's own response is a `WorkoutPreferences` carrying `sessionView`, and `chooseUnit` discards it. **Partial recovery — Issue 1 below.**
- [SettingsState.kt:339](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339) (mode write success): `sessionView` re-confirmed inside `viewWrite.onSuccess(token, it.sessionView)`. `displayUnit` re-confirmed at **`none`** — the response carried it and `chooseSessionView` deliberately does not adopt it (documented at :325–326). Reachability is narrower here: a mode write can only succeed when `unitWrite.confirmed != null` (:335), so the unit always has a prior confirmed value; the unconfirmed case is a failed refresh `load()` whose news about the unit this clear then buries. Same defect shape as :295, lower stakes — folded into Issue 1.

### Summary

The diff adds a `SessionView` (Focus/Quick) preference end-to-end: a nullable field on the preferences request/response, a second `OptimisticWrite` in `SettingsState` sharing the existing `sendMutex`, `loadFailed`, and notification machinery, and a new Settings row that saves on tap. Error-return handling mirrors the existing unit path (revert-to-confirmed, or retain-optimistic plus retry when nothing is confirmed) and is mostly sound; the network call is bounded by the client's existing 15s `HttpTimeout` ([GymieApiClient.kt:256](/work/shared/src/commonMain/kotlin/com/gymie/shared/api/GymieApiClient.kt:256)), so the timeout lens finds nothing new. Two lenses do bite: the shared `loadFailed` flag is now cleared by single-field successes that leave the other field unconfirmed (error return / recovery), and the mode write's forced carriage of a stale confirmed `displayUnit` can clobber a unit changed elsewhere (concurrent re-entry). No partial-write hazard: each save is a single PUT.

### Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

**1. A unit write's success clears `loadFailed` while `sessionView` is still unconfirmed — the screen claims recovery with the mode's value unknown**
File: [SettingsState.kt:295](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) — lens: **error return** (partial recovery, per the required check above).

Scenario: `load()` fails (nothing confirmed, `loadFailed = true`, retry offered). The user taps Quick — `chooseSessionView` refuses at :335 or the PUT fails, outcome `NOTHING_CONFIRMED`, so the control keeps showing the optimistic Quick and `loadFailed` stays up ([SettingsState.kt:351](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:351)). The user then taps KG and that PUT succeeds: :295 clears `loadFailed`, the retry affordance disappears, and the mode control still displays Quick — a value the server never confirmed and may not hold (its stored mode could be Focus). The `WorkoutPreferences` that PUT returned contained the server's actual `sessionView`; `chooseUnit` reads only `it.displayUnit` and throws the rest away. [SettingsState.kt:339](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339) is the mirror image for the unit after a failed refresh load. Before this diff the flag covered exactly one field, so a unit success clearing it was a full recovery; the diff made the flag shared without making the clears field-aware.

Fix sketch: either adopt the other field from every successful preferences response (`unitWrite.adoptLoad(it.sessionView)` in `chooseUnit` when no view write is in flight/succeeded since, and symmetrically in `chooseSessionView` — the response is already paid for), or split the recovery condition so a write success clears `loadFailed` only when the other field also has a confirmed value.

Reproducer: none — no existing test drives "failed load → failed mode write → successful unit write"; it needs a new `SettingsStateTest` case with `preferencesResult = failure`, then a failing `chooseSessionView`, then a succeeding `chooseUnit`, asserting `loadFailed` stays true / `sessionView` is not presented as confirmed.

**2. A mode write replays the last confirmed unit and silently overwrites a unit changed by another client**
File: [SettingsState.kt:336](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:336) — lens: **concurrent re-entry**.

Scenario: the backend requires `displayUnit` on every PUT, so `chooseSessionView` sends `unitWrite.confirmed`. The diff correctly serialises against *local* queued writes via `sendMutex` (and tests that), but `confirmed` can be arbitrarily stale across processes: tablet loads KG/Focus, phone changes the unit to LBS, tablet taps Quick → `PUT {KG, QUICK}` reverts the phone's LBS change with no signal to either user. The unit write does not have the symmetric problem (it leaves `sessionView` off the wire, :700 in `WorkoutModels.kt`), so this hazard is new with the mode write. Within one `SettingsState` instance re-entry is fine — the mutex plus per-field tokens handle it.

Fix sketch: needs a server-side answer — a partial-update endpoint (or `PATCH`) that accepts `sessionView` alone, or a conditional write (version/ETag) that fails on a stale base so the client can reload and retry. Client-only mitigation is limited to re-fetching preferences inside `sendMutex` immediately before the PUT, which narrows but does not close the window; at minimum the stale-carriage should be a named, accepted risk in the KDoc rather than only a local-queue guarantee.

Reproducer: none — demonstrating it requires two clients against a real backend; the fake collapses both into one `preferencesResult`.

#### Minor (Nice to Have)

None. (Checked and clean: the PUT/GET are timeout-bounded at the client level; cancellation is rethrown by `onFailureNotCancelled` and `viewWrite.end()` runs in `finally`; rapid repeated taps on the always-enabled control are ordered by `sendMutex` and superseded tokens are ignored as `STALE`.)

### Assessment

**Ready for the human gate?** With fixes

**Reasoning:** The new mode write's own failure paths — revert, refuse-without-a-confirmed-unit, stale-token suppression — are carefully done, and no lens found a torn write or an unbounded wait. But the shared `loadFailed` flag now over-claims recovery: one field's success dismisses the retry affordance while the other field's displayed value was never confirmed, even though the very response that cleared the flag carried the missing value and the code discarded it (Issue 1). Issue 2's cross-client unit clobber may be acceptable if the backend cannot offer a partial update, but it should be a conscious decision, not an unnoticed consequence of the required-`displayUnit` payload.