# D20. The driver finds the size of SQLWCHAR when it loads the manager

Status: Decided.

The driver supports a driver manager whose `SQLWCHAR` is 2 bytes, as UTF-16, and one whose `SQLWCHAR` is 4 bytes, as UTF-32. The driver finds out which when it loads the manager. Every wide string passes through `encode` and `decode` in `wchar.go`. They convert between Go strings and the width of the manager.

## The question

The wide functions, such as `SQLExecDirectW`, take text as arrays of `SQLWCHAR`. On Windows the type is 2 bytes. In `unixODBC` it is 2 bytes unless that was built with `SQL_WCHART_CONVERT`. In `iODBC`, and in that build, it is 4 bytes. The driver passes a pointer and a length counted in characters. A wrong width reads garbage or overruns a buffer. Go converts both widths easily, with `unicode/utf16` and with `[]rune`. The hard part is knowing the width.

## The probe

On Windows the width is 2, and nothing is probed. Elsewhere the driver allocates an environment. It calls `SQLSetEnvAttr` with an attribute that no manager knows, so the manager records a diagnostic of its own. It then calls `SQLGetDiagRecW` into a state buffer of 24 bytes. That holds six characters of either width, because the call takes no length for the state.

A SQLSTATE is five characters of ASCII, so the bytes show the width. With a width of 2, each pair of bytes is a printable character. With a width of 4, each group of four bytes is one. The probe uses the byte order of the host. It fails with a clear error when the bytes fit neither width or both. It also fails when the manager records no diagnostic.

The probe finds the width of the manager. The manager converts for the database driver, so that is the width the application sees.

## The escape hatch

The key `wchar` of the data source name has the value 2 or 4. It sets the width and skips the probe. Use it for a manager that the probe cannot read.

## Reason

Gemini, DeepSeek and Qwen were asked. Gemini reviewed the probe and found it sound. It said to size the state buffer for the wider case. Qwen suggested a build time define, which does not apply to a driver with no cgo. Qwen also suggested the narrow functions with UTF-8. On Windows the narrow functions use the legacy code page, so they are rejected. A wide function never uses UTF-8, so the driver cannot detect UTF-8 and does not need to.

## Rejected

- Assuming 2 bytes. It fails on `iODBC`, which is the manager on part of macOS and Linux.
- Refusing a manager of 4 bytes. It is less code and gives an `iODBC` user nothing.

## Open

The probe has run against `iODBC` from Homebrew on Apple silicon. It found a width of 4, and the manager worked with that width. A forced width of 2 broke the connection. The strings that left the driver were still wrong. The Homebrew database drivers are built for `unixODBC`, where `SQLWCHAR` is 2 bytes, and `iODBC` passes its 4-byte text to them without conversion. The manager and the database driver do not match, and nothing in this driver can fix that. An `iODBC` user needs database drivers built for `iODBC`.
