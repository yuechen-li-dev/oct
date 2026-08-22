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
- When several items fail, the failure from the lowest input index is returned.
- No partial output is exposed on failure.
- Batch has an implicit join at expression completion.

## Concurrency and observability

The implementation may evaluate independent items in parallel and does not
guarantee item execution order. Result placement is nevertheless deterministic:
output index `i` belongs to input index `i`, and failure selection uses the
lowest failing input index after the implicit join.

Batch bodies can currently call ordinary functions and runtime operations; Oct
does not yet have an effect system that proves those calls pure. Programs must
therefore not depend on the relative order of externally observable side
effects performed by separate items. This is an existing language-semantic gap,
not a promise of source-order effects. Pure computation and immutable captures
are safe independent work.

Nested batches preserve the same result and join semantics. The M0 CPU runtime
executes an inner batch sequentially while its enclosing batch item is active,
which bounds goroutine creation without changing ordered results.

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
