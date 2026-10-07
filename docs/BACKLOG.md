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

- MariaDB Connector/ODBC before 3.2 lists no primary key of a MySQL 8 or later
  table, because it compares `COLUMN_KEY` with the text `pri` and MySQL compares
  it case sensitively. The Ubuntu package is 3.1.15. `TestCatalog` logs it.

- The SQLite ODBC driver converts the text of a statement to a narrow string. On
  macOS and in a container with no locale it cuts each character to its low
  byte, so non-ASCII text in a literal is wrong. An argument is fine, and the
  runner of CI converts it correctly. The round trip test skips that case.
- A manager of 4 bytes (`iODBC`) works, but the Homebrew database drivers are
  built for `unixODBC` and read its text wrongly (D20). Test with a driver built
  for `iODBC` if one exists.

### CI

- Check the license of the Microsoft ODBC driver. The Linux job installs it with
  `ACCEPT_EULA=Y`, which accepts the terms for the project (D21).
- Test a database through the MySQL Connector/ODBC of Oracle, which the tests do
  not run (D14).
- Test on Apple silicon with MacPorts. The CI job uses Homebrew.
