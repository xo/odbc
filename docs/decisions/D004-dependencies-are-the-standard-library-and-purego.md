# D4. Dependencies are the standard library and purego

Status: Decided, and amended by D17.

The module imports the standard library and `github.com/ebitengine/purego`. Anything else needs Ken to agree first.

## Reason

A driver is imported by many programs, and each dependency it adds is one that every one of them must accept. `purego` is the one the design needs.

## Consequence

`depguard` in `.golangci.yml` holds the list. D17 later added `dbimp` and the `apd` package that comes with it.
