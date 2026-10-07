# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

The workflow passes on GitHub for all three systems (D21), with the databases that
each runner holds. The `test` module also passes locally on Linux against all six
databases, on Windows 11 against five, and on macOS against PostgreSQL, MariaDB,
MySQL, SQLite and DuckDB.

| System | Passes against |
| --- | --- |
| Linux | PostgreSQL, MariaDB, MySQL, SQL Server, SQLite and DuckDB |
| Windows | PostgreSQL, MySQL, SQLite and DuckDB in CI, and MariaDB in the VM |
| macOS | PostgreSQL, MariaDB, SQLite and DuckDB in CI, and MySQL on a Mac with Homebrew |

SQL Server is tested on Linux only (D13).

The driver has the options of D22, the tools of D23 and the block fetch, catalog
and location of D24. The first release is v0.1.0.

Next: see [`BACKLOG.md`](BACKLOG.md).

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
