# SDSL-V M15: Register Tiles

SDSL-V M15 adds structured per-thread local/register storage for register-blocked accumulators:

```sdslv
let Acc: reg_tile<f32, 2u, 2u> = reg_tile_zero();
Acc[0u, 1u] = Acc[0u, 1u] + 2.0;
```

This is intentionally distinct from the existing M12 storage/view surface:

- `tile<T, Rows, Cols>` is workgroup memory.
- `matrix_view<T>` is a row-major view over global resource storage.
- `reg_tile<T, Rows, Cols>` is per-thread local/register storage.

All three support explicit `x[row, col]` indexing, but they lower through different VD-MIR/HLSL paths.

## Syntax

```sdslv
let Acc: reg_tile<f32, R, C> = reg_tile_zero();
```

M15 currently supports:

- element type `f32`;
- compile-time positive integer `Rows` and `Cols`;
- a conservative maximum of `64` elements total;
- direct 2D read/write indexing;
- zero initialization only.

Example diagnostic:

```text
reg_tile has 128 elements; M15 limit is 64
```

## Rules

- `reg_tile<T, Rows, Cols>` is valid only for local variables in M15.
- `reg_tile` is not valid for workgroup declarations, resources, push-constant records, or stream/record fields.
- `reg_tile` values must be initialized with `reg_tile_zero()`.
- Whole-tile copy, whole-tile assignment, parameter passing, and return values are not part of M15.
- One-dimensional indexing such as `Acc[i]` is rejected.
- Runtime index expressions are allowed; M15 does not insert bounds checks.

## Lowering

VD-MIR keeps `reg_tile` explicit as its own type plus a `reg_tile_zero()` initializer expression.

M15 lowers `reg_tile` to HLSL local flat arrays:

```hlsl
float Acc[4];
Acc[0] = 0.0;
Acc[1] = 0.0;
Acc[2] = 0.0;
Acc[3] = 0.0;
```

2D indexing lowers deterministically to flat row-major indexing:

```hlsl
Acc[((i) * (Cols)) + (j)]
```

This favors correctness and structured authoring first. Future scalarization can specialize compile-time-indexed `reg_tile` usage without changing the source model.

## Relationship to `comptime for`

M15 is a stepping stone toward structured register-blocked kernels:

```sdslv
let Acc: reg_tile<f32, C.Outputs.M, C.Outputs.N> = reg_tile_zero();
```

M16 adds constrained `comptime for` so shader authors can iterate a structured accumulator surface instead of forcing generated names such as `acc00`, `acc01`, `acc10`, and `acc11`.

M15a keeps this storage model unchanged while aligning condition syntax with Oct source style: shader authors use `and`, `or`, and `not` in SDSL-V source, and emitted HLSL continues to use `&&`, `||`, and `!`. M16 builds directly on this storage model with `comptime for`; see `docs/SDSL_V_M16_COMPTIME_FOR.md`.
