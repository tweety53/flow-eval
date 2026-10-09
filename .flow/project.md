# flow project configuration — flow-eval

Read by globally installed flow skills.

## apps

| App | Repo root | Kind | URL | Notes |
|-----|-----------|------|-----|-------|
| flow-eval | `/Users/tweety53/Projects/flow-eval` | Go CLI + Docker | — | Eval runner. Trials run in Docker containers; reads case sources from the gymie checkouts named in each case. |

### artifact tree

The pipeline's artifact tree is `spectre/`.

## run

```bash
go run ./cmd/flow-eval --help
```

## stop

No local stack: the CLI is run on demand and every trial container is removed when it ends.

## test

```bash
go test ./... -race -count=1
```

## lint

```bash
gofmt -w .
test -z "$(gofmt -l .)"
go vet ./...
```

## jira

KAN

## default landing route

pull request
