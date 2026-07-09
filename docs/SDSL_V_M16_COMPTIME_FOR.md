# SDSL-V M16: Constrained `comptime for`

M16 adds constrained compile-time loop expansion for structured shader staging.

`comptime for` is not a runtime loop, not arbitrary compile-time execution, and not generated identifiers. It expands repeated statement structure over a compile-time integer range before VD-MIR lowering.

The pipeline remains:

```text
source
  -> lex
  -> parse
  -> validate
  -> template/config monomorphization
  -> comptime evaluation and expansion
  -> lower expanded concrete shaders to VD-MIR
  -> emit HLSL
  -> optional DXC / SPIR-V / generated header
```

VD-MIR remains the compiler boundary. HLSL emits from VD-MIR and does not understand `comptime for`.

## Syntax

```sdslv
comptime for i in 0u..C.Outputs.M {
    comptime for j in 0u..C.Outputs.N {
        Acc[i, j] = 0.0;
    }
}
```

M16 supports only half-open integer ranges:

- `start <= i < end`
- `start..end`

It does not support inclusive ranges, steps, descending ranges, runtime iterators, array iteration, `break`, `continue`, comptime functions, or generated identifiers.

## Bounds

- `start` and `end` must be compile-time integer expressions.
- Bounds must be non-negative in M16.
- `start <= end` is required.
- `start == end` is valid and expands to zero statements.

Example diagnostics:

- `comptime for bounds must be compile-time integers`
- `comptime for bounds must be non-negative in SDSL-V M16`
- `comptime for range start must be <= end`

## Loop Index

The loop index is a compile-time integer binding scoped to the loop body.

- it may be used in compile-time expressions;
- it may be used in runtime code as a constant after expansion;
- it may be used in nested comptime bounds, `static assert`, `comptime if`, `comptime match`, and `comptime when utility`;
- it may not be assigned to;
- it does not exist outside the loop body.

Example:

```sdslv
comptime for i in 0u..2u {
    Acc[i, 0u] = 1.0;
}
```

expands equivalently to:

```sdslv
Acc[0u, 0u] = 1.0;
Acc[1u, 0u] = 1.0;
```

## `reg_tile` Relationship

This is the M16 motivating use case:

```sdslv
let Acc: reg_tile<f32, C.Outputs.M, C.Outputs.N> = reg_tile_zero();

comptime for i in 0u..C.Outputs.M {
    comptime for j in 0u..C.Outputs.N {
        Acc[i, j] = Acc[i, j] + 1.0;
    }
}
```

M16 removes hand-written repeated accumulator indexing while preserving the M15 storage model. VD-MIR and HLSL still receive only concrete expanded statements.

## Expansion Limit

M16 enforces a conservative expansion guard:

- a `comptime for` expansion may not exceed `256` expanded statements in the expanded body it contributes.

Example diagnostic:

```text
comptime for expansion exceeds M16 limit of 256 iterations
```

This is intentionally conservative to avoid accidental AST explosion from nested compile-time loops.

## Relationship to Earlier Comptime Milestones

`comptime for` composes with the earlier constrained staging forms:

- M13 `comptime let`
- M13 `comptime if`
- M14 `comptime match`
- M14a `comptime when utility`

Those constructs may appear inside a `comptime for` body, and the loop index may participate in their compile-time expressions.

M16a guarded memory access composes with this expansion model in runtime statements inside the expanded body:

```sdslv
comptime for oi in 0u..C.Outputs.M {
    comptime for oj in 0u..C.Outputs.N {
        write CView[row + oi, col + oj] = Acc[oi, oj]
            when row + oi < params.M and col + oj < params.N;
    }
}
```

The loop indices may contribute constants to the guard, but the guarded read/write itself remains a runtime memory operation rather than a compile-time expression.

## Examples

- `examples/SDSL-V/M16/ComptimeForBasic.sdslv`
- `examples/SDSL-V/M16/ComptimeForRegTile.sdslv`
- `examples/SDSL-V/M16/ComptimeForSgemmMicroKernel.sdslv`
