# D21. The CI jobs test the databases that each runner can hold

Status: Decided, and amends D13.

D13 gives each system its databases. The workflow in `.github/workflows/test.yml` follows it with three changes, each from what a runner offers.

| System | Servers | Embedded |
| --- | --- | --- |
| Linux | PostgreSQL, MariaDB, MySQL and SQL Server, in containers that `dbrun` starts | SQLite and DuckDB |
| macOS | PostgreSQL and MariaDB, installed with Homebrew | SQLite and DuckDB |
| Windows | PostgreSQL and MySQL, from the runner image | SQLite and DuckDB |

- macOS does not test MySQL, because the Homebrew formulas of MySQL and MariaDB conflict. The MariaDB Connector/ODBC serves both servers (D14), so the driver is still tested.
- Windows does not test a MariaDB server. The runner image has MySQL, and the same MariaDB driver connects to it.
- A test reads the URL of each server from `ODBC_<DATABASE>_URL` and the name or path of each driver from `ODBC_<DATABASE>_DRIVER`. On Linux the URL is the one that `dbrun` prints, from a pinned `dbmeta` commit, which is the commit of `v0.2.0` (D15). The macOS and Windows jobs start their servers themselves.
- Three drivers are downloads: DuckDB, the MariaDB driver for Windows and the SQLite driver for Windows. The workflow checks the SHA-256 of each before it runs one. The other drivers come from the package manager of the system.
- Installing the Microsoft driver on Linux sets `ACCEPT_EULA=Y`, which is how that package takes its license.

## Reason

`dbrun` starts containers, and only a Linux runner can run them (D13). A native server is the only choice elsewhere. The checksums mean a changed download fails the job, and a pinned `dbmeta` commit means the servers do not change under the tests.

## Found on the first runs

The first runs showed what the runner images hold, and the workflow follows them.

- `dbrun` prints a line that names the release before the JSON, so the Linux job keeps the JSON alone.
- PowerShell splits an option such as `-h127.0.0.1`, so the Windows job writes the long form of each option.
- The PostgreSQL that the Windows runner provides makes a cluster in WIN1252 by default, which cannot hold the test text. The job makes its cluster in UTF8.
- A process that a step starts ends with the step. The Windows job starts MySQL through WMI, so that the process belongs to no step.
- The driver manager of Windows opens the trace file when the trace is turned on, so the driver sets the file first (D23).
- MariaDB Connector/ODBC 3.1.15, the version of the Ubuntu package, lists no primary key of a MySQL 8 table. The catalog test knows it.
