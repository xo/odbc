# D7. Only four documents in the root

Status: Decided.

`README.md`, `AGENTS.md`, `CLAUDE.md` and `CONTRIBUTING.md` are the only Markdown files in the repository root. Everything else is in `docs/`.

## Reason

A root that fills with documents has no order. `AGENTS.md` and `README.md` each have a table that names every document in `docs/`. A reader always has one place to start.

`TestTheRootHoldsFourDocuments` and `TestEveryDocumentIsInBothTables` check this.
