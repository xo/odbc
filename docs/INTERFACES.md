# Interfaces of the standard library

`database/sql` asks a driver for many optional interfaces. This file lists each
one, whether this driver implements it, and what it does. A test or a decision
backs each line. D27 holds the choices that were made for the gaps.

## Driver, connector and connection

| Interface | State | Notes |
| --- | --- | --- |
| `driver.Driver`, `driver.DriverContext` | implemented | `Open` refuses, because a connection needs a context. `OpenConnector` loads the driver manager. |
| `driver.Connector`, `io.Closer` | implemented | `Close` frees the ODBC environment. |
| `driver.Conn` | implemented | `Begin` and `Prepare` have no context, so they call the same code as the context forms. |
| `driver.ConnBeginTx` | implemented | Isolation levels work. A read only transaction fails with `dbimp.ErrNotSupported`, because three of six databases accept the ODBC read only mode and still write (D22). |
| `driver.ConnPrepareContext` | implemented | |
| `driver.ExecerContext`, `driver.QueryerContext` | implemented | A statement with no arguments goes to `SQLExecDirect`. |
| `driver.Execer`, `driver.Queryer` | not implemented | These are the forms without a context. `database/sql` uses the context forms when they exist, so they never run. |
| `driver.Pinger` | implemented | It asks the driver whether the connection is dead. For PostgreSQL, MySQL, MariaDB, SQL Server, SQLite and DuckDB it also sends `SELECT 1`, so a server that went away shows at once (D27). |
| `driver.Validator` | implemented | It asks the driver whether the connection is dead, and nothing more, because `database/sql` calls it each time a connection returns to the pool. |
| `driver.SessionResetter` | implemented | It ends a transaction that nobody ended, and reports a lost connection with `driver.ErrBadConn` (D27). |
| `driver.NamedValueChecker` | implemented | It keeps the options of D22, refuses a named argument, and sends a `uint64` with the high bit set as its decimal text (D27). |
| `driver.ColumnConverter` | not implemented | `NamedValueChecker` replaces it. It needs the type of each parameter, and `SQLDescribeParam` is not reliable across drivers. |

## Statements and results

| Interface | State | Notes |
| --- | --- | --- |
| `driver.Stmt` | implemented | `NumInput` is -1. A count is wrong, because the options of a statement are arguments that `Resolve` removes later. |
| `driver.StmtExecContext`, `driver.StmtQueryContext` | implemented | |
| `driver.Result` | implemented | `RowsAffected` works. `LastInsertId` works with the option `WithLastInsertID` for MySQL, MariaDB and SQLite, and fails with `dbimp.ErrNotSupported` for the others, where `RETURNING` or `OUTPUT` reads the id (D27). |

## Rows

| Interface | State | Notes |
| --- | --- | --- |
| `driver.Rows` | implemented | |
| `driver.RowsColumnScanner` | implemented | It hands each value to `dbimp.Assign`, so a decimal and the civil types reach the destination that the caller names (D18, D26). |
| `driver.RowsNextResultSet` | implemented | `HasNextResultSet` is always true, because ODBC cannot tell without moving on. `NextResultSet` returns `io.EOF` when there is none. A SQL Server batch of two statements returns both results. |
| `driver.RowsColumnTypeDatabaseTypeName` | implemented | It is the type name that the database driver reports, or the name of the ODBC type code when it reports none. |
| `driver.RowsColumnTypeLength` | implemented | A type with no limit reports `math.MaxInt64`, as the contract says (D27). |
| `driver.RowsColumnTypePrecisionScale` | implemented | It answers for `DECIMAL` and `NUMERIC` only, as the contract says. |
| `driver.RowsColumnTypeNullable` | implemented | A driver that does not know answers `ok` false. |
| `driver.RowsColumnTypeScanType` | implemented | Types follow D18. |

## Types that `database/sql` reads from a value

| Interface | State | Notes |
| --- | --- | --- |
| `driver.Valuer`, `sql.Scanner` | implemented by the types that the driver returns | The civil types of `dbimp` and `*apd.Decimal` have both. |
| the decimal pair `Decompose` and `Compose` | implemented | A decimal is an `*apd.Decimal`, which has `Decompose`. A destination with `Compose` gets the parts and not text (D26). |

## What a program reaches through `sql.Conn.Raw`

`odbc.Conn` has `GetInfoString`, `GetInfoUint16`, `GetInfoUint32`, `Tables`,
`Columns` and `PrimaryKeys` (D23, D24).
