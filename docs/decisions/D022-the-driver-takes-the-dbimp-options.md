# D22. The driver takes the dbimp options

Status: Decided.

The driver takes per-statement options in the way that every `dbimp` driver does (dbimp D109). The type is `type Option = dbimp.Option[options]`. An option comes from the context through `odbc.WithOptions`, then from an argument of the statement, and a later one wins. `CheckNamedValue` keeps an `Option` so that `dbimp.Resolve` takes it out of the arguments, and the other arguments are numbered again from 1.

```go
db.QueryContext(ctx, "SELECT ... WHERE id = ?", odbc.WithTimeout(5*time.Second), id)
```

## The options

- `WithTimeout(d)` sets the statement attribute `SQL_ATTR_QUERY_TIMEOUT`. ODBC counts whole seconds, so a part of a second rounds up. A negative value fails with `dbimp.ErrInvalidValue`, and zero asks for nothing.
- `WithReadonly(v)` always fails with `dbimp.ErrNotSupported` when `v` is true, and asks for nothing when it is false. The ODBC access mode is a hint. The tests showed that PostgreSQL, SQL Server and DuckDB accept it and still write.
- `WithDatabase(name)` sets `SQL_ATTR_CURRENT_CATALOG` for the statement and puts it back when the statement ends, after its rows are closed. It reads the catalog back, because `psqlODBC` accepts the change and stays where it was. A statement of a transaction cannot name another catalog.
- `WithParameter(name, value)` sets a statement attribute by name. The names are `query_timeout`, `max_rows`, `no_scan`, `max_length`, `cursor_type`, `concurrency` and `keyset_size`, and the value is an integer or a bool. An unknown name fails with `dbimp.ErrInvalidValue`.

## The rule that governs them

A caller must never believe that a limit holds when it does not. An option that the database driver rejects, or that it answers with the state `01S02` (option value changed), fails with an error that wraps `dbimp.ErrNotSupported` and the diagnostic. The driver also does not trust an acceptance. `TestTimeoutIsEnforced` runs a statement that sleeps and checks that the timeout stops it. It stops it on PostgreSQL, MariaDB and SQL Server. The MariaDB driver refuses it for a MySQL server.

The same sentinel now covers the other things the driver refuses: a named argument, a read only transaction and an unknown isolation level.

## What stays different from the other drivers

- The DSN has no keys for the options. The DSN keys of this driver pass through to the ODBC connection string (D16), and a reserved name such as `timeout` can clash with a key of a database driver. An option for a key of the DSN, as dbimp D109 asks, has nothing to attach to.
- `ParseDSN` does not refuse unknown keys, for the same reason. It returns a `Config` by value, and `Config` has no `FormatDSN`, because a connection string and the key `manager` cannot be written back as a URL.
- `NewConnector` returns an error, because it loads a library. The `couchbase` connector does not.

## Rejected

- Setting `SQL_ATTR_ACCESS_MODE` for `WithReadonly`. Three of the six databases accept it and write.
- A `WithParameter` that sets a key of the request. ODBC has no request body, so it names a statement attribute.

## Open

`WithReadonly` can work on a database that has a statement for it, chosen by the name the database reports (D19), such as a read only transaction on PostgreSQL. DuckDB and SQLite accept `WithTimeout`, and the tests have no statement that sleeps to show that they enforce it.
