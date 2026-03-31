# Batch

## Overview

`batch` is Oct's structured data-parallel mapping construct for arrays.
Each input item maps to one output item.
Output order matches input order.
Batch semantics are explicit and deterministic.

## Rules

- Input must be an array.
- Syntax is `batch <array> as <item> { ... return <expr> }`.
- Body must end with exactly one `return <expr>`.
- Body return type defines the output element type.
- Output length equals input length.
- Output index order matches input index order.
- Item failures fail the entire batch expression.
- No partial output is exposed on failure.
- Batch has an implicit join at expression completion.

## Examples

Valid:

```oct
package Main

fn Main() -> Int[] {
    let xs = [1, 2, 3]
    return batch xs as item {
        return item * item
    }
}
```

Invalid:

```oct
package Main

fn Main() -> Int[] {
    return batch 5 as item {
        return item
    }
}
```
