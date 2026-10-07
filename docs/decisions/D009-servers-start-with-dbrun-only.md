# D9. Servers start with dbrun only

Status: Decided, and amended by D13.

A database for a test is started with `dbrun` from `dbmeta`, never with a container command typed by hand. The person or agent sets `DBMETA_OWNER_NAME` to `odbc`.

## Scope

D13 limits this to Linux and developer machines. A macOS or Windows CI job installs its server natively, because `dbrun` starts containers and those runners cannot run Linux ones.

## Reason

`dbrun` knows the images, the ports and how to tell that a server is ready, and it records who started each one. A server started by hand is one the next person cannot find or stop.

## Settled

`dbrun` starts `postgres`, `mariadb`, `mysql` and `sqlserver` as containers. `sqlite3` and `duckdb` are embedded, so it starts nothing for them. The ODBC driver library for each database is installed on the machine that runs the tests, and is not part of `dbrun`.
