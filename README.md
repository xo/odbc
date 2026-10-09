<div align="center">
  <a href="#about" title="About">About</a> |
  <a href="#installing" title="Installing">Installing</a> |
  <a href="#using" title="Using">Using</a> |
  <a href="#faq" title="FAQ">FAQ</a> |
  <a href="#platforms" title="Platforms">Platforms</a> |
  <a href="#testing" title="Testing">Testing</a> |
  <a href="#documents" title="Documents">Documents</a> |
  <a href="#contributing" title="Contributing">Contributing</a>
</div>

<br/>

[![Unit Tests][odbc-ci-status]][odbc-ci]
[![Go Reference][goref-odbc-status]][goref-odbc]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

[odbc-ci]: https://github.com/xo/odbc/actions/workflows/test.yml "Test CI"
[odbc-ci-status]: https://github.com/xo/odbc/actions/workflows/test.yml/badge.svg "Test CI"
[goref-odbc]: https://pkg.go.dev/github.com/xo/odbc "Go Reference"
[goref-odbc-status]: https://pkg.go.dev/badge/github.com/xo/odbc.svg "Go Reference"
[release-status]: https://img.shields.io/github/v/release/xo/odbc?display_name=tag "Latest Release"
[releases]: https://github.com/xo/odbc/releases "Releases"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"

# About

`odbc` is a `database/sql` driver for ODBC, written in pure Go. ODBC is a
standard way for a program to talk to a database. The driver loads the ODBC
driver manager of the system at run time with [`purego`][purego]. The driver
manager is the system library that finds the database drivers and calls them.
The driver needs no cgo and no C compiler. One code base serves Windows, macOS
and Linux.

CI tests the driver on Linux, macOS and Windows against PostgreSQL, MariaDB,
MySQL, SQL Server, SQLite and DuckDB. Each system tests the databases that its
runner can hold. [`docs/PROGRESS.md`](docs/PROGRESS.md) says where the work
stands.

# Installing

```sh
go get github.com/xo/odbc
```

The dependencies are `purego` and [`dbimp`][dbimp], which brings `apd`. You
also need an ODBC driver manager and the ODBC driver of your database installed
on the machine.

# Using

```go
import (
	"database/sql"

	_ "github.com/xo/odbc"
)

db, err := sql.Open("odbc", "odbc+PostgreSQL+Unicode://user:pass@localhost:5432/dbname")
```

The data source name is a URL with the scheme `odbc+<driver>`. Write the driver
name as the driver manager knows it, with a plus sign for each space. The name
can also be an ODBC connection string, such as `DRIVER={SQLite3};Database=/tmp/a.db`. The
query key `driver` names a library by its path, and `manager` names the driver
manager. See D16 for the rest.

Placeholders are `?`. Values have the Go types of the `dbimp` kinds. A decimal
is a `*apd.Decimal`, a date is a `dbimp.Date`, and a timestamp with no zone is
a `dbimp.LocalDateTime`. D18 has the whole table.

A statement takes options as arguments or through the context. They are
`odbc.WithTimeout`, `odbc.WithDatabase`, `odbc.WithReadonly` and
`odbc.WithParameter`, as in the other `dbimp` drivers. A database driver that
cannot honor an option makes the statement fail with `dbimp.ErrNotSupported`.
D22 has the rules.

A tool can ask the driver about the database. `odbc.Drivers` and
`odbc.DataSources` list what the driver manager knows. `odbc.Conn`, which
`sql.Conn.Raw` gives, wraps `SQLGetInfo`. `Config.OnWarning` receives the
informational diagnostics of a statement, and `Config.TraceFile` turns on the
trace of the driver manager. `errors.Is(err, odbc.ErrIntegrity)` tests a class
of SQLSTATE. D23 has the rules.

`odbc.WithFetchSize` reads a large result in blocks, `odbc.WithMaxRows` limits
it, and `Config.Location` makes a timestamp a `time.Time` in a location.
`odbc.Conn` also has `Tables`, `Columns` and `PrimaryKeys`, which read the
metadata of any database. D24 has the rules.

# FAQ

## How do I turn on ANSI SQL mode for MariaDB and MySQL?

MariaDB and MySQL read double quotes as text, and `||` as a logical or, unless
the session is in ANSI SQL mode. Turn the mode on with the `INITSTMT` key, which
runs a statement each time a connection opens:

```
odbc+MariaDB://user:pass@host:3306/db?INITSTMT=SET+SESSION+sql_mode%3D%27ANSI%27
```

In a connection string, put the statement in braces:

```
DRIVER={MariaDB Unicode};SERVER=host;PORT=3306;UID=user;PWD=pass;DATABASE=db;INITSTMT={SET SESSION sql_mode='ANSI'}
```

[`usql`][usql] takes the same URL:

```sh
usql 'odbc+MariaDB+Unicode://user:pass@host:3306/db?INITSTMT=SET+SESSION+sql_mode%3D%27ANSI%27'
```

To check it, run `SELECT @@sql_mode`. The answer holds `ANSI` and `ANSI_QUOTES`,
and `SELECT "name" FROM "table"` reads a column and a table. On MariaDB the
answer is `REAL_AS_FLOAT,PIPES_AS_CONCAT,ANSI_QUOTES,IGNORE_SPACE,ANSI`.

- `INITSTMT` is a key of the ODBC driver of the database and not of this driver.
  It works the same for any Go ODBC driver that sends the connection string on.
- It runs on every connection that a pool opens, so every session has the mode.
  To change one session, run `SET SESSION sql_mode='ANSI'` as a statement.
- The word ANSI also names a kind of ODBC driver, as in "MariaDB ANSI" and
  "MariaDB Unicode". That is a different setting. This driver calls the wide
  functions, so it needs the Unicode driver.
- `TestInitStmt` runs the setting against MariaDB and MySQL, with the MariaDB
  driver for both (D14). MySQL Connector/ODBC documents the same key, and the
  tests do not run it.

## Why does my program crash when it links `go-sqlite3` and DuckDB?

A program links both `github.com/mattn/go-sqlite3` and the DuckDB bindings and
uses the SQLite ODBC driver. Its first query ends in a segmentation fault. The
DuckDB bindings link with `-rdynamic`. That makes the program export every
global symbol of its own, including the 268 `sqlite3_*` functions of the SQLite
that `go-sqlite3` carries. The SQLite ODBC driver is a shared library, and it
needs `sqlite3_*` functions too. The dynamic linker gives it the copy in the
program for some functions. It gives it the copy in `libsqlite3.so` for the
others, and the two copies do not match. The same can happen to any ODBC driver
that shares a library with a copy that your program carries and exports.

This is not a fault of `odbc`, and the driver cannot prevent it. Use one of
these:

- Build `go-sqlite3` against the SQLite of the system, with the build tag
  `libsqlite3`. The program then exports no `sqlite3_*` function, and both sides
  use the same library.
- Leave `go-sqlite3` or DuckDB out of the program.
- Build with `CGO_ENABLED=0`.

[`usql`][usql] shows it. With the default tags and `odbc`, the first query
crashes. With `-tags 'odbc libsqlite3'` it works.

## How do I pass another setting to an ODBC driver?

Every ODBC driver has its own keys. Add one to the query of the URL, or to the
connection string, and this driver passes it on. A value that holds a space, an
equals sign or a semicolon is put in braces for you when you use the URL form.
The keys `driver`, `manager` and `wchar` are the exceptions. They configure this
driver, and D16 and D20 describe them.

## How do I find the name of an ODBC driver?

Call `odbc.Drivers`, which lists every driver that the driver manager knows, with
its name and its attributes. On Linux and macOS, `odbcinst -q -d` prints the same
names. A driver that is not registered can be named by the path of its library,
as in `?driver=/usr/lib/psqlodbcw.so`, on Linux and macOS.

## Which placeholder does a statement take?

Whatever the database takes, and for every database that the tests use it is `?`.
The driver passes the text of the statement on as it is. `psqlODBC` rejects `$1`
(D16).

## Why is a timestamp a `dbimp.LocalDateTime`?

An ODBC timestamp has no time zone, and a `time.Time` always has one. A
`time.Time` here claims a zone that the database did not state. D18 gives the Go type of each kind. Set `Config.Location`
to get a `time.Time` in a location instead (D24).

## How do I see what the driver manager does?

Set `Config.TraceFile`. The driver manager then writes every ODBC call to that
file (D23).

# Platforms

| System | Driver manager the driver loads |
| --- | --- |
| Windows | the one built into the system, `odbc32.dll` |
| macOS | `unixODBC` or `iODBC`, for example from Homebrew |
| Linux | `unixODBC` |

The driver tries the usual names of the library on each system. D2 lists them,
and the `manager` key of the data source name overrides them.

# Testing

The driver is tested against these databases, with their own ODBC drivers.
PostgreSQL is the baseline. Other databases are not tested.

| Database | ODBC driver |
| --- | --- |
| PostgreSQL | `psqlODBC` |
| SQLite | the SQLite ODBC driver |
| SQL Server | the Microsoft ODBC driver |
| MySQL and MariaDB | MariaDB Connector/ODBC, for both |
| DuckDB | the DuckDB ODBC driver |

SQL Server is tested on Linux only.

# Documents

| To do this | Read |
| --- | --- |
| Send a change | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Read the rules for a coding agent | [`AGENTS.md`](AGENTS.md) |
| Find out why something is the way it is | [`docs/decisions/README.md`](docs/decisions/README.md) |
| Find work that is known and not done | [`docs/BACKLOG.md`](docs/BACKLOG.md) |
| See where the work stands | [`docs/PROGRESS.md`](docs/PROGRESS.md) |

# Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md). There are 24 decisions so far.

<br/>

<div align="center">
  <a href="https://github.com/xo/usql" title="A command line client for many databases">usql</a> |
  <a href="https://github.com/xo/dburl" title="Database connection URLs">dburl</a> |
  <a href="https://github.com/xo/dbmeta" title="Database metadata">dbmeta</a> |
  <a href="https://github.com/xo/dbimp" title="Database drivers in pure Go">dbimp</a> |
  <a href="https://github.com/xo/cql" title="A database/sql driver for Cassandra">cql</a> |
  <a href="https://github.com/xo/dbtpl" title="Go code generated from a database">dbtpl</a> |
  <a href="https://github.com/xo/tblfmt" title="Tables of database results">tblfmt</a> |
  <a href="https://github.com/xo/rline" title="The line editor of usql">rline</a> |
  <a href="https://github.com/xo/odbc" title="A database/sql driver for ODBC, this project">odbc</a> |
  <a href="https://github.com/xo/transit" title="tree-sitter in pure Go">transit</a>
</div>


[purego]: https://github.com/ebitengine/purego "purego"
[usql]: https://github.com/xo/usql "usql"
[dbimp]: https://github.com/xo/dbimp "dbimp"
