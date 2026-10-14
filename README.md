# flow-eval

flow-eval measures how well a model, at a given effort and through a given agent harness, does each
flow role — implementer, code reviewer and visual-verify — on real cases mined from the gymie
projects, each trial in an isolated container. `compare` then says whether a cheaper or newer
candidate can take over a role from its baseline. A role is judged on its own, never in aggregate.
The design is `kan-964-flow-eval-harness/design.md` in [`spectre/changes/`](spectre/changes/), or in
`spectre/changes/archive/` once the change has landed.

## Setup

- Go (the version `go.mod` names): `go build -o flow-eval ./cmd/flow-eval`, or use `go run
  ./cmd/flow-eval` in place of `flow-eval`.
- Docker, for the images and every trial.
- The agents repository checked out at `../agents`, holding the commit `prompts/PIN` names.
- impl and review cases need their source checkout at `<repos>/<source.repo>` (`--repos`, default `..`).
- Credentials, from the environment only:

| Harness | Set one of |
|---------|------------|
| `claude-code` | `CLAUDE_CODE_OAUTH_TOKEN` or `ANTHROPIC_API_KEY` (with both set, a trial gets only the first) |
| `codex` | `OPENAI_API_KEY` |
| `muse-code` | none: run the one-time [Muse Code setup](docs/muse-code.md) |

`review` and `visual` runs also call the judge `judge.json` pins on the host, through the host's
`claude` CLI, so they need `CLAUDE_CODE_OAUTH_TOKEN` or `ANTHROPIC_API_KEY` whatever the candidate's
harness. A missing credential exits 2 before any container starts.

Build both images from the repository root, and rebuild the runner after `prompts sync` moves the
pin (its build refuses an agents archive of any other commit):

```bash
docker build -f docker/proxy.Dockerfile -t flow-eval-proxy .
agents=$(mktemp -d)
git -C ../agents archive -o "$agents/agents.tar" "$(cat prompts/PIN)" stats
docker build -f docker/runner.Dockerfile --build-context agents="$agents" -t flow-eval-runner .
```

**Security: a case with `needsDocker: true` runs a privileged Docker-in-Docker sidecar the agent can
reach, so a deliberate escape reaches the Docker host.** Do not run such cases on a shared host or
with an agent you do not trust (decision `dind-privileged-accepted-v1`; isolation is design.md §2).

## Commands

`-h` after any command prints its flags.

### `run`

```bash
flow-eval run --role impl --harness claude-code --model sonnet --effort low -k 1 \
  --cases kan-757-offline-l5-be-per-item-if-match-on-group-session-task-3
```

- Required: `--role impl|review|visual`, `--harness claude-code|codex|muse-code`, `--model`, `--effort`.
- `-k` trials per case (default 3), `--cases id,…` limits the run, `--parallel` (default 2).
- `--slot primary|principles|failure-modes` picks the reviewer prompt (default `primary`). A case
  with no must-find for the slot is refused.
- `--no-flow` runs the role with no flow prompt, rules or tooling — impl gets the task as a plain
  spec plus the change's proposal and design; review runs claude-code's own `/code-review` on the
  change as the working tree's diff, or on muse-code, which ships no review command, a plain request to
  review that diff (codex is refused), graded on `--slot`'s must-finds; visual
  compares the pair with flow's measuring tool removed. Its runs and baselines are keyed
  `<role>-no-flow[-<slot>]`; compare one with its flow counterpart by
  `compare <run> --baseline <flow run>`. The prompts are `prompts/no-flow/`, which `prompts sync`
  never touches.
- Measure Haiku as `--model claude-haiku-5-5`: the CLI's `haiku` alias resolves to Haiku 4.5.

Writes `runs/<id>.jsonl`, `runs/<id>.md` and `runs/<id>.html`, and rewrites `runs/index.html`. Raw
transcripts, outputs, `work.patch` and `verify.log` go to `runs/<id>/`; commit it with the run files.

**Pricing:** `cost_usd` comes from `pricing.json` — per model id, USD per million tokens of `input`,
`output`, `cache_read`, `cache_write` (5-minute), `cache_write_1h`, and the `source` page. A `long`
tier (`over_prompt_tokens` and the five prices) prices each call whose prompt exceeds it; it needs
per-call usage, which codex does not report. A trial model missing from the file is an infra error;
a judge model missing from it fails the trial (design.md §6).

### `compare`

```bash
flow-eval compare <candidate-run-id>                              # against baselines/<role>[-no-flow][-<slot>].json
flow-eval compare <candidate-run-id> --baseline <run-id> --margin 5
```

The pass rule is design.md §7. Writes `runs/<cand>-vs-<base>.md`. Runs with different flow-eval
shas, judges, prompt pins (never warned against a `--no-flow` run, which records none) or served models are still compared, with a `DIFFER` warning: check
`git log <base sha>..<cand sha> -- internal/grade` before trusting a verdict across shas.

### `baseline set`

```bash
flow-eval baseline set <run-id>
```

Writes `baselines/<role>.json` — `baselines/review-<slot>.json` for a review run, one per slot, and
`<role>-no-flow[-<slot>]` for a `--no-flow` run — and
repaints the index and run pages. Commit it with the run's
`.jsonl` and `.md`.

### `regrade`

```bash
flow-eval regrade <run-id>…
```

Grades every graded trial of each stored review run again, against the case labels and
`judge.json` as they stand now — after a label is corrected, say. The trial's own metrics, cost and
wall-clock are kept; its review grade, score, pass and judge usage are replaced, and the header
takes the cases' current hashes and judge pin and records the regrading flow-eval sha as
`regraded`. Each trial's report is read from `runs/<id>/<case>/<trial>/output.md`, or, when that
file is gone — an older run's directory was never committed — from the copy the committed run
page carries, which the re-rendered page keeps; a copy the page cut at its size cap is refused,
and the run is left as it was.
Rewrites the run's `.jsonl`, `.md` and `.html` and the index; needs the judge's credential, as `run`
does. Commit the rewritten files.

### `case add` and `case validate`

```bash
flow-eval case add review --repo ../gymie-frontend --plan-repo ../gymie \
  --change kan-867-qm-2-step-1-frontend-settings-workout-mode-focus
flow-eval case validate
```

`case add impl|review|visual` mines a draft; edit its labels and commit — the commit review is the
label approval. `case add visual` uses the installed `compose-mockup-frames.sh` from the agents
repository. The how-to for each role is [`docs/cases.md`](docs/cases.md).

### `prompts sync`

```bash
flow-eval prompts sync --agents ../agents --sha <agents commit>
```

Vendors the role prompts and `rules/*.mdc` at that commit, less what `prompts/strip.txt` lists, and
records it in `prompts/PIN`. Then rebuild the runner image, re-run each role and set new baselines.
Tuning a prompt with flow-eval is [`docs/prompt-tuning.md`](docs/prompt-tuning.md).

## Reading results

- **`runs/index.html`** — start here: the baselines first, one per role and review slot, flow and no-flow apart, then every
  run by role, newest first, with its score, passes, infra errors, cost, wall-clock and tokens in
  (the whole prompt: uncached input, cache reads and writes) and out, as min/avg/max over its valid
  trials, each linking to its page. The `vs baseline` columns give a run's role-score difference
  from its baseline in points and its cost, mean wall-clock and mean tokens as a percentage change,
  green where the run is better and red where it is worse; `compare` gives the verdict.
- **Colours** (valid trials only): green — every trial passed; yellow — some did; red — none did;
  uncoloured — no valid trial; blue — beats the role's baseline in a run `compare` would accept.
- **`runs/<id>.html`** — one self-contained page per run: header, role score, the case table,
  infra errors, and every trial with its metrics, error, `output.md`, `verify.log` and `work.patch`.
- **`served`** — the model id(s) the run's trials were served, read from their `served_model`, so it
  names what an alias such as `sonnet` resolved to; `–` when the harness names none (codex).
- **Case table:** `valid/k` (infra errors are excluded and listed apart), `passes` and `pass^k`,
  `score` and `stddev`, and the candidate's cost, wall-clock and tokens. `judge cost` is never
  charged to the candidate. A review trial passes when its recall is 1; its score is F1 × 100 plus 5
  per net extra, at most 15. An extra is a finding of any severity that matches no label and that
  the judge rates a plausible defect in the diff; net extras are extras less the unlabelled findings
  it rates implausible, floored at 0. A finding that names a must-find defect away from its label is
  neither (decision `review-extras-net`).
- Every field of `runs/<id>.jsonl` is defined in design.md §6; the run id, the page and the colours
  in §8. Scoring is §7, and the visual score is decision `visual-score-balanced-accuracy`.

## Exit codes

| Exit | `compare` | Other commands |
|------|-----------|----------------|
| 0 | non-inferior | success (`run`: every trial recorded, infra errors included) |
| 1 | regression (wins over 3) | `case validate`: an invalid case, listed with why |
| 2 | usage error, or the runs cannot be compared (role, slot, case set, k, missing trials) | usage or infrastructure error |
| 3 | non-inferior, with an improvement to review | — |
