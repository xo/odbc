# Contributing to odbc

Read these first:

- [`AGENTS.md`](AGENTS.md) holds the rules. It is written for a coding agent,
  and everything in it applies to a person. `CLAUDE.md` holds one line that
  imports it.
- [`docs/decisions/README.md`](docs/decisions/README.md) lists every decision
  with its status. Read the status before the decision.
- [`docs/BACKLOG.md`](docs/BACKLOG.md) lists the work that is not done and the
  questions that are open. Do not decide a question on your own. Ask Ken.

## Before you send a change

```bash
gofmt -l . && go vet ./... && CGO_ENABLED=0 go build ./... && go test -race -count=2 ./...
golangci-lint run ./...
(cd test && gofmt -l . && go vet ./... && go test -race -count=2 ./... && golangci-lint run ./...)
```

`gofmt -l .` must print nothing. The `test` directory is a module of its own, and
`./...` in the root does not reach it.

## What a change needs

- No cgo, and no dependency beyond the standard library, `purego` and `dbimp`.
- A test. A change to the code that calls the driver manager is tested on
  Windows, macOS and Linux.
- Plain English in every document, comment and message. The skill is in
  `.agents/skills/simple-english`, and `TestProseIsSimpleEnglish` checks the
  rules a machine can check.
- A decision file when you choose between two ways. Give the reason and say
  what you rejected.

## Servers for the tests

Start a database with `dbrun` from [`dbmeta`](https://github.com/xo/dbmeta)
and never by hand:

```bash
cd dbmeta/test && go run ./cmd/dbrun start postgres
```

The integration tests read one variable for each database, such as
`ODBC_POSTGRES`, and skip a database whose variable is empty.
`docs/PROGRESS.md` lists them and shows a data source name.

## Quick tests on macOS and Windows

For a quick check on the other two systems, use the virtual machines in the
`vm` repository, which run as Podman containers. Both are already set up.
Start one and shell in as `user`: Windows 11 takes ssh on port 2222 and macOS
15 takes ssh on port 2223. Each VM uses 16 GB of memory, so start one only when
you need it. See `README.md` in that repository.

The macOS VM is Intel. Homebrew has no bottles for Intel there, so it uses
MacPorts, and it cannot show whether a driver has an Apple silicon build. CI
on `macos-latest` covers that.

## The agent skills

The two skills are copies, and a link breaks on Windows. To install one again,
keep `--copy`. This example installs `simple-english`, and `skills-lock.json`
names the source of each skill:

```bash
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
```

`TestSkillsAreCopies` fails on a link, and when the two folders differ.
