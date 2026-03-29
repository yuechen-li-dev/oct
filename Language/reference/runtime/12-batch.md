# Batch

## Overview

`batch` is Oct's structured data-parallel mapping construct over arrays. Each input item maps to one output item. Output ordering matches input ordering. Batch semantics are deterministic and explicit.

## Rules

- Input must be an array.
- Syntax: `batch <array> as <item> { ... return <expr> }`.
- Body must end with exactly one `return <expr>`.
- Body return type defines output element type.
- Output length equals input length.
- Output index order matches input index order.
- Item failures fail the whole batch expression.
- No partial output is exposed on failure.
- Batch has an implicit join at expression completion.

## Examples

Valid:

```oct
fn Main() -> Int[] {
    let xs = [1, 2, 3]
    return batch xs as item {
        return item * item
    }
}
```

Invalid:

```oct
fn Main() -> Int[] {
    return batch 5 as item {
        return item
    }
}
```
