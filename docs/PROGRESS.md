# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

The `test` module passes on all three systems, with `CGO_ENABLED=0`. Nothing is
committed. The work is staged for Ken to review.

| System | Passes against |
| --- | --- |
| Linux (Arch), with `-race -count=1` | PostgreSQL, MariaDB, MySQL, SQL Server, SQLite and DuckDB |
| Windows 11 | PostgreSQL, MariaDB, MySQL, SQLite and DuckDB |
| macOS 15 (Intel, MacPorts) | PostgreSQL and DuckDB |
| macOS 26 (Apple silicon, Homebrew) | PostgreSQL, MariaDB, MySQL, SQLite and DuckDB |

SQL Server is tested on Linux only (D13). MariaDB, MySQL and SQLite are not
tested in the Intel macOS VM, because MacPorts has no driver for them, and an
Apple silicon Mac with Homebrew covers them.

Done:

- The project setup: the documents, the skills, the decisions, the lint
  configuration and the workflow.
- The driver: loading the driver manager with `purego`, the connector, the
  connection, prepared and direct statements with bound parameters, rows with
  chunked reads, transactions with isolation levels, cancellation through the
  context, and errors that carry the SQLSTATE.
- The types follow D18, and the few quirks of a driver follow D19.
- The `test/` module of D15. It runs the `dbmeta` fixture of each database and
  every `dbmeta` query that the database answers, round trips each kind of
  value through `dbimptest.RoundTrip`, and checks for leaked goroutines.

The CI workflow is written for all three systems (D21) and passes `actionlint`. It
has not run on a GitHub runner. The first run is the next step. See
[`BACKLOG.md`](BACKLOG.md).

## Running the tests here

Start the servers with `dbrun` from `dbmeta` and set one variable for each
database. The tests skip a database whose variable is empty.

```bash
cd ../dbmeta/test && DBMETA_OWNER_NAME=odbc go run ./cmd/dbrun start postgres mariadb mysql sqlserver
```

Run `go test ./...` in the `test` directory. The variables are `ODBC_POSTGRES`, `ODBC_MARIADB`, `ODBC_MYSQL`,
`ODBC_SQLSERVER`, `ODBC_SQLITE` and `ODBC_DUCKDB`. Each is a data source name for the driver
and names the driver library with the `driver` key, as in
`odbc+PostgreSQL+Unicode://postgres:pass@127.0.0.1:55009/postgres?driver=/usr/lib/psqlodbcw.so`.
`dbrun dsn <name>` prints the host, port and password.

## Running the tests on Windows and macOS

The VMs of the `vm` repository start with `podman start windows11` or
`podman start macos15`, and take ssh as `user` on ports 2222 and 2223. Their
host keys change when a VM is installed again, so use
`-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no`. The default shell
of Windows is PowerShell. Copy the repository over with `tar` and `scp`, and set
`CGO_ENABLED=0`. The macOS VM needs `GOCACHE` set to a folder of its own, because
the default one is not writable.

A VM cannot reach the ports of the `dbrun` containers. Forward them from the
host, as in `ssh -p 2222 -fN -R 55009:127.0.0.1:55009 user@127.0.0.1`, and the
VM sees the server on its own 127.0.0.1. Run one VM at a time, because each
uses 16 GB of memory.

## Notes for the next session

- The peer sessions `dbmeta` and `dbimp` gave the set up and the layout. The
  model survey of ODBC drivers by database mostly failed, so D10 records
  Ken's choice and not a measurement.
- DuckDB has no Arch package. Its driver is a zip on the GitHub releases of
  `duckdb/duckdb-odbc`, and the tests ran it from `~/.cache/odbc-test/duckdb`.
  On Windows it needs `odbc_install.exe /CI /Install`, because the driver manager
  there takes a registered name and not a path.
