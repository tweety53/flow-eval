# Review panel — kan-757-offline-l5-be-per-item-if-match-on-group-session

Rendered from the store. Do not edit: the findings are rows, and the next render overwrites this file.

| ID | Slot | Severity | Location | Note | Lineage |
|---|---|---|---|---|---|
| F1 | primary | Important | src/app/workout/src/main/kotlin/com/gymie/workout/core/services/workout/impl/WorkoutFieldPatchServiceImpl.kt:209 | An unchanged set PATCH is no longer a no-op: it writes, bumps the workout revision and re-attributes the set |   |
| F2 | primary | Important | src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionExceptionHandler.kt:100 | A never-stamped set's 409 set_changed carries updatedAt null, contrary to R19 and decision unknown-actor-omitted; the test asserting omission cannot see it |   |
| F3 | primary | Minor | src/app/groupsession/src/test/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionFloorSetConflictTest.kt:134 | Test names contradict what they assert |   |
| F4 | principles | Important | src/app/workout/src/main/kotlin/com/gymie/workout/core/services/workout/impl/WorkoutFieldPatchServiceImpl.kt:209 | Idempotency — an identical set PATCH is no longer idempotent |   |
| F5 | principles | Important | src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionExceptionHandler.kt:92 | DRY / Single Source of Truth — the {weight, effort, confirmed} wire shape is stated twice |   |
| F6 | principles | Important | src/app/groupsession/src/test/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionFloorSetConflictTest.kt:144 | Testing principles — the omission assertion cannot fail for the defect it guards |   |
| F7 | principles | Minor | src/app/groupsession/src/test/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionFloorSetConflictTest.kt:134 | Least Astonishment — test names say null, tests assert absence |   |

findings-total: 7
finding-status: F1 fixed
finding-status: F2 withdrawn premise false — ProblemDetail keeps null; contract restored to explicit null under decision unknown-actor-null
finding-status: F3 fixed
finding-status: F4 fixed
finding-status: F5 fixed
finding-status: F6 withdrawn the omission assertion is gone — tests now assert the raw "updatedAt":null body under decision unknown-actor-null
finding-status: F7 fixed

reproducers-total: 7
finding-reproducer: F1 .superpowers/sdd/reproducers/0-primary-1.sh
finding-reproducer: F2 .superpowers/sdd/reproducers/0-primary-2.sh
finding-reproducer: F3 none — naming only
finding-reproducer: F4 .superpowers/sdd/reproducers/0-principles-3.sh
finding-reproducer: F5 .superpowers/sdd/reproducers/0-principles-1.sh
finding-reproducer: F6 .superpowers/sdd/reproducers/0-principles-2.sh
finding-reproducer: F7 none — naming only

## Pass log

### Round 0

- roster: compact — 48
- diff size 2192 lines, under cap; docs-only exit 1 (first non-doc path GroupSessionModels.kt); roster dispatched primary+principles in one bundle (opus/high); no operator-named addition — the resolved list ran alone; standards: CLAUDE.md, CONTRIBUTING.md, kotlin-backend-development-standard.mdc

### Round 1

- panel-fix-1 inline (parent): fixed F1/F4 (stamp only on change), F5 (current emitted as FloorSetValuesDto), F3/F7 (contract restored to null, names now true); F2/F6 withdrawn — premise superseded (decision unknown-actor-null)
- F1/F4 reproducers re-authored: their premise was the defect line itself (voided by the fix, KAN-839); prove-reproducer pre-fix leg exit 3 (cold Gradle in scratch worktree timed out), post-fix leg not demonstrated; pre-fix demonstrated taken from the dispatch-time run (sha e5d5b69f…, exit 1) plus the mutation flip below
- re-run primary+principles on fix-round-1.diff (sonnet/low): all findings verified, no new finding
fix-mutation: src/app/workout/src/main/kotlin/com/gymie/workout/core/services/workout/impl/WorkoutFieldPatchServiceImpl.kt — stamp unconditionally (drop the patched == original check) — WorkoutFieldPatchServiceImplSetConflictTest.an unchanged patch keeps the stored stamps
fix-mutation: src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionExceptionHandler.kt — skip null updatedAt / actor properties with ?.let — GroupSessionFloorSetConflictTest.a never-stamped set answers null updatedBy and updatedAt; GroupSessionTransitionProblemTest.an actor from before the migration is null
fix-mutation: src/app/groupsession/src/main/kotlin/com/gymie/groupsession/infrastructure/presentation/web/GroupSessionExceptionHandler.kt — none — current built from FloorSetValuesDto is a same-shape refactor; GroupSessionFloorSetConflictTest current.* assertions cover the wire shape
fix-mutations-total: 3
