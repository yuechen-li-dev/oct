# SDSL-V M12: Tile/Matrix Views and 2D Indexing

SDSL-V M12 raises tiled compute authoring above manual flat row-major indexing while preserving the existing compiler boundary:

```text
source -> lex -> parse -> validate -> template/config monomorphization -> comptime expansion -> lower to VD-MIR -> emit HLSL -> optional DXC/SPIR-V
```

VD-MIR remains the shader MIR boundary. HLSL lowering is deterministic flat index math; source code no longer has to hand-write every `row * stride + col`.

## Workgroup Tiles

M12 adds shader-scoped workgroup tile storage:

```sdslv
workgroup TileA: tile<f32, 16, 16>;
workgroup TileB: tile<f32, C.Tile.K, C.Tile.N>;
```

Rules:

- `tile<T, Rows, Cols>` is valid only for `workgroup` declarations in M12.
- `T` must be a workgroup-supported scalar/vector element type.
- `Rows` and `Cols` must be positive compile-time integer expressions.
- storage size is `Rows * Cols`.
- HLSL emits a flat groupshared array, for example `groupshared float TileA[16 * 16];`.

Tile access is explicitly 2D:

```sdslv
TileA[localRow, kk] = value;
let x: f32 = TileB[kk, localCol];
```

`TileA[i]` is rejected so tile intent stays visible in source.

## Matrix Views

M12 adds lightweight row-major views over resource arrays:

```sdslv
let AView: matrix_view<f32> = row_major(A, params.m, params.k);
let CView: matrix_view<f32> = row_major(C, params.m, params.n);
```

A matrix view is a lowered alias containing:

- the underlying resource buffer;
- row/column extents;
- row-major stride, currently the `cols` expression;
- access mode inherited from the resource.

It does not allocate memory and does not emit as an HLSL local. The backend substitutes view indexing directly:

```sdslv
CView[row, col] = AView[row, col];
```

lowers to flat row-major HLSL indexing:

```hlsl
C[((row) * (params.n)) + (col)] = A[((row) * (params.k)) + (col)];
```

## `row_major(...)`

The builtin constructor has the form:

```sdslv
row_major(buffer, rows, cols)
```

Rules:

- `buffer` must be a shader resource array: `readonly array<T>` or `readwrite array<T>`.
- `rows` and `cols` must be integer expressions.
- the result type is `matrix_view<T>`.
- access mode is inherited from the resource.
- `rows` is retained in VD-MIR for diagnostics/inspection; indexing uses `cols` as the row-major stride.

## Access Rules

- Reading from readonly or readwrite matrix views is allowed.
- Assigning through a readwrite matrix view is allowed.
- Assigning through a readonly matrix view is rejected.
- Workgroup tiles are mutable.

## Limits

M12 intentionally does not add:

- `x[i][j]` chained indexing;
- tensor rank greater than 2;
- implicit Einstein notation;
- reductions inferred from repeated indices;
- automatic tiling or layout optimization;
- Prometheus selector retuning or dispatch authority changes.

Future tensor notation can build on this explicit 2D indexing surface without changing the current contract.

M13/M14/M14a `comptime` may compute tile dimensions and select code paths from resolved config values, including multi-way `comptime match` over values such as `C.Tile.K` and utility-scored `comptime when utility` policy choices. It may not inspect runtime `tile` or `matrix_view` values.

M15 adds `reg_tile<T, Rows, Cols>` as the local per-thread sibling to these M12 storage/view forms. It uses the same `x[row, col]` source syntax but lowers to per-thread local storage rather than `groupshared` memory or resource-backed views.
