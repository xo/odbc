# D3. Test on all three operating systems

Status: Decided.

Every change to the code that loads or calls the driver manager is tested on Windows, macOS and Linux.

## Reason

The point of the project is that one code base behaves the same on all three. A test on Linux alone cannot show that. The faults that matter here appear on one system and not another. Examples are a wrong calling convention and a wrong `SQLWCHAR` size.

## Consequence

The unit job in CI runs on `ubuntu-latest`, `macos-latest` and `windows-latest`. The integration jobs follow when the driver exists. Which database runs on which system is in D13.
