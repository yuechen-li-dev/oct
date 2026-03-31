# octagon

## Overview

`.octagon` is Oct's data artifact format for typed value interchange.
It stores one Oct value expression.
Load and write are explicit through builtins.

## Rules

- `.octagon` payload is a single top-level value.
- Allowed surface is data-only values: scalar literals, arrays, record literals, and enum values.
- Disallowed surface includes package declarations, function declarations, bindings, calls, control flow, and multiple top-level values.
- `WriteOctagon(path, value)` writes `.octagon` data and returns `Int` status.
- `WriteOctagon` path must end with `.octagon`.
- `WriteOctagon` value must be `.octagon`-representable.
- `LoadOctagon[T](path)` loads a value as type `T` and is fallible.
- `LoadOctagon` path must end with `.octagon`.
- `LoadOctagon` type argument `T` must be `.octagon`-representable.
- Load performs runtime type materialization checks.
- Load rejects top-level type mismatches.
- Load rejects record field type/shape mismatches.
- Load rejects enum type/variant mismatches.
- Load rejects array element type mismatches.
- Load rejects dimension mismatches.

See also [31 octest](./31-octest.md) for artifact and benchmark workflows.

## Examples

Valid `.octagon` content:

```oct
package Main

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
    let cfg = LoadOctagon[SimulationConfig]("config.octagon")?
    return WriteOctagon("copy.octagon", cfg)
}
```

Invalid `.octagon` content:

```oct
package Main

let x = 1
```
