# myflow project configuration — gymie

Read by globally installed myflow skills. Every key is optional; anything absent is
auto-detected from the repository instead.

## apps

Absolute roots are the **main checkout**. When an apply worktree exists for the change,
resolve each app's root from `git worktree list` in that repo and use the worktree root
instead — never a relative sibling path, never the main checkout while a worktree holds
uncommitted work.

| App | Repo root | Kind | URL | Notes |
|-----|-----------|------|-----|-------|
| Backend + gateway | `/Users/tweety53/Projects/gymie` | Kotlin / Spring Boot / Gradle | http://localhost:8080 | Primary repo. Include for any backend, API, Flyway, or module work. |
| KMP frontend | `/Users/tweety53/Projects/gymie-frontend` | Kotlin Multiplatform / Compose | http://localhost:3000 | Include when the change mentions KMP, Compose, web, or mobile. |
| Admin frontend | `/Users/tweety53/Projects/gymie-admin-frontend` | Next.js / TypeScript | http://localhost:3001 | Include when the change mentions the admin panel. |
| Regression suite | `/Users/tweety53/Projects/gymie-playwright` | Node / Playwright | — | Include when the change adds or edits a capture spec, a `.mockups` sidecar, or otherwise touches this checkout. Also declared as `regression checkout` under `## visual verification` below — that row is `flow.visual-verify`'s own resolution of the same repository, not a second one. |

**The URLs above are the main checkout's, which keeps every documented default.** An apply worktree
gets its own database, cache index, bucket and block of application ports automatically — see
`## workspace isolation` below — so all four applications answer on *that* worktree's ports rather
than on 8080, 8081, 3000 and 3001; `FRONTEND_PORT` and `GATEWAY_PORT` are exported in every workspace
exactly like the other two ports, though `devStart` only moves them under `-PisolateOAuthPorts`. Read
the worktree's own URLs from what `./gradlew devStart` prints, never from this table.

## run

Run from the backend root. `devStart` starts the full stack and auto-detects the frontend and
admin-frontend roots — the matching sibling worktree when one exists, else the sibling checkout
(`resolveSiblingCheckout`, `gradle/dev-lifecycle.gradle.kts`) — so no `-P` flags are needed in the
common case. Pass `-PfrontendRoot=<path>`/`-PadminFrontendRoot=<path>` only to override detection.

```bash
docker compose up -d
./gradlew dbSeed                     # when seed data is needed
./gradlew devStart
./gradlew devStatus
./gradlew -q devStackFresh || { ./gradlew devStop && ./gradlew devStart -PisolateOAuthPorts; }   # before any Playwright run; devRestart drops -PisolateOAuthPorts
./gradlew devLogs
./gradlew devStop
```

`dbRepair` is the fix for one specific failure: `Migration checksum mismatch`, printed when a migration's
file changed — comments included — after Flyway already applied it. Flyway names repair as the remedy
itself; this task runs it against the same database `dbReset` and `dbSeed` resolve, and needs the same
confirmation `dbReset` does — `-PconfirmDb=<name>` or `-Pforce` — because in the main checkout that
database is the shared `gymie` one, and repair cannot tell a comment-only edit from a genuinely changed
migration: it realigns the checksum either way, on whichever database it is pointed at.

```bash
./gradlew dbRepair -PconfirmDb=<name>   # after a checksum mismatch — see README's "Database reset" section
```

Single services, when the full stack is not wanted:

```bash
./gradlew :app:bootRun --args='--spring.profiles.active=local'
./gradlew :gateway:bootRun --args='--spring.profiles.active=local'
./gradlew :webApp:wasmJsBrowserDevelopmentRun     # from the KMP frontend root
```

**In an apply worktree `devStart` is automatically isolated, and almost nothing has to be asked for.**
It derives this worktree's workspace from the change name, creates the database and bucket the first
time and says so, claims a free cache index, offsets the backend's and admin panel's ports, and prints
every value it resolved. **In the main checkout it does exactly what it always did** — `gymie`,
cache index 0, `gymie-media`, and 8080 / 8081 / 3000 / 3001 — because there is no workspace id there
and every value falls back to the default in the table below.

`dbReset` and `dbSeed` follow the same rule: in a worktree they default to that worktree's own
database, in the main checkout to `gymie`, and `-PpgDb` still overrides both. To see what a checkout
resolves without starting anything:

```bash
./gradlew devWorkspace                # prints the workspace and the environment it exports
./gradlew devWorkspaceInventory       # every workspace resource on this machine, whatever change made it
```

**`devWorkspaceInventory` is the one command here that starts from no id.** `create`, `remove` and
`survivors` each act on a workspace id the pipeline already knows, which answers nothing about a
change abandoned before `/myflow-finish` run 2 — its database, its bucket and its claimed cache index
are then held by an id nothing remembers. It is read-only, runs from any checkout including the main
one, and lists rather than sweeps: a workspace whose change is still open belongs in its output.

**`-PworkspaceChange` is also reachable as the environment variable
`ORG_GRADLE_PROJECT_workspaceChange`,** because that is how Gradle exposes every project property. A
value exported in a shell — by CI, a `direnv` file, or a line left in a profile — therefore decides
the workspace of every checkout started from it, the main one included. Nothing refuses it: setting
it in an environment is a legitimate way to drive this from CI, and the destructive paths validate
the id they are given, so a stray value cannot aim a drop somewhere unexpected. What it can do is
quietly isolate a main checkout or de-isolate a worktree, so **every run that sees it set says so**
before it resolves anything.

**`GYMIE_WORKSPACE_ID` is the db tasks' assertion, not a second way to resolve.** `dbSeed`, `dbReset` and
`dbRepair` print the workspace and database they resolved before acting, and when this variable is set —
to the workspace id or the change name behind it — refuse unless that is the workspace they actually
resolved, the main checkout's shared `gymie` database included. It exists for the failure a driving
variable cannot catch: the shell's working directory reset between calls, so the command ran in the main
checkout while the caller meant the change's own. `-Pforce=true` acts anyway.

## stop

```bash
./gradlew devStop
```

**This key exists because omitting it cost a real incident.** `/myflow-finish` run 2 runs this
command before its stack-stopped check, so nothing holds a port or a file handle open in a worktree
about to be removed; an absent key skips that check rather than failing it. `devStop` was declared
only inside the `## run` block above, which is prose the pipeline does not read as a command — so
the check was skipped, and KAN-129's worktree was removed with its stack still running. Five JVMs
and a webpack dev server outlived it by days, still holding 3000 and 8080, the two ports every
workspace shares because Google's OAuth client has them registered. The next `devStart` in another
worktree refused to bind them and said so correctly, but the operator had no remedy: `devStop` reads
its pid files from the worktree's own `.dev-state`, which had been deleted with the worktree.

Run from the backend root, and it stops every service `devStart` started in that worktree.

## workspace isolation

Two apply worktrees share one Postgres, one Redis and one MinIO. What is isolated is the **logical
resource inside** each of those services — a database, a cache index, a bucket — never the service
itself: the containers keep their names and host ports and `docker-compose.yml` is untouched. Every
value below is derived from this worktree's workspace id, except the cache index, which is claimed by
probing. The derivation is not restated here; it is stated once under **What the id derives**
(`skills/myflow-contracts/workspace-isolation.md`), which is canonical.

**The cache index's `probed` cell is the literal that contract's `cache index` row requires, and this
project's claim is what makes probing mean anything.** The claim is written *into* the index it
claims, as the key `gymie:workspace-claim` naming the workspace id, with a conditional write — so it
is visible to the next run's probe at the moment it is made, two simultaneous claims cannot both
succeed, and a claim can be released by name. `devWorkspaceRemove` releases and empties **every** index
a workspace holds, `devWorkspaceSurvivors` reports each one that outlived cleanup, and
`devWorkspaceInventory` lists every claim on the machine. A run that can get no index of its own
refuses and names every holder rather than sharing one.

**Fifteen claimable indices is a real ceiling, so two things watch it.** A successful claim that
leaves three or fewer free says so, because the alternative is that the pool is first mentioned by a
run that cannot start — in a session that had nothing to do with filling it. And an index holding keys
but carrying **no** claim is out of the pool with no id to aim a removal at: `devWorkspaceInventory`
reports it as such, distinctly from a claim, and `./gradlew devCacheIndexReclaim -PcacheIndex=<n>
-Pforce` empties it. That command refuses any index that carries a claim, which is what keeps it off a
live workspace's sessions — it is the one destructive path here with no workspace id to check against.

| Resource | Variable | Default | In a workspace |
|----------|----------|---------|----------------|
| `database` | `DB_URL` | `jdbc:postgresql://localhost:5432/gymie` | `jdbc:postgresql://localhost:5432/gymie_<id_underscored>` |
| `bucket` | `STORAGE_BUCKET` | `gymie-media` | `gymie-media-<id>` |
| `cache index` | `REDIS_DATABASE` | `0` | `probed` |
| `port` | `APP_SERVER_PORT` | `8081` | `+<offset>` |
| `port` | `ADMIN_PANEL_PORT` | `3001` | `+<offset>` |
| `port` | `FRONTEND_PORT` | `3000` | `+<offset>` |
| `port` | `GATEWAY_PORT` | `8080` | `+<offset>` |
| `url` | `STORAGE_ENDPOINT` | `http://localhost:9000` | `http://localhost:9000` |
| `url` | `STORAGE_PUBLIC_BASE_URL` | `http://localhost:9000/gymie-media` | `http://localhost:9000/<value:STORAGE_BUCKET>` |
| `url` | `BACKEND_URL` | `http://localhost:8081` | `http://localhost:<value:APP_SERVER_PORT>` |
| `url` | `FRONTEND_URL` | `http://localhost:3000` | `http://localhost:<value:FRONTEND_PORT>` |
| `url` | `ADMIN_FRONTEND_URL` | `http://localhost:3001` | `http://localhost:<value:ADMIN_PANEL_PORT>` |


The pipeline's own start (`start` under **visual verification**) passes `-PisolateOAuthPorts`, so
every port moves in a worktree the agent is testing. The operator's `## run` does not, so the stack
`/flow` hands off runs on the registered 3000/8080 — one handed-off stack at a time; a second
`devStart` on those ports refuses and names the holder, and the handoff relays that.

| Command | Runs |
|---------|------|
| `create` | `./gradlew devWorkspaceCreate` |
| `remove` | `./gradlew devWorkspaceRemove -PworkspaceId=<id>` |
| `survivors` | `./gradlew -q devWorkspaceSurvivors -PworkspaceId=<id>` |

**The `-q` on `survivors` is load-bearing and must not be tidied away.** Survivor lines are written to
standard output with `println`; every explanation the task can emit goes through `logger.lifecycle`,
which `--quiet` suppresses. Without it a workspace with nothing left over still prints
`> Task :devWorkspaceSurvivors`, `BUILD SUCCESSFUL` and a dozen up-to-date task lines, and
`scripts/check-cleanup-complete.sh` reads each of them as a surviving resource — Gradle's own chatter
would block the terminal state of a change whose cleanup succeeded.

**Three rows are named here that a reader might expect to be somewhere else.** Neither
`ADMIN_PANEL_PORT` nor `FRONTEND_PORT` existed before this project was isolated, because each names a
port that used to be fixed in a repository this one does not own — and the two are moved by different
mechanisms, which is why they are worth naming together. The admin panel's port is set by
`next dev -p 3001` in `gymie-admin-frontend`'s `package.json`, so the dev lifecycle moves it by
appending a second `-p` on the command line. The KMP frontend's is set by `port = 3000` in
`gymie-frontend`'s `webApp/build.gradle.kts`, which no argument overrides — so it is moved through the
environment instead, by `webApp/webpack.config.d/devServerPort.js` reading `FRONTEND_PORT`,
`webpack.config.d` being Kotlin/JS's own extension point for exactly this. **That variable is
exported in every workspace, and is either a valid port or absent — never empty**: `devServerPort.js`
keeps 3000 when it is absent and fails the frontend build on anything set that is not a port, so an
export resolving to empty would otherwise reinstate the shared 3000 in silence. Both are exported, and
not merely passed, so everything else that has to reach an application — the manual test guide, a
project's own tooling — reads one value rather than re-deriving it. `STORAGE_ENDPOINT` is the third:
it carries no token at all, its value being genuinely the same in both checkouts, and it is declared
so that the exported set is the complete set rather than a subset a reader has to diff against the
defaults to trust.

**`FRONTEND_PORT` is checked at both ends, because it is the one contract here that crosses a
repository boundary with no shared type.** Every other moved port is either passed on the command line
of the process being started or read by an application in this repository, so a mismatch cannot
survive the run; `FRONTEND_PORT` is exported by this repository and read by a file in `gymie-frontend`,
and a checkout predating that file ignores the export in silence — leaving webpack on the shared 3000
while `devWorkspace`, `announceWorkspace` and the manual test guide all name the moved port. So
`devStart` refuses to launch a frontend checkout whose `webApp/webpack.config.d/` contains no file
naming `FRONTEND_PORT`, whenever the port actually moved, and then health-checks the dev server on the
**resolved** port exactly as it does the other three. The refusal is the diagnosis, since it names the
checkout and the fix; the health check is the proof, and catches a skew the refusal cannot — a reader
that is present but ineffective. Neither can fire in the main checkout, where nothing is exported and
the resolved port is the declared one.

**Two values this project derives are deliberately absent from the table, and this is where to look
for them.** `ADMIN_API_URL` and `NEXT_PUBLIC_ADMIN_API_URL` do follow the workspace's gateway port,
but they are **written into `gymie-admin-frontend/.env.local`** by the dev lifecycle rather than
exported into the environment — and every row above declares an *exported* variable. Declaring them
would have a run export two variables nothing reads from the environment, and put a second writer on
a file the dev lifecycle already owns. So the table is right to omit them and this paragraph is here
instead: an operator asking why the admin panel calls the wrong backend should read that `.env.local`
and the code that writes it, not this table. The OAuth redirect URI is a third such value and needs
no row of its own — `application-local.yml` composes it inline from `${GATEWAY_PORT}`, which the
table does declare.

**What this project deliberately does not isolate.** Both are stated limitations, not gaps:

- **The integration tests are untouched.** They use Testcontainers, so the test database is already
  ephemeral and per-JVM, and `./gradlew test` needs nothing from this section. `application-test.yml`
  is deliberately not parameterised.
- **Mailpit stays shared.** It is a development mail catcher with a single inbox and no natural
  namespace, so two changes testing email flows at the same time see each other's messages
  interleaved in one inbox. The failure mode is confusion rather than corruption, and giving each
  workspace its own instance would need the per-service duplication this design rejects.

## visual verification

`gymie-frontend` (Compose Multiplatform, canvas-only) and `gymie-admin-frontend` (Next.js) are
this project's two dedicated UI repos; the third `ui paths` entry is Gymie's own
web/presentation layer — `infrastructure/presentation/web/**` in each of the `auth`, `admin`,
`profile` and `workout` domain modules, plus `:common`'s cross-cutting `web/` package. It is
there because one of KAN-16's three defects was a **server-side** allowlist mismatch that
showed only as a broken page; frontend-only globs would have missed it. The glob's literal
`src/main/kotlin` segment already excludes every `build/**` output directory — Gradle's
compiled classes land under `build/classes/kotlin/...`, a path the glob cannot match — so no
separate exclusion is needed. The `invite` module carries no `presentation/web` package of its
own; its HTTP surface is exposed through `admin`'s controllers, already covered by that entry.
`:gateway` is a separate top-level Gradle module (`src/gateway`, not under `src/app/`), so none
of the domain-module globs reach it — it is named on its own row because it is the OAuth-registered
public entry point on port 8080, and its Spring Cloud Gateway routing and CORS live in
`application.yml` beside its JWT-validation and rate-limit filters, so the glob covers its
`src/main/` wholesale rather than `kotlin/` alone; the same literal `src/main/` segment excludes
its `build/**` output the same way. The domain modules' own `infrastructure/configurations/**` is a
second such surface — `auth`'s `SecurityConfig.kt` and `admin`'s `AdminSecurityConfig.kt` hold the
`permitAll()`/`authenticated()` route allowlists every controller sits behind, so a change there can
401 a page with no controller diff at all, exactly the defect class this section exists to catch.
`:common` needed a third, separate entry rather than folding into that same glob: it has no
`infrastructure/` split at all, and its own security- and filter-relevant files —
`ActuatorSecurityConfig.kt`, `CorrelationIdFilter.kt`, `MetricsFilterRegistrationConfig.kt`,
`MetricsScrapeTokenFilter.kt` — sit directly under `common/observability/`. `:app`, the
composition-root module, needed a fourth, standalone entry for the same structural reason
`:gateway` did: `settings.gradle.kts` puts its `projectDir` at `src/app` itself, one level
shallower than every domain module's `src/app/<module>`, so no `src/app/*/...` glob can ever
reach it — the `*` consumes `src` and the pattern then needs a second `src` where the real path
has `main`. Its own `application.yml` and its `-dev`/`-local`/`-prod` siblings under
`src/app/src/main/resources/` decide the OAuth redirect URI, `app.frontend-url` (the CORS/redirect
origin `auth`'s `SecurityConfig.kt` reads), the actuator exposure list, and
`server.forward-headers-strategy` — the same class of server-side defect the domain-module
`configurations/**` entry exists to catch, recurring in the composition module's own config. The
entry covers `src/app/src/main/**` wholesale, matching `:gateway`'s own shape, rather than
`resources/**` alone: `:app` also carries a handful of Kotlin files
(`com/gymie/configurations/AsyncSupportConfig.kt`, `com/gymie/dev/*`) too small to be worth
splitting out, and the same literal `src/main/` segment excludes its `build/**` output exactly as
it does for every other entry here.

`gymie-playwright` is a Node/Playwright checkout kept outside this Gradle repository on
purpose — its own README says why — but it is a regular app for `/flow`'s own worktree
resolution: it is declared in `## apps` above, so a change touching it gets its own apply
worktree and its own `spectre/<name>` branch there, exactly like `gymie-frontend` or
`gymie-admin-frontend`. **Correction (2026-09-10):** earlier revisions of this section, and
kan-486's own `sidecars-on-playwright-main` decision, treated it as the one app with no
worktree — commits landed straight on its `main`. That is reversed: `flow.visual-verify` now
resolves `regression checkout` through the same worktree-or-main-checkout rule every other
declared app follows (**Project configuration**,
`skills/flow-contracts/project-configuration.md`), and commits the per-change spec and its
PNGs on the change's own branch there.

**The automatic push carve-out this section once declared has been dropped, and stays dropped
under the worktree.** The operator first chose "always allow direct pushes to main for
gymie-playwright, always merge and push," and a `push to default branch: allowed` row stood
here on that basis. A panel slot on `kan-171-generic-visual-verification-step` then
demonstrated that the carve-out's own origin-equality check compared two fields that both live
in this same pull-request-editable file — `regression checkout` and `regression repo` agree by
construction, so a file inside a repository cannot authorise a push to another repository. The
operator was shown the reproduction and chose to drop the automatic push rather than relocate
the authorisation; see `no-automatic-push` in `kan-171`'s design. That reasoning is about
*authorisation*, not about worktrees, so it holds unchanged now that a real branch exists to
push: `flow.visual-verify` commits the per-change spec and its PNGs on that branch and stops
there — the handoff prints the push command for the operator to run by hand, and landing the
branch itself follows the same merge-and-push route every other worktree in the change takes.

`regression repo` below still records which repository the checkout is expected to be, and
`check-visual-verification.sh` still reports a mismatch against its real `origin` — but that is an
identity assertion, not an authorisation.

**`screenshots` names the regression checkout's own root, not `tests`.** A per-change `capture`
spec and its PNGs commit at the checkout's root beside `tests/` — `kan-339-route-a.spec.ts` and
its `kan-339-route-a.spec.ts-snapshots/` are the checked-in example — so a search rooted at `tests`
would never reach them. The root also carries a `node_modules/`, which a naive recursive search
would walk; this is harmless because the stage's own read step matches a PNG's path against the
capture spec's own basename, not just its location under `screenshots`, so nothing under
`node_modules` can spuriously match a spec name it does not carry.

**`mockups` lives in this repository, not in `gymie-frontend`.** `flow.visual-verify` step 9 resolves
the root as `<worktree>/<mockups>` in *this* repository's worktree, so a root under the frontend
repository would never resolve. `docs/design/mockups/` holds one `<frame id>.png` per drawn frame
of the group-sessions package plus the dead-ends follow-on; `J1.png` is the dead-ends frame (the bar
with exits, which is what the app implements) and `J1-pre-exits.png` the original. A capture spec
opts in with a `<spec>.mockups` sidecar beside it in `gymie-playwright/fidelity/` — or in
`gymie-playwright/tests/`, which is where the pre-KAN-514 sidecars lived; after KAN-514 every
mockup sidecar is under `fidelity/` and none remain under `tests/`.

**A capture spec for a mockup frame belongs to the `fidelity` Playwright project, never to
`tests/`.** `gymie-playwright/playwright.config.ts` carries two projects: `functional`, at
1200x1164 / `deviceScaleFactor` 1, for every other spec in the checkout, and `fidelity`, whose
`testDir` is `fidelity/` and which captures at **388 logical px wide** with `deviceScaleFactor` 2
and a per-test height taken from `fidelity/frames.ts`. 388 rather than 390 because the drawn
frame's 1px border sits outside the screen it encloses. Captures there are the whole viewport —
never a `clip:` crop — so a capture and its frame compare 1:1.

A capture taken under `functional` is **not** comparable with any frame here and must not be given
a sidecar: at 1200 wide the app lays out for a desktop while every frame is a phone, so the
composed pair is apples to oranges by construction, which is what let KAN-30 pass visual
verification four times with paddings, colours, borders and fonts wrong across most screens.

The three numbers in the `mockup frame` row are the frames' own export scale, the height of the
drawn status line, and the border width — `scale=2 status=26 border=1` — measured on the PNGs and
matching the frame markup in `docs/design/group-sessions/design/Gymie Group Session Flow.html`
(`width:390px; min-height:748px; border:1px; box-sizing:border-box`, a 26px status line). They are
what lets `flow.visual-verify` crop a frame down to the screen the app actually renders: the app
draws no status band in a plain Chromium context, since `rememberTopSystemInsetDp()` reads
`env(safe-area-inset-top)` and gets 0 there.

**Any pre-existing bug found while verifying or capturing — a stale coordinate, a real-clock date
drift, a missing local service — that is out of scope for the change in hand must be logged in
`gymie-playwright/KNOWN-BUGS.md` (spec name, root cause, the commit that introduced it) rather than
only mentioned in the change's own `tasks.md`.** This is required, not optional: `tasks.md` is
per-change and gets archived away with it, so a bug noted only there is invisible to the next change
that trips over the same failure. Delete an entry once its bug is actually fixed — the file tracks
open bugs, not a changelog.

| Setting | Value |
|---------|-------|
| `ui paths` | `gymie-frontend/**`, `gymie-admin-frontend/**`, `src/app/*/src/main/kotlin/**/infrastructure/presentation/web/**`, `src/app/*/src/main/kotlin/**/infrastructure/configurations/**`, `src/app/common/src/main/kotlin/com/gymie/common/web/**`, `src/app/common/src/main/kotlin/com/gymie/common/observability/**`, `src/gateway/src/main/**`, `src/app/src/main/**` |
| `screenshots` | `.` |
| `regression checkout` | `/Users/tweety53/Projects/gymie-playwright` |
| `regression repo` | `git@github.com:tweety53/gymie-playwright.git` |
| `mockups` | `docs/design/mockups` |
| `mockup frame` | `scale=2 status=26 border=1` |

| Command | Runs |
|---------|------|
| `setup` | `npm install && npx playwright install chromium` |
| `verify` | `BACKEND_URL=http://localhost:${GATEWAY_PORT:-8080} npx playwright test <specs>` |
| `capture` | `BACKEND_URL=http://localhost:${GATEWAY_PORT:-8080} npx playwright test <spec> --update-snapshots` |
| `start` | `docker compose up -d && ./gradlew devStart -PisolateOAuthPorts -PfrontendRoot=<FRONTEND_ROOT> -PadminFrontendRoot=<ADMIN_FRONTEND_ROOT>` |
| `fingerprint` | `./gradlew -q devStackFresh` |
| `specs` | `node scripts/specs-for-diff.mjs <frontend-root> <merge-base>` |

`specs` prints the specs reaching a tag the touched frontend UI files declare — a `testTag("…")` literal or a
tag-shaped `const val` — through the spec's own text, `expectRoom`/`openRoom`, or a helper it calls. A touched UI
file declaring no spec-reached tag, a deleted file, a non-test file outside the UI tree, or an empty diff selects
every spec. visual-verify substitutes the list for `verify`'s `<specs>`; with no list, `verify` runs every spec.

**`verify` and `capture` set `BACKEND_URL` to the gateway because the suite and the gateway read that one name
as two different servers.** The suite reads it as the gateway the page calls (`api.ts`'s `API`), and
`fixtures.ts` refuses to load when its port is not `GATEWAY_PORT`. The gateway reads it as the app server it
routes to (`src/gateway/src/main/resources/application.yml`, every `uri: ${BACKEND_URL:…}`), which is why
`## workspace isolation` exports it as `APP_SERVER_PORT` and must keep doing so — remapping that row would break
the gateway's routing. Each row therefore overrides the export for the suite's own process only. In the main
checkout `GATEWAY_PORT` is unset and the prefix falls back to the main checkout's gateway, 8080.

**`fingerprint` is `devStackFresh` because the backend, gateway and wasm frontend never reload (KAN-692)** — what it
checks, and why that is enough, is `buildSrc/src/main/kotlin/com/gymie/gradle/DevStackFreshness.kt`.

## credentials

Local development only. These are seeded by `./gradlew dbSeed` into a local database and
are not valid against any deployed environment.

| Where | User | Password |
|-------|------|----------|
| Admin panel | `superadmin@gymie.dev` | `GymieDev1` |
| DEV_INVITE_EMAIL account | (your `DEV_INVITE_EMAIL` value) | `Qwerty123!` |

`dbSeed` also issues a live invite for `DEV_INVITE_EMAIL` (env var, in `.env`/`.env.local`
alongside `GOOGLE_CLIENT_ID`) so Google sign-in works against a fresh database — registration is
invite-only and neither seeded user above is a real Google account. Issued through
`InviteService.create`, not a direct row insert. Skipped, not failed, when unset (the normal case
for CI and for a developer not using Google sign-in), and skipped again once the address is
registered.

**`dbSeed` now also creates a real user row for `DEV_INVITE_EMAIL` itself** (`emailVerified = true`,
password `Qwerty123!` / `DEV_INVITE_PASSWORD`), so the first Google sign-in reaches
`OAuthUserServiceImpl.linkToExistingAccount` and adopts that row — with the friend cast below
already attached — instead of registering a fresh account. The seeded password lets this same
account sign in by email+password too, as an alternative to Google OAuth. Still gated on
`DEV_INVITE_EMAIL` being set; skipped, not failed, when it isn't.

Local-only, seeded by `dbSeed` alongside the accounts above, and hung off that `DEV_INVITE_EMAIL`
row — a developer with no invite address set gets none of this:

| Account | Relationship to DEV | Grants (W = Workouts, B = Body) | Visibility |
|---------|--------------------|--------------------------------|------------|
| `friend-plain@gymie.dev` | accepted | none either way | `friends` |
| `friend-i-edit@gymie.dev` | accepted | DEV → them W Edit | `everyone` |
| `friend-editable@gymie.dev` | accepted | they → DEV W Edit | `friends_who_can_edit` |
| `friend-mutual@gymie.dev` | accepted | W Edit both ways | `everyone` |
| `friend-reads-workouts@gymie.dev` | accepted | they → DEV W Read | `friends` |
| `friend-reads-body@gymie.dev` | accepted | they → DEV B Read | `friends` |
| `friend-reads-both@gymie.dev` | accepted | they → DEV W Read + B Read | `friends` |
| `friend-body-editable@gymie.dev` | accepted | they → DEV B Edit | `friends` |
| `friend-fully-editable@gymie.dev` | accepted | they → DEV W Edit + B Edit | `friends` |
| `friend-mixed@gymie.dev` | accepted | they → DEV W Read + B Edit | `friends` |
| `friend-i-share@gymie.dev` | accepted | DEV → them W Read + B Read | `everyone` |
| `dev-wife@gymie.dev` (`DEV's wife`) | accepted | W Edit both ways | `friends` |
| `req-search@gymie.dev` | incoming pending request, origin `search` | — | `everyone` |
| `req-link@gymie.dev` | incoming pending request, origin `link` | — | `friends` |
| `req-sent@gymie.dev` | outgoing pending request from DEV | — | `everyone` |
| `stranger-open@gymie.dev` | none — search target | — | `everyone` |
| `stranger-closed@gymie.dev` | none — search target | — | `nobody` |

Password `GymieDev1` (the same `DEV_PASSWORD` the two seeded users above use) for every account in
this table. Together the rows reach every per-scope rights state the app distinguishes, and every
scope DEV may read has data behind it:

- **Workouts** — `friend-editable`, `friend-mutual`, `friend-reads-workouts`, `friend-reads-both`,
  `friend-fully-editable` and `friend-mixed` carry workouts. The `DEV_INVITE_EMAIL` account trains
  Tuesday, Thursday and Saturday of last week and this week, plus next week's Tuesday, each a group
  session with `dev-wife` — finished before today, in progress today, planned after. DEV's week
  shows `friend-editable` and `friend-mutual` ("in your week").
- **Body** — `friend-reads-body`, `friend-reads-both`, `friend-body-editable`,
  `friend-fully-editable` and `friend-mixed` carry about eight weeks of weigh-ins, a goal and two
  calorie targets. `friend-reads-body` is imperial and records in lb, so DEV (metric) switched into
  it reads kg — the viewer's own unit.

All of it is re-anchored to today on every `dbSeed` run. A seeded workout you edit is lost on the
next run unless you clear its `[gymie-dev-seed]` note first — see the backend `README.md`'s
database section; a seeded account's weigh-ins and targets are replaced wholesale on every run.

## test

```bash
./gradlew test                                                # full suite
./gradlew verifyChange :<module>:test --tests '<selectors>'   # one-invocation verify bundle
```

## lint

```bash
./gradlew ktlintFormat                # auto-fix formatting first
./gradlew verifyChange                # verify — ktlintCheck + detekt across every project, one invocation
```

Pre-approved suppressions and documented deviations live in `CONTRIBUTING.md`.

## execution mode

dynamic

## implementer model

dynamic

## review panel

dynamic

## jira

KAN

## self review

defer

## default landing route

merge and push

Read by `/flow`'s integrate phase, which resolves this body byte-for-byte against the three literals
`pull request`, `merge and push` and `manual` after trimming whitespace — anything else is reported
by name and dropped, resolving as absent. It moves the `(default, recommended)` marker on the
landing question; it does not remove the question, so any run can still choose another route.

**`merge and push` rather than `pull request`** because this project's changes are landed by the
same person who runs the pipeline, on `develop`, with the review already done by the pipeline's own
panel and the human gate — a pull request adds a round trip and a second approval surface for a
branch nobody else is waiting on. `main` is still reached only through a PR, unchanged: that is the
no-direct-pushes rule and this key does not touch it.

**This also settles the archive route**, which is keyed on how run 2 was reached rather than on this
key: choosing merge-and-push continues into archiving in the same invocation, so the archive branch
is merged into the base too. Choosing `pull request` here instead makes the archive a standalone
invocation, which opens an archive PR.

## toggles

**Correction (2026-09-10): both facts below have changed since this section was written.**
This repository now declares `## execution mode`, `## implementer model` and `## review panel`
as `dynamic`, matching `gymie-frontend/.flow/project.md`, so a small plan on either side of a
cross-repo change gets a lighter panel/execution mode rather than always the full default
roster. And `/flow`'s cross-repo toggle resolution is fixed (`agents` PR #102,
`skills/flow/SKILL.md`'s `STATE_WORKTREE_ROOTS`): a resumed, fix, or bare `IN_PROGRESS` run now
consults every repo in the change's own state-record `worktrees` map, not `MAIN_CHECKOUT` alone
— a satellite repo's `dynamic` is honored regardless of which repo the shell started in. The one
gap that remains, inherent rather than a bug: a brand-new **creating** run resolves toggles
before any worktree exists, so it still reads `MAIN_CHECKOUT` alone at that point — starting
`/flow` from the repository carrying the change's task volume is still the right call for a
frontend-heavy *first* run of a cross-repo change, though `gymie` and `gymie-frontend` now agree
on `dynamic` for everything else regardless.

## standards

Files the principles reviewer receives as this project's written standards, and the rules
this project opts into.

- `CLAUDE.md`
- `CONTRIBUTING.md`
- `kotlin-backend-development-standard.mdc` — opt-in rule from the agents repo. Applies to
  `src/**/*.kt` and `**/*.kts`. This project is the Spring backend the rule was written
  for; sibling Kotlin repos deliberately do not opt in.

## progress

quiet
