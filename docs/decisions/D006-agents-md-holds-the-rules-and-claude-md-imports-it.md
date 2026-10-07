# D6. AGENTS.md holds the rules and CLAUDE.md imports it

Status: Decided.

`AGENTS.md` holds every rule. `CLAUDE.md` holds the one line `@AGENTS.md`.

## Reason

Codex and other agents read `AGENTS.md`. Claude Code reads `CLAUDE.md`. One file of rules and one line of import means every agent reads the same rules. A symbolic link is not used, for the reason in D5.

`TestClaudeImportsAgents` checks this.
