# odbc

`odbc` is a `database/sql` driver for ODBC, written in pure Go. It loads the
ODBC driver manager of the system at run time with
[`purego`](https://github.com/ebitengine/purego). It needs no cgo and no C
compiler, and it runs the same way on Windows, macOS and Linux (D1, D2).

The driver works on Linux against PostgreSQL, MariaDB, MySQL, SQL Server and
SQLite. It has not run on macOS or Windows, and DuckDB is not tested yet.
[`docs/PROGRESS.md`](docs/PROGRESS.md) says where the work stands. A value has
the Go type that `dbimp` gives its kind (D17, D18).

## Standing rules

These hold in every `xo` repository, for every coding agent.

1. Stage changes for review. Commit and push only when Ken says so.
2. Load the `simple-english` skill before you write any text that a person
   reads: a document, a code comment, an error message or a commit message.
   Follow it for that text.
3. Load the `go-pedantry` skill before you write or review Go code. Follow it
   where it does not conflict with a rule in this file. A rule here wins.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Write a rule here and never there.

## Hard rules

Each rule is here because breaking it is costly and a reviewer will not
always see it.

1. No cgo. `CGO_ENABLED=0 go build ./...` must pass on Windows, macOS and
   Linux. A file that imports `"C"` is a defect (D1).
2. The dependencies are the standard library, `github.com/ebitengine/purego`,
   `github.com/xo/dbimp` and the `apd` module that `dbimp` brings. Add nothing
   else without asking Ken. `depguard` holds this (D4, D17).
3. The context comes first and is named `ctx`. Library code never calls
   `context.Background` or `context.TODO`. `forbidigo` holds this.
4. Every ODBC handle is freed once, by the code that allocated it, and never
   used after. A leaked handle leaks a connection on the server, and a used
   one corrupts memory.
5. The driver manager can keep the address of a buffer after a call returns.
   This happens with `SQLBindCol` and `SQLBindParameter`. Keep such a buffer
   reachable until the statement is freed or unbound. The Go runtime does not
   know that the manager holds it.
6. Text crosses the boundary through the wide functions, and only through
   `encode` and `decode` in `wchar.go`, which know the size of `SQLWCHAR`
   (D20). Never build or read a `uint16` string elsewhere.
7. A NULL is `nil`. Never turn it into a zero value.
8. Return `driver.ErrBadConn` only when the statement did not reach the
   server. Any other use makes `database/sql` run a statement twice.
9. On Linux, start a server with `dbrun` from `dbmeta` and no other way (D9).
   Set `DBMETA_OWNER_NAME` to `odbc`. A macOS or Windows CI job installs its
   server natively (D13).
10. A change to the code that loads or calls the driver manager is tested on
    all three operating systems before it is called done (D3).
11. Do not decide an open question alone. The questions are in
    [`docs/BACKLOG.md`](docs/BACKLOG.md). Ask Ken.

## Layout

| File | Holds |
| --- | --- |
| `odbc.go` | the package comment, the driver name and its registration |
| `dsn.go` | `ParseDSN`, which reads a URL or an ODBC connection string (D16) |
| `api.go`, `lib_unix.go`, `lib_windows.go` | loading the driver manager and the table of ODBC functions |
| `wchar.go` | the size of `SQLWCHAR`, its probe, and the conversion of strings |
| `connector.go`, `conn.go`, `tx.go` | the environment, connections and transactions |
| `stmt.go` | statements and the binding of arguments |
| `options.go` | the per-statement options of D22 |
| `list.go` | `Drivers` and `DataSources`, which list what the driver manager knows (D23) |
| `rows.go` | result sets, the column metadata and the mapping of types (D18) |
| `plan.go`, `block.go` | how each column is read, and the block fetch of `WithFetchSize` (D24) |
| `catalog.go` | the catalog functions on `odbc.Conn` (D24) |
| `errors.go` | the error type, which carries the SQLSTATE, and its classes (D23) |
| `*_test.go` in the root | the unit tests of `ParseDSN` and the tests that keep the documents true |
| `test/` | a module of its own (D15) with the integration tests. They read one variable for each database and skip a database whose variable is empty |

## Which document to read

A document that is not in this table does not exist.

| To do this | Read |
| --- | --- |
| Learn what the project is and how to use it | [`README.md`](README.md) |
| Send a change as a person | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Find out why something is the way it is | [`docs/decisions/README.md`](docs/decisions/README.md) |
| Find work that is known and not done | [`docs/BACKLOG.md`](docs/BACKLOG.md) |
| Resume after a session ended | [`docs/PROGRESS.md`](docs/PROGRESS.md) |

Only `README.md`, `AGENTS.md`, `CLAUDE.md` and `CONTRIBUTING.md` belong in the
root. Every other document goes in `docs/`, and in the table above and the
table in `README.md`.

## Decisions

Every decision is a file in `docs/decisions/`, named `D<nnn>-<title>.md`. It
opens with its number, its title and its status, and it gives the reason and
what was rejected. There are 24 decisions so far. Add the row to the index in
`docs/decisions/README.md` when you add a file, and `TestTheDecisionIndexIsComplete`
fails if you forget. Never write "see D<n> of another project" in place of a
reason.

## Go conventions

- Format with `gofmt`. Lint with `golangci-lint`. The configuration is
  `default: all` and each disabled linter has its reason beside it.
- Wrap an error with `%w`. Write the message in lower case, starting with a
  gerund, as in `opening the connection: %w`.
- A sentinel error is a constant of a defined string type.
- Use the `Context` form of every `database/sql` call.

## Running the checks

```bash
gofmt -l . && go vet ./... && CGO_ENABLED=0 go build ./... && go test -race -count=2 ./...
golangci-lint run ./...
(cd test && gofmt -l . && go vet ./... && go test -race -count=2 ./... && golangci-lint run ./...)
```

The `test` module is separate, and `./...` in the root does not reach it.

`gofmt -l .` prints nothing when the code is formatted. CI runs the tests with
`-count=2`.

## Skills

The two skills are committed as copies under `.agents/skills` and
`.claude/skills`, and `skills-lock.json` names where they came from (D5).
`TestSkillsAreCopies` fails on a link and on a difference between the folders.
