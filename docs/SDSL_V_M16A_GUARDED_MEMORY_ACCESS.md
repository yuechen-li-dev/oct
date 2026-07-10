# SDSL-V M16a: Guarded Memory Access

M16a adds source-backed guarded memory operations for boundary and tail handling:

```sdslv
read AView[row, col] when row < params.M and col < params.N else 0.0
write CView[row, col] = value when row < params.M and col < params.N;
```

The intent is shader source that reads like executable GPU pseudocode. Internal compiler names may use guarded load/store terminology, but source syntax remains `read ... when ... else ...` and `write ... when ...`.

The pipeline contract is unchanged:

```text
source -> lex -> parse -> validate -> template/config monomorphization -> comptime expansion -> lower to VD-MIR -> emit HLSL -> optional DXC/SPIR-V
```

VD-MIR remains the compiler boundary. HLSL emits from VD-MIR, not directly from AST.

## Guarded Read

Syntax:

```sdslv
read <indexed_memory_expr> when <bool_expr> else <fallback_expr>
```

Example:

```sdslv
let a: f32 =
    read AView[row, k] when row < params.M and k < params.K else 0.0;
```

Rules:

- `read` is an expression form.
- the target must be an indexed readable memory expression in M16a.
- the guard must typecheck as `bool`.
- the fallback must typecheck as the same element type as the target.
- the result type is the target element type.
- guarded read is not a compile-time expression.

Current M16a targets:

- `matrix_view<T>` 2D indexing such as `AView[row, col]`;
- raw resource array 1D indexing such as `A[index]`;
- workgroup `tile<T, Rows, Cols>` 2D indexing when used as indexed memory.

Rejected example:

```sdslv
read (a + b) when guard else 0.0
```

Diagnostic:

```text
guarded read target must be an indexed memory expression
```

## Guarded Write

Syntax:

```sdslv
write <indexed_memory_expr> = <value_expr> when <bool_expr>;
```

Example:

```sdslv
write CView[row, col] = acc when row < params.M and col < params.N;
```

Rules:

- `write` is a statement form.
- the target must be a writable indexed memory expression.
- the guard must typecheck as `bool`.
- the value must typecheck as the target element type.
- readonly matrix views reject guarded writes.
- there is no `else` branch.
- when the guard is false, no store occurs.

Representative diagnostics:

```text
guarded write target must be a writable indexed memory expression
cannot guarded-write to readonly matrix view
guarded write condition must be bool
guarded write value type does not match target element type
```

## Lowering Strategy

M16a is the scalar/thread-level analogue of masked memory access in Triton-style kernels. It is not tensor/block masking and not vector-lane masking.

Guarded read lowers conservatively to safe control flow:

```hlsl
T tmp = fallback;
if (guard) {
    tmp = target;
}
```

`tmp` is the guarded read's result value. When a guarded read is used on an assignment RHS, the surrounding destination assignment is unconditional and follows this block:

```hlsl
destination = tmp;
```

Fallback and guard are evaluated once; the target is evaluated only in the true branch. This is deliberately distinct from guarded write lowering.

Guarded write lowers to:

```hlsl
if (guard) {
    target = value;
}
```

This keeps out-of-bounds-sensitive memory accesses behind backend control flow instead of relying on source-level helper names or direct AST-to-HLSL string lowering.

## Relationship to `if`

Use guarded read/write when the branch exists only to protect a memory access or provide a fallback value.

Use `if` when the control flow itself is semantically meaningful.

M19 adds Oct-style runtime guard `when` for ordered bounded statement flow:

```sdslv
when {
    case fullTile -> {
        TileA[localRow, localCol] = AView[row, k];
    }
    else -> {
        TileA[localRow, localCol] =
            read AView[row, k] when row < params.M and k < params.K else 0.0;
    }
}
```

This is distinct from guarded memory access. `read/write ... when ...` protects one memory operation; guard `when { case ... -> ... }` selects one bounded statement body.

M21 shader-local board fields may be used naturally in guarded memory targets and guards:

```sdslv
TileA[p.row, p.col] =
    read AView[groupBaseRow + p.row, tileBaseK + p.col]
        when groupBaseRow + p.row < params.M and tileBaseK + p.col < params.K
        else 0.0;
```

## Examples

- `examples/SDSL-V/M16a/GuardedReadBasic.sdslv`
- `examples/SDSL-V/M16a/GuardedWriteBasic.sdslv`
- `examples/SDSL-V/M16a/GuardedSgemmTileLoad.sdslv`
- `examples/SDSL-V/M19/GuardWhenTilePath.sdslv`

## Current Boundaries

M16a does not add:

- general expression-level `when`;
- guarded arithmetic;
- full Triton-style block/tensor masks;
- vector lane masks;
- selector tuning or runtime dispatch changes.
