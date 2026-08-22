# Records

## Overview

Records are nominal product types with named fields.
A record value has the identity of its declared record name, not just the shape of its fields.
Record literals construct values by field name, and field access is explicit.

Records are immutable values.
To change record-shaped state, create a new record value with `with` and then return or rebind that new value.
Individual record fields are not assigned in place.

`concept Name { ... }` is the preferred Concepts-M0 spelling when the declaration is intended as a user-facing valid-value description. It lowers to this same nominal record machinery. Existing `record` declarations remain supported and unchanged; `record table` remains record-only.

## Rules

- Record declaration form is `record Name { Field: Type ... }`.
- Record field names are unique within a record.
- Record construction form is `Name { Field: value ... }`.
- Record construction requires all declared fields.
- Record construction uses field names; construction order is independent of declaration order.
- Record identity is nominal by record name. See [02 Types](./02-types.md).
- Field access form is `value.Field`.
- Record update form is `value with { Field: value ... }`.
- Record update requires at least one field and returns a new value of the same record type.
- Record update field names must exist on the source record type.
- Record update field values must match declared field types exactly, including dimensions and nominal types.
- `with` is immutable: it does not mutate the source value.
- `with` preserves fields not listed in the update block.
- `with` evaluates the source expression once.
- Record values are whole-value mutable only: rebind the record value, not individual fields.

## Record tables

`record table` is a distinct nominal record form for immutable validated
columnar data:

```oct
record table Measurements {
    Stage: String
    Latency: Float
}

let results = Measurements {
    Stage: ["Attention", "FFN"]
    Latency: [2.4, 0.96]
}
```

The declared field type is the cell type. Storage adds one implicit array
depth, so `results.Latency` has type `Float[]`; a declared cell type `Float[]`
would therefore produce `Float[][]` storage. Every column is required exactly
once and all columns have one shared row count. Literal length disagreement is
a type-checking error. Dynamically computed columns are checked once during
construction and an invalid construction terminates with a deterministic
runtime diagnostic; no malformed table value is produced.

`Len(results)` returns the shared row count. `results[i]` performs ordinary
bounds checking and returns a compiler-owned immutable row value whose fields
have the declared cell types. The row type has no source-level name. Direct
column access returns the stored typed array.

Tables are immutable. Table `with` replaces one or more complete columns and
returns a new value of the same nominal table type. Each replacement must have
the declared column type and the same extent as the source table. Columns not
listed in the update are preserved, the source table is unchanged, and a
dynamic extent mismatch terminates with `OCT-RTBL004`. The current interpreter
and Go backend construct a new table header while sharing unchanged column
values; no observable mutable alias is introduced because arrays and tables
retain value semantics.

Row/column assignment, append/delete,
nested table-valued cells, row iteration syntax, queries, joins, and mutable
views are not part of this bounded form. Iterate row indices with
`for i in 0..Len(table)` and project `table[i]`.

## Immutable `with` updates

Use `with` when a function needs to return an updated record while preserving all fields not mentioned by the update.
The original record remains available and unchanged.

```oct
package Main

record StoreState {
    CartCount: Int
    SelectedTab: String
}

fn AddToCart(state: StoreState) -> StoreState {
    return state with {
        CartCount: state.CartCount + 1
    }
}
```

In this example, `SelectedTab` is preserved from `state`, `CartCount` is replaced in the returned value, and `state` itself is not mutated.

Single-field update:

```oct
package Main

record Point {
    X: Int
    Y: Int
}

fn MoveX(point: Point) -> Point {
    let next = point with { X: 3 }
    return next
}
```

Multi-field update:

```oct
package Main

record SampleState {
    Time: Float
    Value: Float
}

fn Advance(state: SampleState, nextValue: Float) -> SampleState {
    let next = state with {
        Time: state.Time + 1.0
        Value: nextValue
    }
    return next
}
```

Configuration-style update:

```oct
package Main

record AppConfig {
    Theme: String
    RetryCount: Int
    TelemetryEnabled: Bool
}

fn DisableTelemetry(config: AppConfig) -> AppConfig {
    return config with {
        TelemetryEnabled: false
    }
}
```

Use records and `with` for immutable data-lane or application-state updates.
Use Octomata blackboards for behavior-local control memory owned by flows; see [21 Octomata](../runtime/21-octomata.md).

## Examples

Valid:

```oct
package Main

record Point {
    X: Int
    Y: Int
}

fn Main() -> Int {
    var p = Point { Y: 2 X: 1 }
    p = p with { X: p.X + 3 }
    return p.X
}
```

Invalid:

```oct
package Main

record Point {
    X: Int
    Y: Int
}

fn Main() -> Int {
    var p = Point { X: 1 Y: 2 }
    p.X = 3
    return p.X
}
```
