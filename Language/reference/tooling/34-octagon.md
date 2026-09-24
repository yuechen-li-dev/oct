# octagon

## Overview

`.octagon` is Oct's data artifact format for typed value interchange.
It stores one Oct value expression.
Load and write are explicit through builtins.

## Rules

- `.octagon` payload is a single top-level value.
- Allowed surface is data-only values: scalar literals, arrays, record literals, and enum values.
- Octagon payload enum literals are the data-only subset of ordinary Oct enum
  construction syntax. Payload expressions must themselves be Octagon data
  expressions. They use the ordinary one-payload constructor spelling,
  such as `Result.Ok(42)`. Their payload must recursively be an Octagon data
  expression. Tag-only `Enum.Case` remains valid. Ordinary function calls,
  including calls with the same punctuation, remain invalid data.
- Signed `Int` and `Float` scalar literals are data literals (for example `-1`
  and `-0.5`), including inside arrays and record fields.
- Disallowed surface includes package declarations, function declarations,
  bindings, arbitrary calls, control flow, and multiple top-level values.
- `WriteOctagon(path, value)` writes `.octagon` data and returns `Int` status.
- `WriteOctagon` path must end with `.octagon`.
- `WriteOctagon` value must be `.octagon`-representable.
- Canonical output ends with one newline after the single value expression;
  nested values have no document terminator of their own.
- `LoadOctagon<T>(path)` loads a value as type `T` and is fallible.
- `LoadOctagon` path must end with `.octagon`.
- `LoadOctagon` type argument `T` must be `.octagon`-representable.
- Load performs runtime type materialization checks.
- Load rejects top-level type mismatches.
- Load rejects record field type/shape mismatches.
- Load rejects enum type/variant mismatches.
- Load rejects missing, extra, and mistyped enum payloads. Payload refinement
  admission uses the same constructor checks as an ordinary loaded value.
- Load rejects array element type mismatches.
- Load rejects dimension mismatches.
- Compiled loading checks the same declared dimension for numeric literals,
  including literals nested in arrays and records. Parenthesized data values
  remain data values in either execution mode.
- A nominal `record table` is represented by its declared table literal: each
  field is one complete column array. The loader applies the schema's implicit
  column array depth exactly once in interpreted and compiled execution.
- Enum-valued table cells retain their nominal enum and refined-Concept cells
  are checked through the same authoritative refinement admission used by
  ordinary construction.
- `Language/Data/Octagon/valid/payload_enum_nested.octagon` and
  `Language/Data/Octagon/Load/valid/concept_catalog.octagon` are shared
  Concept/Oct byte goldens for a nested payload enum and a two-column
  `record table`. The table fixture stores complete `ID` and `Active` arrays,
  and the interpreted and compiled loaders agree on its three rows.

See also [31 octest](./31-octest.md) for artifact and benchmark workflows.

## Examples

Valid `.octagon` content:

```oct
SimulationConfig {
    Name: "Cantilever"
    Dt: 0.001s
    Steps: 1000
}
```

Valid usage:

```oct
package Main

fn Main() -> Int ! Error {
    let cfg = LoadOctagon<SimulationConfig>("config.octagon")?
    return WriteOctagon("copy.octagon", cfg)
}
```

Invalid `.octagon` content:

```oct
package Main

let x = 1
```
