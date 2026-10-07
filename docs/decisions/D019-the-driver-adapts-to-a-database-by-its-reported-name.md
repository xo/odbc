# D19. The driver adapts to a database by the name it reports

Status: Decided.

The driver asks the database for its name with `SQLGetInfo` when it connects, and a few places change what they do by that name. It also reads the type name of a column, which the ODBC driver reports beside the SQL type.

## What changes

- A boolean parameter is bound with the SQL type `SQL_BIT`, except for DuckDB, which refuses that type for a parameter. For DuckDB it is `SQL_TINYINT`. PostgreSQL refuses `SQL_TINYINT` for a boolean, so no one type serves both.
- A column whose type name is `bool` or `boolean` is read as a `bool`, even when the driver reports its SQL type as text. DuckDB does this, and `psqlODBC` does it unless the option `BoolsAsChar` is 0. The text `t`, `true`, `1`, `y` and `yes` are true, and the opposites are false.

## Reason

ODBC drivers differ in what they accept, and the SQL type cannot say it. A table of the few cases, found by the tests, is smaller than a setting for each. A user of `psqlODBC` no longer needs `BoolsAsChar=0` to read a boolean.

## Rejected

- Binding a boolean as text. MySQL turns the text `true` into 0 with a warning.
- A key in the data source name for each quirk. The user has to know each one.

## Open

Each new database can add a case. The list is the code in `stmt.go` and `rows.go`, and a case needs a test that fails without it.
