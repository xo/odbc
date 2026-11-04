# D28. The path of a URL means what the driver needs

Status: Decided, and amends D16.

D16 said that the path of the URL is the database name, or the instance and then the database name, for every driver. That broke a file database. Ken ran `usql odbc+sqlite://home/ken/.local/share/dbmeta/embedded/northwind.db` on 2026-11-04 and found an empty `northwind.db` in the current directory.

## What happened

The URL has the host `home` and the path `/ken/.local/share/dbmeta/embedded/northwind.db`. `dburl` passes a URL of the scheme `odbc` to the driver unchanged. The driver cut the path at its last slash, so the instance was `ken/.local/share/dbmeta/embedded` and the database was `northwind.db`. The connection string held `Database=northwind.db`, and the SQLite ODBC driver took that as a path in the current directory. SQLite makes a file that does not exist. Three parts each did what they were told, and the fault is in the rule of D16, so it is a fault of this driver.

## The rule now

What the path means depends on the driver:

- A driver with `sql server` in its name takes the path as the instance and then the database, as before: `host/instance/database` becomes `Server=host\instance` and `Database=database`.
- A driver with `sqlite` or `duckdb` in its name takes the path as the path of the file. The host must be empty or `localhost`. Any other host is an error that says the host can be the start of a path that lost a slash. No file is made.
- Every other driver takes the whole path, without its first slash, as the database name. A name with a slash in it stays whole.

The forms that work through `dburl` and `usql` for a file database are `odbc+SQLite3://localhost/home/ken/a.db` and `odbc+SQLite3:///?Database=/home/ken/a.db`. `dburl` refuses `odbc+sqlite:///home/ken/a.db` with "missing driver", which is a rule of `dburl` and not of this driver.

## Reason

The path of a URL for a file database is a path, and a rule that was made for a server cannot also serve it. Refusing a host is better than guessing the slash that is missing, because a wrong guess opens or makes the wrong file. The error costs a retype and the guess can cost a file.

## Tests

`TestParseDSN` holds each form above, and the two URLs that must fail. The run through `usql` here made no file in the current directory for the refused URL, and read the file for the `localhost` form and for the `Database` key.

## Open

Other file databases, such as the Microsoft Access and Excel drivers, are not named, so they take the default rule. The names are a list in `dsn.go` and a new one needs a test.
