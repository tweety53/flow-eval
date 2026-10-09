# flow-eval — agent instructions

## Go skills

Any agent that writes, changes, reviews or debugs Go in this repository uses the Go skills that apply
to the work, at every depth. A dispatcher names the applicable ones in every Go implementer, fixer and
reviewer prompt.

- `modern-go-guidelines:use-modern-go` — invoke through the Skill tool for any Go written or changed.
- The `go-*` skills at `~/.agents/skills/<name>/SKILL.md` — not in the Skill list; read the file
  before the work it governs.

| When | Read |
|------|------|
| Writing or changing any Go | `go-coding-standards`, `go-error-handling`, `go-modernize`, `go-test-quality`, `go-test-table-driven` |
| A CLI command, flags, exit codes | `go-cli` |
| Running processes or containers, timeouts, cancellation | `go-context`, `go-concurrency-review` |
| A boundary faked in tests (sandbox, harness, judge) | `go-interface-design` |
| Isolation, the egress proxy, credentials, untrusted input | `go-security-audit` |
| New packages or module layout | `go-project-layout` |
| Godoc, `docs/`, README | `go-documentation` |
| Reviewing a diff | `go-code-review`, plus every row above that the diff touches |
| Reviewing a whole branch | `go-architecture-review` as well |
| A panic, deadlock, race report or leak | `go-troubleshooting` |
| Adding a module dependency | `go-dependency-audit` |

The rest (`go-grpc`, `go-database`, `go-observability`, `go-api-design`, `go-ci`,
`go-dependency-injection`, `go-design-patterns`, `go-performance-review`, `go-data-structures`,
`go-refactoring`, `go-semantic-tools`) apply only when the work is about their subject.
