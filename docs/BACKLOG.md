# Backlog

This document lists work that is known and not done, and the questions that
are open for Ken. A decision is not a backlog item. It goes in
[`decisions/`](decisions/README.md). When an item here is done, delete it, and
record in `decisions/` anything that was decided on the way.

## Questions for Ken

None open.

## Work

### Tests

- Add the types that the round trip does not cover yet. They are a literal for
  binary values, SQL Server `datetimeoffset`, the SQL Server variant type, and
  an interval.
- Run `dbimptest.TypeTable` and `InterfaceTable` to write the type table and the
  interface table into `docs/`.
- Write a contract test and a fuzz test for `ParseDSN`.

### Driver

- Make `WithReadonly` work for a database that has a statement for it, chosen by
  the name the database reports (D22).
- Report `driver.ErrBadConn` only when a statement did not reach the server.
  `ResetSession` and `Ping` return it today, and nothing else does.
- Read a PostgreSQL `timestamptz` with its zone. `psqlODBC` reports it as a
  timestamp with no zone.

### Known limits

- The Homebrew SQLite ODBC driver cuts each character of a statement to its low
  byte. Non-ASCII text in a literal is wrong on macOS. An argument is fine. The
  round trip test skips that case there.
- A manager of 4 bytes (`iODBC`) works, but the Homebrew database drivers are
  built for `unixODBC` and read its text wrongly (D20). Test with a driver built
  for `iODBC` if one exists.

### CI


- Run the workflow on GitHub and fix what the runner images show (D21). Nothing in
  it has run there.
- Where the drivers come from, as found in the VMs: `psqlODBC`, Go and the
  PostgreSQL ODBC are on `winget`. MariaDB Connector/ODBC 3.2.9 is an MSI on
  `dlm.mariadb.com`. The SQLite ODBC driver is `sqliteodbc_w64.exe` on
  ch-werner.de. DuckDB is a zip on the `duckdb/duckdb-odbc` releases, with
  `osx-universal` and `windows-amd64` builds. MacPorts has `unixODBC` and
  `psqlODBC` for macOS.
- Write the install steps for each ODBC driver on each system, with the
  environment variable that names the library. Check that the macOS and
  Windows runners ship PostgreSQL, MySQL and MariaDB. Check that each driver has
  a macOS arm64 build and a 64-bit Windows build.
- Check the license of the Microsoft ODBC driver, and the flags that accept it
  in CI on each system.
