# D8. Lint with default all and a reason for each exception

Status: Decided.

`.golangci.yml` enables every linter and disables some, each with its reason written beside it. CI pins the version of `golangci-lint`.

## Reason

A new linter in a release then gets a look. The pinned version means an upgrade cannot fail a build that changed nothing.

## Rule

A linter that makes idiomatic Go worse is disabled with a reason. Only a real defect gets a change to the code.
