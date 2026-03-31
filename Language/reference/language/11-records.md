# Records

## Overview

Records are nominal product types with named fields.
Record literals construct values by field name.
Field access is explicit.

## Rules

- Record declaration form is `record Name { Field: Type ... }`.
- Record field names are unique within a record.
- Record construction form is `Name { Field: value ... }`.
- Record construction requires all declared fields.
- Record construction uses field names; construction order is independent of declaration order.
- Record update form is `value with { Field: value ... }`.
- Record update requires at least one field and returns a new value of the same record type.
- Record update field names must exist on the source record type.
- Record update field values must match declared field types exactly (including dimensions and nominal types).
- Record update is immutable (it does not mutate the source value).
- Record update evaluates the source expression once.
- Field access form is `value.Field`.
- Record values are whole-value mutable only (rebind the record, not individual fields).
- Record identity is nominal by record name. See [02 Types](./02-types.md).

## `with` examples

Single-field `with` update:

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

Multi-field `with` update:

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

Use this when you want immutable record updates while preserving nominal type identity.
Avoid this when you need behavior-local mutable control memory in flows; use `board` for that.

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
    p = Point { X: p.X + 3 Y: p.Y }
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
