# octagon

## Overview

`.octagon` is Oct's data artifact format for typed value interchange.
It stores one Oct value expression.
Load and write are explicit through builtins.

## Rules

- `.octagon` payload is a single top-level value.
- Allowed surface is data-only values: scalar literals, arrays, record literals, and enum values.
- Signed `Int` and `Float` scalar literals are data literals (for example `-1`
  and `-0.5`), including inside arrays and record fields.
- Disallowed surface includes package declarations, function declarations, bindings, calls, control flow, and multiple top-level values.
- `WriteOctagon(path, value)` writes `.octagon` data and returns `Int` status.
- `WriteOctagon` path must end with `.octagon`.
- `WriteOctagon` value must be `.octagon`-representable.
- `LoadOctagon<T>(path)` loads a value as type `T` and is fallible.
- `LoadOctagon` path must end with `.octagon`.
- `LoadOctagon` type argument `T` must be `.octagon`-representable.
- Load performs runtime type materialization checks.
- Load rejects top-level type mismatches.
- Load rejects record field type/shape mismatches.
- Load rejects enum type/variant mismatches.
- Load rejects array element type mismatches.
- Load rejects dimension mismatches.
- A nominal `record table` is represented by its declared table literal: each
  field is one complete column array. The loader applies the schema's implicit
  column array depth exactly once in interpreted and compiled execution.
- Enum-valued table cells retain their nominal enum and refined-Concept cells
  are checked through the same authoritative refinement admission used by
  ordinary construction.

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
