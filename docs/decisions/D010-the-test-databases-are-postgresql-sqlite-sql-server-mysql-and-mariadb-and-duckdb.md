# D10. The test databases are PostgreSQL, SQLite, SQL Server, MySQL and MariaDB, and DuckDB

Status: Decided, and amended by D14.

The driver is tested against PostgreSQL, SQLite, SQL Server, MySQL, MariaDB and DuckDB, each through its own ODBC driver. PostgreSQL is the baseline. No other database is tested.

## Reason

Ken chose these. PostgreSQL has a maintained ODBC driver for all three systems. SQLite and DuckDB need no server. SQL Server has the one commercial driver that many programs use. MySQL and MariaDB are the two other common servers.

## Rejected

Oracle, Db2, Snowflake, SAP HANA, Teradata and the rest. They are heavy to install or need an account, and Ken ruled them out of the test plan.

## Note

A survey of which databases ship ODBC drivers on each system was started with several models. Most requests failed, and the priority above is Ken's choice and not a measured result.
