# D7. Only four documents in the root

Status: Decided.

`README.md`, `AGENTS.md`, `CLAUDE.md` and `CONTRIBUTING.md` are the only Markdown files in the repository root. Everything else is in `docs/`.

## Reason

A root that fills with documents has no order. Each document in `docs/` is named in the table of `AGENTS.md` and in the table of `README.md`, so a reader always has one place to start.

`TestTheRootHoldsFourDocuments` and `TestEveryDocumentIsInBothTables` check this.
