# D2. One code base for Windows, macOS and Linux

Status: Decided.

The driver has one set of calls into the ODBC API and one small file per operating system that finds and opens the library.

## Reason

The ODBC functions are the same everywhere. The differences are the name of the library, how it is opened, and the size of `SQLWCHAR`. Keeping those in per system files keeps the rest of the driver free of build constraints.

## Settled

- The names tried. Windows has `odbc32.dll`. Linux has `libodbc.so.2`, then `libodbc.so`, from `unixODBC`. macOS has `libodbc.2.dylib` from `unixODBC`, with the Homebrew and MacPorts paths after it. A data source name can name another path with the `manager` key (D16).
- How the library is opened. `lib_unix.go` uses `purego.Dlopen`. `purego` has no `Dlopen` on Windows, but its function registration accepts the handle that `syscall.LoadLibrary` returns, and `lib_windows.go` does that. Both compile on every system. Both have run: CI tests Linux, macOS and Windows against the databases that each runner holds (D21).

## Open

- `SQLWCHAR` is two bytes under Windows and `unixODBC`, and four under `iODBC`. D20 settles this: the driver finds the size when it loads the manager.
- Whether the manager is optional. A driver can load the database driver library directly and skip the manager, but then it loses data source names. This is not planned.
