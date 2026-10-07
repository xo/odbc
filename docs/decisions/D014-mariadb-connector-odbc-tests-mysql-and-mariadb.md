# D14. MariaDB Connector/ODBC tests MySQL and MariaDB

Status: Decided, and amends D10.

One driver, MariaDB Connector/ODBC, is used to test both the MySQL server and the MariaDB server. MySQL Connector/ODBC is not tested.

## Reason

Ken chose it. The MariaDB driver speaks the protocol of both servers. The MySQL driver in the Arch User Repository is version 8.0.32, which is nearly four years old. One driver halves the work of installing and testing on three systems.

## Rejected

Testing the MySQL server with Oracle's own driver. It adds a second driver to install on every system, and the driver here is the part under test, not the server.

## Consequence

A fault that only Oracle's driver shows is not found by these tests. `dbrun` still starts `mysql` and `mariadb` as two servers, and the same driver connects to each.
