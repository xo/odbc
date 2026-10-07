# D20. The driver finds the size of SQLWCHAR when it loads the manager

Status: Decided.

The driver supports a driver manager whose `SQLWCHAR` is 2 bytes, as UTF-16, and one whose `SQLWCHAR` is 4 bytes, as UTF-32. It finds out which when it loads the manager, and every wide string passes through `encode` and `decode` in `wchar.go`, which convert between Go strings and the width of the manager.

## The question

The wide functions, such as `SQLExecDirectW`, take text as arrays of `SQLWCHAR`. The size of that type is 2 bytes on Windows and in `unixODBC` unless it was built with `SQL_WCHART_CONVERT`, and 4 bytes in `iODBC` and in that build. The driver passes a pointer and a length counted in characters, so a wrong width reads garbage or overruns a buffer. Go converts both widths easily, with `unicode/utf16` and with `[]rune`. The hard part is knowing the width.

## The probe

On Windows the width is 2, and nothing is probed. Elsewhere the driver allocates an environment and calls `SQLSetEnvAttr` with an attribute that no manager knows, so that the manager records a diagnostic of its own. It then calls `SQLGetDiagRecW` into a state buffer of 24 bytes, which holds six characters of either width, because that call takes no length for the state. A SQLSTATE is five characters of ASCII, so the bytes show the width: each pair of bytes is a printable character for 2, and each four for 4. The probe uses the byte order of the host. It fails with a clear error when the bytes fit neither width or both, and when the manager records no diagnostic.

The probe finds the width of the manager. The manager converts for the database driver, so it is the width the application sees.

## The escape hatch

The key `wchar` of the data source name, with the value 2 or 4, sets the width and skips the probe, for a manager that the probe cannot read.

## Reason

Gemini, DeepSeek and Qwen were asked. Gemini reviewed the probe and found it sound, with the state buffer as the one thing to size for the wider case. Qwen suggested a build time define, which does not apply to a driver with no cgo, and narrow functions with UTF-8, which on Windows means the legacy code page and not UTF-8. The narrow functions are rejected. A wide function never uses UTF-8, so UTF-8 cannot be detected and need not be.

## Rejected

- Assuming 2 bytes. It fails on `iODBC`, which is the manager on part of macOS and Linux.
- Refusing a manager of 4 bytes. It is less code and gives an `iODBC` user nothing.

## Open

The probe has run against `iODBC` from Homebrew on Apple silicon. It found a width of 4, the manager worked with that width, and forcing a width of 2 broke the connection. The strings that left the driver were wrong all the same: the Homebrew database drivers are built for `unixODBC`, whose `SQLWCHAR` is 2 bytes, and `iODBC` passes its 4-byte text to them without conversion. That is a mismatch between the manager and the database driver, and nothing in this driver can fix it. An `iODBC` user needs database drivers built for `iODBC`.
