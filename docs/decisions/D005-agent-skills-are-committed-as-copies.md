# D5. Agent skills are committed as copies

Status: Decided.

The `simple-english` and `go-pedantry` skills are ordinary folders under `.agents/skills` and `.claude/skills`, and `skills-lock.json` names their source.

## Reason

A symbolic link becomes a text file in a Windows checkout, and Claude Code then loads no skill and says nothing. A copy always works.

## Note

The files were copied from the checkout of `dbmeta` that was on this machine, and not downloaded again. They are the same bytes, and the hashes in `skills-lock.json` are the ones that project wrote. Run the command in `CONTRIBUTING.md` to update them.

`TestSkillsAreCopies` fails on a link and on a difference between the two folders.
