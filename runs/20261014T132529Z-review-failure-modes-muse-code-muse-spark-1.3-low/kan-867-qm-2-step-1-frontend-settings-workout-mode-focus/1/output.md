### Failure-recovery check

Failure states set by this diff's code (all in `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`):

| Failure state | Set at | Clears at (whole-file grep) |
|---|---|---|
| `loadFailed = true` after failed `load` | `SettingsState.kt:275` | `SettingsState.kt:270`, `SettingsState.kt:295`, `SettingsState.kt:339` |
| `loadFailed = true` after `chooseUnit` with nothing confirmed | `SettingsState.kt:310` | same three lines |
| `loadFailed = true` after `chooseSessionView` with nothing confirmed (includes the refused-send `error(...)` at `SettingsState.kt:335`, which flows into this path) | `SettingsState.kt:351` | same three lines |
| `notifications.failure(...)` | `SettingsState.kt:276,303,311,347,352` | notifications are transient; no clear line |
| Optimistic `unitWrite.value` left unconfirmed (`NOTHING_CONFIRMED`) | `SettingsState.kt:297-313` (`onFailure` leaves `value` at the optimistic choice) | `unitWrite.adoptLoad` at `SettingsState.kt:271`, `unitWrite.onSuccess` at `SettingsState.kt:295` |
| Optimistic `viewWrite.value` left unconfirmed (`NOTHING_CONFIRMED`) | `SettingsState.kt:341-354` | `viewWrite.adoptLoad` at `SettingsState.kt:272`, `viewWrite.onSuccess` at `SettingsState.kt:339` |

Per clearing line × per unconfirmed value:

- Clearing at `SettingsState.kt:270` (`load` success): unconfirmed unit re-confirmed at `SettingsState.kt:271`; unconfirmed view re-confirmed at `SettingsState.kt:272`. Both conditional on no intervening write success, otherwise correctly left alone. Full recovery.
- Clearing at `SettingsState.kt:295` (`chooseUnit` success): unconfirmed unit re-confirmed at `SettingsState.kt:295` (`unitWrite.onSuccess`). Unconfirmed view: `none` — the success response (`WorkoutPreferences`) carries `sessionView`, but `chooseUnit` adopts only `it.displayUnit` and discards `it.sessionView`. Partial recovery (see Important issue).
- Clearing at `SettingsState.kt:339` (`chooseSessionView` success): unconfirmed view re-confirmed at `SettingsState.kt:339`. Unconfirmed unit cannot exist on this path because the send is refused unless `unitWrite.confirmed != null` (`SettingsState.kt:335`). Full recovery.

Boundary-call confirmation: network calls go through `WorkoutApiImpl.updatePreferences` (`shared/src/commonMain/kotlin/com/gymie/shared/api/WorkoutApiImpl.kt:192-202`) → `ApiRequestSupport.apiCall` (`shared/src/commonMain/kotlin/com/gymie/shared/api/ApiRequestSupport.kt:336-357`) with `HttpTimeout` 15s/10s/15s (`shared/src/commonMain/kotlin/com/gymie/shared/api/GymieApiClient.kt:41-43,255-259`) mapped in `mapException` (`ApiRequestSupport.kt:110-124`); writes serialised by `sendMutex` (`SettingsState.kt:249,293,332`); filesystem/subprocess/database/lock boundaries other than the coroutine `Mutex` are absent.

### Summary

The diff adds a second preferences field (`sessionView`) backed by its own `OptimisticWrite`, a `chooseSessionView` write that piggybacks the confirmed unit, per-field `load` adoption, and a new Settings row plus API plumbing. All four lenses were examined: network errors are checked and reported via `onFailureNotCancelled`; the external call is bounded by the shared 15s `HttpTimeout`; the write is a single PUT with no multi-resource torn state; concurrent taps are serialised by `sendMutex` with per-field staleness tokens. The one genuine gap is partial failure recovery: a `chooseUnit` success clears the shared `loadFailed` retry flag while the workout-mode value is still unconfirmed, discarding the `sessionView` its own response already carried.

### Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

- `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295`, lens: error return. Scenario: `load` fails (both fields unconfirmed, `loadFailed=true`), then `chooseUnit` succeeds. Line 295 clears `loadFailed` while `viewWrite.confirmed` is still `null` and `sessionView` still shows the default `FOCUS`, so the retry affordance vanishes while the mode is still unknown. The PUT response already carried the truth (`WorkoutPreferences.sessionView`) and the code discarded it by adopting only `it.displayUnit`. Fix sketch: adopt `it.sessionView` into `viewWrite` on a unit-write success under the same guard used by `load` (no view write succeeded/in-flight since), or gate the `loadFailed=false` clear on both fields being confirmed. Reproducer: `none — requires sequencing a failed load followed by a successful unit PUT and observing the unconfirmed view default with no retry affordance; no single CLI command demonstrates it against this client-side state holder`.

#### Minor (Nice to Have)

None.

### Assessment

**Ready for the human gate?** With fixes
**Reasoning:** Timeout, partial-write, and concurrent re-entry handling are sound for the changed behaviours; the single gap is the partial recovery above, where one field's success claims recovery for a still-unconfirmed sibling value its own response could have confirmed.
