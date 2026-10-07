# D15. The integration tests are a separate module that uses dbmeta fixtures

Status: Decided.

The integration tests live in `test/`, a module of their own. It imports `github.com/xo/dbmeta` for its fixtures, which build a schema with tables, keys, views, indexes and rows for each of the test databases. The root module does not import `dbmeta`, so D4 holds for it. D17 later added `dbimp` to the root module.

## Reason

`SELECT 1` proves a connection and little more. The fixtures create one of every object kind on each database, so the driver reads real types, NULLs, keys and views. They are SQL text and are kept in step with each product, which is work this project must not repeat.

A test module keeps the dependency out of the driver. A program that imports `odbc` does not download `dbmeta`.

## Rejected

- A fixture of our own for each database. It repeats `dbmeta` and drifts from it.
- Importing `dbmeta` in the root module. It adds a dependency that every user of the driver pays for.

## Notes

- `dbmeta` agrees with the split. It does not add a type fixture to itself. A fixture makes objects for metadata queries, and a round trip of values is a driver concern. This project writes its own table with one column per ODBC type.
- A fixture is `models/<db>/fixture`. `fixture.Everything.ResolveSetup(versions)` returns plain SQL steps, and a `dbmeta.VersionSet` is built without running a query. A step that this driver cannot run is a finding.
- `dbmeta` is at v0, so the test module pins a tag.
- `dbrun` is not tagged, so CI checks out `dbmeta` at a pinned commit and runs `go run ./cmd/dbrun` in its `test` folder.
