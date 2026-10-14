# Tuning a reviewer prompt

A loop for lifting a slot whose must-find a cheap model misses: read why the trials missed it, add
one generic required step to the prompt, and measure each wording as a versioned variant against a
same-k before run. Every variant is a pushed agents commit, so every run's `prompt pin` names
exactly the bytes it measured.

Run the commands from the `flow-eval` repository root. `<key>` is the change's Jira key, `<slot>`
the reviewer slot, `<agents>` the agents checkout and `<repos>` the directory holding the case's
source checkouts.

## When

- A slot or role scores 0, or close to it, on a cheap model (haiku-low) for a case whose must-find
  a stronger model or a human reviewer does catch.
- Tune the agents prompt (`skills/flow/<slot>-reviewer-prompt.md`), never the vendored copy under
  `prompts/`: `prompts sync` overwrites it, and agents stays the prompt's single source.

## Read why it missed

Open the before run's `runs/<id>.html` and read each trial's output against the case's
`labels.json` must-find. Note what the trials reported instead, and name the one reasoning step
they skipped — for primary, re-reading the unchanged writers of a reused flag; for principles,
asking whether a stored flag summarises other state. The step, not the defect, is what the prompt
gains.

## Write one generic required step and an evidence output section

The shape both tunings below share:

- **A numbered step, done before anything else**, stated in the prompt's own voice: list the kind
  of thing to examine, grep every line that writes each one (unchanged lines included), and the
  test that turns a line into a finding, with its severity.
- **An output section listing the evidence** — each item, its writers as `path:line` and a verdict
  per writer — so a trial cannot skip the step silently.
- **Never a case identifier.** No field, file or domain word from the case: the step must hold for
  any diff, or the run measures memorisation.

The agents lint `check-verbatim-moves.sh` fails new prompt text until the change acknowledges it:
list each sentence its `FAIL` lines print, after `::`, in
`spectre/changes/<change>/verbatim-moves.txt` (a heading as `\### …`), committed as a planning
commit.

## Version every variant

Each variant is one commit on a pushed agents branch `<key>-<slot>-variants`, off the change
branch:

```bash
git -C <agents> worktree add -b <key>-<slot>-variants <agents>-variants HEAD
# edit skills/flow/<slot>-reviewer-prompt.md in <agents>-variants
git -C <agents>-variants commit -am "feat(flow): <slot> <step> step, variant <n>"
git -C <agents>-variants push -u origin <key>-<slot>-variants
```

Pushed, every run's `prompt pin` resolves for whoever reads the run later; a losing variant never
reaches the change branch.

## Measure

First a before run on the current pin, at the same k every variant will use, so the counts compare
like for like:

```bash
agents=$(mktemp -d)
git -C <agents> archive -o "$agents/agents.tar" "$(cat prompts/PIN)" stats
docker build -f docker/runner.Dockerfile --build-context agents="$agents" -t flow-eval-runner .
go run ./cmd/flow-eval run --role review --slot <slot> --harness claude-code \
  --model claude-haiku-5-5 --effort low -k 6 --repos <repos>
```

Then, per variant — sync, rebuild the runner (its build refuses another pin's archive), run:

```bash
go run ./cmd/flow-eval prompts sync --agents <agents> --sha <variant sha>
agents=$(mktemp -d)
git -C <agents> archive -o "$agents/agents.tar" "$(cat prompts/PIN)" stats
docker build -f docker/runner.Dockerfile --build-context agents="$agents" -t flow-eval-runner .
go run ./cmd/flow-eval run --role review --slot <slot> --harness claude-code \
  --model claude-haiku-5-5 --effort low -k 6 --repos <repos>
```

- Keep k at 6 or below; every trial also pays a judge call.
- **Finds** = the trials whose `passed` is true in `runs/<id>.jsonl` (a review trial passes at
  recall 1):

  ```bash
  jq -s 'map(select(.trial)) | map(select(.passed)) | length' runs/<id>.jsonl
  ```

- Read the variant's page for whether the trials filled the evidence section and what they listed;
  that is what the next variant's wording answers.
- Read the failed trials too. One that describes the must-find but anchors it at another line
  more than 15 lines from the label is never put to the judge — a defect at several writers is
  reported at whichever the reviewer met first. Add each such writer to the label's `also`
  ([`docs/cases.md`](cases.md)), then `flow-eval regrade` every run of the case, and count again.
- A run that ends with an infra error is re-run once whole; a second infra error is reported,
  never counted as a miss.
- Do not rebuild the runner or sync while a run is in flight: its remaining trials would start on
  the new image.

## Stop

- At most three variants.
- Stop early once a variant finds the must-find in at least half the trials.
- The winner has the most finds; a tie goes to the shorter wording.
- No variant ever finds it: stop, decide, and land nothing.

## Land

1. Cherry-pick the winner onto the agents change branch as one commit. Its body says what the step
   makes the reviewer do, one line per variant — `variant <n> <sha>: <one-line wording difference>
   — finds <n>/<k>` — and the before run's finds. It carries the change's `Task-Id: <n>` trailer
   before it is pushed: adding it after the push rewrites a pushed sha and forces a force-push. Push it.
2. Vendor the change branch's tip into flow-eval, and check its prompt bytes equal the winner's, so
   the winner run measured exactly what ships:

   ```bash
   go run ./cmd/flow-eval prompts sync --agents <agents> --sha "$(git -C <agents> rev-parse HEAD)"
   git -C <agents> diff <winner sha> HEAD -- skills/flow/<slot>-reviewer-prompt.md   # empty
   ```

3. Commit `prompts/PIN`, the vendored prompt, and every variant run's `.jsonl`, `.md` and `.html`
   with `runs/index.html`; the before run is its own earlier commit.
4. Baselines change only by an explicit `flow-eval baseline set <run-id>`: a cheap-model run is a
   candidate against the slot's baseline, not its replacement.
5. If landing rebases the agents branch and rewrites its sha, re-sync `prompts/PIN` to the landed
   sha. The bytes are the same, so nothing is re-run.

## Worked examples

| Slot | Step | Before | After | agents |
|------|------|--------|-------|--------|
| primary | Shared-state check: re-read every writer of state the diff reuses | 0/33 | 14/33 across models (opus-low 8/9, haiku-low 4/15, sonnet-low 2/9) | `2e053aaa` |
| principles | Derived-state check: a stored summary of other state set or cleared from part of its facts is a Single Source of Truth violation | 0/6 haiku-low k=6; 0/9 across models at k=3 | k=6: haiku-low 3/6 (variant 1, `df6cae35`), sonnet-low 3/6, opus-low 6/6 | `0ff796c0` (read-order fix `55252252`), vendored at `2441dced` |
| failure-modes | Failure-recovery check: a success that clears a failure's state must re-confirm everything the failure left unconfirmed | 0/6 haiku-low k=6 | haiku-low 4/6 at k=6 (variant 2, `c0372d03`; variant 1 `0502293c` 1/6); k=3: haiku-low 2/3, sonnet-low 3/3, opus-low 3/3 | `c0372d03` (variant 2, on the pushed `kan-979-failure-modes-variants` branch) |

All three ran on the `kan-867-qm-2-step-1-frontend-settings-workout-mode-focus` review case. Before the
principles step, haiku-low reported the inlined failure-message literal, the tripled save-failure
string and the duplicated default instead of `loadFailed`'s partial writers; with it, every trial
filled the Derived-state check and listed `loadFailed`'s writers, and four judged a writer partial; the judge credited three.

Before the failure-modes step, haiku-low reported a cancelled write leaving the optimistic value,
a lost-response revert, a stale-unit overwrite and the mutex held across the PUT, and never asked
what `chooseUnit`'s later success clears after the failed load. Variant 1 had every trial fill the
Failure-recovery check, but most judged `:295` a full recovery because the mode was "untouched",
or never grepped past the diff hunks; variant 2 made the reviewer grep the whole file and name a
re-confirming line per unconfirmed value, and four trials wrote `none` for the mode at `:295` and
named the discarded `sessionView` in the unit PUT's answer.

The principles figures are as re-graded after F6's label gained `also: [295]`. Scored against
`:339` alone, the same runs read sonnet-low 1/6 and opus-low 0/6: every opus trial reported F6 at
`chooseUnit`'s `:295`, 44 lines from the label, so none was ever judged.
