# D24. The driver fetches in blocks, reads the catalog and takes a location

Status: Decided.

D23 left four things for later. The driver now has all of them, and two options that D22 reached only by name.

## Fetch size

`WithFetchSize(n)` reads the rows of a result n at a time. The driver sets `SQL_ATTR_ROW_ARRAY_SIZE`, binds a buffer and an array of indicators to each column with `SQLBindCol`, and fills a block of rows with one call of `SQLFetch`. On the test databases it is two to four times faster than one row at a time for a result of about 1200 rows, and every value is the same in both reads (`TestFetchSize`).

A fetch size is a hint about speed and not a limit, so the driver never fails a statement because it cannot honor one. If any column cannot be bound, the rows are read one at a time all the same. A column cannot be bound when its length is unknown or longer than 4000 characters or 8000 bytes, which includes every long text and long binary type. The same happens when the database driver refuses the setting.

A value that does not fit the buffer of its column is an error that wraps `odbc.ErrTruncated`, and never a value cut short. The driver reads the indicator of each value to find it. SQLite keeps the text of a `varchar(5)` column whatever its length, and `TestFetchSizeNeverCutsAValue` shows the error there. A caller who sees it reads the result without `WithFetchSize`.

One plan per column (`plan.go`) now serves both reads. It names the C type, the size and the conversion to a Go value, so a block read and a read with `SQLGetData` give the same Go value.

## Catalog functions

`odbc.Conn` has `Tables`, `Columns` and `PrimaryKeys`, which wrap `SQLTables`, `SQLColumns` and `SQLPrimaryKeys`. They return `TableInfo`, `ColumnInfo` and `PrimaryKeyInfo`. An empty argument means "not given", and the name arguments are patterns, as in ODBC. The tests read a table with a primary key from all six databases. The DuckDB driver lists no primary key, and the test logs that.

## Location

`Config.Location` makes a timestamp a `time.Time` in that location, and sends a `time.Time` as its time in that location. Without it a timestamp stays a `dbimp.LocalDateTime`, as D18 says. A caller chooses the Go type, and the database still stores a wall clock time with no zone.

## Two options of their own

`WithMaxRows(n)` limits the rows of a result, with `SQL_ATTR_MAX_ROWS`. `WithNoScan(true)` turns off the escape sequences of ODBC, with `SQL_ATTR_NOSCAN`. Both were reachable through `WithParameter`.

## Reason

These were the second group that D23 named. A tool reads a large result and the metadata of a database that `dbmeta` has no model for, and a program that works in one zone wants a `time.Time`.

## Rejected

- A fetch size that fails when a column cannot be bound. It is a hint, and a failure makes a caller try a second time without it.
- Cutting a value to the buffer. A caller then stores a wrong value without a word.
