# D16. The data source name is a URL or an ODBC connection string

Status: Decided.

`sql.Open("odbc", dsn)` accepts two forms. A URL whose scheme is `odbc+<driver>`, as D12 describes, and anything else, which is taken to be an ODBC connection string and passed on as it is.

## Reason

`dburl` turns the URL form into a connection string before it opens a database, so a program that uses `dburl` passes the second form. A program that does not use it can pass the URL. Accepting both costs one `strings.Contains`.

## Details

- The driver name keeps its case, because `url.Parse` lowercases a scheme and the driver manager looks a name up by what the file registers.
- An instance in the path becomes `host\instance`.
- For a driver with `SQL Server` in its name the port is written `host,port`, because the Microsoft drivers ignore a `Port` key. `dburl` writes a `Port` key and the Microsoft driver then connects to the default port.
- The query key `driver` replaces the driver name, so it can be the path of a library, which is how the tests avoid registering a driver. The key `manager` is the path of the driver manager.
- A value with a character that ends it early is written in braces.

## Placeholders

The driver passes the query text on as it is, and every database tested takes `?`. Ken decided to assume `?` for now and to return to it later. `psqlODBC` rejects `$1`.
