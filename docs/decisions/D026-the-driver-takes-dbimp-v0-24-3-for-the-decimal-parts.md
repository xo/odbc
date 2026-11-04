# D26. The driver takes dbimp v0.24.3 for the decimal parts

Status: Decided.

The module requires `github.com/xo/dbimp` v0.24.3, and no longer v0.11.0. The test module does too.

## Reason

`database/sql` has an interface for a decimal that is not named in its documents. A decimal that has `Decompose` gives its parts, and a destination that has `Compose` takes them. `Scan` uses the pair when the driver gives a decimal and the destination takes one, and the decimal then needs no trip through text. The decimal packages of the community implement `Compose`.

This driver gives a decimal as `*apd.Decimal` (D18), which has `Decompose`. The `ScanColumn` method hands the value to `dbimp.Assign`. Before v0.24.3, `dbimp.Assign` turned the decimal into text first, so a destination with `Compose` got a string and failed with "unsupported Scan, storing driver.Value type string". From v0.24.3 it calls `Compose` with the parts of the decimal (dbimp W53 and dbimp D229).

`TestDecimalComposes` scans a decimal into a destination with `Compose` and checks the parts: the sign, the exponent and the coefficient. It fails with v0.11.0 and passes with v0.24.3.

## Also in the new version

dbimp W52 gives a decimal the scale of its column. A `decimal(10,3)` that holds `1.5` now reads as `1.500`.

## Rejected

- A copy of the parts logic in this driver. It drifts from the other drivers.
