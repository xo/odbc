# D25. An ODBC error says whether the credential was refused

Status: Decided.

The `usql` session asked on 2026-11-02 for one way to ask a driver error whether the database refused the credential. Its W66 is the work on its side. Ken chose the contract: a method `AuthenticationRefused() bool` on the error type of the driver. There is no sentinel and no shared package. `usql` declares the interface on its own side, as the standard library does for `Timeout()` of `net.Error`. This driver imports nothing from `usql`.

The `dbimp` drivers answer `errors.Is(err, dbimp.ErrAuthentication)` (dbimp D197). The `dbimp` version of this module does not have it. This driver implements the method of `usql` and nothing more.

## The rule

True means that asking the user for the password again can cure the failure. `*odbc.Error` returns true for these:

- A wrong password, a user that the database does not know, or both. Most servers do not tell the last two apart.
- A password that the server needs and the client did not send.

It returns false for these:

- A refusal for lack of a permission (class 42).
- A locked account and an expired password.
- A failure of the network, of TLS or of a timeout (class 08 or HYT).
- A database that does not exist, and a driver that is not registered (`IM002`).

The method answers true when this record or any record of `Next` says so. The records of one call belong together. A caller that uses `errors.As` gets only the first record. `Unwrap` returns the other records, so a caller that walks the tree, as `usql` does, finds them too. Wrapping with `%w` or `errors.Join` keeps the method visible. Wrapping with `%v` loses it.

## What a record says

- SQLSTATE class 28 is "invalid authorization specification". MariaDB and MySQL send 28000 with the native code 1045. SQL Server sends 28000 with 18456. PostgreSQL has 28P01 in the standard.
- Some native codes in class 28 are a login that a new password does not cure. They are 18486, 18487 and 18488 of SQL Server, for a locked account, an expired password and a password that must change. They are also 3118 and 1862 of MySQL, for a locked account and an expired password. These return false.
- `psqlODBC` is the exception to the class. It answers a wrong password and an unknown user with SQLSTATE 08001 and the native code 101. That is the class of a failed connection. It puts the text of the server in the message, as in `FATAL: password authentication failed for user "x"`. The tests showed this on PostgreSQL 18.
- So a record with the state 08001 counts when its message holds `authentication failed` or `no password supplied`. Those are the texts of libpq. The same state with the text `Connection refused` is a network failure and does not count.
- `08004` is not counted at all, because a server sends it for other reasons too.

## Tests

`TestAuthenticationRefused` in the root builds an `*odbc.Error` for each case. One case has a first record of class 01 and a class 28 record after it. The test wraps each error with `%w`, joins it, and wraps it with `%v`. It walks the tree as `usql` does.

The test of the same name in the `test` module uses PostgreSQL, MariaDB, MySQL and SQL Server. A wrong password and an unknown user must be true. A statement that a user with no privilege runs must be false, and must be in class 42. `TestAuthenticationNotRefusedForANetworkFailure` points each server at a port that holds nothing.

## Open

Other ODBC drivers can word a refused login in another way. Oracle and Db2 are not tested here. A database that sends 08001 for a wrong password needs a text rule, which is a code change with a test, as for `psqlODBC`.
