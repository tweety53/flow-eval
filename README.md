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
- Measure Haiku as `--model claude-haiku-5-5`: the CLI's `haiku` alias resolves to Haiku 4.5.

Writes `runs/<id>.jsonl`, `runs/<id>.md` and `runs/<id>.html`, and rewrites `runs/index.html`. Raw
transcripts, outputs, `work.patch` and `verify.log` go to `runs/<id>/`, which is gitignored.

**Pricing:** `cost_usd` comes from `pricing.json` — per model id, USD per million tokens of `input`,
`output`, `cache_read`, `cache_write` (5-minute), `cache_write_1h`, and the `source` page. A `long`
tier (`over_prompt_tokens` and the five prices) prices each call whose prompt exceeds it; it needs
per-call usage, which codex does not report. A trial model missing from the file is an infra error;
a judge model missing from it fails the trial (design.md §6).

### `compare`

```bash
flow-eval compare <candidate-run-id>                              # against baselines/<role>.json
flow-eval compare <candidate-run-id> --baseline <run-id> --margin 5
```

The pass rule is design.md §7. Writes `runs/<cand>-vs-<base>.md`. Runs with different flow-eval
shas, judges, prompt pins or served models are still compared, with a `DIFFER` warning: check
`git log <base sha>..<cand sha> -- internal/grade` before trusting a verdict across shas.

### `baseline set`

```bash
flow-eval baseline set <run-id>
```

Writes `baselines/<role>.json` and repaints the index and run pages. Commit it with the run's
`.jsonl` and `.md`.

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

## Reading results

- **`runs/index.html`** — start here: every run by role, newest first, with its score, passes,
  infra errors, cost and wall-clock, each linking to its page.
- **Colours** (valid trials only): green — every trial passed; yellow — some did; red — none did;
  uncoloured — no valid trial; blue — beats the role's baseline in a run `compare` would accept.
- **`runs/<id>.html`** — one self-contained page per run: header, role score, the case table,
  infra errors, and every trial with its metrics, error, `output.md`, `verify.log` and `work.patch`.
- **`served`** — the model id(s) the run's trials were served, read from their `served_model`, so it
  names what an alias such as `sonnet` resolved to; `–` when the harness names none (codex).
- **Case table:** `valid/k` (infra errors are excluded and listed apart), `passes` and `pass^k`,
  `score` and `stddev`, and the candidate's cost, wall-clock and tokens. `judge cost` is never
  charged to the candidate. A review trial passes when its recall is 1.
- Every field of `runs/<id>.jsonl` is defined in design.md §6; the run id, the page and the colours
  in §8. Scoring is §7, and the visual score is decision `visual-score-balanced-accuracy`.

## Exit codes

| Exit | `compare` | Other commands |
|------|-----------|----------------|
| 0 | non-inferior | success (`run`: every trial recorded, infra errors included) |
| 1 | regression (wins over 3) | `case validate`: an invalid case, listed with why |
| 2 | usage error, or the runs cannot be compared (role, slot, case set, k, missing trials) | usage or infrastructure error |
| 3 | non-inferior, with an improvement to review | — |
