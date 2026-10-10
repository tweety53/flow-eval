> **Execution:** `/flow` implements this plan. Mark a task's own checkbox when
> `check-task-commit-fields.sh` passes on that task's commit.
> **Relocation:** no
> **Tasks appended:** 16

# kan-964-flow-eval-harness

KAN-964: a Go CLI that runs flow's implementer, reviewer and visual-verify roles on real gymie cases
in isolated containers, under Claude Code or Codex, records time, tokens, cost, tool calls and
quality, and decides non-inferiority against a baseline. `design.md` beside this file is canonical
for each decision a task cites (by `**Decision:**` ID) and for the section numbers (§) cited below;
`proposal.md` for what ships.

**Repository:** single — the worktree `/Users/tweety53/Projects/flow-eval-worktrees/kan-964-flow-eval-harness`; every path below is relative to it.

## Global constraints

- **Go stdlib only.** No third-party Go module unless a task names it. A new dependency or base
  image goes through the latest-stable-version lookup (`~/.claude/rules/dependency-versions.md`)
  and names the version in its commit.
- **Go version:** `go 1.27` with `toolchain go1.27.2` in `go.mod`; the local `go1.26.5` fetches the
  toolchain itself (`GOTOOLCHAIN=auto`). Latest stable read from `https://go.dev/VERSION?m=text`
  on 2026-10-09.
- **Verify** (worktree root): `gofmt -w .`, `test -z "$(gofmt -l .)"`, `go vet ./...`, then
  `go test ./<pkg> -race -count=1 -run '<the task's tests>'`.
- **No test touches the network, a real model or the host Docker daemon** except where a task says
  so; the sandbox, harness and judge are behind interfaces faked in unit tests. The real path is
  exercised by design.md's `## Live check`.
- **Never read or mount** host `~/.claude`, `~/.codex`, `/var/run/docker.sock` or a gymie checkout
  from inside a trial container (decision `isolation-container-per-trial`). The host-side miners
  (Task 13) read gymie checkouts read-only and never write to them.
- **No new inline lint suppressions** and no weakening of lint config.

## Review focus

- Answer leakage: the trial snapshot must hold one commit and no reference-commit object, and
  `heldout.patch` must reach the container only after the agent process exits. Pinned in Tasks 4
  and 11.
- Isolation: the trial network is `--internal`, no host path is bind-mounted, the dind sidecar is
  created only for `needsDocker` cases. Pinned in Task 4.
- The pass rule's exit codes and precedence (1 beats 3; 2 on role, slot, case-set, k or trial-count mismatch;
  a prompt-pin mismatch warns, Task 31).
  Pinned in Task 12.
- Judge cost never charged to the candidate. Pinned in Task 9.

---

- [x] 1. Go module and CLI entrypoint

**Build:** green
**Files:** `go.mod`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `.gitignore`
**Tests:** `TestUnknownCommandExits2`, `TestHelpListsCommands`
**Regression:** `TestUnknownCommandExits2` — revert → an unknown subcommand no longer exits 2 with usage; `TestHelpListsCommands` — revert → `--help` no longer lists run, compare, baseline, case, prompts.
**Baseline:** before=0 after=2
**Commit:** `feat(cli): add flow-eval module and subcommand router`
**After:** none

**Decision:** go-runner

  - [x] **Step 1: Write the failing tests.** `main_test.go` builds the binary with `go build` into
    `t.TempDir()` and asserts: `flow-eval nope` exits 2 and prints usage on stderr; `flow-eval --help`
    exits 0 and names `run`, `compare`, `baseline`, `case`, `prompts`.
  - [x] **Step 2: Implement.** `go.mod` (`module github.com/tweety53/flow-eval`, `go 1.27`,
    `toolchain go1.27.2`). `main.go` routes `os.Args[1]` to a `map[string]func([]string) int`;
    commands not yet built print `not implemented` and exit 2. Each later task replaces its entry.
    Add `runs/*/` to `.gitignore` (raw per-trial artifacts, decision `results-in-repo-files`).
    Correction (2026-10-09): the router shipped as an ordered `[]command` slice rather than a map — one list the router and the usage text both read, in a deterministic usage order.
  - [x] **Step 3: Verify** per Global constraints with `go test ./cmd/flow-eval -race -count=1 -run 'TestUnknownCommandExits2|TestHelpListsCommands'`.

- [x] 2. Case schema, loader and `case validate`

**Build:** green
**Files:** `internal/cases/case.go`, `internal/cases/case_test.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`
**Allowed-collateral:** `internal/cases/testdata/**`
**Tests:** `TestLoadValidCases`, `TestRejectMissingAsset`, `TestRejectUnknownRole`, `TestCaseSetHashStable`, `TestRejectInvalidFields`, `TestCaseValidateExitCodes`
**Regression:** `TestLoadValidCases` — revert → valid impl/review/visual fixtures no longer load; `TestRejectMissingAsset` — revert → a review case without `diff.patch` passes validation; `TestRejectUnknownRole` — revert → `role: "foo"` passes; `TestCaseSetHashStable` — revert → the hash changes with directory read order.
**Baseline:** before=2 after=8
**Commit:** `feat(cases): load and validate eval cases`
**After:** Task 1

**Decision:** case-add-miners-commit-review

  - [x] **Step 1: Write the failing tests** over fixtures in `internal/cases/testdata/` — one valid
    case per role, one review case missing `diff.patch`, one with an unknown role.
  - [x] **Step 2: Implement** `Case` (`id`, `role`, `source{repo,change,url}`, `notes`,
    `needsDocker`, `verify []string` for impl, `files []string` for impl's declared scope) and the
    per-role required assets of design.md §3 (impl: `task.md`, `context.md`, `heldout.patch`,
    `reference.stat`; review: `diff.patch`, `labels.json`; visual: `mockup.png`, `capture.png`,
    `labels.json` with `verdict` and `defects`). `LoadAll(root, role)` returns cases sorted by id;
    `SetHash(cases)` is sha256 over sorted `id` + file contents.
    Correction (2026-10-09), round-1 panel (finding F16): `case.json` and `labels.json` are decoded
    with `encoding/json/v2` (`RejectUnknownMembers` for `case.json`), the codec the miner writes them
    with, so a key in another case (`"ID"`) is no longer matched case-insensitively; the two
    `errors.As` sites (`sandbox.run`, the CLI test helper) use `errors.AsType`. Tests: the
    `key in another case` and `labels key in another case` rows of `TestRejectInvalidFields`, and
    `TestRunReportsANonZeroExitAsAResult`.
  - [x] **Step 3: Wire** `flow-eval case validate [--role R] [--root cases]`: exit 0 all valid, 1
    listing each invalid case and why, 2 usage.
  - [x] **Step 4: Verify** with `-run 'TestLoadValidCases|TestRejectMissingAsset|TestRejectUnknownRole|TestCaseSetHashStable'` on `./internal/cases`.

- [x] 3. Egress allowlist proxy

**Build:** green
**Files:** `cmd/egress-proxy/main.go`, `cmd/egress-proxy/main_test.go`, `docker/egress-allowlist.txt`
**Tests:** `TestProxyAllowsListedHost`, `TestProxyRefusesUnlistedHost`, `TestProxyRefusesPlainHTTP`, `TestShippedAllowlist`, `TestAllowlistRejectsMalformedLine`
**Regression:** `TestProxyAllowsListedHost` — revert → CONNECT to an allowlisted host is not tunnelled; `TestProxyRefusesUnlistedHost` — revert → an unlisted host is tunnelled; `TestProxyRefusesPlainHTTP` — revert → a non-CONNECT request is forwarded.
**Baseline:** before=8 after=13
**Commit:** `feat(proxy): add allowlist CONNECT egress proxy`
**After:** Task 1

**Decision:** isolation-container-per-trial

  - [x] **Step 1: Write the failing tests** with `httptest`: a TLS-less TCP echo server stands in for
    the upstream; the allowlist names `127.0.0.1` for the allow case only.
  - [x] **Step 2: Implement** a `net/http` server: `CONNECT host:port` whose host matches a line of
    the allowlist file (exact host or `*.suffix`) is hijacked and piped both ways; anything else gets
    403 and one stderr line `egress-denied <host>`; non-CONNECT methods get 405.
  - [x] **Step 3: Write `docker/egress-allowlist.txt`:** `api.anthropic.com`, `*.anthropic.com`,
    `api.openai.com`, `*.openai.com`, `chatgpt.com`, `repo.maven.apache.org`, `repo1.maven.org`,
    `plugins.gradle.org`, `services.gradle.org`, `downloads.gradle.org`, `registry.npmjs.org`,
    `registry-1.docker.io`, `auth.docker.io`, `production.cloudflare.docker.com`.
    Correction (2026-10-09): `plugins-artifacts.gradle.org` and `production.cloudfront.docker.com` added — measured: a live gymie Testcontainers trial reached both (Gradle plugin artifacts, Docker Hub blobs); the plan's `production.cloudflare.docker.com` is kept. Folded into this task's one commit.
    unverified:confirm the hosts each harness's auth and API calls actually reach by running one trial per harness with the proxy logging allowed hosts; add any missing provider host, never a wildcard beyond the provider's own domain
  - [x] **Step 4: Verify** with `-run 'TestProxyAllowsListedHost|TestProxyRefusesUnlistedHost|TestProxyRefusesPlainHTTP'` on `./cmd/egress-proxy`.

- [x] 4. Isolated trial sandbox and runner image

**Build:** green
**Files:** `internal/sandbox/sandbox.go`, `internal/sandbox/sandbox_test.go`, `docker/runner.Dockerfile`, `docker/proxy.Dockerfile`
**Tests:** `TestTrialNetworkIsInternal`, `TestNoHostMounts`, `TestDindOnlyWhenNeeded`, `TestSnapshotHasSingleCommit`, `TestStopRemovesEverythingButTheVolume`, `TestStopRunsWithCancelledContext`, `TestStartFailureTearsDown`, `TestStartRefusesUnsetCredentialAndBadNames`, `TestStartWaitsForDind`, `TestStartFailsWhenDindNeverReady`
**Regression:** `TestTrialNetworkIsInternal` — revert → the trial network is created without `--internal`; `TestNoHostMounts` — revert → a `-v`/`--mount` of a host path appears in a container's argv; `TestDindOnlyWhenNeeded` — revert → a dind sidecar starts for a case without `needsDocker`, or none for one with it; `TestSnapshotHasSingleCommit` — revert → the snapshot carries the source history and the reference commit is reachable.
**Baseline:** before=13 after=23
**Commit:** `feat(sandbox): run trials in isolated per-trial containers`
**After:** Task 1, 3

**Decision:** isolation-container-per-trial
**Decision:** impl-db-cases-isolated-docker

  - [x] **Step 1: Write the failing tests.** The sandbox runs `docker` through a `Runner` interface;
    tests record argv with a fake. `TestSnapshotHasSingleCommit` is real git: build a repo with two
    commits in `t.TempDir()`, snapshot the first, assert `git rev-list --count HEAD` is 1 and
    `git cat-file -e <second-sha>` fails.
  - [x] **Step 2: Implement** `Snapshot(repo, sha) (tarPath)` — `git -C repo archive sha` into a
    tar, extracted, `git init` + one commit `base` (so the agent's own diff is `git diff HEAD`).
    `Start(TrialSpec)` creates network `fe-<trial>` with `--internal`, the proxy container (attached
    to the internal network and the default bridge), the optional `docker:dind` sidecar
    (`--privileged`, internal network only, `DOCKER_TLS_CERTDIR=`) and the trial container with
    `HTTPS_PROXY`/`HTTP_PROXY`/`NO_PROXY=dind`, `DOCKER_HOST=tcp://dind:2375` and
    `TESTCONTAINERS_RYUK_DISABLED=true` when dind is present, named volume
    `flow-eval-gradle-<repo>` at `/root/.gradle`, credentials passed as `-e NAME` (value from the
    host env, never on argv). `CopyIn`, `Exec(cmd) (exit, stdout, stderr)`, `CopyOut`, and
    `Stop()` which removes containers, network and never the gradle volume, in a `defer` so it runs
    on every outcome.
  - [x] **Step 3: Write the Dockerfiles.** `runner.Dockerfile`: a JDK 25 base (gymie's
    `JavaLanguageVersion.of(25)`), Node LTS, `npm i -g @anthropic-ai/claude-code @openai/codex` at
    pinned versions, `git`, and the `flow-eval` binary. `proxy.Dockerfile`: the `egress-proxy`
    binary plus the allowlist. Look up each base image and npm version per Global constraints.
    measured: (live gymie trial on `flow-eval-runner:dev`, 2026-10-09) gymie's Gradle build fails with JDK 25 as the Gradle runtime (`:buildSrc:detekt`, detekt 1.23.8: `> 25.0.4`) and passes as root with HOME=/root on a JDK 21 runtime with JDK 25 as the toolchain.
    Correction (2026-10-09): the plan declared "a JDK 25 base"; shipped a Temurin 21 runtime (`JAVA_HOME`, runs Gradle) plus a Temurin 25 toolchain at `/usr/lib/jvm/temurin-25` and `procps` (gymie's buildSrc test calls `ps`), because the JDK 25 runtime breaks gymie's detekt. The sandbox also sets `JAVA_TOOL_OPTIONS` proxy properties (the JVM ignores `HTTPS_PROXY`), writes the snapshot tar with Go `archive/tar` (root-owned, no xattrs — macOS bsdtar xattrs and uid 501 broke `docker cp` and git), and polls dind readiness up to 90s.
    Correction (2026-10-09), round-1 panel: the volume is mounted read-only at `/gradle-ro`, not at
    `/root/.gradle`, and only for a trial with a repository (decision `gradle-volume-read-only`, see
    Task 16); `CopyOut` is removed — the impl grader returns the agent's diff and verify output as
    strings, so nothing copied a file out.
  - [x] **Step 4: Verify** with `-run 'TestTrialNetworkIsInternal|TestNoHostMounts|TestDindOnlyWhenNeeded|TestSnapshotHasSingleCommit'` on `./internal/sandbox`, then `docker build -f docker/runner.Dockerfile .` and `docker build -f docker/proxy.Dockerfile .` succeed.

- [x] 5. Harness adapters for Claude Code and Codex

**Build:** green
**Files:** `internal/harness/harness.go`, `internal/harness/claude.go`, `internal/harness/codex.go`, `internal/harness/harness_test.go`
**Allowed-collateral:** `internal/harness/testdata/**`
**Tests:** `TestClaudeCommandArgs`, `TestClaudeParseUsage`, `TestCodexCommandArgs`, `TestCodexParseUsage`, `TestClaudeParseRejectsTranscriptWithoutResult`, `TestParseRejectsMalformedLine`
**Regression:** `TestClaudeCommandArgs` — revert → model/effort/stream-json flags missing from the claude argv; `TestClaudeParseUsage` — revert → tokens, turns, tool calls or served model are not extracted from a stream-json transcript; `TestCodexCommandArgs` — revert → `model_reasoning_effort` not passed; `TestCodexParseUsage` — revert → `turn.completed` usage is not summed.
**Baseline:** before=23 after=29
**Commit:** `feat(harness): add claude-code and codex headless adapters`
**After:** Task 1

**Decision:** harnesses-v1

  - [ ] **Step 1: Capture real transcripts first.** Run one tiny prompt per harness inside the
    runner image (`echo hi`, one tool call) and save the stdout JSONL to `testdata/claude.jsonl` and
    `testdata/codex.jsonl`. Reproduce before writing the parser; do not guess the shapes.
    Correction (2026-10-09): no live transcript was captured — the host permission classifier refused a headless `claude -p` and no harness credentials are set — so both fixtures are hand-built from the schemas compiled into claude 2.1.295 / codex 0.145.0 (the runner pins 2.1.286 / 0.162.0). Replacing them with real captures from the runner image stays with design.md's Live check. `FirstEditAt` is the 1-based index of the first editing tool call, since neither CLI timestamps events.
  - [x] **Step 2: Write the failing tests** against those files.
  - [x] **Step 3: Implement** `Harness` with `Name()`, `Command(prompt, model, effort, workdir)
    (argv, env)` and `Parse(io.Reader) (Usage, error)`. `Usage`: `Input`, `Output`, `CacheRead`,
    `CacheWrite`, `Reasoning`, `Turns`, `ToolCalls map[string]int`, `ToolErrors`,
    `PermissionDenials`, `APIErrors`, `Compactions`, `PeakContext`, `FirstEditAt`,
    `ServedModels []string`, `ReportedCostUSD *float64`, `FinalText`.
    - claude: `claude -p <prompt> --model M --effort E --output-format stream-json --verbose
      --dangerously-skip-permissions`, env `CLAUDE_CONFIG_DIR=<fresh dir>`. Usage and cost from the
      final `result` event (`usage`, `total_cost_usd`, `num_turns`, `modelUsage` keys as served
      models), tool calls from `assistant` `tool_use` blocks.
      ```text verified:(result schema compiled into claude 2.1.295, 2026-10-09; no live transcript — see Task 5 Correction) also modelUsage.<id>.{inputTokens,outputTokens,cacheReadInputTokens,cacheCreationInputTokens,costUSD}, permission_denials[], system api_retry/compact_boundary; no event carries a timestamp
      result.usage.{input_tokens,output_tokens,cache_read_input_tokens,cache_creation_input_tokens}, result.total_cost_usd, result.num_turns, result.modelUsage
      ```
    - codex: `codex exec --json --skip-git-repo-check -m M -c model_reasoning_effort=E
      --dangerously-bypass-approvals-and-sandbox <prompt>`, env `CODEX_HOME=<fresh dir>`. Usage
      summed over `turn.completed.usage`, tool calls from `item.completed` items of type
      `command_execution`, `file_change`, `mcp_tool_call`, `web_search`.
      ```text verified:(usage fields from a local codex rollout + codex 0.145.0 binary, 2026-10-09; input_tokens includes cached_input_tokens — measured total=input+output; event names seen only as binary strings, no live exec run) usage also carries cache_write_input_tokens, total_tokens
      turn.completed.usage.{input_tokens,cached_input_tokens,output_tokens,reasoning_output_tokens}
      ```
    The container is the sandbox; both CLIs' own permission sandboxes are bypassed inside it.
  - [x] **Step 4: Verify** with `-run 'TestClaudeCommandArgs|TestClaudeParseUsage|TestCodexCommandArgs|TestCodexParseUsage'` on `./internal/harness`.

- [x] 6. Metrics and pricing

**Build:** green
**Files:** `internal/metrics/metrics.go`, `internal/metrics/metrics_test.go`, `pricing.json`
**Tests:** `TestCostFromPricing`, `TestUnknownModelPricingErrors`, `TestCacheHitRatio`, `TestServedModelMismatch`, `TestPassK`, `TestLoadPricingRejectsIncompleteEntry`, `TestRoleScoreBalancesVisualVerdicts`
**Regression:** `TestCostFromPricing` — revert → cost no longer sums per token class; `TestUnknownModelPricingErrors` — revert → a model missing from `pricing.json` silently costs 0; `TestCacheHitRatio` — revert → ratio wrong; `TestServedModelMismatch` — revert → a served model differing from the requested one is not flagged; `TestPassK` — revert → pass^k counts a case with one failed trial.
**Baseline:** before=29 after=36
**Commit:** `feat(metrics): derive trial metrics and cost`
**After:** Task 5

**Decision:** metrics-all-four-groups

  - [x] **Step 1: Write the failing tests.**
  - [x] **Step 2: Implement** `Trial` (all design.md §6 fields), `FromUsage(Usage, requestedModel,
    pricing, wall)`, `CaseAggregate(trials)` (mean score, stddev, pass^k, cost_per_pass) and
    `LoadPricing(path)`. `pricing.json` maps model id → USD per million tokens for `input`,
    `output`, `cache_read`, `cache_write`; a requested alias (`sonnet`) resolves through the served
    model id. An unknown model is an error, never 0. Fill current prices for the Claude and OpenAI
    models the operator uses, each with its source URL in a sibling `"source"` field.
    Correction (2026-10-09), round-1 panel (operator decision F5): `RoleScore` is the one role-score
    function `run` and `compare` call — the mean of the case means, and for visual balanced accuracy,
    the mean of the `match` and `mismatch` classes' case means (decision
    `visual-score-balanced-accuracy`); `Visual.Label` carries the case's labelled verdict.
    Correction (2026-10-09), round-1 panel (finding F7): a requested model matches an id only when
    the two are equal or the request is a Claude CLI alias and the id is `claude-<alias>-<version>`;
    the substring match that priced an unpriced `gpt-5` at `gpt-5.6-terra`'s rates is gone, and an
    unknown model is a pricing error (`TestUnpricedModelIsNotPricedAsAnIdContainingIt`).
    Correction (2026-10-09), round-1 panel re-review (finding F17): an id that is the requested id
    plus a date snapshot suffix (`-` and eight digits, `claude-sonnet-5-5-20260301`) also matches,
    and an unpriced dated id is priced at its undated id; a version suffix (`claude-opus-5-5` for
    `claude-opus-5`) still does not match (`TestServedModelMismatch`).
    Correction (2026-10-09), round-1 panel (finding F13): `CaseAggregate` is the one place a case's
    valid trials' cost, wall time and tokens of every class are summed (`Aggregate.Cost`, `WallMS`,
    `Tokens`); `run`'s report and `compare`'s summary read them instead of each re-summing, and the
    judge-usage check reads `harness.Usage.Tokens()`. The role-score half was closed by `RoleScore`
    (F5). Tests: `TestPassK` (sums), `TestUsageTokensCountsEveryClassOnce`, the report rows pinned in
    `TestRunWritesHeaderAndTrials` and `TestNonInferiorWithinMargin`, and
    `TestInfraErrorFalsePassIsNotCounted`.
  - [x] **Step 3: Verify** with `-run 'TestCostFromPricing|TestUnknownModelPricingErrors|TestCacheHitRatio|TestServedModelMismatch|TestPassK'` on `./internal/metrics`.

- [x] 7. Prompt vendoring and rendering

**Build:** green
**Files:** `internal/prompts/prompts.go`, `internal/prompts/prompts_test.go`, `prompts/PIN`, `prompts/strip.txt`, `prompts/impl.md`, `prompts/review-primary.md`, `prompts/visual.md`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`
**Tests:** `TestStripRemovesListedSections`, `TestStripFailsOnMissingHeading`, `TestRenderFillsInputs`, `TestSyncVendorsAtSha`, `TestPromptsSyncExitCodes`
**Regression:** `TestStripRemovesListedSections` — revert → a listed Claude-only section stays in the vendored prompt; `TestStripFailsOnMissingHeading` — revert → a strip heading that no longer exists upstream is silently ignored; `TestRenderFillsInputs` — revert → a template field stays unrendered.
**Baseline:** before=35 after=40
**Commit:** `feat(prompts): vendor and render pinned flow role prompts`
**After:** Task 1, 2

**Decision:** real-flow-prompts-pinned
**Decision:** review-primary-slot-v1

  - [x] **Step 1: Write the failing tests.**
  - [x] **Step 2: Implement** `Sync(agentsRepo, sha)`: for each target, `git -C <agents> show
    <sha>:<path>` of its source (`skills/flow/implement.md` implementer dispatch blocks,
    `skills/flow/primary-reviewer-prompt.md`, `skills/flow/visual-verify-verifier.md`), remove each
    section whose heading `prompts/strip.txt` lists for that target (error if a listed heading is
    absent — upstream drift must be seen), append the role's output contract and `{{.…}}` input
    slots, write `prompts/<target>.md` and `prompts/PIN`. `Render(target, inputs)` uses
    `text/template` with `missingkey=error`.
    Correction (2026-10-09): shipped as `Sync(ctx, dir, agentsRepo, sha)` and `Render(dir, target, inputs map[string]string)` — the directory parameter lets tests and the CLI point at a directory; a map makes `missingkey=error` take effect.
  - [x] **Step 3: Vendor** at the current `/Users/tweety53/Projects/agents` `origin/main` sha and
    commit the three prompts, `strip.txt` and `PIN`. The visual prompt keeps the verifier's
    judging procedure and report shape and drops its capture/tooling steps, since cases supply the
    captures (decision `visual-static-pairs`); it requires a final `verdict: match|mismatch` line.
    Correction (2026-10-09): the verifier's own `## Report` template is all tooling lines, so it is dropped and the appended contract (`- defect:` lines, then the `verdict:` line) replaces it; the reviewer prompt also gets `skills/flow/reviewer-calibration.md` appended at the pin (its `## Calibration` section only names that file).
  - [x] **Step 4: Wire** `flow-eval prompts sync --agents <path> --sha <sha>`.
  - [x] **Step 5: Verify** with `-run 'TestStripRemovesListedSections|TestStripFailsOnMissingHeading|TestRenderFillsInputs'` on `./internal/prompts`.

- [x] 8. Implementer grader

**Build:** green
**Files:** `internal/grade/impl.go`, `internal/grade/impl_test.go`
**Tests:** `TestImplPassRequiresAllCommands`, `TestImplOutOfScopeFiles`, `TestImplOverbuildRatio`, `TestImplPreexistingBroken`
**Regression:** `TestImplPassRequiresAllCommands` — revert → a trial passes with one verify command failing; `TestImplOutOfScopeFiles` — revert → files outside the case's `files` are not counted; `TestImplOverbuildRatio` — revert → ratio not computed against `reference.stat`; `TestImplPreexistingBroken` — revert → a failing test whose file exists on the base is not counted.
**Baseline:** before=43 after=47
**Commit:** `feat(grade): grade implementer trials`
**After:** Task 2, 4

**Decision:** impl-db-cases-isolated-docker


Correction (2026-10-09), from Task 14's mining: before `heldout.patch` applies, every path it
touches returns to its snapshot state (checked out, or removed when the patch adds it) — `task.md`
asks the agent to write those very tests, so a plain apply collided on every trial;
`heldout-conflict` now means the patch does not apply over the snapshot state of its paths.
  - [x] **Step 1: Write the failing tests** with a fake sandbox `Exec`.
  - [x] **Step 2: Implement** `GradeImpl(sb, case)`: record `git diff --stat HEAD` and changed
    paths (diff lines, tests added = new `@Test`/`func Test` lines, out-of-scope = paths matching no
    `case.files` glob), apply `heldout.patch` (`git apply`; a conflict with the agent's own files
    fails the trial with reason `heldout-conflict`), run each `case.verify` command in order, pass =
    all exit 0. Pre-existing broken: parse JUnit XML under `**/build/test-results/**`; a failing test
    whose source file exists in the base snapshot (`git show HEAD:<path>` succeeds) counts.
    Correction (2026-10-09): shipped as `GradeImpl(ctx, sb Sandbox, c cases.Case) (Impl, error)`, `Sandbox` being `Exec` + `CopyIn` (satisfied by `*sandbox.Trial`). GradeImpl itself copies `heldout.patch` into the container, so Task 11 calls it only after the agent command returns and never copies the patch itself. `lint_failures` is not computed: `case.verify` does not mark which command is lint.
    Correction (2026-10-09), round-1 panel: `Impl` also returns the agent's diff against the
    snapshot (`Patch`, full context, from which `tests_added` is counted) and each verify command's
    output and exit (`Log`), as far as grading got; `err` covers a tree the agent left unusable and a
    verify command cut off by the grading deadline, which the run scores as a fail (Task 11).
    Correction (2026-10-09), round-1 panel (finding F12): the held-out reset script builds its git
    prefix from `_git`, the one definition of the hook-disabling flags
    (`TestImplResetOfHeldoutPathsRunsNoAgentHook`).
    Correction (2026-10-09), round-1 panel (finding F15): `lint_failures` is removed from the
    record rather than computed — `case.verify` does not mark which command is lint, and the field was
    written 0 on every impl trial (`TestImplRecordCarriesNoUnmeasuredLintCount`, Task 6's package;
    design.md §6 edited in place).
  - [x] **Step 3: Verify** with `-run 'TestImplPassRequiresAllCommands|TestImplOutOfScopeFiles|TestImplOverbuildRatio|TestImplPreexistingBroken'` on `./internal/grade`.

- [x] 9. Reviewer grader and judge

**Build:** green
**Files:** `internal/grade/review.go`, `internal/grade/judge.go`, `internal/grade/review_test.go`, `judge.json`
**Tests:** `TestParseReviewReport`, `TestCandidatePairsWithinWindow`, `TestReviewScoresWithFakeJudge`, `TestKnownFalseCountsAsFalsePositive`, `TestJudgeCostSeparate`, `TestLoadJudgeRefusesAnotherHarness`
**Regression:** `TestParseReviewReport` — revert → severity sections or `file:line` not parsed; `TestCandidatePairsWithinWindow` — revert → pairs outside ±15 lines or across files are sent to the judge; `TestReviewScoresWithFakeJudge` — revert → recall/precision/F1 wrong; `TestKnownFalseCountsAsFalsePositive` — revert → a finding matching `knownFalse` is not a false positive; `TestJudgeCostSeparate` — revert → judge tokens are added to the trial's own usage.; `TestLoadJudgeRefusesAnotherHarness` — revert → a codex judge, which cannot run with its tools disabled, is accepted.
**Baseline:** before=47 after=53
**Commit:** `feat(grade): grade reviewer trials with a pinned judge`
**After:** Task 2, 5

**Decision:** review-match-location-plus-judge
**Decision:** review-cases-prefix-diffs

  - [x] **Step 1: Write the failing tests** with a fake `Judge`.
  - [x] **Step 2: Implement** `ParseReview(text) []Finding` (`#### Critical|Important|Minor`
    sections, each issue's first `path:line`), `Judge` interface (`Same(label, finding) (bool,
    Usage)`, `Plausible(diff, finding) (bool, Usage)`), the real judge running `judge.json`'s
    harness/model/effort (`claude-code`, `opus`, `medium`) host-side on the diff text only, and
    `GradeReview` per design.md §5: pairs within ±15 lines in the same file, judge decides,
    recall over `mustFind`, precision over Critical+Important findings, `severity_agreement`,
    `duplicates`, `known_false_hits`. Judge usage returns in a separate `JudgeUsage` field.
    Correction (2026-10-09): the judge shipped as `Same(ctx, labelled, reported string) (bool, harness.Usage, error)` and `Plausible(ctx, diff, finding string) (bool, harness.Usage, error)` — the real judge is a process that can fail, and text arguments let the visual grader reuse `Same`. `GradeReview(ctx, report, diff, labels ReviewLabels, j Judge) (Review, error)`. The judge runs with `--tools ""` in an empty temp dir (its input carries candidate text), and `LoadJudge` refuses any harness but `claude-code`. A mustFind label gains an optional `severity` for `severity_agreement`.
  - [x] **Step 3: Verify** with `-run 'TestParseReviewReport|TestCandidatePairsWithinWindow|TestReviewScoresWithFakeJudge|TestKnownFalseCountsAsFalsePositive|TestJudgeCostSeparate|TestLoadJudgeRefusesAnotherHarness'` on `./internal/grade`.

- [x] 10. Visual grader

**Build:** green
**Files:** `internal/grade/visual.go`, `internal/grade/visual_test.go`
**Tests:** `TestParseVisualVerdict`, `TestFalsePassCounted`, `TestLocalizationWithFakeJudge`
**Regression:** `TestParseVisualVerdict` — revert → the `verdict:` line or the defect list is not parsed, or a missing verdict is not scored wrong; `TestFalsePassCounted` — revert → `match` on a `mismatch` case is not flagged `false_pass`; `TestLocalizationWithFakeJudge` — revert → localization share wrong.
**Baseline:** before=53 after=56
**Commit:** `feat(grade): grade visual-verify trials`
**After:** Task 9

**Decision:** visual-static-pairs

  - [x] **Step 1: Write the failing tests.**
  - [x] **Step 2: Implement** `GradeVisual(report, labels, judge)`: last `verdict: match|mismatch`
    line (absent → incorrect), `correct`, `false_pass`, and for mismatch cases `localization` =
    labelled defects the judge matches to a reported defect ÷ labelled defects.
    Correction (2026-10-09): shipped as `GradeVisual(ctx, report, labels VisualLabels, j Judge) (Visual, error)` with a separate `JudgeUsage`.
    Correction (2026-10-09), round-1 panel: `GradeVisual` records the case's labelled verdict as
    `visual.label`, the class the balanced role score groups by (Task 6).
  - [x] **Step 3: Verify** with `-run 'TestParseVisualVerdict|TestFalsePassCounted|TestLocalizationWithFakeJudge'` on `./internal/grade`.

- [x] 11. Run orchestration

**Build:** green
**Files:** `internal/run/run.go`, `internal/run/run_test.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `internal/cases/case.go`, `internal/cases/case_test.go`, `internal/cases/testdata/valid/impl/i1/case.json`, `internal/cases/testdata/valid/review/r1/case.json`, `internal/sandbox/sandbox.go`, `internal/sandbox/sandbox_test.go`, `README.md`
**Tests:** `TestRunWritesHeaderAndTrials`, `TestInfraErrorExcludedFromScore`, `TestHeldoutAppliedAfterAgentExit`, `TestEgressDeniedCountsProxyRefusals`, `TestRunRefusesBeforeAnyContainer`, `TestTimedOutTrialIsScoredNotInfra`, `TestRunIDsNeverShareSandboxesOrRecords`, `TestSIGTERMCancelsARun`, `TestOnlyBuildingTrialsGetTheGradleVolume`, `TestGradingHasADeadline`, `TestImplFailureIsRecorded`, `TestAgentCausedGradeFailureIsScored`, `TestVisualRoleScoreIsBalancedAccuracy`, `TestOnlyTheFirstSetCredentialEntersATrial`, `TestRunCancelledDuringGradingIsInfra`, `TestJudgePricingFailureIsScored`
**Regression:** `TestRunWritesHeaderAndTrials` — revert → the JSONL header lacks the prompt pin, case-set hash or flow-eval sha, or a trial line is missing; `TestInfraErrorExcludedFromScore` — revert → an infra-error trial is scored as a fail; `TestHeldoutAppliedAfterAgentExit` — revert → `heldout.patch` is copied into the container before the agent command returns; `TestTimedOutTrialIsScoredNotInfra` — revert → a trial cut off at its timeout is recorded as `infra_error`; `TestRunIDsNeverShareSandboxesOrRecords` — revert → two runs in one second share sandbox names or a second run overwrites the first's record; `TestSIGTERMCancelsARun` — revert → a SIGTERM kills the run without tearing its containers down.
**Baseline:** before=61 after=74
**Commit:** `feat(run): orchestrate trials and write run records`
**After:** Task 1, 2, 4, 5, 6, 7, 8, 9, 10, 16

**Decision:** results-in-repo-files
**Decision:** gradle-cache-warmed-host-side

  - [x] **Step 1: Write the failing tests** with fake sandbox and harness recording call order.
  - [x] **Step 2: Implement** `flow-eval run --role R --harness H --model M --effort E [-k 3]
    [--cases id,…] [--parallel 2] [--slot primary]`: run id `<utc-stamp>-<role>-<harness>-<model>-<effort>`;
    per case × trial: snapshot (impl/review) → sandbox start → copy inputs → render prompt → run
    the harness command with a per-role timeout (impl 45m, review 15m, visual 10m; flow's own
    review ceiling is 15m) → copy transcript and output out → grade (impl grades inside the
    container, then copy-in of `heldout.patch` happens only here) → metrics → append the trial
    line → sandbox stop.
    Correction (2026-10-09), from the Tasks 5–10 reviews: the rendered prompt is copied into the
    container as a file and passed to `Harness.Command` as `promptFile` (the CLI reads it on stdin;
    Linux caps one argument at 128 KiB); before any impl/review case of a repository runs,
    `sandbox.WarmGradle` (Task 16) warms its cache volume; after the harness command returns and
    before grading, every process the agent left in the container is killed
    (`sh -c 'kill -KILL -1'`) so nothing it started can touch the held-out tests; `GradeImpl` copies
    `heldout.patch` itself, so this task never copies it; the judge's `JudgeUsage` is recorded apart
    and never added to the trial's `cost_usd`.
    Correction (2026-10-09): shipped with a required `snapshot` sha in an impl or review `case.json`
    (impl: the task's base; review: the reviewed head, read beside `/case/diff.patch`) — nothing
    else named the commit to snapshot; `--repos <dir>` (default `..`) locates `<dir>/<source.repo>`;
    `(*sandbox.Trial).EgressDenied` counts the proxy's `egress-denied` lines; a review trial passes
    (for pass^k and flips) when its recall is 1; a grader failure is `infra_error` with
    `error: "grade: …"`; review and visual runs need a host Claude credential for the judge, its
    usage in the trial's `judge` object; setup failures (snapshot, warm-up, render) exit 2 with no
    record. A failure before the harness produced any output is `infra_error`.
    Correction (2026-10-09), from the Task 11 review: a trial cut off at its timeout whose transcript
    has no result line is scored `timed_out` with zero usage, never `infra_error` (a harness writes
    its usage only in the final result line; `ponytail:` comment in `run.go` names the ceiling —
    `cost_per_pass` reads low for a candidate that times out often).
    Correction (2026-10-09), from the Task 11 review: SIGTERM cancels a run like Ctrl+C, so teardown
    runs; a trial's sandbox name is the run id plus its number, and the run record is created
    exclusively, so a second run with the same id is refused before any container starts.
    Correction (2026-10-09), round-1 panel, operator decision F4 — supersedes the sentence "a grader
    failure is `infra_error` with `error: "grade: …"`" above: once the agent has produced output, any
    failure — grading included — is a scored fail, `passed: false`, score 0, its reason in `error`
    (`grade: …` for a grading failure); `infra_error` stays only for container, proxy or API failures
    before the agent produced output, and for a run cancelled mid-trial (decision
    `after-output-failures-scored`). Grading runs under its own deadline, the role's timeout again.
    An impl trial's line carries its grade reason in `error`, and its trial directory keeps
    `work.patch` and `verify.log`. A visual trial's sandbox gets no repository, so no Gradle volume.
    A trial gets only the first set credential of its harness (`CLAUDE_CODE_OAUTH_TOKEN` before
    `ANTHROPIC_API_KEY`). The report's role score is `metrics.RoleScore` (Task 6).
    Correction (2026-10-09), round-1 panel fix, dispatcher decision — narrows the F4 correction
    above: after the agent's output, a failure of the harness's own stays `infra_error` — the trial's
    model missing from `pricing.json`, the egress-denial count failing, an artifact (`output.md`,
    `work.patch`, `verify.log`) failing to write; a transcript or grading failure, a judge pricing
    failure included, stays a scored fail (decision `after-output-failures-scored`,
    `TestHarnessSideFailureAfterOutputIsInfra`, `TestJudgePricingFailureIsScored`).
    Write `runs/<id>.jsonl` (header per design.md §8) and `runs/<id>.md` (per-case table).
    Credentials missing for the chosen harness → exit 2 before any container starts.
  - [x] **Step 3: Verify** with `-run 'TestRunWritesHeaderAndTrials|TestInfraErrorExcludedFromScore|TestHeldoutAppliedAfterAgentExit'` on `./internal/run`.

- [x] 12. Compare, baseline and report

**Build:** green
**Files:** `internal/compare/compare.go`, `internal/compare/compare_test.go`, `baselines/.gitkeep`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`
**Tests:** `TestNonInferiorWithinMargin`, `TestRegressionBeyondMargin`, `TestAlwaysPassFlipIsRegression`, `TestVisualFalsePassIncreaseIsRegression`, `TestImprovementExit3`, `TestRegressionBeatsImprovement`, `TestPinMismatchExit2`, `TestMissingTrialsExit2`, `TestCompareAndBaselineExitCodes`, `TestKOrMarginMismatchExit2`, `TestVisualRoleScoreIsBalancedAccuracy`
**Regression:** `TestNonInferiorWithinMargin` — revert → a candidate within margin is not exit 0; `TestRegressionBeyondMargin` — revert → a drop beyond margin is not exit 1; `TestAlwaysPassFlipIsRegression` — revert → a pass^k→0 flip within margin passes; `TestVisualFalsePassIncreaseIsRegression` — revert → more false-passes pass; `TestImprovementExit3` — revert → a gain beyond margin is not exit 3; `TestRegressionBeatsImprovement` — revert → a run with both exits 3; `TestPinMismatchExit2` — revert → different prompt pins compare; `TestMissingTrialsExit2` — revert → a case with fewer than k valid trials compares; `TestKOrMarginMismatchExit2` — revert → a k mismatch or a non-positive margin is compared instead of exit 2.
**Baseline:** before=66 after=76
**Commit:** `feat(compare): decide non-inferiority against the baseline`
**After:** Task 1, 2, 7, 11

**Decision:** non-inferiority-vs-baseline
**Decision:** compare-prints-both-shas

  - [x] **Step 1: Write the failing tests** over in-memory run fixtures.
  - [x] **Step 2: Implement** per design.md §7: `flow-eval compare <candidate-run-id>
    [--baseline <run-id>] [--margin 5]` (baseline defaults to `baselines/<role>.json`), writing
    `runs/<cand>-vs-<base>.md` and exiting 0/1/3/2; `flow-eval baseline set <run-id>` writing
    `baselines/<role>.json` `{runId, setAt}`. Case-set hash mismatch → exit 2 naming the differing
    case ids.
    Correction (2026-10-09), from the Task 12 review: a candidate and baseline of different `k` →
    exit 2 (pass^k and false-pass counts over different trial counts are not comparable; a k=1
    candidate otherwise reports one lucky trial as a 0 → pass^k flip), and a `--margin` that is not
    positive → exit 2 (with the literal `≥` a zero margin makes two equal runs an improvement).
    Correction (2026-10-09), round-1 panel (operator decision F5): the role score is
    `metrics.RoleScore` — balanced accuracy for visual (Task 6).
    Correction (2026-10-09), round-1 panel (operator decision on the flow-eval sha): `compare` prints
    both runs' flow-eval shas after the verdict line and in the report, flagged `DIFFER` when unequal,
    and never refuses on a difference (decision `compare-prints-both-shas`;
    `TestFlowEvalShasArePrintedAndADifferenceFlagged`, `TestCompareAndBaselineExitCodes`).
    Correction (2026-10-10), Task 31 (decision `compare-warns-on-mixed-prompt-pins`) — supersedes
    `TestPinMismatchExit2` in Tests, Regression and Step 3 above: a prompt-pin mismatch warns and does
    not refuse; the verdict and exit code are the pass rule's alone. Task 31 renamed the test
    `TestCaseSetMismatchExit2`, which pins the case-set refusal (revert → runs over different case
    sets compare instead of exit 2), and Step 3's `-run` filter selects it in place of
    `TestPinMismatchExit2`.
  - [x] **Step 3: Verify** with `-run 'TestNonInferiorWithinMargin|TestRegressionBeyondMargin|TestAlwaysPassFlipIsRegression|TestVisualFalsePassIncreaseIsRegression|TestImprovementExit3|TestRegressionBeatsImprovement|TestPinMismatchExit2|TestMissingTrialsExit2|TestKOrMarginMismatchExit2'` on `./internal/compare`.

- [x] 13. Case miners and the how-to

**Build:** green
**Files:** `internal/mine/mine.go`, `internal/mine/mine_test.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `docs/cases.md`
**Allowed-collateral:** `internal/mine/testdata/**`
**Tests:** `TestMineImplFromLedger`, `TestMineReviewLabelsFromPanel`, `TestMineReviewSupersededGoesToIgnored`, `TestMineVisualCopiesImages`, `TestMineVisualCropsTheMockupToItsFrame`
**Regression:** `TestMineImplFromLedger` — revert → task n's commit, base, held-out test files or `files` scope are not mined; `TestMineReviewLabelsFromPanel` — revert → fixed Critical/Important findings are not `mustFind` or false-premise withdrawals are not `knownFalse`; `TestMineReviewSupersededGoesToIgnored` — revert → a withdrawal superseded by a decision lands in `knownFalse`; `TestMineVisualCopiesImages` — revert → images or verdict not written; `TestMineImplFromLedger` also — revert → `task.md` or `context.md` carries the archived plan's later correction; `TestMineReviewLabelsFromPanel` also — revert → a finding in another repository, at a file outside `diff.patch`, or withdrawn without a false premise is labelled; `TestMineVisualCropsTheMockupToItsFrame` — revert → a declared geometry's board is committed uncropped.
**Baseline:** before=76 after=80 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 70954b7 -->
**Commit:** `feat(mine): add case add for impl, review and visual`
**After:** Task 1, 2, 7, 11, 12

**Decision:** case-add-miners-commit-review
**Decision:** review-cases-diff-only


Correction (2026-10-09), from the Task 13 review: an impl case's `task.md` and `context.md` are read
at the snapshot (`git show <base>:spectre/changes/<change>/…`), never from the archive, which
carries later Corrections — the answer; a review finding in another repository or at a file
`diff.patch` does not change goes to `ignored[]` with that reason; `knownFalse` holds only a
withdrawal saying the premise was false, every other withdrawal `ignored[]` (decision
`known-false-explicit-only`); `case add visual` cuts the mockup to its frame with flow's
`compose-mockup-frames.sh` (`--compose`) under the source repository's `mockup frame` geometry
(decision `visual-mockup-frame-crop`), and `case add` cancels on Ctrl+C or SIGTERM.

Correction (2026-10-09), from Task 14's mining: `case add impl` skips a `-fix-` implementer
dispatch when choosing the reference (a fix round's parent already holds the answer); `case add
impl|review` take `--plan-repo` for a change whose archive is in another checkout, reading the plan
there as of its last `tasks.md` commit made no later than the snapshot; a review `diff.patch` leaves
out `spectre/changes/` and `openspec/changes/`; `docs/cases.md` gains the snapshot-ref step
(decision `snapshot-refs-pushed`).

Correction (2026-10-09), from the Task 13 re-review on real kan-867 history: the `--plan-repo` plan
is the commit (any ref) with the latest author date no later than the reference commit's — a
rebase re-stamps committer dates after the code, and the snapshot predates the plan; a checkout
whose `.flow/project.md` declares no `## lint` (gymie-frontend) gets test-only `verify`, the
curator adding its lint commands.
  - [x] **Step 1: Write the failing tests** over a fixture repo built in `t.TempDir()` with a
    `spectre/changes/archive/<name>/` holding a `ledger.md`, `panel.md`, `tasks.md`, copied from the
    shapes in gymie `kan-757-offline-l5-be-per-item-if-match-on-group-session`.
  - [x] **Step 2: Implement** per design.md §3. `case add impl`: the ledger dispatch whose `Task:`
    is n and `Role: implementer` gives the commit; held-out = that commit's changed test files
    (`src/test/**`, `*Test.kt`, `*_test.go`, `commonTest/**`, `desktopTest/**`) as a patch;
    `files` from task n's `**Files:**`; `verify` from the source repo's `.flow/project.md` `## lint`
    plus `--tests` selectors for the held-out classes plus the module test task; `needsDocker` by
    grepping the module build file for `testcontainers`. `case add review`: range per §3; labels
    from the findings table + `finding-status` lines; a withdrawal reason containing `decision` or
    `superseded` goes to `ignored[]`, any other withdrawal to `knownFalse`; each mustFind label
    carries the finding row's `severity`, and a plan-alignment finding is left out (decision
    `review-cases-diff-only`). Every impl and review case writes the `snapshot` sha Task 11 requires
    (impl: the task commit's parent; review: the reviewed range's head). `case add visual`: copy
    both PNGs and write `labels.json`. Every miner reads gymie checkouts read-only and prints the
    draft path plus "edit labels, then commit".
  - [x] **Step 3: Write `docs/cases.md`:** per role, the `case add` command, what it mined, what to
    check and edit in `labels.json`/`case.json`, then `flow-eval case validate` and commit — the
    commit review is the label approval; plus how to retire a case (delete the dir; the case-set
    hash change forces a new baseline).
  - [x] **Step 4: Verify** with `-run 'TestMineImplFromLedger|TestMineReviewLabelsFromPanel|TestMineReviewSupersededGoesToIgnored|TestMineVisualCopiesImages'` on `./internal/mine`.

- [x] 14. Seed case set

**Build:** green
**Files:** `cases/review/kan-757-offline-l5-be-per-item-if-match-on-group-session/case.json`
**Allowed-collateral:** `cases/**`
**Tests:** **none** — data; `flow-eval case validate` is the check.
**Regression:** revert → no cases; `flow-eval run` has nothing to run.
**Baseline:** before=87 after=87 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 240f2d0 -->
**Commit:** `feat(cases): seed v1 impl, review and visual cases from gymie`
**After:** Task 13

**Decision:** case-set-small-v1
**Decision:** review-cases-prefix-diffs
**Decision:** visual-static-pairs


Correction (2026-10-09), from the first two attempts: visual labels are flow's recorded verdicts
(decision `visual-labels-from-flow-verdicts`); gymie-frontend changes planned in gymie count, mined
with `--plan-repo`; every impl and review case's snapshot is listed for the operator's
`refs/flow-eval/<case-id>` push (decision `snapshot-refs-pushed`). Commit `75a66e5` holds the first
4 impl and 4 review cases; this task's commit is amended or followed by the rest.

Correction (2026-10-09), shipped in `240f2d0`: 6 impl (4 gymie, kan-867 task 1 and kan-868 task 9
from gymie-frontend), 6 review — the operator accepted 6: every other archive's ledger commits are
split across checkouts or gone — and 3 match + 10 mismatch visual: only three pairs carry a recorded
`match`, and kan-758's "sidecar-accepted" pairs are left out because their differences are visible
(operator). Visual captures come from the archived change's `visual-verification/evidence/`, the
images the record judged, not gymie-playwright's moved `-snapshots/`.
Correction (2026-10-09), from the Task 14 review (operator): kan-868 Q3/Q5 are relabelled
`mismatch` by kan-870's record of the same frames (their rail done-mark departure, excused in
kan-868's record only by a decision a trial never sees), leaving 1 match + 12 mismatch; kan-847 X2's
third defect, false of the capture, is dropped; kan-868 task 9's `context.md` carries the
`notes/step-2.md` sentence its held-out test depends on.
  - [x] **Step 1: impl.** `case add impl` for six tasks: four gymie backend tasks drawn from
    `kan-757-offline-l5-be-per-item-if-match-on-group-session`,
    `kan-755-offline-l4-be-create-endpoints-accept-a-client`, `kan-961-api-metrics-dashboard`,
    `kan-863-security-prod-actuator-*` (at least two with `needsDocker`), and two gymie-frontend
    tasks whose tests run under `:shared:desktopTest`. Prefer tasks whose reference commit adds
    tests (the held-out set must not be empty).
  - [x] **Step 2: review.** `case add review` for ten gymie archived changes whose `panel.md`
    round 0 has at least one `fixed` Critical or Important finding; kan-757 is one. Check every
    `mustFind` still reproduces in `diff.patch` (the line exists in the pre-fix diff).
  - [x] **Step 3: visual.** Ten `match` pairs from gymie-playwright `fidelity/*.spec.ts.mockups`
    sidecars (mockup `gymie/docs/design/mockups/<frame>.png` vs the `-snapshots/<name>-darwin.png`)
    and ten `mismatch` pairs from archived `visual-verification/evidence` or fidelity snapshot
    history where a later commit fixed the drawn difference, each with its defect text and region.
  - [x] **Step 4: Verify** `go run ./cmd/flow-eval case validate` exits 0.

- [x] 15. README

**Build:** green
**Files:** `README.md`
**Tests:** **none** — docs.
**Regression:** revert → the operator loop, credentials and exit codes are undocumented.
**Baseline:** before=81 after=81
**Commit:** `docs: document the eval loop and commands`
**After:** Task 12, 13, 17

**Decision:** dind-privileged-accepted-v1

  - [x] **Step 1: Write** the loop from design.md §1, image build (`docker build` both
    Dockerfiles, with the agents-repo build input Task 17 adds), the privileged-dind risk of decision
    `dind-privileged-accepted-v1`, credentials (`CLAUDE_CODE_OAUTH_TOKEN` or `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`),
    each command with an example, the `compare` exit codes, reading a report, and a pointer to
    `docs/cases.md`.
    Correction (2026-10-09), round-1 panel (finding F8): the `run` example's `--cases` names the
    shipped `kan-757-…-task-3`; the `-task-1` it named does not exist under `cases/`.
    Correction (2026-10-09), round-1 panel (primary-pass recommendation): the judge prerequisite
    names the environment credential, not a host `claude` login — the judge runs with a fresh
    `CLAUDE_CONFIG_DIR` and `run` requires the credential.
  - [x] **Step 2: Verify** lint per Global constraints.

- [x] 16. Warm the Gradle cache outside the sandbox

**Build:** green
**Files:** `internal/sandbox/sandbox.go`, `internal/sandbox/sandbox_test.go`, `docker/runner.Dockerfile`, `README.md`
**Tests:** `TestWarmGradleRunsNoAgentAndNoCredential`, `TestWarmGradleSkipsAWarmVolume`, `TestTrialGradleCacheIsReadOnlyAndPrivate`, `TestNoGradleVolumeWithoutRepo`, `TestWarmGradleNamesEachWarmUpApart`, `TestRealDockerGradleCacheIsReadOnlyAndPrivate`
**Regression:** `TestWarmGradleRunsNoAgentAndNoCredential` — revert → the warm-up container is not created, mounts a host path, joins the trial network, or receives a credential; `TestWarmGradleSkipsAWarmVolume` — revert → a volume holding the marker is warmed again.
**Baseline:** before=59 after=65
**Commit:** `feat(sandbox): warm a repository's Gradle cache outside the trial sandbox`
**After:** Task 4

**Decision:** gradle-cache-warmed-host-side

  - [x] **Step 1: Write the failing tests** with the fake `Runner`.
  - [x] **Step 2: Implement** `WarmGradle(ctx, docker Runner, image, repo, snapshotTar string) error`:
    when the volume `flow-eval-gradle-<repo>` holds `/root/.gradle/.flow-eval-warm`, return; else run
    one container from `image` on the default bridge (ordinary egress), the volume at `/root/.gradle`,
    no credential and no host path, copy the snapshot in, run `./gradlew --version` in it (fetches the
    wrapper distribution), write the marker, and remove the container in a `defer` on every outcome.
    A snapshot with no `gradlew` writes the marker and returns.
    Correction (2026-10-09), round-1 panel (findings: shared writable Gradle home leaks held-out
    tests and reference code; Least Privilege) — decision `gradle-volume-read-only`: the volume is
    renamed `flow-eval-gradle-ro-<repo>`, so one filled under the shared home is never mounted; the warm-up
    container (named `flow-eval-warm-<repo>-<random>`, so concurrent runs never remove each other's)
    mounts the volume at `/gradle-ro`, builds the snapshot in its own Gradle home (`./gradlew
    --version`, then `./gradlew testClasses --continue`, best effort) and copies only `wrapper/` and
    `caches/modules-2` (less `*.lock` and `gc.properties`) into the volume, the marker at
    `/gradle-ro/.flow-eval-warm`. A trial mounts the volume `readonly` at `/gradle-ro` with
    `GRADLE_RO_DEP_CACHE=/gradle-ro/caches`, keeps `/root/.gradle` as its own Gradle home, and copies
    the wrapper into it at start (the wrapper locks its distribution, which a read-only mount
    refuses). `TestRealDockerGradleCacheIsReadOnlyAndPrivate` runs against real Docker when
    `FLOW_EVAL_IT_RUNNER`, `FLOW_EVAL_IT_PROXY` and `FLOW_EVAL_IT_GRADLEW_FROM` are set, and skips
    otherwise.
  - [x] **Step 3: Verify** with `-run 'TestWarmGradleRunsNoAgentAndNoCredential|TestWarmGradleSkipsAWarmVolume'` on `./internal/sandbox`, then once against real Docker with an empty gymie volume: the warm-up downloads the distribution, and a following `needsDocker` trial runs `./gradlew` with the allowlist unchanged.

- [x] 17. Visual measurement tool in the runner image

**Build:** green
**Files:** `docker/runner.Dockerfile`, `prompts/strip.txt`, `prompts/visual.md`, `internal/prompts/prompts.go`, `internal/prompts/prompts_test.go`
**Tests:** `TestVisualPromptKeepsMeasurement`, `TestStripCutsAClauseAcrossLineWraps`
**Regression:** `TestVisualPromptKeepsMeasurement` — revert → the vendored visual prompt loses the `measure-visual-properties.sh` procedure, or still orders a live state capture, or loses sweep 6 or keeps its seeded-value clause; `TestStripCutsAClauseAcrossLineWraps` — revert → a `~` clause entry cuts nothing or a missing clause passes silently.
**Baseline:** before=80 after=81
**Commit:** `feat(prompts): run the visual verifier's measurement tool in the runner image`
**After:** Task 4, 7

**Decision:** visual-measure-tool-in-runner


Correction (2026-10-09), from the Task 17 review: sweep 6 stays, less its clause driving a seeded
value, cut by a `~` clause entry in `prompts/strip.txt`; the prompt's tail says the mockup is
already cut to its frame and measured at `--scale 1`.
  - [x] **Step 1: Establish how the script runs.** `scripts/measure-visual-properties.sh` at
    `prompts/PIN` is a shim over the agents repo's `flow-guard` Go binary
    (`scripts/lib/flow-guard.sh`). Find how the shim locates or builds `flow-guard`, and the
    smallest way to put a working `measure-visual-properties.sh` on the runner image's `PATH` built
    from the agents repo at exactly `prompts/PIN` (a multi-stage build fed by a `git archive` of
    the pin, or a named build context) — never from a working tree, never a committed binary.
  - [x] **Step 2: Write the failing test** asserting the vendored `prompts/visual.md` keeps the
    measurement procedure and drops the live-capture order.
  - [x] **Step 3: Implement** the image change and the strip change, re-run `flow-eval prompts sync`
    at the pin.
  - [x] **Step 4: Verify** with `-run 'TestVisualPromptKeepsMeasurement'` on `./internal/prompts`,
    then `docker build -f docker/runner.Dockerfile .` and, in the built image,
    `measure-visual-properties.sh <a png> <a png> --scale 1` exits 0 with JSON.

- [x] 18. In-run fix 1 — verify: claude refuses --dangerously-skip-permissions as root

**Build:** green
**Files:** `internal/harness/claude.go`, `internal/harness/harness_test.go`
**Tests:** `TestClaudeCommandArgs`
**Regression:** `TestClaudeCommandArgs` — revert → the claude-code trial env lacks `IS_SANDBOX=1`, and every claude-code trial in the root runner container exits 1 before its first turn.
**Baseline:** before=109 after=109 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 389d0b0 -->
**Commit:** `fix(harness): mark a claude-code trial as sandboxed so it runs as root`
**After:** Task 5

The live check's three claude-code trials (visual `kan-870-qm-q6`, review
`kan-961-api-metrics-dashboard`, impl `kan-757-…-task-3`) all ended `infra_error` with
`agent produced no output: exit 1: --dangerously-skip-permissions cannot be used with root/sudo
privileges for security reasons`. The runner container runs the agent as root. measured: (runner
image `flow-eval-runner`, 2026-10-09) `claude --dangerously-skip-permissions -p hi` as root prints
that refusal; with `IS_SANDBOX=1` it passes the check and stops at `Not logged in`.

  - [x] **Step 1: Write the failing test.** Extend the claude-code `Command` test's expected env
    with `IS_SANDBOX=1`.
  - [x] **Step 2: Implement.** `Claude.Command` adds `IS_SANDBOX=1` to the trial env.
  - [x] **Step 3: Verify** `go test ./internal/harness -count=1`, then re-run the live check's
    three trials.

- [x] 19. One HTML page per run

**Build:** green
**Files:** `internal/run/run.go`, `internal/run/page.go`, `internal/run/run_test.go`, `README.md`
**Tests:** `TestRunWritesHTMLPage`, `TestCaseRowMeanWallIsPerValidTrial`
**Regression:** `TestRunWritesHTMLPage` — revert → a run writes no `runs/<id>.html`, or the page lacks a trial's output, verify log or diff, or renders agent text unescaped, or lists trials in finish order; `TestCaseRowMeanWallIsPerValidTrial` — revert → the case row's mean wall time is not the valid trials' mean.
**Baseline:** before=109 after=111 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 736eee6 -->
**Commit:** `feat(run): write every run's results as one self-contained HTML page`
**After:** Task 11, 12

Operator request (2026-10-09, fix run): "lets make the results an html page where I can see
everything clearly and in one place (every run must produce it)". Operator answers: `run` only
writes it (not `compare`); everything inline in one self-contained file.

Every `flow-eval run` writes `runs/<id>.html` beside `runs/<id>.jsonl` and `runs/<id>.md`, from the
same header and trial lines: the run header (role, harness, model, effort, k, slot, prompt pin,
case set hash, flow-eval sha, started), the role score and how it is computed, the per-case table
the Markdown report carries, the infra errors, and one section per trial holding every metric of
its JSONL line (judge usage apart), its error, and — collapsible — its `output.md`, `verify.log`
and `work.patch` read from `runs/<id>/<case>/<trial>/`, plus the relative path of its
`transcript.jsonl`. One file: inline CSS, no script, no external asset; every value is
HTML-escaped (`html/template`). The case table's figures are computed once and rendered into both
the Markdown and the HTML report.

Correction (2026-10-10), from the Task 19 review: a cancelled run writes both reports from the
trials recorded so far, then returns its error; a metric renders as its record writes it (a
number in full, never `1.5e+06`; null as `–`); an artifact longer than 256 KiB is cut, naming its
file.
  - [x] **Step 1: Write the failing test.** A fake visual run's `runs/<id>.html` exists and carries
    the case row, the role score, each trial's metrics and output text; an agent output holding
    `<script>` appears escaped.
  - [x] **Step 2: Implement** `internal/run/page.go` and the call beside the Markdown report.
  - [x] **Step 3: Verify** `go test ./internal/run -count=1`, then open a page from a real run.

- [x] 20. An index page linking every run

**Build:** green
**Files:** `internal/run/run.go`, `internal/run/page.go`, `internal/run/index.go`, `internal/run/run_test.go`, `internal/compare/compare.go`, `README.md`
**Tests:** `TestRunWritesIndexOfEveryRun`
**Regression:** `TestRunWritesIndexOfEveryRun` — revert → a run writes no `runs/index.html`, or the index misses an earlier run, groups or orders runs wrongly, or a run page lacks its link back and its contents list.
**Baseline:** before=111 after=112 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 992c49c -->
**Commit:** `feat(run): link every run from one index page`
**After:** Task 19

Operator request (2026-10-10, fix run): "About the htmls - I would like to see everything together.
Structured with links". Operator answer: an index page of all runs.

Every `flow-eval run` — a cancelled one included — rewrites `runs/index.html` from every
`runs/*.jsonl` in the runs directory: runs grouped by role (impl, review, visual), newest first,
one row per run with started, harness, model, effort, slot, k, cases, role score, valid/total
trials, passes, infra errors, cost, judge cost and flow-eval sha, its id linking to
`<id>.html`. A run file that cannot be read is listed with its error, never failing the run. Each
run page links back to `index.html` and opens with a contents list linking to every case row and
every trial. The JSONL reader moves from `compare` into `run` (`run.Load`), which both the index
and `compare` call.

Correction (2026-10-10), from the Task 20 review: the index is replaced through a temp file and a
rename, so two runs sharing a runs directory never leave it torn; a role outside impl, review and
visual is listed after them, by name; a run link is `./<id>.html`, so an id with a colon (a model
such as `qwen:7b`) still links; a run's cases are the ones its header names, whether or not a
trial line was written (`TestIndexListsEveryRunFile`, `TestIndexWriteFailureLeavesNoTempFile`).
  - [x] **Step 1: Write the failing test.** Two runs of different roles in one runs directory, plus
    a malformed `.jsonl`: `index.html` lists both under their roles, newest first, linking each
    page, and names the malformed file; a run page carries `href="index.html"` and anchors for
    its cases and trials.
  - [x] **Step 2: Implement** `internal/run/index.go`, `run.Load`, the page's links.
  - [x] **Step 3: Verify** `go test ./... -count=1`, then a real run's index.

- [x] 21. Every impl case passable by its own reference

**Build:** green
**Files:** `cases/impl/kan-755-offline-l4-be-create-endpoints-accept-a-client-task-5/case.json`, `cases/impl/kan-863-security-prod-actuator-prometheus-and-actuator-task-1/case.json`, `cases/impl/kan-867-qm-2-step-1-frontend-settings-workout-mode-focus-task-1/case.json`, `cases/impl/kan-868-qm-3-step-2-frontend-quick-runner-finished-view-task-9/case.json`, `docker/runner.Dockerfile`, `docs/cases.md`
**Tests:** none — case data and the runner image; the check is the reference run below
**Regression:** none — case data and the runner image; reverting puts back a verify command no agent can pass in the sandbox
**Commit:** `fix(cases): make every impl case passable by its own reference in the sandbox`
**After:** Task 14

Operator request (2026-10-10): "fix the case so its verify step can pass". measured: (sonnet-low run
`20261009T214828Z`, 2026-10-10) every kan-755 impl trial failed `./gradlew :app:test` on
`AuthFlowIntegrationTest.appSchemaContainsExpectedTables`: the snapshot `09323102` carries
migration `V38__client_create.sql` (table `client_create`) while that test's expected table list
does not name it, and the reference commit `75197d04` does not touch the test, so no agent — the
reference included — can pass the command. The other gymie snapshots carry no such gap.

Correction (2026-10-10), scope: the check runs in the trial's sandbox shape, not a host worktree,
and it refuted three more cases. measured: (sandbox reference check, runner image as of Task 14,
2026-10-10) kan-863's `ActuatorExposureTest` "health stays public" got 503 — the gateway's Redis
health indicator is down with no Redis in the trial (the host passes only because a Redis listens
on its 6379); every Compose desktop UI test of kan-867 and kan-868 failed to load skiko's native
library (`libGL.so.1`, then `libEGL.so.1` and `libfontconfig.so.1`, missing from the runner);
with those three installed, gymie-frontend's whole `:shared:desktopTest` (3,145 tests) still
failed one or two different timing-bound UI tests per run (`ShellDrawerUiTest`, then
`OwnProfileSwitchedUiTest`) while each passed alone. So kan-863's gateway commands run with
`MANAGEMENT_HEALTH_REDIS_ENABLED=false`, the runner image gains `libgl1 libegl1 libfontconfig1`,
and kan-867 and kan-868 drop the whole-suite command, keeping their held-out tests by name.
measured: (sandbox reference check after the fix, 2026-10-10) every remaining command of the four
cases exits 0; kan-863's `verifyChange` lost its Gradle daemon once while a trial ran beside it
and exited 0 alone at 10:41.

  - [x] **Step 1: Reference check.** Run every impl case's reference commit (named in its `notes`)
    through its `verify` commands in the trial's sandbox shape (the runner image, no host services); every command
    of every case must exit 0, except one the case's own snapshot already fails.
  - [x] **Step 2: Fix** each case the check refutes by dropping the command its snapshot fails; for
    kan-755, `./gradlew :app:test` (the held-out `WeighInClientIdIntegrationTest` still runs by
    name, and `./gradlew :tracking:test` still runs whole).
  - [x] **Step 3: Verify** `flow-eval case validate` and the reference check again on the fixed
    cases.

- [x] 22. Price a model whose rate depends on the prompt's size

**Build:** green
**Files:** `internal/harness/harness.go`, `internal/harness/claude.go`, `internal/harness/harness_test.go`, `internal/metrics/metrics.go`, `internal/metrics/metrics_test.go`, `pricing.json`, `README.md`
**Tests:** `TestClaudeParseCalls`, `TestClaudeParseSumsEveryResult`, `TestLongPromptCallsPricedAtTheLongRate`
**Regression:** `TestClaudeParseCalls` — revert → the claude transcript's per-call usage is not recorded, a call streamed as several events is counted more than once, or a message_delta that names no message (the runner's CLI 2.1.286) is dropped and the call keeps its stream-start output; `TestClaudeParseSumsEveryResult` — revert → a session's later turns are dropped from its usage, turns and permission denials, or its cumulative cost and modelUsage are summed across result events; `TestLongPromptCallsPricedAtTheLongRate` — revert → a call whose prompt is over the model's threshold is priced at the base rate, or a tiered model without per-call usage is priced at all.
**Baseline:** before=115 after=118 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 9956687 -->
**Commit:** `feat(metrics): price a call over a model's prompt threshold at its long-prompt rate`
**After:** Task 5, 6

Operator answer (2026-10-10): measure Haiku 5.5 (`--model claude-haiku-5-5`), not the Haiku 4.5
the CLI's `haiku` alias resolves to. measured: (platform.claude.com/docs/en/about-claude/pricing,
2026-10-10) Claude Haiku 5.5 costs $0.10 / $0.125 / $0.20 / $0.01 / $0.50 per MTok (input, 5m
cache write, 1h cache write, cache read, output) for prompts up to 100,000 tokens and $0.50 /
$0.625 / $1 / $0.05 / $2.50 over it; Sonnet 5.5 and Opus 5.5 are flat.

`harness.Usage` gains `Calls`: each model call's tokens, one per Anthropic message id (a message
streamed as several events counts once). A `pricing.json` entry may carry `long`: a prompt-token
threshold and a full set of prices; a call whose prompt (input + cache read + both cache writes)
is over the threshold is priced at them, every other token at the base prices. A tiered model with
no per-call usage (codex, or a transcript with no assistant event) is a pricing error, never a
guess. `pricing.json` gains `claude-haiku-5-5`.

Correction (2026-10-10), from the Task 22 review: claude-code's assistant events carry a call's
usage as of its stream's start, so a call's output is short of its final count; the trial runs the
CLI with `--include-partial-messages` and takes each call's final output from its `message_delta`
(keyed by `api_message_id`), its cache-write TTL split from its `message_start` or first assistant
event. measured: (claude CLI, 2026-10-10) a `message_delta` carries the final `output_tokens` but no
top-level TTL split. A session running several turns writes one `result` event per turn: their
usage, turn counts and permission denials are summed, and the last one's cumulative `modelUsage` and
cost are taken (`TestClaudeParseSumsEveryResult`).
Correction (2026-10-10), Step 3: the CLI's own cost cannot be the yardstick for Haiku 5.5.
measured: (review trial `kan-961`, `claude-haiku-5-5`, runner CLI 2.1.286, 2026-10-10) the CLI
reported $0.2682 with `"costBasis":"unknown"` in its `modelUsage` — it has no Haiku 5.5 price and
falls back to another rate; flow-eval priced the same usage (6 in, 9,615 out, 122,495 cache read,
36,893 1-hour write, no call over 100k) at $0.0134, which the published per-MTok prices reproduce
by hand.
  - [x] **Step 1: Write the failing tests.** Parse of a transcript with a message streamed as two
    events and one call over 100k; pricing of calls on both sides of the threshold and of a tiered
    model without calls.
  - [x] **Step 2: Implement** `Usage.Calls`, the `long` tier, the Haiku 5.5 entry.
  - [x] **Step 3: Verify** `go test ./... -count=1`, then a real Haiku 5.5 trial priced within a
    few percent of the CLI's own reported cost.

- [x] 23. Run the agent's own test classes by name where a case drops its module suite

**Build:** green
**Files:** `internal/grade/impl.go`, `internal/grade/impl_test.go`, `internal/grade/gradle.go`, `internal/mine/mine.go`, `docs/cases.md`
**Tests:** `TestGradeImplRunsTheAgentsOwnTestClasses`, `TestGradeImplOwnTestRunsSkip`
**Regression:** `TestGradeImplRunsTheAgentsOwnTestClasses` — revert → a test class the agent wrote in a module whose suite the case does not run is never run, so a failing one still passes the trial; or one in a module whose suite the case runs is run twice.; `TestGradeImplOwnTestRunsSkip` — revert → a helper declaring no test becomes a `--tests` selector Gradle fails, a suite run behind an environment prefix runs again by name, an agent-chosen module name reaches the shell, or a path grep cannot open errors the grading.
**Baseline:** before=118 after=120 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 22fc7da -->
**Commit:** `feat(grade): run the agent's own test classes by name in a module whose suite the case drops`
**After:** Task 21

Operator answer (2026-10-10): "Run agent's new tests" — the grader also runs, by name, every test
class the agent added or changed, and the whole `:shared:desktopTest` stays dropped from kan-867
and kan-868. measured: (round 6 panel, 2026-10-10) with a module's suite dropped from `verify`,
nothing runs a test the agent wrote itself in that module, so a failing one passes the trial —
kan-867, kan-868 (`:shared:desktopTest`) and kan-755 (`:app:test`).

After the case's `verify` commands, the grader takes every Kotlin or Java file the agent's diff
adds or modifies under a module's `src/test/`, `src/commonTest/` or `src/desktopTest/` that still
exists after the held-out reset and is a file declaring `@Test` (checked in the trial container
with `grep`, no shell), groups them by module and test task by the same rule `case add` derives its
selectors with (`commonTest` under `desktopTest`, project `:<directory name>`, a directory base
name outside `^[\w.-]+$` skipped), and for each module whose suite command `./gradlew <task>` ends
no `verify` command (an environment prefix such as `MANAGEMENT_HEALTH_REDIS_ENABLED=false` still
counts as the suite running) runs `./gradlew <task> --tests '<Class>' …` — logged, and failing the
trial, as a verify command is. The derivation moves from `internal/mine` to `internal/grade`,
which both call; the `@Test` content check is the grader's alone, so `case add` still derives its
selectors from the reference commit's held-out paths unchanged.

Correction (2026-10-10), Task 23 review: the plan ran every test-source file by name and matched a
suite only as the exact `verify` command `./gradlew <task>`; shipped runs only a file declaring
`@Test` (Gradle fails a `--tests` run that matches no test, so a fake or fixture failed a correct
trial), counts a suite as run when a `verify` command ends with `./gradlew <task>` (kan-863's
`MANAGEMENT_HEALTH_REDIS_ENABLED=false ./gradlew :gateway:test` re-ran the agent's gateway tests
without the environment), and skips a module directory whose base name is not `^[\w.-]+$`, so no
agent-chosen name reaches `sh -c`. Fixed in `fix(grade): run only files declaring a test, match a
suite behind an env prefix, refuse unsafe module names`.
`TestGradeImplOwnTestRunsSkip` guards the three, and a path grep cannot open (a dangling symlink
the agent left) declares no test rather than erroring the grading, fixed in `fix(grade): read past
a test-source path grep cannot open`. Panel round 7: `@Test` matched as a prefix selected a
`@TestConfiguration` helper (gymie's `AuthControllerTestConfig.kt` holds one); the grader's run
and its `tests_added` count now share one whole-annotation pattern, fixed in `fix(grade): match a
whole @Test annotation, one pattern for the count and the run`.

  - [x] **Step 1: Write the failing test** — a fake sandbox recording each command: an agent diff
    adding a test class in a module whose suite `verify` lacks, one in a module whose suite
    `verify` runs, and a held-out path; only the first is run, by name, and its non-zero exit
    fails the trial with its command as the reason.
  - [x] **Step 2: Implement** the run and move the derivation.
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; amend design §5's impl grading line.

- [x] 24. Read a judge's verdict from its last word when its first is not one

**Build:** green
**Files:** `internal/grade/judge.go`, `internal/grade/review_test.go`
**Tests:** `TestJudgeVerdict`
**Regression:** `TestJudgeVerdict` — revert → a judge that reasons before its one-word answer ("… different defects.\n\n**no**") fails the trial as unparsed instead of answering no.
**Baseline:** before=120 after=121 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ ecb5fb9 -->
**Commit:** `fix(grade): read a judge's yes or no from its last word when its first is neither`
**After:** Task 9

measured: (Haiku 5.5 visual run `20261010T085550Z`, kan-870 trial 3, 2026-10-10) the judge
answered "The two descriptions refer to different elements: a note field versus buttons. So these
are different defects.\n\n**no**"; the grader reads only the first word, so the trial failed with
"judge answered neither yes nor no" and scored 0 — a judge's formatting charged to the candidate.

The verdict is the answer's first word when it is yes or no, else its last word when that is;
anything else stays the error it is.

Correction (2026-10-10), Task 24 review: "the last word" read "The answer is neither yes nor no" as
no; the fallback now takes the last line's last word only where that line names one of yes and no.
Panel round 8: the first word won over the last line, so "Yes, both mention the note field, but
they describe different defects.\n\n**no**" read as yes; the last line is now read first, the first
word only where it names no answer.

  - [x] **Step 1: Write the failing test** — a table of answers: leading yes/no, a reasoned answer
    ending in a bold verdict, and one naming neither.
  - [x] **Step 2: Implement** `verdict(text)` in `judge.go`, used by `ask`.
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; re-run the Haiku 5.5 visual run.

- [x] 25. Paint every case and run row by its outcome

**Build:** green
**Files:** `internal/run/page.go`, `internal/run/index.go`, `internal/run/run.go`, `internal/run/run_test.go`, `internal/compare/compare.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `README.md`
**Tests:** `TestRowsPaintedByOutcome`
**Regression:** `TestRowsPaintedByOutcome` — revert → a case or run row carries no outcome colour, a row beating the role's baseline is not blue, or a row whose trials are all infra errors is coloured.
**Baseline:** before=121 after=122 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 62ffae1 -->
**Commit:** `feat(run): paint case and run rows by outcome, blue where they beat the baseline`
**After:** Task 20

Operator request (2026-10-10): "Paint rows green - OK, red - FAIL, yellow - Partial OK, blue -
Better than reference results". Operator answers (2026-10-10): both the run page's case rows and
the index's run rows; blue is "beats the baseline run" — the run `baselines/<role>.json` names,
the one `compare` reads; green, yellow and red count valid trials.

A case row is green when every valid trial passed, yellow when some did, red when none did, and
uncoloured when it has no valid trial; it is blue instead when the role has a baseline run, the
row's run is not that run, and the case's score is above the baseline run's score for the same
case. A run row on the index is green, yellow or red by the same rule over all its valid trials,
and blue when its role score is above its role's baseline run's. Each trial card on a run page
keeps its own pass, fail and infra marks. Every run page is re-rendered from its JSONL whenever
the index is written — at the end of each run and when `compare` records a baseline — so every
colour follows the current baseline, and pages written before this task gain theirs. Colours come
from the shared stylesheet's tokens in both light and dark schemes; a legend names them.

Correction (2026-10-10), panel round 10: a case or run row is blue only when its run is comparable
with the baseline run as `compare` defines it — the same role, slot, prompt pin, case set and k,
and k valid trials of every case on both sides; one predicate in `run` serves both. The pages are
repainted at the end of each run and on `baseline set`, not by `compare`. Another run's page that
cannot be repainted is a warning on stderr, never a failed run; the run's own page and the index
still fail it.

Correction (2026-10-10), Task 31 (decision `compare-warns-on-mixed-prompt-pins`) — narrows the
round-10 correction above: the prompt pin is no longer a comparability condition, so a differing pin
does not block the blue paint; the role, slot, case set and k still do.

  - [x] **Step 1: Write the failing test** — runs with all-pass, partly passing, all-fail and
    infra-only cases, and a baseline run one case score below: each case row and run row carries
    its colour class, the baseline run's own rows are not blue, and a role with no baseline has no
    blue row.
  - [x] **Step 2: Implement** the classes, the legend, the re-render on index write and on
    baseline record.
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; rebuild the index over the real `runs/`
    and look at it in light and dark schemes.

- [x] 26. A Muse Code harness

**Build:** green
**Files:** `internal/harness/muse.go`, `internal/harness/harness_test.go`, `internal/harness/testdata/muse-exec.jsonl`, `internal/sandbox/sandbox.go`, `internal/sandbox/sandbox_test.go`, `internal/run/run.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `cmd/egress-proxy/main_test.go`, `internal/metrics/metrics_test.go`, `docker/egress-allowlist.txt`, `docker/runner.Dockerfile`, `pricing.json`, `README.md`
**Tests:** `TestMuseCommandArgs`, `TestMuseParse`
**Regression:** `TestMuseCommandArgs` — revert → a muse trial runs without its pinned flags (headless JSON, no approval prompts, its own session id) or without its session logs on stdout; `TestMuseParse` — revert → a muse transcript's tokens, calls, tool calls, final text or served model are lost or double-counted across its subagent sessions.
**Baseline:** before=128 after=130 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ d3de28e and @ 23ce395 -->
**Commit:** `feat(harness): run and price Meta's Muse Code CLI as a candidate harness`
**After:** Task 5, 22

Operator request (2026-10-10): support Muse Code and run it on muse-spark-1.3.
measured: (Muse Code 1.4.4-R5419.1 on this Mac, 2026-10-10)
- `muse exec --json` writes MSP records (`payload_type` `run.output.delta`,
  `run.terminal.completed`, `tool.result`, `task.lifecycle.*`) and no token usage; usage lives
  only in the session log `$XDG_DATA_HOME/muse/sessions/<y>/<m>/<d>/<session-id>/session.jsonl`
  and in each `subagent/<id>/session.jsonl` beside it, as `runtime.session` records whose
  `payload.event.kind` is `model_completed` with `usage` {`input_tokens`, `output_tokens`,
  `cached_tokens`, `cache_write_tokens`, `cache_read_tokens`, `reasoning_tokens`} and `model`.
  A two-tool-call prompt wrote one main session and two subagent sessions.
- The host's account login cannot be carried into a Linux container; `META_API_KEY` takes
  priority over a login.
- The launcher (`https://api.meta.ai/muse-launcher.sh`, installed by `https://dev.meta.ai/install.sh`
  into `$MUSE_INSTALL_DIR`) downloads a 353 MB per-platform binary (Linux arm64 and x86_64
  included) only after a login, and self-updates.
- (dev.meta.ai/docs/pricing-rate-limits, 2026-10-10) `muse-spark-1.3`: $0.15 cached
  input, $1.25 input, $4.25 output per 1M tokens; no long-context premium; no cache-write price.

Design. A one-time setup, run by the operator (an interactive device-code login), fills a Docker
volume `flow-eval-muse`: the launcher and the binary under `/muse/bin`, the Linux login under
`/muse/config` (`XDG_CONFIG_HOME`), via `docker run -it --rm -v flow-eval-muse:/muse
flow-eval-runner …` — the exact commands in the README. A `muse-code` trial mounts that volume
(the binary read-only; the login where the CLI can refresh it) and runs the pinned binary
directly — never the launcher, so no trial self-updates or downloads — headless:
`exec --json --model <m> --reasoning-effort <e> --approval-mode never --approval-judge off
--session-id <uuid> --workspace <workdir> --prompt-file <prompt>`, with `XDG_DATA_HOME` in the
trial's own home; after it exits, the command appends every session log of that session id (main
then subagents) to stdout, so `Parse` reads one stream. `Parse` takes the final text from
`run.terminal.completed`, tool calls from the MSP tool records, and one `Call` per
`model_completed` across main and subagent sessions; token fields map onto `Usage` by what a real
multi-turn transcript shows (whether `cached_tokens` is inside `input_tokens`, `reasoning_tokens`
inside `output_tokens`) — measured, never assumed, and stated in a code comment. The served model
is `model_completed.model`. No reported cost (the CLI gives none). `pricing.json` gains
`muse-spark-1.3`. The egress allowlist gains exactly the Meta hosts a real trial
needs, found from the proxy's denials. A missing volume or login fails the trial as an infra
error naming the setup command.

measured: (muse-spark-1.3, effort low, a five-tool-call prompt with one failing read, 2026-10-10;
the trimmed stream is `internal/harness/testdata/muse-exec.jsonl`)
- `cached_tokens` is inside `input_tokens` (successive calls 33226, 33510, 33827 input with 0,
  32625, 33393 cached: the prompt grows by one tool round, never doubles), and
  `cache_read_tokens` repeats `cached_tokens`; `reasoning_tokens` is inside `output_tokens` (a
  final one-sentence answer: 48 output, 19 reasoning); `cache_write_tokens` was 0 in every call.
  The evidence is restated in `Muse.Parse`'s comment.
- Both subagent sessions were memory-reminder children with their own `model_completed` calls; a
  `tool.result` carries `correlation_facts` {`tool_name`, `outcome` `success`|`failure`}, and
  `edit_facts` when it edited a file.
- On Linux the CLI stores its login in `$XDG_CONFIG_HOME/muse/auth.json`, the file the launcher
  reads.

Correction (2026-10-10): Files names each file, not the globs `muse-*.jsonl` and
`internal/sandbox/*.go` the commit-fields guard cannot match, and also `internal/run/run.go` (the
harness's mounts reach the trial's `TrialSpec` through `run.Config`), `docker/runner.Dockerfile` (Debian's `curl`, which the
launcher downloads with during the setup), and the two existing tests that pin the shipped
allowlist and prices, `cmd/egress-proxy/main_test.go` and `internal/metrics/metrics_test.go`.
Correction (2026-10-10): a volume that does not exist cannot fail the trial command: docker
creates it empty and then refuses the `volume-subpath` mount before the container starts
(measured: "cannot access path …/flow-eval-muse/_data/bin: no such file or directory"). So `run`
checks the volume exists before any trial and exits 2 naming the setup; a volume lacking the
binary or the login is the trial infra error naming the setup.
Correction (2026-10-10): the allowlist gains `api.meta.ai` (the CLI's API base URL) and
`auth.meta.com` (the launcher's sign-in host, for token refresh), read from the launcher and the
CLI before any trial; hosts a real trial's proxy denials show are added after it.
Correction (2026-10-10): the setup signs in twice — the launcher holds its own device login in
memory only to download the binary, and `muse login` writes the CLI's login; `pricing.json`
gives `muse-spark-1.3` its input price as `cache_write`, since the pricing page names none and
`LoadPricing` requires every price.
Correction (2026-10-10), Task 26 review: the trial's config home is no longer the volume's login.
A shared, writable `XDG_CONFIG_HOME` let a file one trial's agent or CLI wrote there reach every
later trial (operator decision (a)). The login mounts read-only at `/muse/login`; the trial command
copies only `muse/auth.json` into a fresh `XDG_CONFIG_HOME` in the trial's own home before muse
runs, so nothing a trial writes reaches another, and a token the CLI refreshes is not saved back —
a refresh token Meta rotates means re-running the setup's login step. The sandbox accepts a harness
mount only when it holds exactly one `type=` key, `volume`, read as docker reads it (one CSV
record, keys case-insensitive), so `type=volume,type=bind,…` is refused. `run`'s preflight also runs
the runner image with the harness's mounts and `--network none` and tests the binary and the login
paths, exiting 2 naming the setup when either is missing — superseding "a volume lacking the binary
or the login is the trial infra error naming the setup" above. New tests:
`TestStartRefusesABindMountDressedAsAVolume`, and `TestMuseVolumeCheckedAndMountedIntoTrials`
(the preflight probe, and the mounts reaching the trial's `docker run` from the harness registry
through `run.Config`); `TestMuseCommandArgs` now asserts the copied login and that a file muse
writes to its config home stays out of the login.
Correction (2026-10-10), panel round 11:
- **muse's own sandbox is off.** The pinned argv gains `--disable-sandbox`. muse's shell sandbox is
  on by default. It wraps each shell call in bubblewrap, which needs a user namespace that an
  unprivileged trial container cannot create. With the sandbox on, every `bash` call the agent
  made in the first real muse-code trials failed, and each escalation was denied under
  `--approval-mode never`. The container is the sandbox, as it is for the other two harnesses.
  `--yolo` is not used, because it also trusts the workspace and loads its skills and rules.
  Reminders and web tools stay at the CLI's defaults (the CLI as shipped). `TestMuseCommandArgs`
  pins the flag. Only a real trial can confirm that the shell then works; that check stays in
  Step 3.
- **A denied call is a permission denial.** muse writes a denied call's `tool.result` with no
  `correlation_facts`, only `text` "tool denied: …". `Parse` now counts such a record in
  `PermissionDenials`, not in `ToolErrors`. It names the call from the session log's
  `assistant_tool_calls_committed` record with the same `call_id`. Because those logs follow stdout,
  tool calls are counted, in stdout's order, once the whole transcript is read. A denied call with
  no committed record stays out of `ToolCalls`. `internal/harness/testdata/muse-denied.jsonl` holds
  real records from a muse-code visual trial: its stdout tool and terminal records, and the main
  session's committed records for those calls. New tests:
  `TestMuseParseCountsADeniedCallAsADenialUnderItsName`, and
  `TestMuseFirstEditAtCountsADeniedCallBeforeIt`.
- **Only allow-listed mount keys.** A harness mount may carry only the keys a named volume needs:
  `type`, `src`/`source`, `dst`/`destination`/`target`, `volume-subpath`, `readonly`/`ro` and
  `volume-nocopy`, case-insensitive. The local driver's `volume-driver` and `volume-opt` keys bind
  a Docker-host path through a volume, so they are refused. `TestStartRefusesABindMountDressedAsAVolume`
  gains that case.
- **The preflight never pulls.** Its `docker run` carries `--pull never`. Exit 2 is the ready
  check's own, and only that exit names the volume setup. Any other failure, such as a runner image
  that is not built (docker exits 125), names the image. `TestMuseVolumeCheckedAndMountedIntoTrials`
  covers both.
- **One copy of the muse layout.** The mount destinations and their environment are constants that
  build the mounts, the trial's environment and the preflight. The preflight runs the trial
  wrapper's own check, not a second copy. `MuseMounts` is now a function that returns a fresh slice,
  so no caller can change another trial's mounts (`TestMuseMountsAreAFreshCopy`).
- **Comments match the code.** The `Harness` contract names all three harnesses and allows a prompt
  delivered by its path and a config home holding a copied login. `run.go`'s `_promptFile` comment
  no longer says "stdin".
- **design.md covers muse-code.** §2 (Network, Credentials, Harness configuration), "Harness
  adapters", and the decisions `harnesses-v1` and `isolation-container-per-trial` now state that
  its login arrives through the read-only volume, not the environment.

  - [x] **Step 1: Write the failing tests** — command args, and parse of a real (trimmed) muse
    stdout+session-log stream with a subagent session.
  - [x] **Step 2: Implement** the adapter, the volume mount, the pricing entry, the README setup.
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; the operator's setup; one real
    `muse-code` trial; then impl, review and visual at k=3, effort low, on the three cases.

- [x] 27. Grade a review trial on the must-finds its own slot raised

**Build:** green
**Files:** `internal/mine/mine.go`, `internal/mine/mine_test.go`, `internal/grade/review.go`, `internal/cases/case.go`, `internal/cases/case_test.go`, `internal/cases/testdata/valid/review/r1/labels.json`, `internal/run/run.go`, `internal/run/run_test.go`, `docs/cases.md`
**Tests:** `TestLabelsCarryTheirSlot`, `TestReviewGradedOnItsSlotsMustFinds`
**Regression:** `TestLabelsCarryTheirSlot` — revert → a label loses the panel slot that raised it, or a finding another slot's must-find duplicates is ignored instead of becoming its own slot's must-find; `TestReviewGradedOnItsSlotsMustFinds` — revert → a `--slot` trial is graded on must-finds other slots raised, or a case with none for the slot runs and scores 0.
**Baseline:** before=135 after=137 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 33f8d3b and @ 062620f -->
**Commit:** `fix(grade): grade a review trial on the must-finds its own slot raised`
**After:** Task 9, 15

Operator request (2026-10-10): fix, do not re-run trials. measured: (review runs of sonnet,
claude-haiku-5-5 and muse-spark-1.3 on `kan-867`, 2026-10-10) all nine trials scored 0 with recall
0; the case's must-finds F6 (principles) and F10, F11, F12 (mutation) were raised by slots other
than the `primary` slot the trials run, and primary's own finding F1 is `ignored` as a duplicate
of F6 whose site lies outside the diff's hunks — so a primary trial has no must-find it can match.

Each label carries the slot that raised it, from `panel.md`'s Slot column. A `--slot X` review trial
is graded only on the must-finds slot X raised. A finding that duplicates another slot's must-find
is a must-find of its own slot, not ignored, wherever its cited line lies. A run whose case set
holds a review case with no must-find for the chosen slot is refused before any trial, naming the
case. The six review cases are re-mined.

Correction (2026-10-10): the refusal lives in `run.Run`, before any snapshot or trial, and `runCmd`
already exits 2 on it, so `cmd/flow-eval/main.go` is unchanged. `TestReviewGradedOnItsSlotsMustFinds`
is in `internal/run/run_test.go`, not `internal/grade/review_test.go`: it drives `Run` end to end,
so it covers the per-slot grading and the refusal together. `GradeReview` is unchanged; the run
hands it the slot's must-finds. Every known-false label applies to every slot's trial, whichever
slot raised it. A review case whose labels carry no `slot` (mined before labels carried one) fails
`case validate`, and so `run`, with one problem naming those labels and saying to re-mine the case
(a subtest of `TestRejectInvalidFields`). Files names each file the task commit changes. The six
re-mined `cases/review/<id>/labels.json` land in a later commit of the operator's, because a run
was reading `cases/` at the time. The re-mined labels keep every hand edit of the seeded ones but
one kind: a cross-slot duplicate the seeded labels moved to `ignored` is now its own slot's
must-find (kan-753 F5 principles, kan-757 F4 principles, kan-867 F1 primary and F9 failure-modes,
the last two at the panel's line 295). kan-755-offline-l4-be-create-endpoints-accept-a-client has
no primary must-find, so a `--slot primary` run refuses it.

Correction (2026-10-10), Task 27 review: `TestReviewGradedOnItsSlotsMustFinds` pins that a known-false
label another slot raised counts against a primary trial (`known_false_hits` 1), and a third subtest
pins that a `--slot` with no `prompts/review-<slot>.md` fails first, naming the missing prompt, before
the no-must-find refusal. A subtest of `TestRejectInvalidFields` pins that a `knownFalse` or `ignored`
label without a slot fails `case validate` too. The re-mined `cases/review/*/labels.json` are placed
(`3edbc91`) and `case validate` reports 25 cases valid.

Correction (2026-10-10), panel round 13: README's review example passes `--cases` with the five
review cases that have a primary must-find, leaving out kan-755. `prompts.Vendored` matches a slot
to its prompt by listing `prompts/` and comparing names exactly, so a mis-cased `--slot` is refused
for its missing prompt on a case-insensitive filesystem too; `prompts` owns the prompt path and
`run` builds `review-<slot>` in one place. The no-must-find refusal names every offending case in
one error. Both are pinned by subtests of `TestReviewGradedOnItsSlotsMustFinds`.

  - [x] **Step 1: Write the failing tests** — labels from a panel with findings by several slots,
    a cross-slot duplicate and a site outside the hunks; grading a primary trial on a mixed label
    set; a run refused for a case with no primary must-find.
  - [x] **Step 2: Implement** the slot field, the per-slot grading, the run-start refusal.
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; re-mine the six review cases and
    `flow-eval case validate`.

- [x] 28. Each run's trial wall-clock range on the index

**Build:** green
**Files:** `internal/run/index.go`, `internal/run/run_test.go`
**Tests:** `TestIndexShowsTrialWallRange`
**Regression:** `TestIndexShowsTrialWallRange` — revert → the index's run rows lose the min, average and max trial wall-clock, or count an infra-error trial in them.
**Baseline:** before=137 after=138 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ b91ab86 -->
**Commit:** `feat(run): show each run's min, average and max trial wall-clock on the index`
**After:** Task 20

Operator request (2026-10-10): "Add min/max/avg run wall-clock to the \"flow-eval runs\" tables."
Each run row of the index gains three columns — the shortest, mean and longest wall-clock of its
valid trials, in seconds to one decimal — as the case rows' mean wall already counts valid trials
only; a run with no valid trial shows "–" in each.

  - [x] **Step 1: Write the failing test** — a run with three valid trials and one infra error.
  - [x] **Step 2: Implement** the three columns.
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; repaint the real index.

- [x] 29. Judge with sonnet at medium effort

**Build:** green
**Files:** `judge.json`, `README.md`, `internal/grade/review_test.go`
**Tests:** none — a pin change; `TestJudgeCostSeparate` already loads the repository's pin
**Regression:** none — reverting puts back the opus judge
**Baseline:** before=138 after=138 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 7bf4b0b -->
**Commit:** `chore(grade): judge with sonnet at medium effort`
**After:** Task 9

Operator request (2026-10-10): "use sonnet-medium as judge". measured: (review run
`20261010T122505Z`, opus-high, kan-867, 2026-10-10) the opus/medium judge cost $3.79 over three
trials against the candidate's $2.14. `judge.json` pins `claude-code`, `sonnet`, `medium`; runs
judged before this change are not re-judged.

  - [x] **Step 1: Pin** the judge and update its mentions.
  - [x] **Step 2: Verify** `go test ./... -race -count=1`.

- [x] 30. Record the judge a run was graded with, and warn when a comparison mixes judges

**Build:** green
**Files:** `internal/grade/judge.go`, `internal/run/run.go`, `internal/run/page.go`, `internal/run/index.go`, `internal/run/run_test.go`, `internal/compare/compare.go`, `internal/compare/compare_test.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `README.md`
**Tests:** `TestRunHeaderRecordsItsJudge`, `TestCompareWarnsOnMixedJudges`
**Regression:** `TestRunHeaderRecordsItsJudge` — revert → a review or visual run's header, page or index row does not name its judge's harness, model and effort; `TestCompareWarnsOnMixedJudges` — revert → comparing runs graded by different (or unrecorded) judges says nothing, or refuses.
**Baseline:** before=138 after=140 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ bf05e39 and @ ef1ae62 -->
**Commit:** `feat(run): record the judge a run was graded with, warn when a comparison mixes judges`
**After:** Task 29

Operator request (2026-10-10): add the judge pin to the run header, show it on the run page and
index; "except \"and have compare refuse to compare runs that used different judges\" just show a
warning somewhere it is enough". measured: (run `20261010T122505Z`, 2026-10-10) the header carries
no judge field; only each trial's judge usage names the served model, and the effort is recorded
nowhere.

A review or visual run's header records the judge pin it ran with — harness, model, effort — and an
impl run, which calls no judge, records none. The run page's header and the index's run rows show
it ("–" where a run records none, as every run before this task does). `compare` still compares
runs graded by different judges, and runs where either records none, but prints a warning naming
both judges on stderr and in its report; its exit code is unchanged.

  - [x] **Step 1: Write the failing tests** — a review run's header, page and index row carry the
    pin; an impl run's carry none; compare of two runs with different judges (and one with none)
    warns and keeps its verdict and exit code.
  - [x] **Step 2: Implement** the header field, the page and index cells, the warning.
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; repaint the real index.

- [x] 31. Vendor the principles and failure-modes reviewer prompts; warn, not refuse, on a prompt-pin mismatch

**Build:** green
**Files:** `internal/prompts/prompts.go`, `internal/prompts/prompts_test.go`, `prompts/strip.txt`, `prompts/PIN`, `prompts/review-primary.md`, `prompts/review-principles.md`, `prompts/review-failure-modes.md`, `internal/compare/compare.go`, `internal/compare/compare_test.go`, `internal/run/index.go`, `internal/run/run_test.go`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `README.md`
**Tests:** `TestSyncVendorsEveryReviewSlot`, `TestCompareWarnsOnMixedPromptPins`
**Regression:** `TestSyncVendorsEveryReviewSlot` — revert → `prompts sync` writes no `review-principles.md` or `review-failure-modes.md`, so `run --role review --slot principles|failure-modes` refuses; `TestCompareWarnsOnMixedPromptPins` — revert → comparing runs under different prompt pins exits 2 instead of warning.
**Baseline:** before=140 after=142 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ ff6a3e4 and @ e3f3357 -->
**Commit:** `feat(prompts): vendor the principles and failure-modes reviewer prompts, warn on mixed prompt pins`
**After:** Task 30

Operator request (2026-10-10): "add principles and failure-modes, sonnet-low. Allow compare for
different pin but show a warning. Before run check that agents tooling is up to date and then run".
measured: (2026-10-10) flow's panel roster is `primary`, `principles`, `failure-modes`, `mutation`
(`skills/flow/review-panel.md`, **The roster**); flow-eval vendors only `review-primary`, so
`run --role review` accepts `--slot primary` alone. The review labels already carry each must-find's
slot: principles has must-finds in 4 of the 6 review cases (not kan-527 or kan-961), failure-modes in 1
(kan-867). Mutation writes sabotage tests in a throwaway worktree rather than a findings list, and
stays out of scope.

`prompts sync` also vendors `skills/flow/principles-reviewer-prompt.md` as `prompts/review-principles.md`
and `skills/flow/failure-modes-reviewer-prompt.md` as `prompts/review-failure-modes.md`, each cut as
`review-primary` is: the fenced prompt, its sections that need flow's change artifacts, diff file or
context bundle removed per `prompts/strip.txt`, and the diff appended inline. The principles slot's
`skills/flow/engineering-principles.md` is vendored whole after its body, as `reviewer-calibration.md`
is for primary; a case supplies no project standards or global constraints. Each new slot's output
contract matches what `ParseReview` already reads for primary, so the existing per-slot grading
(Task 27) scores it unchanged.

`compare` no longer refuses runs whose prompt pins differ: it compares them and prints
`warning: prompt pin <cand> vs <base> — DIFFER: prompts may have changed between the runs` on stderr
and in its report, with the verdict and exit code the pass rule's alone — the same treatment as the
flow-eval sha and the judge. The index's blue "beats the baseline" paint follows `compare`, so a
differing pin no longer blocks it. Different case sets, k or slots still refuse with exit 2.

  - [x] **Step 1: Write the failing tests** — sync at a fixture agents commit writes both new
    prompts with their stripped sections gone and the diff slot present; compare of two runs under
    different pins warns and keeps the pass rule's exit code.
  - [x] **Step 2: Implement** the two targets and their strip entries, the compare warning, and
    update the README and design.md's pin paragraph and decision.
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; `prompts sync` at the agents HEAD and
    `go run ./cmd/flow-eval run --role review --slot principles --dry-run` (or the nearest
    no-trial check the CLI offers) renders each new prompt.

- [x] 32. Pass the principles reviewer its project's standards, resolved from the case snapshot's `.flow/project.md`

**Build:** green
**Files:** `internal/prompts/prompts.go`, `internal/prompts/prompts_test.go`, `prompts/strip.txt`, `prompts/review-principles.md`, `prompts/rules/be-brief.mdc`, `prompts/rules/build-the-simplest-thing.mdc`, `prompts/rules/commit-scope-is-the-module.mdc`, `prompts/rules/context7.mdc`, `prompts/rules/dependency-versions.mdc`, `prompts/rules/design-mockups-are-specs.mdc`, `prompts/rules/dispatch-carries-the-baseline.mdc`, `prompts/rules/fix-determinism-at-the-source.mdc`, `prompts/rules/flow-manual-review.mdc`, `prompts/rules/kotlin-backend-development-standard.mdc`, `prompts/rules/lint-fix-priority.mdc`, `prompts/rules/never-touch-production.mdc`, `internal/run/run.go`, `internal/run/run_test.go`, `internal/sandbox/sandbox.go`, `README.md`
**Tests:** `TestPrinciplesGetsTheSnapshotStandards`, `TestStandardsResolvesPerFlowContainment`, `TestStandardsMatchesTheHeadingAsFlowDoes`, `TestStandardsResolvesAnAbsoluteRuleFromARelativePromptsDir`
**Regression:** `TestPrinciplesGetsTheSnapshotStandards` — revert → a principles trial on a case whose snapshot declares `## standards` gets no standards (or a frontend case with none gets some), unlike flow's principles slot. `TestStandardsResolvesPerFlowContainment` — revert → an entry escaping its root, naming a directory, a symlink out of a bare name's root or a wrong-case name resolves, or a symlinked `.flow/project.md` or a second `## standards` is read. `TestStandardsMatchesTheHeadingAsFlowDoes` — revert → a `## standards ` heading with a trailing space declares a section and a BOM-prefixed `project.md` declares none, unlike flow's `project-get`. `TestStandardsResolvesAnAbsoluteRuleFromARelativePromptsDir` — revert → an absolute entry naming a vendored rule is dropped under the CLI default relative `--prompts prompts`.
**Baseline:** before=142 after=146 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ 504ba0e and @ efdf81b -->
**Commit:** `feat(prompts): pass the principles reviewer its project's standards, as flow does`
**After:** Task 31

Operator request (2026-10-10): "add it" — the principles reviewer gets its project's standards the
way flow passes them, instead of the universal principles alone (which Task 31's review cut, D1).
measured: (2026-10-10) flow fills the principles prompt's `[STANDARDS_PATHS]` from the project's
`.flow/project.md` `## standards` (`skills/flow/review-panel.md`, "Resolve `[STANDARDS_PATHS]`";
`skills/flow-contracts/project-configuration-standards.md`), empty when none resolve. At the review
cases' snapshots: gymie (kan-755, kan-757, kan-961) declares `CLAUDE.md`, `CONTRIBUTING.md` and
`kotlin-backend-development-standard.mdc` (an agents `rules/` file); gymie-frontend (kan-527, kan-753,
kan-867) declares none.

A principles trial's standards derive from the case snapshot's own `.flow/project.md` — nothing
is stored in the case, so case hashes and the case-set hash do not change — resolved per flow's
three entry forms and containment rules: a bare `*.mdc` name to that file in the agents `rules/`
at `prompts/PIN` (vendored at the pin, never read from the live checkout), any other bare name to
the project root, a path relative to the project root, and anything escaping either root, or
resolving to no file, dropped and named in the trial's record. The resolved files are readable in
the trial container at absolute paths — project files at their `/work/...` path, agents rules
outside `/work` — and the vendored principles prompt keeps upstream's `**Project standards:**` line
and its standards-are-data and no-standards-resolved paragraphs, filled per trial: the paths when
any resolve, empty otherwise, exactly as flow does. Task 31's D1 cuts to the intro, the Critical
calibration's hard-invariant clause and the Summary are reverted to upstream; `## Hard Invariants`
stays stripped only if it needs flow artifacts a case lacks, and the reason is recorded in
`prompts/strip.txt`. `[GLOBAL_CONSTRAINTS]` stays out: a review case carries no change artifacts.

The `## standards` section is read as flow's `project-get` reads it (design.md §4).

`Strip` gains its whole-line `~` clause rule (design.md §4) here, because cutting the principles
prompt's whole `**Global constraints:**` line otherwise glued the next paragraph to
`**Project standards:**`. Operator-confirmed intended (2026-10-10); `prompts sync` at the PIN drifts
no other vendored prompt.

  - [x] **Step 1: Write the failing test** — a principles prompt rendered for a gymie case lists
    the three resolved standards (the `.mdc` at its in-container path, its content the agents file
    at the pin), and one for a gymie-frontend case passes none and keeps the no-standards
    paragraph; an escaping entry is dropped.
  - [x] **Step 2: Implement** the resolution, the in-container placement, the prompt fill, the
    strip.txt changes; update README and design.md (§4 and decision `review-three-slots`).
  - [x] **Step 3: Verify** `go test ./... -race -count=1`; `prompts sync` at the PIN reproduces the
    committed prompts.

- [x] 33. Keep one case per role, and make the README brief

**Build:** green
**Files:** `README.md`, `cases/impl/kan-755-offline-l4-be-create-endpoints-accept-a-client-task-5/case.json`, `cases/impl/kan-755-offline-l4-be-create-endpoints-accept-a-client-task-5/context.md`, `cases/impl/kan-755-offline-l4-be-create-endpoints-accept-a-client-task-5/heldout.patch`, `cases/impl/kan-755-offline-l4-be-create-endpoints-accept-a-client-task-5/reference.stat`, `cases/impl/kan-755-offline-l4-be-create-endpoints-accept-a-client-task-5/task.md`, `cases/impl/kan-863-security-prod-actuator-prometheus-and-actuator-task-1/case.json`, `cases/impl/kan-863-security-prod-actuator-prometheus-and-actuator-task-1/context.md`, `cases/impl/kan-863-security-prod-actuator-prometheus-and-actuator-task-1/heldout.patch`, `cases/impl/kan-863-security-prod-actuator-prometheus-and-actuator-task-1/reference.stat`, `cases/impl/kan-863-security-prod-actuator-prometheus-and-actuator-task-1/task.md`, `cases/impl/kan-867-qm-2-step-1-frontend-settings-workout-mode-focus-task-1/case.json`, `cases/impl/kan-867-qm-2-step-1-frontend-settings-workout-mode-focus-task-1/context.md`, `cases/impl/kan-867-qm-2-step-1-frontend-settings-workout-mode-focus-task-1/heldout.patch`, `cases/impl/kan-867-qm-2-step-1-frontend-settings-workout-mode-focus-task-1/reference.stat`, `cases/impl/kan-867-qm-2-step-1-frontend-settings-workout-mode-focus-task-1/task.md`, `cases/impl/kan-868-qm-3-step-2-frontend-quick-runner-finished-view-task-9/case.json`, `cases/impl/kan-868-qm-3-step-2-frontend-quick-runner-finished-view-task-9/context.md`, `cases/impl/kan-868-qm-3-step-2-frontend-quick-runner-finished-view-task-9/heldout.patch`, `cases/impl/kan-868-qm-3-step-2-frontend-quick-runner-finished-view-task-9/reference.stat`, `cases/impl/kan-868-qm-3-step-2-frontend-quick-runner-finished-view-task-9/task.md`, `cases/impl/kan-961-api-metrics-dashboard-task-2/case.json`, `cases/impl/kan-961-api-metrics-dashboard-task-2/context.md`, `cases/impl/kan-961-api-metrics-dashboard-task-2/heldout.patch`, `cases/impl/kan-961-api-metrics-dashboard-task-2/reference.stat`, `cases/impl/kan-961-api-metrics-dashboard-task-2/task.md`, `cases/review/kan-527-group-workouts-body-fixes-2/case.json`, `cases/review/kan-527-group-workouts-body-fixes-2/diff.patch`, `cases/review/kan-527-group-workouts-body-fixes-2/labels.json`, `cases/review/kan-753-offline-l2-own-data-screens-read-only-offline/case.json`, `cases/review/kan-753-offline-l2-own-data-screens-read-only-offline/diff.patch`, `cases/review/kan-753-offline-l2-own-data-screens-read-only-offline/labels.json`, `cases/review/kan-755-offline-l4-be-create-endpoints-accept-a-client/case.json`, `cases/review/kan-755-offline-l4-be-create-endpoints-accept-a-client/diff.patch`, `cases/review/kan-755-offline-l4-be-create-endpoints-accept-a-client/labels.json`, `cases/review/kan-757-offline-l5-be-per-item-if-match-on-group-session/case.json`, `cases/review/kan-757-offline-l5-be-per-item-if-match-on-group-session/diff.patch`, `cases/review/kan-757-offline-l5-be-per-item-if-match-on-group-session/labels.json`, `cases/review/kan-961-api-metrics-dashboard/case.json`, `cases/review/kan-961-api-metrics-dashboard/diff.patch`, `cases/review/kan-961-api-metrics-dashboard/labels.json`, `cases/visual/kan-753-off-l2-3/capture.png`, `cases/visual/kan-753-off-l2-3/case.json`, `cases/visual/kan-753-off-l2-3/labels.json`, `cases/visual/kan-753-off-l2-3/mockup.png`, `cases/visual/kan-754-off-l1-1a/capture.png`, `cases/visual/kan-754-off-l1-1a/case.json`, `cases/visual/kan-754-off-l1-1a/labels.json`, `cases/visual/kan-754-off-l1-1a/mockup.png`, `cases/visual/kan-754-off-l1-2b/capture.png`, `cases/visual/kan-754-off-l1-2b/case.json`, `cases/visual/kan-754-off-l1-2b/labels.json`, `cases/visual/kan-754-off-l1-2b/mockup.png`, `cases/visual/kan-754-off-l3-11/capture.png`, `cases/visual/kan-754-off-l3-11/case.json`, `cases/visual/kan-754-off-l3-11/labels.json`, `cases/visual/kan-754-off-l3-11/mockup.png`, `cases/visual/kan-756-off-l4-2b/capture.png`, `cases/visual/kan-756-off-l4-2b/case.json`, `cases/visual/kan-756-off-l4-2b/labels.json`, `cases/visual/kan-756-off-l4-2b/mockup.png`, `cases/visual/kan-756-off-l4-4/capture.png`, `cases/visual/kan-756-off-l4-4/case.json`, `cases/visual/kan-756-off-l4-4/labels.json`, `cases/visual/kan-756-off-l4-4/mockup.png`, `cases/visual/kan-847-x2/capture.png`, `cases/visual/kan-847-x2/case.json`, `cases/visual/kan-847-x2/labels.json`, `cases/visual/kan-847-x2/mockup.png`, `cases/visual/kan-868-qm-q3/capture.png`, `cases/visual/kan-868-qm-q3/case.json`, `cases/visual/kan-868-qm-q3/labels.json`, `cases/visual/kan-868-qm-q3/mockup.png`, `cases/visual/kan-868-qm-q5/capture.png`, `cases/visual/kan-868-qm-q5/case.json`, `cases/visual/kan-868-qm-q5/labels.json`, `cases/visual/kan-868-qm-q5/mockup.png`, `cases/visual/kan-870-qm-f1/capture.png`, `cases/visual/kan-870-qm-f1/case.json`, `cases/visual/kan-870-qm-f1/labels.json`, `cases/visual/kan-870-qm-f1/mockup.png`, `cases/visual/kan-870-qm-q11/capture.png`, `cases/visual/kan-870-qm-q11/case.json`, `cases/visual/kan-870-qm-q11/labels.json`, `cases/visual/kan-870-qm-q11/mockup.png`, `cases/visual/kan-870-qm-q8/capture.png`, `cases/visual/kan-870-qm-q8/case.json`, `cases/visual/kan-870-qm-q8/labels.json`, `cases/visual/kan-870-qm-q8/mockup.png`, `cmd/flow-eval/main.go`, `cmd/flow-eval/main_test.go`, `docker/runner.Dockerfile`, `docs/cases.md`, `docs/muse-code.md`, `internal/harness/harness_test.go`, `internal/harness/muse.go`
**Tests:** none
**Regression:** none — case data and docs.
**Baseline:** before=146 after=146 <!-- measured: git grep -h '^func Test' -- '*_test.go' | wc -l | tr -d ' ' @ a34b00a and @ 695d715 -->
**Commit:** `chore(cases): keep one case per role`
**After:** Task 32

Operator request (2026-10-10): "Lets cleanup impl, review and visual-verify cases. Leave only one for
each that was used. Cleanup all runs. Update judge to haiku-low. Do clean re-run for haiku-low and
sonnet-low. Make README.md brief and easy to read and understand". measured: (2026-10-10) the runs
used `kan-757-offline-l5-be-per-item-if-match-on-group-session-task-3` (impl),
`kan-867-qm-2-step-1-frontend-settings-workout-mode-focus` (review — must-finds for primary,
principles and failure-modes) and `kan-870-qm-q6` (visual); the set holds 6 impl, 6 review and 13
visual cases.

Every other case directory is deleted, with anything that names a deleted case only to exercise the
set (docs, examples) updated; tests keep their own fixtures. The README is rewritten brief: what
flow-eval is, setup, the commands with one example each, how to read results; detail that design.md
or `docs/` states canonically is linked, not restated. The untracked `runs/` content is deleted outside this commit, and the re-runs
recorded in live-verification.md.

Operator (2026-10-10): don't change judge — the sonnet/medium pin stays.

The Muse Code setup moves verbatim from the README to `docs/muse-code.md`, and the code and
Dockerfile pointers at it follow, so the README stays brief.

  - [x] **Step 1: Delete** every case directory but the three kept, and update what names a deleted
    case.
  - [x] **Step 2: Implement** the brief README.
  - [x] **Step 3: Verify** `flow-eval case validate` passes on the three remaining cases; `go test
    ./... -race -count=1`.
