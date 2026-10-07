# Decisions

Every decision this project made is a file in this folder, one per decision,
named by its number and its title. This table is the index. Find the number
here, then open the file.

Each file opens with its status. "Decided" means Ken chose it. "Proposed"
means an agent or a peer session suggested it and Ken has not confirmed it.
"Open" means nobody has chosen yet. A decision that changes an earlier one
says so in its status. It writes "Amends" and the number. The earlier one says
it back, with "Amended by" and the number. Read the status before the decision.

A new decision gets the next number and a file of its own. Add its row here.
`TestTheDecisionIndexIsComplete` fails when a decision has no row or a row is
wrong, and it prints the row to add.

| # | Decision | Status |
| --- | --- | --- |
| [D1](D001-pure-go-with-purego-and-no-cgo.md) | Pure Go with purego and no cgo | Decided |
| [D2](D002-one-code-base-for-windows-macos-and-linux.md) | One code base for Windows, macOS and Linux | Decided |
| [D3](D003-test-on-all-three-operating-systems.md) | Test on all three operating systems | Decided |
| [D4](D004-dependencies-are-the-standard-library-and-purego.md) | Dependencies are the standard library and purego | Decided, and amended by D17 |
| [D5](D005-agent-skills-are-committed-as-copies.md) | Agent skills are committed as copies | Decided |
| [D6](D006-agents-md-holds-the-rules-and-claude-md-imports-it.md) | AGENTS.md holds the rules and CLAUDE.md imports it | Decided |
| [D7](D007-only-four-documents-in-the-root.md) | Only four documents in the root | Decided |
| [D8](D008-lint-with-default-all-and-a-reason-for-each-exception.md) | Lint with default all and a reason for each exception | Decided |
| [D9](D009-servers-start-with-dbrun-only.md) | Servers start with dbrun only | Decided, and amended by D13 |
| [D10](D010-the-test-databases-are-postgresql-sqlite-sql-server-mysql-and-mariadb-and-duckdb.md) | The test databases are PostgreSQL, SQLite, SQL Server, MySQL and MariaDB, and DuckDB | Decided, and amended by D14 |
| [D11](D011-the-prose-test-has-no-exceptions.md) | The prose test has no exceptions | Decided |
| [D12](D012-the-connection-string-is-a-dburl-style-url.md) | The connection string is a dburl style URL | Decided |
| [D13](D013-ci-tests-native-databases-on-windows-and-macos-and-containers-on-linux.md) | CI tests native databases on Windows and macOS and containers on Linux | Decided, amends D9, and amended by D21 |
| [D14](D014-mariadb-connector-odbc-tests-mysql-and-mariadb.md) | MariaDB Connector/ODBC tests MySQL and MariaDB | Decided, and amends D10 |
| [D15](D015-the-integration-tests-are-a-separate-module-that-uses-dbmeta-fixtures.md) | The integration tests are a separate module that uses dbmeta fixtures | Decided |
| [D16](D016-the-data-source-name-is-a-url-or-an-odbc-connection-string.md) | The data source name is a URL or an ODBC connection string | Decided |
| [D17](D017-the-driver-imports-dbimp-for-its-shared-types.md) | The driver imports dbimp for its shared types | Decided, and amends D4 |
| [D18](D018-odbc-types-map-onto-the-dbimp-kinds.md) | ODBC types map onto the dbimp kinds | Decided |
| [D19](D019-the-driver-adapts-to-a-database-by-its-reported-name.md) | The driver adapts to a database by the name it reports | Decided |
| [D20](D020-the-driver-finds-the-size-of-sqlwchar-when-it-loads-the-manager.md) | The driver finds the size of SQLWCHAR when it loads the manager | Decided |
| [D21](D021-the-ci-jobs-test-the-databases-that-each-runner-can-hold.md) | The CI jobs test the databases that each runner can hold | Decided, and amends D13 |
| [D22](D022-the-driver-takes-the-dbimp-options.md) | The driver takes the dbimp options | Decided |
| [D23](D023-the-driver-exposes-what-a-tool-needs-to-know-about-the-database.md) | The driver exposes what a tool needs to know about the database | Decided |
| [D24](D024-the-driver-fetches-in-blocks-reads-the-catalog-and-takes-a-location.md) | The driver fetches in blocks, reads the catalog and takes a location | Decided |
