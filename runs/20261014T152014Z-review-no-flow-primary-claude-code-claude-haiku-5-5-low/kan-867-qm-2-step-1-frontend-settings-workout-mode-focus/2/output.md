No critical or important issues in the hunks. One minor item: the `load()` failure path now hardcodes a message that duplicates the `FailureMessages` constant it replaced.

#### Critical
- (none)

#### Important
- (none)

#### Minor
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:~286` — `notifications.failure(it.message ?: "Could not load your settings.")` hardcodes a string, replacing `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT`, which the diff removes from the import. The duplicate string can drift from the shared constant, and the constant may now be unused.