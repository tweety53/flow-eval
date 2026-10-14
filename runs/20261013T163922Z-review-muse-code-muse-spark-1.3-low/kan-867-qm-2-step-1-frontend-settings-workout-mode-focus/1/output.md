### Shared-state check

Pre-existing state the new code reads or writes, with every writer grepped and re-read in full. All line numbers are the post-diff working tree.

- `SettingsState.loadFailed` (`shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`)
  - Writers: `SettingsState.kt:270` (load success clears), `:275` (load failure sets), `:295` (unit-write success clears if current), `:310` (unit-write `NOTHING_CONFIRMED` sets), `:339` (new view-write success clears if current), `:351` (new view-write `NOTHING_CONFIRMED` sets). Verdict: still correct. Gating load success/failure on `noWriteSucceeded = noUnit && noView` (`:267,:270,:274`) is the right generalisation: a view write that succeeded after the load began already answered the server-truth question, so the stale load must not clear/raise the flag. Per-field `adoptLoad` gating (`:271,:272`) preserves each field's independence.
- `unitWrite` confirmed/value/succeededWriteCount (`SettingsState.kt:205`, plus `OptimisticWrite` `:46,:54,:69,:80,:103,:119,:135`)
  - Writers: `begin` `:80`, `onSuccess` `:102-103`, `onFailure` `:119`, `adoptLoad` `:135-136`, called from `load` `:271`, `chooseUnit` `:291,:295`, `chooseSessionView` read-only `:335` plus deliberately *not* adopting `it.displayUnit` (`:339` adopts only `it.sessionView`). Verdict: still correct — the untouched `chooseUnit` writers set/revert only the unit field, and the new code correctly refuses to let a mode answer clobber it.
- `loadingCalls` / `hasLoaded` / `loadSeq` / `sendMutex` (`:219,:230,:246,:249`)
  - Writers: `loadingCalls++/--` `:256,:280`, `hasLoaded = true` `:281`, `loadSeq.next()` `:253`, `sendMutex.withLock` `:293,:332`. Verdict: still correct. The diff leaves the ref-count and the unconditional `hasLoaded = true` alone, and shares `sendMutex` across both write paths as documented.
- `isSaving` (`:234-235`): widened to `unitWrite.isSaving || viewWrite.isSaving`. Verdict: still correct in itself (a view save is a save). Note the UI never reads it — both segmented controls pass `enabled = true` (`AppScreens.kt:311,413`) — so the widening has no visible effect; see Minor below.
- `WorkoutPreferences.sessionView` default (`WorkoutModels.kt:697`): new field, default `FOCUS`. Old payloads without the key decode to `FOCUS`, matching the claimed server default. Verdict: correct client-side; server agreement is assumed, not verified here.
- `UpdateWorkoutPreferencesRequest.sessionView` null handling (`WorkoutModels.kt:700-705`, sent at `WorkoutApiImpl.kt:200`): claimed "null is left off the wire (`encodeDefaults = false`)". The client's actual `Json` (`GymieApiClient.kt:244-248`) sets only `ignoreUnknownKeys`/`isLenient` — `encodeDefaults` stays `true` and `explicitNulls` stays `true`, so a `null` default **is** encoded as `"sessionView":null`. The comment's mechanism does not exist. See Important I1. Grep confirmed no `encodeDefaults`/`explicitNulls` configuration anywhere under `shared/` except the two KDoc/test mentions.
- No reviewer directives, severity overrides, or skip-a-check instructions found in the diff, artifacts, or bundle. No Critical injection finding.

### Issues

#### Critical (Must Fix)

None. No data loss, security hole, or missing planned piece proven by the code read.

#### Important (Should Fix)

**I1 — `UpdateWorkoutPreferencesRequest(null)` is sent as explicit `null`, not omitted — `WorkoutModels.kt:700`, `WorkoutApiImpl.kt:200`, `GymieApiClient.kt:244-248`**

What's wrong: the KDoc promises null "is left off the wire (`encodeDefaults = false`)", and `chooseUnit` (`SettingsState.kt:293`) relies on that to mean "leave the mode unchanged". But the wire `Json` never sets `encodeDefaults = false` (or `explicitNulls = false`), and kotlinx defaults encode a nullable default as `"sessionView":null`. Every unit write therefore sends an explicit null. Whether the backend treats that as "unchanged", "clear to default", or "400" is then a backend contract the client is guessing at — and the new `SettingsStateTest` assertion `assertFalse("sessionView" in Json.encodeToString(...))` uses a default `Json` with the same defaults, so it asserts behavior the production serializer does not provide.

Why it matters: under the real client config the "unit write leaves the mode alone" path sends a key the server never agreed to interpret as absent. Realistic outcome is a rejected PUT or a mode reset on each unit tap.

How to fix: either set the request serializer to drop nulls/defaults (e.g. `@Serializable` with `encodeDefaults = false` scope or `explicitNulls = false` on the wire `Json`, checking nothing else depends on explicit nulls — `GroupSessionModels.kt:143` explicitly relies on `explicitNulls = true` for clearing names, so a global flip is unsafe; prefer a per-request encoding or a custom serializer), or change the contract so a unit write echoes the confirmed `sessionView` instead of null. Update the KDoc to describe whichever is true.

Reproducer: none — no runnable backend-wire capture exists in this repo; evidence is the `Json` builder at `GymieApiClient.kt:244-248` (no `encodeDefaults`/`explicitNulls`) versus the KDoc claim at `WorkoutModels.kt:700`.

**I2 — `chooseSessionView` reads `unitWrite.confirmed` inside `sendMutex`, but `chooseUnit` writes it outside the mutex — `SettingsState.kt:293-295` vs `:332-336`**

What's wrong: the KDoc (`:321-324`) claims the read happens "once every earlier write has settled". It does not. `chooseUnit` does `sendMutex.withLock { api.updatePreferences(...) }` and only afterwards, in `.onSuccess` outside the lock, calls `unitWrite.onSuccess` which sets `confirmed`. A queued mode write can acquire the mutex the moment the unit write's network call returns but before that write's continuation runs `onSuccess`, reading the *previous* confirmed unit and sending a stale `displayUnit` — potentially reverting a just-confirmed LBS back to KG on the server.

Why it matters: a fast unit-tap followed by a mode-tap (exactly the interleaving the new test exercises) produces a wrong `displayUnit` on the mode PUT under real dispatch timing. The new test passes because `runTest` ordering is deterministic, not because the race is closed.

How to fix: move the `onSuccess`/`onFailure` bookkeeping inside the `withLock` block (or snapshot-and-publish `confirmed` under the mutex), so mutex order equals confirmation order.

Reproducer: none — deterministic reproduction needs control of dispatcher interleaving between mutex release and the `onSuccess` continuation; the existing `mode-write-sends-confirmed-unit` test covers only the single-threaded ordering.

#### Minor (Nice to Have)

**M1 — Stale `loadFailed` KDoc — `SettingsState.kt:237-241`**

Still says "the latest `[chooseUnit]` failed with nothing yet confirmed"; now `chooseSessionView` (`:350-352`) also raises it. One-word doc fix.

**M2 — `isSaving` widened but unread — `SettingsState.kt:233-235`, `AppScreens.kt:307-315,409-416`**

`isSaving` now ORs both fields, but neither `DisplayUnitSetting` nor `WorkoutModeSetting` reads it (`enabled = true` hardcoded). If double-tap suppression was intended, wire it; otherwise the widening is dead state. No correctness impact either way.

**M3 — `chooseSessionView` failure copy duplicates the unit path but the shared retry only re-issues `load()` — `SettingsState.kt:346-353`, `AppScreens.kt:326-357`**

A refused mode write with a confirmed unit notifies but does not raise `loadFailed`, so the in-place Retry row never appears for it; the user must re-tap the segment. Matches the unit-write `REVERTED_TO_CONFIRMED` behavior, so consistent — just worth a comment that retry-for-mode is manual re-tap.

### Recommendations

- Add a serialization round-trip test that encodes `UpdateWorkoutPreferencesRequest` with the **production** `Json` configuration (or via the mocked Ktor client), not a default `Json`, so I1 cannot regress silently.
- Consider echoing the confirmed `sessionView` on unit writes (symmetric with mode writes echoing the confirmed unit) once the backend contract for explicit null is pinned down — symmetric echo removes the null-means-unchanged convention entirely.
- Gate the mode segmented control on `!settings.isSaving` or document why concurrent taps are safe beyond mutex ordering (the optimistic UI already handles it, but a second tap mid-flight currently launches a second coroutine per the class KDoc).

### Assessment

**Ready for the human gate?** No — With fixes

**Reasoning:** The two-field optimistic-write design, load gating, and UI gating are sound and well tested, but unit writes likely send an explicit `"sessionView":null` the code claims is omitted (I1), and the cross-field confirmed-unit handoff has a real mutex-ordering race (I2).
