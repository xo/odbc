<div align="center">
  <a href="#about" title="About">About</a> |
  <a href="#installing" title="Installing">Installing</a> |
  <a href="#using" title="Using">Using</a> |
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

`odbc` is a `database/sql` driver for ODBC, written in pure Go. It loads the
ODBC driver manager of the system at run time with [`purego`][purego], so it
needs no cgo and no C compiler. One code base serves Windows, macOS and Linux.

The driver works on Linux against PostgreSQL, MariaDB, MySQL, SQL Server and
SQLite. It has not run on macOS or Windows yet, and DuckDB is not tested.
[`docs/PROGRESS.md`](docs/PROGRESS.md) says where the work stands.

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

The data source name is a URL whose scheme is `odbc+<driver>`, with a plus
sign for each space in the name that the driver manager knows. It can also be
an ODBC connection string, such as `DRIVER={SQLite3};Database=/tmp/a.db`. The
query key `driver` names a library by its path, and `manager` names the driver
manager. See D16 for the rest.

Placeholders are `?`. Values have the Go types of the `dbimp` kinds: a decimal
is a `*apd.Decimal`, a date is a `dbimp.Date`, and a timestamp with no zone is
a `dbimp.LocalDateTime`. D18 has the whole table.

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

Read [`CONTRIBUTING.md`](CONTRIBUTING.md). There are 21 decisions so far.

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
[dbimp]: https://github.com/xo/dbimp "dbimp"
