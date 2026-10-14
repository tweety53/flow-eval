### Shared-state check

Read from the diff and the two greps. I did not open `OptimisticWrite`, `chooseUnit`, or `adoptLoad` in full, so this section is incomplete.

- `unitWrite` state (`displayUnit`, `confirmed`, `succeededWriteCount`): writers are `chooseUnit` (unchanged, `SettingsState.kt`, not in the diff) and `adoptLoad` in `load()`. `chooseUnit` sends `UpdateWorkoutPreferencesRequest(displayUnit)` with `sessionView` null, which is omitted from the wire, so the server keeps the mode. Still correct, pending a read of `chooseUnit`'s success path to confirm it does not touch `viewWrite`.
- `loadFailed`: writers are `load()` (`SettingsState.kt`, the `onSuccess` and `onFailureNotCancelled` branches) and `chooseSessionView`. The `noWriteSucceeded` gate is consistent with the unit-only gate it replaced. Still correct.
- `viewWrite` state: writers are `load()` (`adoptLoad`), `chooseSessionView` (`begin`, `onSuccess`, `onFailure`, `end`). Still correct, but this depends on `OptimisticWrite` honouring a pending write in `adoptLoad`, which I did not verify.
- Wire serialization: `sessionView` is null by default and omitted only because kotlinx defaults to `encodeDefaults = false`. The production `Json` in `GymieApiClient.kt:245` does not override it, so this holds.

### Issues

#### Critical (Must Fix)
None found.

#### Important (Should Fix)
None confirmed.

#### Minor (Nice to Have)

1. **`shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt`, `load()` failure branch.** The diff replaces `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` with the literal `"Could not load your settings."` and drops the import. The constant still exists and is used by `WorkoutEditorScreen.kt:387`. Reusing the constant or adding a sibling constant would keep the copy in one place. Reproducer: `git grep -n "Could not load your settings" -- shared/src` shows the literal appears only here. Also, the unit-load failure now reads differently in the Settings room than in the editor.
2. **Code comments naming tickets.** `WorkoutModels.kt` (`KAN-865`), `AppScreens.kt` (`KAN-867`, `KAN-573`), and `SettingsState.kt` (`KAN-867`) reference task IDs in code. This conflicts with the project's no-task-reference convention. Reproducer: `git grep -n "KAN-86" -- shared/src`.
3. **`SettingsStateTest.kt`, `a unit write leaves the workout mode off the wire`.** The test asserts on a default `Json` instance, not the production client's `Json` (`GymieApiClient.kt:245`). The two match today, but the test would not catch a future change to the production config. Reproducer: `none — behavior holds today; a regression test would need to serialize through GymieApiClient's Json`.
4. **`SettingsScreenUiTest.kt`, `api(...)` MockEngine.** The fallback `?: "FOCUS"` for a missing `sessionView` is used only when the PUT body lacks the key. That matches the server default, so this is fine, but a comment would help a reader see why.

### Recommendations
- Read `chooseUnit` and `OptimisticWrite` in full before merging, to close the shared-state gap above. Specifically confirm that `chooseUnit`'s success path does not touch `viewWrite` and that `adoptLoad` respects a pending write.
- Add a test where a view write is in flight when a `load()` returns a stale response. The current suite covers unit-in-flight during a mode write but not the reverse.

### Assessment
**Ready for the human gate?** With fixes

**Reasoning:** I found no correctness defects in the hunks I read, and the wire format and serialization hold. The review is incomplete on the shared-state writers (`chooseUnit`, `OptimisticWrite`), which I did not read in full, so the verdict depends on that check.