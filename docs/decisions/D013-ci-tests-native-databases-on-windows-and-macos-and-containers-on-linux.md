# D13. CI tests native databases on Windows and macOS and containers on Linux

Status: Decided, amends D9, and amended by D21.

Ken chose this layout for the test jobs in GitHub Actions:

| System | Servers | Embedded |
| --- | --- | --- |
| Linux | PostgreSQL, MySQL, MariaDB and SQL Server, in containers that `dbrun` starts | SQLite and DuckDB |
| macOS | PostgreSQL, MySQL and MariaDB, installed natively on the runner | SQLite and DuckDB |
| Windows | PostgreSQL, MySQL and MariaDB, installed natively on the runner | SQLite and DuckDB |

SQL Server is tested on Linux only.

## Reason

GitHub starts a service container only on a Linux runner. The Windows runner runs Windows containers and the macOS runner has no Docker, so a Linux database image cannot start on either. A native install is the only server a macOS or Windows job can have. SQLite and DuckDB are libraries and need no server, so they run everywhere. PostgreSQL is the baseline, and it runs on all three systems.

The ODBC drivers are installed natively on every system, from the system package manager or from the vendor, and never committed to the repository. A test reads the path of each driver library from an environment variable. It connects without a data source name. It skips when the variable is empty.

## Rejected

- A published container image of the drivers. It helps Linux alone, and the license of the SQL Server driver can forbid republishing it.
- Testing the server databases on Linux only. It leaves the part that differs between systems, which is loading the library and calling it, untested against a real server on macOS and Windows.

## Consequence

CI has two ways to start a server, and the test code does not care which one ran. The tests read a connection string from an environment variable. D9 applies to Linux and to a developer machine, and a macOS or Windows job installs the server itself.

## Open

Whether the runner images ship the servers, and under which names, is not checked. The install steps for each driver on each system are in `docs/BACKLOG.md`.
