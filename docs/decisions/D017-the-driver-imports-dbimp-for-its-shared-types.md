# D17. The driver imports dbimp for its shared types

Status: Decided, and amends D4.

The root module imports `github.com/ebitengine/purego` and `github.com/xo/dbimp`. `dbimp` brings `github.com/cockroachdb/apd/v3`, which the driver also imports to return a decimal. Nothing else is allowed.

## Reason

Ken asked for the same types as the sibling drivers. `dbimp` defines the civil types that Go lacks. They are a date, a time of day, a timestamp with no zone and an interval. `dbimp` also returns `*apd.Decimal` for an exact decimal. A program that reads values from several `xo` drivers then gets one Go type for one kind. Copied types drift.

`dbimp` has one dependency, `apd`, so the import adds one module to the graph.

## Rejected

- Copying the types. They drift from `dbimp`.
- Putting this driver inside the `dbimp` repository. `dbimp` and the models agree that it does not fit. Its dependency rule allows `apd` only. Its rule 8 says Linux only. Its gates assume recorded HTTP exchanges. It numbers its decisions on its own. This driver stays a repository of its own and imports `dbimp` and `dbimptest`.

## Risk

`dbimp` is at version 0.11, so its types can still change. Pin the tag and read its change log before an upgrade.
