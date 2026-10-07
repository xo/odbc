# D11. The prose test has no exceptions

Status: Decided.

`TestProseIsSimpleEnglish` checks the rules of the `simple-english` skill that a pattern can check. It has no marker that turns a rule off.

## Reason

A marker that turns a rule off gets used on the first sentence that is hard to rewrite, and then on every one after it. Text in backticks and in double quotes is not checked, because it is code or somebody else's words. For anything else, rewrite the sentence.
