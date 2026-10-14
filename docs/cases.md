# Adding an eval case

A case is a directory `cases/<role>/<id>/` holding `case.json` and the role's assets. `flow-eval
case add <role>` mines a draft from a source checkout's flow history, you check and edit it, then
`flow-eval case validate` and commit. **The commit review is the label approval**: there is no
separate approval flag, so whoever reviews the commit is signing off the labels.

Every miner only reads the source checkout (`git` read commands and the archived change's files);
it never writes to it. A miner refuses to overwrite an existing case directory, and prints:

```text
draft cases/<role>/<id>: edit labels, then commit
```

Run the commands from the `flow-eval` repository root; `--root` (default `cases`) moves the draft
elsewhere. For a change whose code is in one checkout and whose plan and archive are in another —
a gymie-frontend change planned in gymie — `--repo` names the code's checkout and `--plan-repo` the
plan's (`impl` and `review`).

## Implementer case (`impl`)

```bash
flow-eval case add impl --repo ../gymie --change kan-757-offline-l5-be-per-item-if-match-on-group-session --task 3
```

What it mines, from `<plan-repo>/spectre/changes/archive/<change>/`:

- **The reference commit**: the commit of the last `ledger.md` dispatch with `Role: implementer`
  and `Task: <n>` whose `Key` is not a fix round (`-fix-`): a fix round's parent already holds the
  task's answer. Its parent is the case's `snapshot`, the commit the trial's `/work` starts from.
- **`task.md`**: task `n`'s section of `tasks.md`, from its checkbox line to the next task's.
- **`context.md`**: `proposal.md` then `design.md`.
- `tasks.md`, `proposal.md` and `design.md` are read as they stood at the snapshot, from
  `spectre/changes/<change>/` in that commit — never the archived copies, which carry Corrections
  and decisions written from review fixes landed after the task, i.e. the answer. With
  `--plan-repo`, they are read from that checkout's last commit (any ref) to the change's
  `tasks.md` authored no later than the reference commit.
- **`heldout.patch`**: the reference commit's changes to test files — paths under `src/test/`,
  `commonTest/` or `desktopTest/`, or ending `Test.kt` or `_test.go`. The agent never sees it; the
  grader applies it after the agent exits.
- **`reference.stat`**: `git diff --stat` of the reference commit, the size the over-building ratio
  is measured against.
- **`case.json`**:
  - `id`: `<change>-task-<n>`; `source.repo`: the checkout's directory name, which `flow-eval run
    --repos <dir>` finds as `<dir>/<source.repo>`.
  - `files`: the backticked paths of task `n`'s `**Files:**` field.
  - `verify`, in order: every command of the source repository's `.flow/project.md` `## lint` block
    (at the snapshot; none when it declares none, as gymie-frontend does); then, per Gradle module holding held-out Kotlin or Java tests,
    `./gradlew :<module>:<task> --tests '<Class>' …`; then each such module's suite,
    `./gradlew :<module>:<task>`. The module's Gradle project is taken to be `:<its directory
    name>`, the task is the source set (`test`, `desktopTest`), and `commonTest` runs under
    `desktopTest`.
  - `needsDocker`: true when a held-out module's `build.gradle.kts` or `build.gradle` mentions
    `testcontainers`; the trial then gets its own Docker-in-Docker sidecar.

What to check and edit:

- `verify`: the Gradle project path when a module's project name is not its directory name; a
  selector for a held-out test outside `src/<set>/` (none is generated); drop a lint command that
  does not suit a trial; add the lint commands of a checkout that declares none, after confirming
  each Gradle task exists at the snapshot.
- `heldout.patch`: it holds every changed file on a test path, fixtures and helpers included.
  Edit it by hand when holding one out would be unfair to the agent.
- `files`: globs are allowed; the grader counts touched files outside them.
- `needsDocker`: set it when the module suite needs Docker through another build file.
- Run the reference commit through `verify` in the trial's sandbox shape: the snapshot as the base
  commit, the reference diff on top, the runner image with the read-only Gradle cache. A command
  the reference fails for a reason outside the task is unpassable for every agent: drop it when
  the snapshot itself fails it or it fails on timing (a module suite), or give it the environment
  the sandbox lacks (`MANAGEMENT_HEALTH_REDIS_ENABLED=false` for a Spring module whose health
  check needs a Redis the trial does not have). A dropped module suite still runs the agent's own
  test classes by name: the grader runs `./gradlew <task> --tests '<Class>'` for each file declaring
  `@Test` the agent added or changed in a module whose suite no `verify` command runs.

## Reviewer case (`review`)

```bash
flow-eval case add review --repo ../gymie-frontend --plan-repo ../gymie --change kan-867-qm-2-step-1-frontend-settings-workout-mode-focus
```

What it mines, from the same archive:

- **The round-0 range**: base is the parent of the first `ledger.md` implementer commit; head is
  the last implementer commit before the first `panel-fix` dispatch. `diff.patch` is `git diff
  base head` less the change's plan files (`spectre/changes/`, `openspec/changes/`), and
  `case.json`'s `snapshot` is head.
- **`labels.json`**, from `panel.md`'s findings table and its `finding-status:` lines:
  - Every label carries `slot`, the row's Slot column: the panel slot that raised it.
  - `mustFind[]`: `fixed` findings of severity Critical or Important, each `{id, slot, file, line,
    summary, severity}`, plus `also` where edited in (below). `file` and `line` come from the row's Location (the first line number is
    kept, 0 when there is none). A finding that duplicates another slot's finding is a must-find of
    its own slot, wherever its cited line lies.
  - Plan-alignment findings are left out: a fixed finding located in the change's own plan
    artifacts (`spectre/changes/…`, `openspec/changes/…`). A review case carries the diff and
    labels only, so the reviewer runs without flow's plan-alignment check.
  - `knownFalse[]`: `withdrawn` findings whose reason says the premise was false (`premise
    false`, `false premise`, `not a defect`, `false positive`).
  - `ignored[]`, each with its `reason`: every other `withdrawn` finding (a superseding decision, a
    duplicate, a fix made elsewhere, an operator call), and every finding `diff.patch` cannot
    show — one whose Location carries another repository's `<repo>:` prefix, or names a file
    `diff.patch` does not change, as a later fix round's finding does.
  - Fixed Minor findings and `deferred` findings are left out.

What to check and edit:

- Every `mustFind` must be findable in `diff.patch` alone. A later round's finding on a file the
  round-0 diff also changes still lands in `mustFind`, so delete it, and one that needs the plan to
  be seen even though it sits in a code file.
- Move an `ignored` finding to `knownFalse` when its premise was in fact false, or delete it.
  The grader reads `mustFind` and `knownFalse` only.
- Never move a must-find to `ignored` because another slot raised the same defect: a `--slot X`
  review run grades a trial on the must-finds slot X raised only, and on every `knownFalse` label
  whichever slot raised it. A run whose cases include one with no must-find for its slot is
  refused before any trial, naming every such case; leave them out with `--cases`.
- `case validate` refuses a review case whose labels carry no `slot` (one mined before labels
  carried it): re-mine it.
- Check each `line` against `diff.patch`: the grader pairs a reported finding with a label in the
  same file within ±15 lines.
- A defect that sits at several lines of its file — a stored value each of several writers keeps
  wrong — lists the others in `also` (`"also": [295]`), added by hand: a finding near `line` or
  any `also` line is put to the judge, which is shown every one, so a reviewer may anchor the
  defect at any of its writers.
- `severity` feeds `severity_agreement`; correct it if the panel's was later revised.

## Visual case (`visual`)

```bash
flow-eval case add visual --id home-title-low \
  --mockup ../gymie/docs/design/mockups/home.png \
  --capture ../gymie-playwright/fidelity/home.spec.ts-snapshots/home-darwin.png \
  --verdict mismatch --defect "the title sits 2px below the frame's baseline" --region 24,88,320,40
```

- `--verdict` is `match` or `mismatch`. A mismatch needs at least one `--defect`; a match takes
  none. `--defect` repeats; a `--region x,y,w,h` (capture pixels) belongs to the `--defect` before it.
- It copies the capture as it is into `capture.png`. Where the checkout holding the mockup
  declares a `mockup frame` row in `.flow/project.md` (gymie: `scale=2 status=26 border=1`),
  `mockup.png` is the board cut to its frame by flow's `compose-mockup-frames.sh` — exactly the
  crop flow's visual verification measures, the same size as the capture — and a capture of
  another size is refused. `--compose` names that script (default
  `~/.claude/skills/flow/scripts/compose-mockup-frames.sh`). Without the row the mockup is copied as
  it is. It writes `labels.json` (`verdict`, `defects[]` of `{summary, region?}`). `source.repo` is
  the checkout holding the mockup.
- Pair a mockup with its capture through the fidelity spec's `.mockups` sidecar.
- The verdict and defects are the ones flow's own visual verification recorded for that pair in an
  archived change's `visual-verification.md`, its defect text as written; a `--region` only where
  the record gives one. A difference that record did not count — pinned seed data, a font-width
  wrap — is not a departure (decision `visual-labels-from-flow-verdicts`).

What to check: open both images and confirm the recorded verdict still reads true of them, and that
each defect summary names one departure a reviewer could point at.

## Validate and commit

```bash
flow-eval case validate
git add cases/<role>/<id>
git commit -m "feat(cases): add <role> case <id>"
```

`case validate` exits 0 with the case-set hash, 1 listing each invalid case and why, 2 on a usage
error.

### Keep the snapshot alive

An impl or review case's `snapshot` is often on no branch — a squash-merged change's commits are
reachable from nothing — so `git gc` would prune it and the case would stop running. Give it a ref
in its source checkout and push that ref (the miner never writes to a checkout, so this is yours):

```bash
git -C ../gymie update-ref refs/flow-eval/<case-id> <snapshot>
git -C ../gymie push origin refs/flow-eval/<case-id>
```

On another machine, fetch them before a run:

```bash
git -C ../gymie fetch origin 'refs/flow-eval/*:refs/flow-eval/*'
```

## Retiring a case

Delete its directory and commit. The case set's hash changes with it, so `flow-eval compare`
refuses (exit 2) to compare a run on the new set with a baseline on the old one: run the role
again and record a new baseline with `flow-eval baseline set <run-id>`.
