# D1. Pure Go with purego and no cgo

Status: Decided.

The driver is written in pure Go. It loads the ODBC driver manager at run time with `github.com/ebitengine/purego`, and no file in the module imports `"C"`.

## Reason

A cgo driver needs a C compiler and the ODBC headers on the machine that builds it. It cannot be cross compiled with `go build` alone, and it is the part of a Go program that most often breaks a release pipeline. Loading the library at run time moves that cost to the machine that runs the program, which has to have an ODBC driver anyway.

## Rejected

- cgo against `unixODBC`. This is what the other Go ODBC drivers do, and it is the reason for this project.
- A wrapper around an existing cgo driver. It keeps every cost of cgo.

## Consequence

`CGO_ENABLED=0 go build ./...` is part of CI on every operating system. The race detector needs cgo, so it runs in its own job on Linux.
