## Context

New repository; nothing exists yet. The flow pipeline (`/Users/tweety53/Projects/agents`) dispatches
the three roles under Claude Code only, records per-dispatch tokens and cost in its store, and has no
eval harness, golden set or headless adapter. The gymie repositories hold the real history the
cases come from: archived spectre changes with `ledger.md` task commits and `panel.md` findings, and
fidelity mockup/snapshot pairs. One change because every component serves the one loop in §1 and
none is useful alone.

## 1. Purpose and operating loop

This tool exists to let one role move to a cheaper model or effort, a new release or another agent
provider without losing quality. The loop:

1. A new model ships, or the provider changes.
2. `flow-eval run` runs the affected role(s) at the candidate model and effort, `k=3` trials per case.
3. `flow-eval compare` checks the candidate against the role's baseline:
   - Exit 0 (non-inferior): the candidate may take over the role.
   - Exit 1 (regression) or exit 3 (improvement flagged): the operator reads the report and decides.
     The decision may be a flow `decision.json` policy change, a prompt change, or a new baseline.

The candidate is judged per role, never in aggregate. A model can be safe for review and unsafe for
implementation.

## 2. Architecture and isolation

```text
flow-eval run --role R --harness H --model M --effort E -k 3
  for each case x trial:
    docker network (internal, per trial)
      ├─ trial container (runner image: JDK 25, Node, claude + codex CLIs, flow-eval)
      │    /work  ← repo snapshot at the case's base (git archive → fresh git init, 1 commit)
      │    /case  ← prompt inputs (read-only); held-out files are never mounted
      ├─ egress proxy (cmd/egress-proxy, allowlist CONNECT only)
      └─ dind sidecar (only when case.needsDocker)
    adapter(H) → headless CLI → transcript.jsonl → normalized Trial record
    grader(R)  → runs in the same container after the agent exits
  → runs/<run-id>.jsonl, runs/<run-id>.md
```

- **No answer leakage.** The snapshot is a `git archive` of the base sha, re-initialised as one
  commit, so the reference commit is unreachable. Held-out tests are copied in only after the agent
  process has exited, and only for grading.
- **Network.**
  - The trial network is `--internal`. All traffic goes through `HTTPS_PROXY` to the egress proxy.
  - The proxy allows CONNECT to an allowlist file `docker/egress-allowlist.txt` and refuses
    everything else:
    - provider APIs: Anthropic, OpenAI, and Meta's for Muse Code (its API host and its sign-in
      host, for token refresh);
    - Maven Central;
    - the Gradle plugin portal and `services.gradle.org`;
    - the npm registry;
    - Docker Hub, for dind pulls only.
  - Every refusal is logged as a metric (`egress_denied`).
- **Credentials.**
  - `CLAUDE_CODE_OAUTH_TOKEN` (or `ANTHROPIC_API_KEY`) and `OPENAI_API_KEY` reach the container by
    environment only, and only the ones the chosen harness needs.
  - Muse Code's login is the exception: it is a file, not an environment variable. A one-time
    operator setup (`docs/muse-code.md`) signs in inside the runner image and leaves the login and the CLI's
    Linux binary in the named volume `flow-eval-muse`. A `muse-code` trial mounts both read-only,
    and the trial command copies the login into the trial's own config home. Nothing a trial
    writes reaches the volume. The volume is a named volume, never a host path, and the sandbox
    refuses any harness mount that is not one.
  - Host `~/.claude`, `~/.codex`, the host Docker socket and the gymie checkouts are never mounted.
    Snapshots are streamed in with `docker cp`.
- **Testcontainers.**
  - gymie's integration tests start Postgres themselves through Testcontainers, so a case with
    `needsDocker: true` gets a per-trial `docker:dind` sidecar. The trial sets
    `DOCKER_HOST=tcp://dind:2375` and `TESTCONTAINERS_RYUK_DISABLED=true`.
  - The sidecar is a separate daemon, so a trial can never see or touch host containers.
- **Caches.**
  - A named volume `flow-eval-gradle-<repo>` holds `~/.gradle` dependency caches across trials.
    Dependencies are inputs, not answers.
  - Build outputs stay inside the container, which is removed after grading, outcome aside.
- **Harness configuration.** Each trial starts the CLI with a fresh config home: an empty
  `CLAUDE_CONFIG_DIR` for Claude Code, an empty `CODEX_HOME` for Codex, and for Muse Code an
  `XDG_CONFIG_HOME` holding only the copied login. Only the vendored role prompt and the case
  inputs shape the agent. No user rules, hooks or plugins load.
  - `--bare` is not used: it accepts only `ANTHROPIC_API_KEY` and would refuse the subscription
    `CLAUDE_CODE_OAUTH_TOKEN` (measured: `claude --help` @ Claude Code 2.1.295). A fresh container
    with an empty config dir gives the same clean slate.

## 3. Cases and how to add them

A case is a directory `cases/<role>/<id>/` holding `case.json` plus role assets, validated by
`flow-eval case validate`. Common fields are `id`, `role`, `source` (`{repo, change, url?}`) and
`notes`. `case add <role> …` mines a draft from gymie history and writes it. The operator edits the
labels and commits. **The commit review is the label review**: no separate approval flag exists.
`docs/cases.md` is the how-to.

| Role | `case add` input | Mined from | Assets in the case dir | Ground truth |
|------|------------------|------------|------------------------|--------------|
| `impl` | `--repo <path> --change <name> --task <n>` | the change's `ledger.md` implementer commit for task `n`; base = its parent; `tasks.md` task `n` text; `proposal.md`, `design.md` | `task.md`, `context.md` (proposal + design), `heldout.patch` (the reference commit's test files), `reference.stat` (reference diff stat) | held-out tests pass; build and lint pass; module suite passes, or, where the case drops it, the agent's own test classes there |
| `review` | `--repo <path> --change <name>` | the change's round-0 review range: base = parent of the first implementer commit in `ledger.md`; head = the last implementer commit before the first panel-fix; findings and `finding-status` lines in `panel.md` | `diff.patch`, `labels.json` | `labels.json`: `mustFind[]` (status `fixed`, severity Critical/Important) as `{id, file, line, summary, slot}`, `slot` being the panel slot that raised it, `knownFalse[]` (status `withdrawn` whose reason says the premise was false) |
| `visual` | `--mockup <png> --capture <png> --verdict match\|mismatch [--defect "<text>" --region x,y,w,h]` | gymie `docs/design/mockups/*.png` paired through `gymie-playwright/fidelity/*.spec.ts.mockups` sidecars with `*-snapshots/*.png`; mismatches from archived `visual-verification/` evidence and fidelity snapshot history | `mockup.png`, `capture.png` | `verdict`; for a mismatch, `defects[]` each `{summary, region?}` |

- A `withdrawn` finding whose reason does not say the premise was false — a later decision
  superseding it, a duplicate, a fix made elsewhere, an operator call — is neither must-find nor
  known-false (decision `known-false-explicit-only`). Neither is a finding `diff.patch` cannot show:
  one in another repository, or at a file the round-0 diff does not change. `case add` puts each
  under `ignored[]` with its reason, and the operator moves it if they disagree.
- An impl case's `task.md` and `context.md` are the plan as it stood at the snapshot, read from
  that commit: the archived plan carries Corrections and decisions written from review fixes landed
  after the task.
- An impl case sets `needsDocker` when the held-out tests or the module suite use Testcontainers.
  `case add` detects this by grepping the module's build file for `testcontainers`.
- Captures are committed as-is. They come from the macOS `-darwin` baselines, and no rendering
  happens at trial time, so platform drift cannot enter. A mockup is committed as its frame, cut
  from the board by the source repository's `mockup frame` geometry (decision
  `visual-mockup-frame-crop`).

**Case set: one case per role** (operator, 2026-10-10, Task 33; the v1 set was 6 impl, 6 review,
13 visual), the one each recorded run used:

- impl: `kan-757-offline-l5-be-per-item-if-match-on-group-session-task-3`, a gymie backend task;
- review: `kan-867-qm-2-step-1-frontend-settings-workout-mode-focus`, a gymie-frontend change with
  must-finds for `primary`, `principles` and `failure-modes`;
- visual: `kan-870-qm-q6`, a `mismatch` pair. With no `match` case, the balanced accuracy of
  decision `visual-score-balanced-accuracy` has one class, so a verifier that always answers
  `mismatch` scores 100.

## 4. Role prompts

- `prompts/` vendors the real flow prompts from a pinned agents sha (`prompts/PIN`):
  - the implementer dispatch blocks of `skills/flow/implement.md`;
  - `skills/flow/primary-reviewer-prompt.md`, `skills/flow/principles-reviewer-prompt.md` and
    `skills/flow/failure-modes-reviewer-prompt.md`, one prompt per review slot (decision
    `review-three-slots`);
  - `skills/flow/visual-verify-verifier.md`'s judging procedure, with `measure-visual-properties.sh`
    built at the same pin into the runner image (decision `visual-measure-tool-in-runner`);
  - `skills/flow/reviewer-calibration.md`, appended to the primary reviewer prompt, and
    `skills/flow/engineering-principles.md`, appended to the principles reviewer prompt.
- `flow-eval prompts sync --agents <path> --sha <sha>` copies the named source files at that sha,
  then applies `prompts/strip.txt`, and replaces `prompts/rules/` with that sha's `rules/*.mdc`, the
  shared rules a project's `## standards` may name.
- `prompts/strip.txt` is an ordered list of literal section headings whose sections are removed:
  Claude-only scaffolding such as the MODEL HANDSHAKE, ToolSearch, Agent-tool, flow-store and
  `.superpowers/sdd` report-path blocks. An entry led by `~` cuts one literal clause instead, for a
  section that is kept but names an input a trial lacks; a clause that opens a line is cut with
  whichever of the whitespace before and after it holds fewer line breaks (the after, on a tie), so
  a whole line cut by a `~` clause keeps the blank line that follows it.
- The renderer fills `{{.Task}}`, `{{.Context}}`, `{{.Diff}}`, `{{.Standards}}`, `{{.Mockup}}`,
  `{{.Capture}}` with `text/template`.
- A principles trial's `{{.Standards}}` — upstream's `[STANDARDS_PATHS]` — is derived from the case
  snapshot's own `.flow/project.md` `## standards`, never stored in the case, so case hashes do not
  change. The section is read as flow's `project-get` reads it: a leading UTF-8 BOM dropped, the
  heading matched as the exact line `## standards`, the section ending at the next `## ` line. Each entry resolves per flow's three entry
  forms and containment rules
  (`skills/flow-contracts/project-configuration-standards.md`): a bare `*.mdc` name to the rule in
  `prompts/rules/`, vendored at the pin and placed in the container at `/agents/rules/<name>`,
  outside `/work`; any other bare name, or a relative path, to the project file at its `/work/...`
  path. The paths are joined with `, ` as flow joins them, empty when none resolve; an entry
  escaping its root or naming no file is dropped and named in the trial's line as
  `standards_dropped`.
- Changing the pin does not force a re-baseline: `compare` still compares a baseline and a candidate that ran under
  different `prompts/PIN` values, but warns that their prompts may differ (decision
  `compare-warns-on-mixed-prompt-pins`).
- A review run runs one slot's prompt, `--role review --slot primary|principles|failure-modes`
  (default `primary`); the panel's slots are separate roles in flow, so a per-slot model choice is
  evaluated by running each slot. Mutation is not vendored (decision `review-three-slots`).

## 5. Grading

- **impl.** After the agent exits, the grader:
  1. applies `heldout.patch`;
  2. runs the case's `verify` commands (`case.json`: lint, `--tests` selectors for the held-out
     tests, then the module suite), then `./gradlew <task> --tests '<Class>'` for the files
     declaring `@Test` the agent added or changed in each module whose suite no `verify` command
     runs (a command ending `./gradlew <task>`, an environment prefix allowed).
  - The trial passes when every command exits 0.
  - Recorded beside the result:
    - `diff_lines` vs `reference.stat` (over-building ratio);
    - files touched outside the task's declared `**Files:**` (`out_of_scope_files`);
    - tests the agent added;
    - pre-existing tests broken (failing tests whose source file exists on the base).
- **review.**
  1. The agent's report is parsed for `#### Critical|Important|Minor` sections and each issue's
     `file:line`.
  2. Each label is paired with the candidate findings in the same file within ±15 lines.
  3. A pinned judge (`judge.json`: harness `claude-code`, model `sonnet`, effort `medium`) answers
     same-defect yes/no for each pair.
  - Recall is over the trial's slot's `mustFind`. Precision is over the agent's Critical and
    Important findings.
  - A finding is a true positive when matched to the trial's slot's `mustFind`. It counts as unlabelled-not-wrong when
    it matches neither list and the judge rates it plausible. It is a false positive when matched to
    `knownFalse` or the judge rates it implausible.
  - Also recorded: severity agreement for matched findings, and a duplicate rate (two findings
    matched to one label).
  - The judge's own tokens and cost are recorded separately and never charged to the candidate.
- **visual.** The verifier's report states `verdict: match|mismatch` and lists defects.
  - Accuracy is over all cases.
  - False-pass is `match` on a `mismatch` case, the costliest error.
  - Localization is the share of labelled defects the judge matches to a reported defect.

## 6. Metrics

Every trial record carries:

- **Cost and time:**
  - `wall_ms`;
  - tokens: `input`, `output`, `cache_read`, `cache_write` (5-minute cache), `cache_write_1h`
    (input written to a 1-hour cache, priced apart), `reasoning`;
  - `cost_usd` from `pricing.json` per model and token class, and `reported_cost_usd`, the
    harness's own cost figure, kept beside it and omitted when the harness reports none. A model
    with a `long` tier prices each call whose prompt exceeds `over_prompt_tokens` at that tier, so
    it needs per-call usage: a harness reporting tokens but no calls for it (codex) is a pricing
    error, never a cost at the base tier. A pricing error on the trial's own model — the model
    missing from `pricing.json`, a missing per-call usage, a 1-hour cache write with no
    `cache_write_1h` price — is an `infra_error`; on the judge's model it is a scored fail
    (decision `after-output-failures-scored`);
  - `tool_calls` (by tool name) and `tool_calls_total`.
- **Reliability:**
  - `served_model` and `served_matches_requested`;
  - `api_errors`, `timed_out`, `egress_denied`, `infra_error`;
  - across trials: pass^k (passed all k) and score standard deviation.
- **Efficiency:**
  - `turns`, `peak_context_tokens`, `compactions`;
  - `first_edit_tool_call`: the 1-based position of the first file-editing tool call, 0 for none.
    It stands in for a time to first edit: not every harness timestamps its transcript events;
  - `tool_errors`, `permission_denials`;
  - `cache_hit_ratio` (`cache_read / (input + cache_read)`);
  - `cost_per_pass` per case.
- **Work quality** (impl): `diff_lines`, `overbuild_ratio`, `out_of_scope_files`, `tests_added`,
  `preexisting_broken`. There is no lint count: a case's `verify` commands do not mark which one
  is lint, so a recorded `lint_failures` could only ever be an unmeasured 0.
- **Role-specific:**
  - review: `recall`, `precision`, `f1`, `severity_agreement`, `duplicates`, `known_false_hits`;
  - visual: `label` (the case's labelled verdict, which the role score balances over), `correct`,
    `false_pass`, `localization`.
- **Outcome:**
  - `score` (0–100, §7) and `passed`;
  - `error`: with `infra_error`, why the trial was not scored; otherwise why the scored trial failed
    (an impl grade's reason, or a failure after the agent produced output — decision
    `after-output-failures-scored`); omitted when there is nothing to say;
  - `judge`: on a review or visual trial, the judge's own usage and cost, the same fields as above,
    never part of the trial's cost;
  - `standards_dropped`: on a principles trial, each `## standards` entry of the snapshot that
    resolved to no file inside its root, which flow reports and drops (§4); omitted when none is.

An `infra_error` trial (container, proxy or API outage before the agent produced output, or a
failure of the harness's own after it — decision `after-output-failures-scored`) is excluded from
scoring and reported. `compare` refuses (exit 2) when any case has fewer than `k` valid
trials on either side.

## 7. Comparison and the pass rule

Per role, a case score is the mean over its k trials:

- impl: pass 0/100;
- review: F1 × 100;
- visual: correct 0/100.

The role score is the mean over cases. A candidate is **non-inferior** when all of these hold:

1. role score ≥ baseline role score − `margin` (default 5 points, `--margin`);
2. no case flips from pass^k on the baseline to 0 passes on the candidate;
3. visual only: total `false_pass` does not exceed the baseline's.

- **Improvement:** role score ≥ baseline + margin, or a case flips from 0 passes to pass^k.
- `compare` writes `runs/<cand>-vs-<base>.md` with:
  - both runs' flow-eval shas, flagged when they differ (decision `compare-prints-both-shas`);
  - a warning when the two runs' judges differ, or a review or visual pair records none (decision
    `compare-warns-on-mixed-judges`);
  - the per-case table (score, cost, wall clock, tokens, delta);
  - flips;
  - a cost-per-pass delta;
  - a verdict line.
- Exit codes:
  - 0: non-inferior with no improvement flagged;
  - 1: any regression (wins over 3);
  - 3: non-inferior with an improvement to review;
  - 2: usage error, a role or slot mismatch, a case-set mismatch, a k mismatch, or missing trials.
    A prompt-pin mismatch is a warning, never an exit 2 (decision `compare-warns-on-mixed-prompt-pins`).
- Baselines: `baselines/<role>.json` names a run id. `flow-eval baseline set <run-id>` writes it.

## 8. Results storage

- A run id is `<started>-<role>-<harness>-<model>-<effort>`, `<started>` the UTC start time as
  `20060102T150405Z`.
- `runs/<run-id>.jsonl` holds one header line (`role`, `harness`, `model`, `effort`, `k`, prompt
  pin, case-set hash, flow-eval git sha, started, and on a review or visual run the `judge` pin it was
  graded with) and one line per trial.
- `runs/<run-id>.md` is the human report.
- `runs/<run-id>.html` is the run's self-contained page, and `runs/index.html` lists every run.
  The page carries each trial's `output.md`, `verify.log` and `work.patch` inline, each cut at
  256 KiB with a line naming the whole file.
- Row colours, on the pages and the index, count valid trials only: green when every one passed,
  yellow when some did, red when none did, none without a valid trial. A row is blue instead when
  the role has a baseline run, this run is not it, the two are comparable (§7's exit-2 conditions
  do not hold), and the row beats it: a case row whose score is above the baseline's same case
  (with a valid trial there), or a run row whose role score is above the baseline's. The baseline's
  own rows are never blue.
- `runs/<run-id>/<case>/<trial>/` keeps the raw transcript and the agent's output. This directory is
  gitignored; the JSONL and Markdown are committed.

## Live check

Run one real trial per role through the actual containers and harness:

```bash
flow-eval run --role visual --harness claude-code --model sonnet --effort low -k 1 --cases <one-mismatch-case>
flow-eval run --role review --harness codex --model <codex-model> --effort low -k 1 --cases <one-case>
flow-eval run --role impl --harness claude-code --model sonnet --effort low -k 1 --cases <one-needsDocker-case>
```

Record for each run: `wall_ms`, the token columns, `cost_usd`, `tool_calls_total`, `served_model`
and the score.

Not working looks like this:

- zero tokens, or `served_model` empty on a claude-code run (`codex exec` never names its served
  model, so a codex run records it empty and `served_matches_requested` null);
- `cost_usd` 0 on a run that made API calls, other than a `timed_out` trial, whose cut-off
  transcript carries no usage;
- an impl trial that passes before `heldout.patch` is applied, which would mean the answer leaked;
- `egress_denied` 0 after a deliberate `git ls-remote https://example.com/x.git` from inside a
  trial container (the runner image carries no `curl`),
  which would mean the proxy is not in the path.

## Step-by-step breakdown

### Go module and CLI entrypoint

**What:** `go.mod` (Go 1.27, `toolchain go1.27.2`) and `cmd/flow-eval/main.go` routing the
subcommands with stdlib `flag`.
**Why:** Matches agents/stats, giving one static binary to copy into the runner image.
**Uses:** Go stdlib.

### Case schema and validation

**What:** `internal/cases` loads `cases/<role>/<id>/case.json`, checks required fields and assets per
role, and computes the case-set hash. `flow-eval case validate` is the command.
**Why:** Every other component reads cases, and a malformed case must fail before any container
starts.
**Uses:** `encoding/json`, `crypto/sha256`.

### Isolated trial sandbox

**What:** `internal/sandbox` creates the per-trial internal network, proxy, optional dind sidecar
and trial container through the `docker` CLI. It streams the snapshot and inputs in, runs commands,
copies outputs out and tears everything down. `docker/runner.Dockerfile` builds the runner image.
`cmd/egress-proxy` is the allowlist CONNECT proxy.
**Why:** The isolation requirement in §2.
**Uses:** `docker` CLI, `git archive`, `docker/egress-allowlist.txt`.

### Harness adapters

**What:** `internal/harness` with a `Harness` interface: `Command(prompt, model, effort)` returns
argv and env; `Parse(transcript)` returns a normalized `Usage` with signals. There are three
implementations:
- `claude-code`: `claude -p --output-format stream-json --verbose --model --effort
  --dangerously-skip-permissions`;
- `codex`: `codex exec --json -m -c model_reasoning_effort= --dangerously-bypass-approvals-and-sandbox`;
- `muse-code`: `muse exec --json --model --reasoning-effort --approval-mode never --approval-judge off
  --disable-sandbox`, its binary and login from the `flow-eval-muse` volume (§2, Credentials).
**Why:** Harness-agnostic by construction, with real implementations. The container is the
sandbox, so the CLI's own sandbox is bypassed.
**Uses:** `claude` and `codex` CLIs in the runner image; the Muse Code binary in its volume.

### Metrics and pricing

**What:** `internal/metrics` derives the §6 fields from a normalized transcript plus the
grader's output. `pricing.json` holds the per-model, per-token-class prices.
**Why:** One place computes cost and signals the same way for every harness.
**Uses:** `internal/harness`.

### Prompt vendoring and rendering

**What:** `internal/prompts` provides `prompts sync`, the strip list and the `text/template` render.
`prompts/PIN` records the sha.
**Why:** Evaluate the prompts flow really runs (§4).
**Uses:** `git -C <agents> show <sha>:<path>`.

### Graders

**What:** `internal/grade`, with `impl.go`, `review.go` and `visual.go` per §5. `judge.json`
configures the judge.
**Why:** Turns agent output into scores without the operator.
**Uses:** `internal/sandbox`, `internal/harness` (judge call).

### Run orchestration

**What:** `internal/run` and `flow-eval run`. These loop cases × k with `--parallel` (default 2),
write the JSONL and Markdown, and keep raw artifacts under the gitignored per-run directory.
**Why:** The single entry point of the loop in §1.
**Uses:** cases, sandbox, harness, prompts, grade, metrics.

### Compare, baseline and report

**What:** `internal/compare`, `flow-eval compare` and `flow-eval baseline set`, implementing §7
with its exit codes.
**Why:** The automatic safe or not-safe decision.
**Uses:** `runs/*.jsonl`, `baselines/*.json`.

### Case miners and the how-to

**What:** `internal/mine` with `flow-eval case add impl|review|visual` per §3, plus
`docs/cases.md`.
**Why:** Adding a real case must be one command plus a label edit.
**Uses:** `git` on the gymie checkouts, `ledger.md`, `panel.md`, `tasks.md`, the fidelity
`.mockups` sidecars.

### Seed case set

**What:** one case per role under `cases/` (§3), mined with `case add`, labels curated.
**Why:** The v1 set the pass rule runs on.
**Uses:** `case add`, `case validate`.

### README

**What:** `README.md`, holding the operator loop, the commands, credentials, and how to read a
report.
**Why:** The tool is run occasionally, so its use must be re-learnable from the repo.
**Uses:** none.

## Decisions

### Harnesses driven in v1

**ID:** harnesses-v1
**Status:** active
**Chosen:** Claude Code and Codex adapters behind one interface. Two real implementations prove the
abstraction. Muse Code was added as a third adapter at the operator's request (2026-10-10). It runs
as the CLI ships, with two exceptions: approval prompts are off, and its own shell sandbox is off
because the container is the sandbox. Its reminder agents and web tools stay at the CLI's defaults
and are charged to the candidate.
**Considered:**
- Claude Code + Codex + ZCode: ZCode needs its own rollout parser and is not a planned switch target
  now.
- Claude Code only: does not test the agnostic claim.

### Reviewer case source

**ID:** review-cases-prefix-diffs
**Status:** active
**Chosen:** Round-0 pre-fix diffs from `ledger.md` and `panel.md`. Must-find labels are fixed
findings, and known false positives are findings withdrawn for a false premise. Fully real.
**Considered:** Also reverting later `fix:` commits into landed diffs: more cases, but the defect
sits out of its original context.

### Visual-verify case form

**ID:** visual-static-pairs
**Status:** active
**Chosen:** Static mockup and capture pairs with labelled verdicts. No app stack runs, the inputs are
deterministic, and Linux containers are fine.
**Considered:** A live tier booting gymie and the frontend with Playwright: it measures tooling
skill, but costs about 10× and is flaky.

### Result storage

**ID:** results-in-repo-files
**Status:** active
**Chosen:** `runs/<id>.jsonl` and a Markdown report committed in the repo, with raw transcripts
gitignored.
**Considered:** New tables in the flow store (Postgres): couples evals to flowd being up.

### Which prompts the roles run under

**ID:** real-flow-prompts-pinned
**Status:** active
**Chosen:** The real flow prompts, vendored from a pinned agents sha with Claude-only scaffolding
stripped. A pass then proves the model under flow's real prompts.
**Considered:** Neutral prompts in this repo: stable, but disconnected from what flow runs.

### Automatic pass rule

**ID:** non-inferiority-vs-baseline
**Status:** active
**Chosen:** k=3. The role score must be within a 5-point margin of the stored baseline, with no
always-pass to always-fail case flip, and for visual, no increase in false-passes. Gains beyond the
margin are flagged for review.
**Considered:** Fixed absolute thresholds per role: need hand-tuning and ignore the current baseline.

### Matching reviewer findings to labels

**ID:** review-match-location-plus-judge
**Status:** active
**Chosen:** Pair by file and a ±15-line window, then a pinned judge (claude-code, opus, medium;
sonnet, medium since Task 29) decides same-defect. The judge's cost is reported separately.
**Considered:** Location only: misses a correct finding anchored elsewhere, and credits a wrong one
at the right line.

### Metrics beyond the core set

**ID:** metrics-all-four-groups
**Status:** active
**Chosen:** Reliability, efficiency, work-quality and role-specific groups, all as listed in §6.
**Considered:** Core metrics only (time, tokens, cost, tool calls): cannot tell a silent model
fallback, over-building, or a dangerous false-pass from a plain score change.

### Implementation language

**ID:** go-runner
**Status:** active
**Chosen:** Go: the same language as agents/stats, a single static binary in the runner image, and
the stdlib covers the CLI, JSON and the proxy.
**Considered:** Python: quicker graders, but parsers are rewritten from scratch and a runtime must be
added to the image.

### v1 case-set size

**ID:** case-set-small-v1
**Status:** superseded by `case-set-one-per-role`
**Chosen:** 6 impl, 6 review, 13 visual cases. With k=3, one full role run stays near an hour.
A review slot's run covers only the review cases with a must-find for that slot:
kan-755-offline-l4-be-create-endpoints-accept-a-client has must-finds from `principles` only.
**Considered:** 12 / 20 / 40: tighter confidence, at about 2× the time and cost per evaluation.

### Implementer cases needing a database

**ID:** impl-db-cases-isolated-docker
**Status:** active
**Chosen:** Include DB-touching tasks. Each such trial gets its own `docker:dind` sidecar, because
gymie's tests start Postgres through Testcontainers (`testcontainers` is in
`src/app/build.gradle.kts`). It is the per-trial isolated service the operator chose ("compose
sidecar"), realised by the mechanism gymie's tests actually use.
**Considered:**
- A compose Postgres sidecar: gymie's tests would still ask Testcontainers for Docker.
- Mounting the host Docker socket: breaks isolation.
- Unit-test-only tasks: unrepresentative.

### Isolation boundary

**ID:** isolation-container-per-trial
**Status:** active
**Chosen:** A fresh container per trial on an internal network. Egress goes only through an
allowlist CONNECT proxy written in Go. The snapshot has no git history. Credentials come by
environment only, and no host config, socket or checkout is mounted. Muse Code's login is the one
exception. The CLI reads its account login from a file, so the login arrives through the named
volume `flow-eval-muse`, mounted read-only and filled by the operator's one-time setup inside the
runner image. Each trial copies the login into its own config home.
**Considered:**
- Running in host git worktrees: the agent can read host config and history, including the answer
  commit.
- A third-party proxy image (squid or tinyproxy): a new dependency for ~80 lines of stdlib.

### How cases are added and labels approved

**ID:** case-add-miners-commit-review
**Status:** active
**Chosen:** `flow-eval case add <role>` mines a draft case from gymie history and the operator edits
`labels.json` or `case.json`. The git diff review of the commit is the label approval.
`docs/cases.md` is the how-to.
**Considered:** A `reviewed: true` flag in each case: duplicates what the commit review already
records.

### Reviewer slot evaluated in v1

**ID:** review-primary-slot-v1
**Status:** superseded by `review-three-slots`
**Chosen:** The primary reviewer prompt only, behind a `--slot` flag defaulting to `primary`. It is
the slot every roster carries.
**Considered:** All four slots now: three more prompt vendorings, and the labels are not
slot-specific. (Task 27 has since given every label its raising slot; the vendoring cost
still stands.)

### Gradle cache and the wrapper download

**ID:** gradle-cache-warmed-host-side
**Status:** active
**Chosen:** Before a repository's first trial, `flow-eval` warms its `flow-eval-gradle-<repo>` volume
in a setup container that has ordinary egress but runs no agent and receives no credential: the
snapshot's `./gradlew` downloads the wrapper distribution (served from `github.com` via
`services.gradle.org`) into the volume, and a marker file skips the step for later trials.
**Considered:**
- Allowlisting `github.com` and `release-assets.githubusercontent.com`: an agent could fetch any
  public repository, an answer-leak path.
- Baking each case's Gradle distribution into the runner image: the image would track every source
  repository's wrapper version.

### The visual verifier's measurement tool

**ID:** visual-measure-tool-in-runner
**Status:** active
**Chosen:** The runner image carries flow's `measure-visual-properties.sh` built from the agents repo
at `prompts/PIN`, so the vendored verifier procedure, which makes that script the only admissible
measurement, runs as written. Only the steps a static pair cannot perform (launching the stack,
capturing live states) are stripped. Sweep 6 stays — its element-order and ink-bounds checks need
only the two images — less its clause driving a seeded value, cut by a `~` clause entry in
`prompts/strip.txt` (operator, 2026-10-09).
**Considered:** Stripping the measurement paragraphs: cheaper, but the visual role would then run a
reduced procedure that real flow never runs.

### What a visual case's mockup is

**ID:** visual-mockup-frame-crop
**Status:** active
**Chosen:** `case add visual` cuts the mockup board down to its frame with flow's own
`compose-mockup-frames.sh` and the source repository's `mockup frame` geometry, so `mockup.png` is
the crop flow's visual verification measures, the same size as the capture, measured at
`--scale 1` (operator, 2026-10-09). A board with no declared geometry is copied as it is.
**Considered:** Committing the whole board and having the prompt tell the agent to crop first: a
step the agent can get wrong, whose error would count against the prompt under test. Measured on
gymie H2.png (780x2490 board against a 776x1440 capture): band pairing paired 0 bands uncropped.

### Where a visual case's labels come from

**ID:** visual-labels-from-flow-verdicts
**Status:** active
**Chosen:** A visual case's verdict and defects are those flow's own visual verification recorded for
the pair in an archived `visual-verification.md`; a region only where the record gives one. A
difference the record did not count (pinned seed data, font-width wraps) is not a departure
(operator, 2026-10-09).
**Considered:** Fresh labels counting every visible difference: most pairs become mismatch on seed
data alone. Fresh labels by drawn intent: the curator's judgement, not flow's, becomes the truth.

### Keeping a case's snapshot reachable

**ID:** snapshot-refs-pushed
**Status:** active
**Chosen:** Each impl or review case's snapshot gets `refs/flow-eval/<case-id>` in its source
checkout, pushed to that checkout's origin, so `git gc` cannot prune it and any clone can fetch it
(operator, 2026-10-09). The operator creates and pushes the refs; the miner stays read-only.
**Considered:** Vendoring each snapshot into the case: 100–220 MB per gymie snapshot. Local refs only:
another machine cannot run the cases.

### What counts as a known-false finding

**ID:** known-false-explicit-only
**Status:** active
**Chosen:** `knownFalse` holds only a withdrawal whose reason says the premise was false; every
other withdrawal goes to `ignored[]` (operator, 2026-10-09).
**Considered:** Every withdrawal not naming a decision or supersession as `knownFalse`: on real
gymie panels it labels duplicates, operator withdrawals and fixed-elsewhere findings as false
positives, penalising a reviewer for raising real defects.

### Privileged Docker-in-Docker for needsDocker trials

**ID:** dind-privileged-accepted-v1
**Status:** active
**Chosen:** Keep the `--privileged` `docker:dind` sidecar for v1 and record the risk in the README:
an agent can reach the sidecar's daemon and start privileged containers, so a deliberate
kernel-level escape reaches the Docker host (the Linux VM on Docker Desktop, the host itself on
Linux). Trials run the operator's own agents on the operator's machine.
**Considered:** The sysbox runtime or rootless dind: closes the escape, but needs host setup outside
this repository.

### What a review case carries

**ID:** review-cases-diff-only
**Status:** active
**Chosen:** A review case carries the diff and labels only — its `diff.patch` leaves out the change's
own plan files (operator, 2026-10-09). The reviewer prompt therefore runs without
flow's plan-alignment check, and `case add review` leaves out any must-find finding that is a
plan-alignment finding. A mustFind label carries an optional `severity` the miner fills from the
panel's finding row, for `severity_agreement`.
**Considered:** Shipping the change's proposal and design with each review case: truer to flow, but
more case data for a check v1 does not measure.

### What the Gradle volume shares

**ID:** gradle-volume-read-only
**Status:** active
**Supersedes:** the sharing premise of `gradle-cache-warmed-host-side` and §2 Caches ("holds
`~/.gradle` … across trials"), 2026-10-09, from the round-1 panel.
**Chosen:** The volume, renamed `flow-eval-gradle-ro-<repo>` so no volume filled under the shared
home is ever mounted, holds only the wrapper distribution and the
dependency cache (`caches/modules-2`), written by the agent-free warm-up alone: it builds the
snapshot (`./gradlew --version`, then `./gradlew testClasses`, best effort) in its own Gradle home
and copies those two into the volume, less Gradle's lock and `gc.properties` files. A trial mounts
the volume read-only at `/gradle-ro` with `GRADLE_RO_DEP_CACHE=/gradle-ro/caches`; its Gradle user
home is the container's own `/root/.gradle`, seeded with a copy of the wrapper (the wrapper locks
the distribution it runs, which a read-only mount refuses). A visual trial, which never builds, gets
no volume. The warm-up container's name carries a random suffix, so two runs warming one repository
never remove each other's.
**Why:** With the whole Gradle home shared and writable, gymie's `org.gradle.caching=true` put a
review trial's compiled snapshot — which holds impl cases' reference commits — and the held-out
tests the grader compiled into the build cache and daemon logs every later trial reads, and any
trial could plant an init script shaping every later build and grade. Dependencies are inputs, not
answers; build outputs are neither shared nor kept.
**Considered:** Keeping the shared home and passing `--no-build-cache` to grading: leaves the daemon
logs, `init.d` and `gradle.properties` writable and shared, and the agent's own builds still fill
the cache.

### A failure after the agent produced output is scored

**ID:** after-output-failures-scored
**Status:** active
**Supersedes:** Task 11's "a grader failure is `infra_error`", 2026-10-09, operator decision on the
round-1 panel.
**Chosen:** `infra_error` is a container, proxy or API failure before the agent produced output
(§6), and, after it, a failure of the harness's own that the agent cannot cause: the trial's own
model missing from `pricing.json` (a misconfiguration), the proxy's egress denials failing to count,
an artifact (`output.md`, `work.patch`, `verify.log`) failing to write. Any other failure after the
output — the transcript and grading: a work tree the agent left unusable, a verify command or judge
call cut off at grading's deadline, a judge whose usage cannot be priced, a transcript that does not
parse — is a scored fail, `passed: false`, score 0, its reason in the line's `error`. A run
cancelled mid-trial stays `infra_error`. Boundary set 2026-10-09 by the dispatcher on the round-1
fix: the operator's decision covered grading failures after the output, not the harness's own. Grading runs under its own deadline, the role's agent timeout again. An impl trial's
line carries its grade reason (`heldout-conflict` or the first failing verify command), and its
trial directory keeps the agent's diff (`work.patch`) and the verify output (`verify.log`).
**Why:** An infra error is excluded from every aggregate, so a candidate whose worst trials broke
grading would drop them from its score — the bias the timed-out-is-scored correction removed for
timeouts — and a hung held-out test held the run open with no bound.
**Considered:** Splitting grading failures into the agent's and the harness's: the line between
them is the agent's tree, which every grading step reads, so no split is reliable.

### The visual role score

**ID:** visual-score-balanced-accuracy
**Status:** active
**Supersedes:** §5 "Accuracy is over all cases" and, for visual, §7 "The role score is the mean over
cases", 2026-10-09, operator decision on the round-1 panel.
**Chosen:** The visual role score is balanced accuracy: the mean of the accuracy on `match` cases and
the accuracy on `mismatch` cases, each the mean of its cases' scores, over the verdicts the set
holds. A verifier that always answers `mismatch` scores 50 when both verdicts are present (one-verdict
sets: §3). Each visual trial records its case's labelled verdict as `visual.label`, so
`compare` balances from the run record alone. impl and review keep the mean over cases.
**Why:** The shipped set is 1 `match` and 12 `mismatch` cases; under plain accuracy a verifier that
never passes a capture scores 92 with no false pass, and only the flip rule on the one `match` case
could catch it.
**Considered:** Mining `match` pairs until the set is balanced: the operator chose the score change;
more pairs can still be added under it.

### Runs of different flow-eval shas

**ID:** compare-prints-both-shas
**Status:** active
**Chosen:** `compare` prints both runs' flow-eval shas — on stdout after the verdict line, and in the
report — and flags them `DIFFER` when they are not equal. It never refuses on a difference; the
exit code is the pass rule's alone.
**Why:** A grader change between baseline and candidate (`GradeImpl`, `ParseReview`) moves scores
with no candidate change; the operator reads the flag and decides whether to re-run the baseline.
**Considered:** Refusing (exit 2) on any sha difference, or on a change under the grader packages:
the operator decided against a refusal on the round-1 panel — most commits between two runs touch no
grading, and a refusal would force a baseline re-run for each.

### Runs graded by different judges

**ID:** compare-warns-on-mixed-judges
**Status:** active
**Chosen:** `compare` warns — on stderr and in the report — when the two runs' recorded judges differ
(`DIFFER`), or when a review or visual pair records none (`UNRECORDED`, every run from before the
header field). It never refuses; the exit code is the pass rule's alone.
**Why:** A judge change (Task 29 moved it from opus to sonnet) moves review and visual scores with no
candidate change; the operator reads the warning and decides whether to re-run the baseline.
**Considered:** Refusing to compare runs graded by different judges: the operator chose a warning
("just show a warning somewhere it is enough", 2026-10-10).

### Runs under different prompt pins

**ID:** compare-warns-on-mixed-prompt-pins
**Status:** active
**Supersedes:** §4 "`compare` refuses with exit 2 when the baseline and the candidate ran under
different `prompts/PIN` values", 2026-10-10, operator decision.
**Chosen:** `compare` compares runs whose prompt pins differ and prints `warning: prompt pin <cand> vs
<base> — DIFFER: prompts may have changed between the runs` on stderr and in its report. The verdict
and exit code are the pass rule's alone, and the index's blue paint follows `compare`
(`run.Comparable`), so a differing pin no longer blocks it. Different roles, slots, case sets or k
still refuse with exit 2.
**Why:** A pin move often leaves a role's prompt unchanged (Task 31's sync moved the pin without
changing `impl`, `review-primary` or `visual`), and a refusal would force a re-baseline of every role
for each; the operator reads the warning and decides whether to re-run the baseline.
**Considered:** Refusing (exit 2) on a pin mismatch, the original §4 rule: the operator chose a
warning ("Allow compare for different pin but show a warning", 2026-10-10).

### Review slots evaluated

**ID:** review-three-slots
**Status:** active
**Supersedes:** `review-primary-slot-v1`, 2026-10-10, operator request.
**Chosen:** `prompts sync` vendors the primary, principles and failure-modes reviewer prompts, each cut
the same way: the fenced prompt, its sections and clauses that need flow's diff file, change
artifacts or context bundle stripped per `prompts/strip.txt`, and the diff appended inline. Each prompt's `## Scope` loses its path lines and its read-order sentences
(`~` clause cuts) but keeps its order to grep the touched files and confirm a suspected issue before
reporting it, since a review trial's `/work` is the repository at the change's head. The principles
slot gets `skills/flow/engineering-principles.md` appended whole, as primary gets
`reviewer-calibration.md`. Its `**Project standards:**` line stays, filled per trial from the case
snapshot's `## standards` as flow fills it (§4), so the standards-as-data rule, `## Hard Invariants`,
its Critical calibration clause and the no-standards paragraph stay as upstream has them; only
`[GLOBAL_CONSTRAINTS]` goes, since a review case carries no change artifacts. All three share the
`#### Critical/Important/Minor` output contract `ParseReview` reads, so Task 27's per-slot grading
scores each unchanged.
**Why:** The operator asked to evaluate principles and failure-modes on a cheaper model (sonnet, low).
The labels carry each must-find's slot: principles has must-finds in 4 of the 6 review cases (not
kan-527 or kan-961, which a `--slot principles` run refuses), failure-modes in 1 (kan-867).
**Considered:** The mutation slot too: it writes sabotage tests in a throwaway worktree rather than a
findings list, so neither a review case nor `ParseReview` fits it. Cutting the project standards and
applying the universal principle list alone (Task 31): flow passes the principles slot its project's
standards, so a trial without them measured a different reviewer than flow runs (Task 32, operator
request "add it").

### v1 case set: one case per role

**ID:** case-set-one-per-role
**Status:** active
**Supersedes:** `case-set-small-v1`, 2026-10-10, operator request (Task 33).
**Chosen:** One case per role, the one each recorded run used (§3), whose review case covers every
vendored slot; `run --slot` refuses a review case with no must-find for that slot rather than
skipping it.
**Why:** The operator asked to keep only the case each run used ("Leave only one for each that was
used").
**Considered:** Keeping the 6 / 6 / 13 set of `case-set-small-v1`: the operator chose the cut.

## Open questions
