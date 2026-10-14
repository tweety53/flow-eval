### Failure-recovery check

Failure states set when a call fails (whole file, not just hunks — `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt` unless noted):

| Failure state | Lines that set it | Lines that clear / overwrite it | Value the failure left unconfirmed |
|---|---|---|---|
| `loadFailed = true` on failed `load` | `SettingsState.kt:274-276` (plus `notifications.failure` at `:276`) | `SettingsState.kt:270` (load success); `SettingsState.kt:295` (unit-write success); `SettingsState.kt:339` (view-write success) | Both fields: `unitWrite.confirmed` and `viewWrite.confirmed` still `null`/stale; the shown `displayUnit`/`sessionView` are defaults or last optimistic values |
| `loadFailed = true` on `chooseUnit` with `NOTHING_CONFIRMED` | `SettingsState.kt:310-311` (plus notification `:311`) | Same three clearing lines above | `unitWrite.confirmed == null`; `viewWrite.confirmed` independently possibly `null` |
| `loadFailed = true` on `chooseSessionView` with `NOTHING_CONFIRMED` (includes the refused send at `:335` when `unitWrite.confirmed == null`) | `SettingsState.kt:351-352` (plus notification `:352`) | Same three clearing lines | `viewWrite.confirmed == null` (unit also unconfirmed on the refused-send path — no PUT was sent) |
| `viewWrite.value` left on optimistic choice after `NOTHING_CONFIRMED` failure | `SettingsState.kt:78-81` (`begin`), unchanged by `:115-124` (`onFailure` returns `NOTHING_CONFIRMED` without revert) | `SettingsState.kt:105` (`onSuccess` adopts server value), `:119` (later revert), `:135-136` (`adoptLoad`) | `viewWrite.confirmed` — the control shows a value the server never confirmed |
| `notifications.current` = failure message | `SettingsState.kt:276,303,311,347,352` | Dismiss paths in `components/NotificationState.kt:57` (`dismiss`) and `:70` (timed clear); overwritten by any later `success`/`failure` at `NotificationState.kt:46,50-51` | N/A (transient banner, not a persisted truth claim) |

Per clearing line, per unconfirmed value — which line on that success's path re-confirms it (`none` = partial recovery):

- Clearing at `SettingsState.kt:270` (load success): re-confirms unit via `:271` (`unitWrite.adoptLoad(it.displayUnit)`); re-confirms view via `:272` (`viewWrite.adoptLoad(it.sessionView)`). Both re-confirmed — but only because the diff correctly gated the clear on `noWriteSucceeded` (`:267,:270`, i.e. *both* `succeededWriteCount`s unchanged). Full recovery.
- Clearing at `SettingsState.kt:295` (unit-write success): re-confirms unit via `OptimisticWrite.onSuccess` (`SettingsState.kt:102-103`, called at `:295`). Re-confirmation of **view: `none`** — nothing on this path touches `viewWrite.confirmed`/`value`; the success response *did* carry the value (`it.sessionView`) and the code discards it (only `it.displayUnit` is passed to `onSuccess`). Clearing the shared `loadFailed` here while `viewWrite.confirmed` is still `null` is a partial recovery — see Important I1.
- Clearing at `SettingsState.kt:339` (view-write success): re-confirms view via `:339` (`viewWrite.onSuccess(token, it.sessionView)`). Re-confirmation of unit on this path: `none` by design — the response carried `it.displayUnit` and the code discards it (`:336` passes only `it.sessionView`). In practice the unit must already be confirmed (the send at `:335-336` refuses otherwise), so the shared-flag clear is safe *for the unit*; the discarded unit value is instead cross-field staleness — see Minor M1.

### Summary

The diff adds an account-level `sessionView` (FOCUS/QUICK) preference plumbed through `WorkoutApi.updatePreferences`, `WorkoutApiImpl` (single PUT), new `WorkoutModeSetting` UI, and a second per-field `OptimisticWrite` (`viewWrite`) in `SettingsState` with mutex-serialised sends. I checked all four lenses against the actual boundary calls: network goes through `ApiRequestSupport.apiCall`/`bodyOrThrow` with `HttpTimeout` (15s request / 10s connect / 15s socket in `GymieApiClient.kt:41-43,255-259`, mapped to friendly `ApiException`s); there are no filesystem, subprocess, lock (beyond the in-memory `sendMutex`), or multi-row database writes in the changed behaviour, so partial-write reduces to the single-PUT payload shape and concurrent re-entry to overlapping `load`/`choose*` coroutines. Verdict: one partial-recovery flag clear and one wire-encoding gap that breaks the "mode off the wire" contract; no unbounded wait or non-idempotent double effect found.

### Issues

#### Critical (Must Fix)

None found — no torn undetectable state, double charge/send, or unbounded external wait in the changed behaviour.

#### Important (Should Fix)

**I1 — Partial recovery: unit-write success clears the shared `loadFailed` while `sessionView` is still unconfirmed.** `File: shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295`, lens: error return (failure-recovery step).

- Concrete scenario: `load()` fails (offline) → `loadFailed = true`, both fields unconfirmed, `hasLoaded = true` via the `finally` at `:280-281`, so the screen is interactive with skeleton-gated defaults showing. The user taps a weight unit. That PUT succeeds and `:295` sets `loadFailed = false` because the *unit* token is current — while `viewWrite.confirmed` is still `null` and the Workout mode row still shows the unconfirmed `FOCUS` default as though settings are now in sync. The in-place retry affordance `loadFailed` drives disappears although one field's value is still unknown. (Mirror image via `:339` is mostly self-guarding because the view send refuses without a confirmed unit at `:335`, but the shared flag is still cleared per-field rather than per-state.)
- Fix sketch: gate the write-success clears on both fields being confirmed, or track per-field failure flags instead of one shared `loadFailed`; at minimum, only clear at `:295` when `viewWrite.confirmed != null` (and symmetrically at `:339` when `unitWrite.confirmed != null`), leaving the retry visible until a `load()` at `:270-272` re-confirms the missing field.
- Reproducer: `none — requires an orchestrated coroutine interleaving (failed load, then successful unit PUT) with in-memory fakes; no single shell command demonstrates the flag state. The existing `SettingsStateTest` harness with a failing `preferencesResult` followed by `chooseUnit` success observes `loadFailed == false` with `sessionView` never confirmed.`

**I2 — `null` sessionView is actually sent on the wire, contradicting the "unchanged / left off" contract.** `File: shared/src/commonMain/kotlin/com/gymie/shared/api/GymieApiClient.kt:244-248` (with `WorkoutModels.kt:700` comment and `WorkoutApiImpl.kt:200`), lens: error return / partial write.

- Concrete scenario: the KDoc at `WorkoutModels.kt:700` promises "`sessionView` null is left off the wire (`encodeDefaults = false`)". But the production `Json` at `GymieApiClient.kt:244-248` sets only `ignoreUnknownKeys`/`isLenient` — it never sets `encodeDefaults = false` (nor `explicitNulls = false`), so under kotlinx defaults a `UpdateWorkoutPreferencesRequest(displayUnit, sessionView = null)` from every `chooseUnit` call at `SettingsState.kt:293` serializes with an explicit `"sessionView":null` instead of omitting the key. The `FakeWorkoutApi` models omission (`lastUpdatePreferencesView == null` means "left off"), and the unit test asserts omission using a default `Json`, so neither guards the production client. Depending on the backend, the unit PUT either 400s (turning every unit change into a spurious failure) or — worse — interprets explicit null as "clear the mode" and resets a stored QUICK to the default behind the user's back, a torn preference no client state detects.
- Fix sketch: configure the shared `Json` with `encodeDefaults = false; explicitNulls = false` if the backend contract truly is "absent means unchanged" (and check `Models.kt:33`'s identical assumption), or change the contract to send the confirmed view on every PUT; then add a serialization test using the production `Json` configuration, not default `Json`.
- Reproducer: `none — no runnable command in this read-only review demonstrates the production payload without executing the Kotlin serializer; the evidence is the `Json {…}` builder at the cited lines lacking either flag versus the comment claiming `encodeDefaults = false`.`

#### Minor (Nice to Have)

**M1 — Each write discards the other field's truth from its own success response.** `File: shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:293-295` and `:338-339`, lens: error return.

- Concrete scenario: the PUT answers the whole preferences object, but `chooseUnit` adopts only `it.displayUnit` and `chooseSessionView` adopts only `it.sessionView`. If the server's other field moved concurrently (second device, admin change), the local `confirmed` for that field stays stale until the next `load()`. Deliberate per the KDoc ("the unit's own writes and loads own that field"), and harmless while one device writes both fields — but the response *did* carry the value and the code threw it away, so a cross-device change is invisible with no error.
- Fix sketch: adopt the other field's value into that field's `confirmed` (without moving its visible `value` while it has in-flight writes), or document that cross-device skew resolves only on next `load()`.
- Reproducer: `none — requires two writers racing one account; not expressible as a single command.`

### Assessment

**Ready for the human gate?** With fixes

**Reasoning:** The new mode control itself serializes sends correctly (`sendMutex`), refuses to guess the required `displayUnit` after a failed load (`:335`), keeps timeouts bounded via the shared `HttpTimeout` (15s/10s/15s), and keeps PUTs idempotent (absolute values, safe under `apiCall`'s 401 refresh-and-retry). But I1 leaves the screen claiming recovery while `sessionView` is still unknown, and I2 means production unit writes likely send explicit `null` for the mode contrary to the contract the fake and tests assume — either a spurious failure or a silent mode reset. Both should be fixed before the human gate; M1 can ride along or be documented.
