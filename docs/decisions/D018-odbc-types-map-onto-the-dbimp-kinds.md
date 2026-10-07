# D18. ODBC types map onto the dbimp kinds

Status: Decided.

A column has the Go type that `dbimp` gives its kind, chosen by the SQL type that the driver reports for the column. A NULL is `nil`.

| ODBC type | Kind | Go type |
| --- | --- | --- |
| `SQL_TINYINT`, `SQL_SMALLINT`, `SQL_INTEGER`, `SQL_BIGINT` | integer | `int64` |
| unsigned `SQL_BIGINT` | unsigned integer | `uint64` |
| `SQL_BIT` | boolean | `bool` |
| `SQL_REAL`, `SQL_FLOAT`, `SQL_DOUBLE` | float | `float64` |
| `SQL_DECIMAL`, `SQL_NUMERIC` | decimal | `*apd.Decimal` |
| `SQL_CHAR`, `SQL_VARCHAR`, `SQL_LONGVARCHAR` and the wide forms | string | `string` |
| `SQL_BINARY`, `SQL_VARBINARY`, `SQL_LONGVARBINARY` | binary | `[]byte` |
| `SQL_TYPE_DATE` | date | `dbimp.Date` |
| `SQL_TYPE_TIME`, SQL Server `time` | time of day | `dbimp.LocalTime` |
| `SQL_TYPE_TIMESTAMP` | local timestamp | `dbimp.LocalDateTime` |
| SQL Server `datetimeoffset` | timestamp | `time.Time` |
| `SQL_GUID` | string | `string`, in the canonical text of the database |
| any other type | other | the text of the server |

## Reason

dbimp D135 says a value has the Go type that fits the type its database names. ODBC names a type for every column, so no value is guessed from its text.

An ODBC timestamp has no zone, so it is a `dbimp.LocalDateTime` and the caller names the location. The driver used to return a `time.Time` in UTC, which gave a zone the database did not state.

A decimal is `*apd.Decimal` and never a float, so no digit is lost.

## GUID

A `SQL_GUID` is not always a UUID, so it is a `string` and not a `uuid.UUID` or a `[16]byte`. Gemini and DeepSeek both said so, and Qwen disagreed without a reason. The ODBC `SQLGUID` structure is the Windows form, with its first three fields in little-endian order, and SQL Server's `uniqueidentifier` uses it. PostgreSQL, MariaDB and DuckDB store the RFC 4122 order. A GUID also need not carry the version and variant bits of RFC 4122.

The driver reads a GUID as text, and the driver manager and the database settle the byte order. It never reads `SQL_C_GUID`, because the bytes it returns differ by database and a caller cannot tell which. A caller who wants a UUID passes the `string` to the `uuid` package of its choice.

A GUID argument is a `string` or a `driver.Valuer` that returns one, such as a `uuid.UUID`. The driver binds it as text, and the database converts it. A database that refuses the conversion shows in the round trip test.

## Open

- A database that reports a time with a zone as `SQL_TYPE_TIMESTAMP` loses its zone. PostgreSQL `timestamptz` does this through `psqlODBC`. The value is the time in the zone of the session.
- JSON columns arrive as text, so they are `string` and not the decoded value that `dbimp` gives a JSON kind.
- Interval types are text.
