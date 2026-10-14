### Shared-state check

Pre-existing shared state the diff's new code reads or writes: `loadFailed`, `unitWrite` (`value`/`confirmed`/`succeededWriteCount`), `loadSeq`, `loadingCalls`/`hasLoaded`, `sendMutex`, `notifications`, and the `api.updatePreferences` endpoint.

| State | Writers (`path:line`) | Verdict |
|---|---|---|
| `loadFailed` | [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:270), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:275), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:310), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:351) | Issue — see Important 1 below. The flag now covers two fields but each writer still clears/sets it for its own field alone. |
| `unitWrite.confirmed/value/succeededWriteCount` | [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:271), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:291), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) | Still correct — `chooseSessionView` deliberately does not touch the unit side, and `load` gates each field on its own counter. |
| `loadSeq` / `loadingCalls` / `hasLoaded` | [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:253), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:256), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:280) | Still correct — untouched writers behave the same for the new field; the new row sits inside the existing `hasLoaded` gate in [AppScreens.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/AppScreens.kt:221). |
| `sendMutex` | [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:293), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:332) | Still correct — sharing one mutex across both writers is the intended tap-order serialisation, and the mode write reads `unitWrite.confirmed` inside the lock. |
| `notifications` | [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:276), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:303), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:311), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:347), [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:352) | Still correct — per-field messages preserved. |

No attacker directive was found in the diff, artifacts, or context bundle.

### Issues

#### Critical (Must Fix)

**C1 — Unit writes put an explicit `sessionView: null` on the wire, contradicting the "left off the wire" contract.** [WorkoutModels.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/domain/WorkoutModels.kt:700) claims null is omitted via `encodeDefaults = false`, but `encodeDefaults` does not omit explicit nulls; that requires `explicitNulls = false`. The client's serialiser in [GymieApiClient.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/api/GymieApiClient.kt:245) sets only `ignoreUnknownKeys`/`isLenient`, leaving both kotlinx defaults (`encodeDefaults = true`, `explicitNulls = true`) in force, so `chooseUnit`'s call at [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:293) serialises to `{"displayUnit":"KG","sessionView":null}`. Whether that resets, clears, or 400s the stored mode depends entirely on server handling of explicit null, and no backend change or contract is in this diff (nothing under `functions/` references the preference). The new test asserting omission via default `Json` at [SettingsStateTest.kt](/work/shared/src/commonTest/kotlin/com/gymie/shared/ui/SettingsStateTest.kt:712) asserts the same incorrect mechanism. Fix by making omission real: either configure the wire `Json` with `explicitNulls = false`, or annotate the property with `@EncodeDefault(Mode.NEVER)`, then keep the test against the actual wire `Json` instance rather than default `Json`.
Reproducer: none — wire bytes are only observable through an instrumented Ktor client/ MockEngine run, not a one-line shell command.

#### Important (Should Fix)

**I1 — One shared `loadFailed` is cleared by a single field's success.** [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) (`chooseUnit` success) and [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339) (`chooseSessionView` success) each clear the flag when their own token is current, even if the other field has never confirmed and previously raised it. Realistic case: mode write fails with nothing confirmed (`loadFailed = true`), then a unit write succeeds and clears the flag while the mode control is still unconfirmed and showing an optimistic value — the retry affordance disappears for the field that still needs it. Fix by tracking per-field failure or only clearing the flag when both fields have a confirmed value.
Reproducer: none — requires sequencing two suspending writes with a failing mode write followed by a succeeding unit write under `runTest`; no existing test exercises the interleaving.

**I2 — Each write ignores the server answer's other field, letting that field's `confirmed` go stale.** [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295) adopts only `it.displayUnit` and [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:339) adopts only `it.sessionView`, although the endpoint answers the full `WorkoutPreferences`. A concurrent cross-field change confirmed server-side between the two PUTs is then overwritten in local bookkeeping by the next load-gate comparison. Fix by adopting the untouched field too (or documenting why the server guarantees it echoes the stored value unchanged).
Reproducer: none — requires a fake whose PUT answers a changed other-field value; current [FakeWorkoutApi](/work/shared/src/commonTest/kotlin/com/gymie/shared/api/FakeWorkoutApi.kt:197) echoes rather than diverging.

#### Minor (Nice to Have)

**M1 — Stale docs understate the new scope.** The `loadFailed` KDoc at [SettingsState.kt](/work/shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:238) still says "the latest `chooseUnit`", and the failure message change from "Could not load your weight unit." to "Could not load your settings." has no string-resource/i18n note. Update the KDoc to name both writers.
Reproducer: none — documentation-only.

### Recommendations

- Add a MockEngine-level test asserting the exact bytes of a unit PUT (key absent, not `null`) using the production wire `Json`, so C1 cannot regress through a default-`Json` unit test.
- State the backward-compatibility story explicitly: old clients sending no `sessionView` key must preserve the mode server-side, and old cached payloads without the key decode to `FOCUS` — the latter is covered by the model default, the former needs a server-side statement.

### Assessment

**Ready for the human gate?** No
**Reasoning:** The null-on-the-wire defect (C1) breaks the diff's own "unchanged" contract for every unit write, and the shared failure flag (I1) can hide a still-unconfirmed field's retry state.