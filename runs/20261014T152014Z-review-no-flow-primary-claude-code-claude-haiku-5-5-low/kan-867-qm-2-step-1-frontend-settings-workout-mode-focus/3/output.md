#### Critical
- (none)

#### Important
- (none)

#### Minor
- `shared/src/commonMain/kotlin/com/gymie/shared/ui/SettingsState.kt:278` (approx.): the load-failure fallback is now a hard-coded `"Could not load your settings."`, replacing the existing `FailureMessages.COULD_NOT_LOAD_WEIGHT_UNIT` constant, whose import this diff also removes. The new string is not shared with anything, so a future wording change has to be made in two places, and the message shown to the user changes.