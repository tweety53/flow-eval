### Shared-state check

Pre-existing state the new code reads or writes: `unitWrite.confirmed`, `unitWrite` success counter, `loadFailed`, `loadSeq`, `sendMutex`.

- `unitWrite.confirmed` / `OptimisticWrite.adoptLoad`: written at `SettingsState.kt:103` (`onSuccess`, reached from `chooseUnit` at 295) and `:135` (`adoptLoad`, reached from `load` at 271). Still correct. Unit writes send no `sessionView`, so they never clear the mode.
- `unitWrite.succeededWriteCount`: incremented only in `onSuccess`. Still correct. The new view write increments its own counter, and `load` gates each adoption on its own counter.
- `loadFailed`, written at:
  - `:270` (load success, new gate: both writes): correct.
  - `:275` (load failure, new gate: neither write succeeded): **issue**, see item 2.
  - `:295` (`chooseUnit` success, untouched): **issue**, see item 1.
  - `:310` (`chooseUnit` refused with nothing confirmed): correct.
  - `:339` and `:351` (`chooseSessionView`): correct.
- `loadSeq`, `sendMutex`: only read by the new code, and all writers are gated correctly.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)

1. **`shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:295`** — the untouched `chooseUnit` success clears `loadFailed` whenever its own write is current. That flag now also covers the view. Scenario: `load()` fails, so both fields are unconfirmed and `loadFailed` is true. The user picks LBS and the PUT succeeds. `loadFailed` becomes false, the mode control shows the FOCUS default as if it were confirmed, and no retry is offered. The server may hold QUICK. Fix: clear `loadFailed` only when every field has a confirmed value (e.g. also require `viewWrite.confirmed != null`), or track the two cases separately.
   **Reproducer:** none — no existing command exercises this. It needs a `SettingsStateTest` that sets `preferencesResult` to a failure, calls `chooseUnit(LBS)` successfully, and asserts `loadFailed == true` and that the view is unconfirmed.

2. **`SettingsState.kt:273–277`** — the load-failure branch now stays silent if *any* write succeeded since the load began. A failed load after a successful unit write therefore sets no `loadFailed` and shows no notification, yet the view was never loaded. The control silently shows the FOCUS default. Fix: gate the failure on the view's own counter for the view's state, separate from the unit's.
   **Reproducer:** none — needs a test that calls `chooseUnit` successfully while `load()` is in flight with a failing response, then asserts the failure is surfaced.

3. **`SettingsState.kt:276`** (and the import removed at line 7) — `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` was replaced with the literal `"Could not load your settings."`. This changes the existing unit-load failure text, which is unrelated to this feature. The three `"Could not save your workout mode."` literals also duplicate text that belongs in one place. Fix: keep the existing constant for the unit path and put the new messages in `FailureMessages`.
   **Reproducer:** `grep -rn "COULD_NOT_LOAD_WEIGHT_UNIT\|Could not load your settings" shared/src` — the constant is gone from this file and any test asserting the old text would now fail.

4. **`shared/src/commonTest/kotlin/com/gymie/shared/ui/SettingsStateTest.kt`** (`a unit write leaves the workout mode off the wire`) — the test encodes with a fresh `kotlinx.serialization.json.Json` default instance. It does not use the production client's `Json` config, which the KDoc on `UpdateWorkoutPreferencesRequest` relies on (`encodeDefaults = false`). The assertion would still pass if the real client encoded defaults. Fix: encode through the real client's serializer or the same `Json` instance `GymieApiClient` uses.
   **Reproducer:** none — the production `Json` configuration is not in this diff. Verifying it requires reading the client setup.

#### Minor (Nice to Have)

5. **Backward compatibility** — `WorkoutPreferences.sessionView` defaults to `FOCUS`, so a GET without the key decodes as Focus. That is documented. The diff does not state what happens when an older server ignores the unknown `sessionView` in the PUT body, or what the client's `ignoreUnknownKeys` setting is. Add a one-line note or test if the server is not already deployed.
   **Reproducer:** none — server and client JSON configuration are outside this diff.

### Recommendations
- Consider one `loadFailed` per field (or a small enum) so writers stop sharing a single flag. Items 1 and 2 are the same root cause.
- The `FakeWorkoutApi.updatePreferences` change reads `preferencesResult` before the hook. That is commented, which is good.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** The wire contract and the optimistic-write logic for the mode are sound and well tested. However, the shared `loadFailed` flag is now cleared by the untouched unit writer while the view remains unconfirmed, and a failed load can go unreported. Both can leave the mode control showing a default as if it were the stored value.