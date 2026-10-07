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

## Open

Nothing in the macOS and Windows jobs has run on a GitHub runner. The Linux job has not either. The first run will show what the images hold, and each such fact is to be fixed in the workflow and not guessed here.
