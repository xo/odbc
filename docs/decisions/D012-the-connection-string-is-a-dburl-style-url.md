# D12. The connection string is a dburl style URL

Status: Decided.

The driver opens a connection from a URL of this form, the one `dburl` uses for the `odbc` scheme:

```
odbc+<driver>://<user>:<pass>@<host>:<port>/[<instance>]/<dbname>
```

## Reason

Ken chose it. A program that already uses `dburl` then passes the same URL to this driver, and a consumer such as `usql` needs no translation.

## Mapping

D16 says how each part becomes a key of the ODBC connection string, and how the driver finds the library for `<driver>`.
