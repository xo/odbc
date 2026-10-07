# D23. The driver exposes what a tool needs to know about the database

Status: Decided.

Ken asked which settings and functions the driver gives to Go code outside it. The driver exposes five things, which tools such as `usql` and `dbtpl` need and cannot get from `database/sql` alone. The reason for each is that ODBC has the answer and the standard library has no place to ask.

## What the driver exposes

- `odbc.Drivers(cfg)` and `odbc.DataSources(cfg)` list the drivers and the data source names that the driver manager knows, with the attributes of each driver. A tool uses them to offer a choice, and a test uses them to find the name of a driver. They take a `Config`, and only its `Manager` and `WChar` apply.
- `odbc.Conn` is an interface that the connection of the driver implements. A program reaches it through `sql.Conn.Raw`. Its methods `GetInfoString`, `GetInfoUint16` and `GetInfoUint32` wrap `SQLGetInfo`. They give the name and the version of the database, the identifier quote character and the rest of the information that ODBC defines. `dbmeta` can ask the database about itself and not guess from the name of the driver. The constants `InfoDBMSName` and the others in `conn.go` name the common types.
- `Config.OnWarning` is called with each informational diagnostic that a connection or a statement returns with a success code. SQL Server returns the text of a `PRINT` this way, and PostgreSQL returns a `NOTICE`. The driver dropped them before.
- `Config.TraceFile` turns on the trace of the driver manager and names the file. It is the first thing a person needs when a database driver misbehaves.
- `odbc.ClassError` and the constants `ErrConnection`, `ErrData`, `ErrIntegrity`, `ErrRollback`, `ErrSyntax` and `ErrTimeout`. `errors.Is(err, odbc.ErrIntegrity)` is true for an `Error` whose SQLSTATE is in that class. A class is the same in every database, so a caller tests for a duplicate key without knowing the database.

## Reason

These five are cheap, they have a clear user, and `TestGetInfo`, `TestDrivers`, `TestTrace`, `TestWarnings` and `TestErrorClasses` check them on the databases that the project tests.

## Left for later

- A fetch size, which means reading rows in blocks with `SQLBindCol`. Rows are read one at a time with one `SQLGetData` call per column, which is slow for a large result. A block read changes `rows.go` a great deal and needs its own decision. The option is `WithFetchSize`.
- A `Config.Location` that returns a `time.Time` in that location for a timestamp, and not the `dbimp.LocalDateTime` of D18. It reverses a choice in D18, so it waits for a consumer that asks.
- `WithMaxRows` and `WithNoScan` as options of their own. `WithParameter` reaches both today.
- The catalog functions of ODBC, `SQLTables`, `SQLColumns` and `SQLPrimaryKeys`. They are the portable way to read the metadata of a database that `dbmeta` has no model for, and they belong on `odbc.Conn`.

## Rejected

- A setter for a global setting. The driver then depends on state outside a `Config`.
- A hook that overrides the quirks of D19. A new quirk is a change to the code with a test that fails without it.
