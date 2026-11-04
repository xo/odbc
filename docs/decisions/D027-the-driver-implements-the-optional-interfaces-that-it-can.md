# D27. The driver implements the optional interfaces that it can

Status: Decided.

Ken asked on 2026-11-04 for as many of the optional interfaces of the standard library as the driver can implement. `docs/INTERFACES.md` lists every one with its state. This decision holds what was added, and what stays out and why.

## Added

- `RowsColumnTypeLength` returns `math.MaxInt64` for a type with no limit, as the contract says. A long type, a size of zero and the types that the databases name as unbounded, such as `text` and `bytea`, are unbounded. Before, `psqlODBC` gave 8190 for `text`, and SQL Server gave 0 for `nvarchar(max)`.
- `NamedValueChecker` sends a `uint64` or a `uint` with the high bit set as its decimal text. `database/sql` refuses such a value on its own. The database converts the text, as for the largest `uint64` into a `decimal(20,0)`.
- `Result.LastInsertId` works with the option `WithLastInsertID(true)`, for the databases that have a statement that reads the value of the session: `LAST_INSERT_ID()` for MySQL and MariaDB, and `last_insert_rowid()` for SQLite. The statement runs after the insert on the same connection. Without the option, and for the other databases, the method returns an error that wraps `dbimp.ErrNotSupported`.
- `Pinger` sends `SELECT 1` for the databases where that is a statement, so a connection that the server ended fails the next `Ping` at once. `Validator` does not, because `database/sql` calls it each time a connection returns to the pool.
- `SessionResetter` ends a transaction that nobody ended.

## A fault found on the way

The connection never recorded that a transaction was open, so the rule of D22, that a statement of a transaction cannot name another catalog, did not work. The connection records it now, and `TestWithDatabaseInATransaction` checks it. The change also puts the isolation level back when a transaction cannot begin.

## Left out

- `LastInsertId` for SQL Server and PostgreSQL. On SQL Server 2025 with ODBC Driver 18.7, a second statement `SELECT SCOPE_IDENTITY()` returned the right id after an insert with no parameter, and an older id after an insert with a parameter, because that insert runs in a scope of its own. `@@IDENTITY` was right in both cases, but it gives the id of a trigger when a trigger inserts into another table. `lastval()` of PostgreSQL returns an older value when the insert used no sequence. A wrong id is worse than none. `OUTPUT` and `RETURNING` read the id correctly in the statement itself. `INSERT ...; SELECT SCOPE_IDENTITY()` in one batch also works and returns the id in the second result set, so a later change can support SQL Server by adding that statement to the batch.
- `ColumnConverter`, `Execer` and `Queryer`, for the reasons in `docs/INTERFACES.md`.
- `NumInput`. A count of the placeholders does not match the arguments, because the options of a statement are arguments.
- A true `HasNextResultSet`. ODBC cannot say without moving to the next result.
