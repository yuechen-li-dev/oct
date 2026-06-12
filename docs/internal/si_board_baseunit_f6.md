# F6 SI board fields and BaseUnit audit

## Audit result

- `board { LastTemp: Float<K> }` is supported for scalar dimensioned numeric board fields.
- Assigning `board.LastTemp = temp` where `temp: Float<K>` typechecks and runs in interpreted and compiled execution.
- Returning `board.LastTemp` from a flow as `Float<K>` typechecks and runs in interpreted and compiled execution.
- `BoardSnapshot(machine)!` includes dimensioned scalar board fields and preserves their exact unit-qualified type. The F6 contract covers `Float<D>` and `Int<D>` scalar fields.
- Board fields are still scalar-only: arrays, vectors, matrices, records, enums, `Complex`, `Range`, `UI`, `Error`, and `Void` remain unsupported.
- Older docs that described board snapshots as only `Bool`/`Int`/`Float`/`String` were incomplete: dimension-qualified `Int<D>` and `Float<D>` are valid scalar numeric board fields.

## BaseUnit result

- `BaseUnit(value)` is a core builtin alias for unit stripping.
- `BaseUnit(x: Float<D>) -> Float` and `BaseUnit(x: Float) -> Float` are supported in interpreted and compiled execution.
- `BaseValue(...)` remains accepted as the older spelling, but `BaseUnit(...)` is the clearer v0.1 spelling for examples and docs.
- Unit stripping erases only the static dimension; it does not format, choose a display unit, or perform unit conversion.
- Unsupported arguments such as `String`, records, and arrays fail statically.
- Dimensioned `Int<D> -> Int` is not part of the current F6 contract because the existing implementation only accepts `Float` values for this builtin.

## Stale syntax cleanup

The old `base_value_unit_stripping.octest` used angle-bracket literal suffixes such as `0.5<s>` and `9.8<m*s^-2>`. The reference syntax uses suffix literals (`0.5s`, `9.8m*s^-2`) and type-level angle brackets (`Float<s>`, `Float<m*s^-2>`). The test is now updated to current syntax and a dedicated invalid fixture records that angle-bracket literal suffixes are unsupported.

## SmartGreenhouse choice

`SmartGreenhouseController` now uses a dimensioned temperature board field (`Temp: Float<K>`) because SI board fields and `BoardSnapshot` preservation are stable enough to be a v0.1 contract. This is clearer than storing a bare Kelvin scalar (`TempK: Float`) and reconstructing units at the boundary. If an example needs an intentionally dimensionless scalar later, it should use `BaseUnit(...)` instead of manual division by `1.0K`.
