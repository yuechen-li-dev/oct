# SDSL-V M21 - Shader-Local Board Values

M21 adds shader-local immutable `board` values. This gives shader code a structured way to name derived per-phase facts such as load coordinates, output coordinates, tile coordinates, and policy facts.

This intentionally respects Oct's convention that mutable board state belongs inside flow/state control. SDSL-V M22 now adds bounded shader-local `flow` / `state` blocks, but mutable board state and persistent Octomata flow remain deferred.

## Board Declarations

`board` declares a structured shader-local value type:

```sdslv
board LoadCoord {
    linear: u32;
    row: u32;
    col: u32;
}
```

Board declarations are type declarations. They do not allocate memory, do not create resource bindings, and do not affect dispatch metadata or host ABI.

M21 board fields are restricted to shader-local value types: `bool`, `i32`, `u32`, `f32`/`float`, and supported scalar vector types such as `uint2`, `uint3`, `uint4`, `float2`, `float3`, and `float4`.

Not supported as board fields in M21: resources, resource bundles, `matrix_view`, workgroup `tile`, `reg_tile`, arrays, function types, references, pointers, or nested boards.

## Literals And Field Access

Board literals must provide every declared field exactly once:

```sdslv
let p: LoadCoord = LoadCoord {
    linear: linear;
    row: linear / tileK;
    col: linear % tileK;
};
```

Missing, duplicate, unknown, or mistyped fields are rejected. Field initializer expressions do not introduce sequential bindings, so compute shared derived values before the literal:

```sdslv
let linear: u32 = localThreadLinear * 4u + lane;
let p: LoadCoord = LoadCoord {
    linear: linear;
    row: linear / tileK;
    col: linear % tileK;
};
```

Field access works as a normal expression:

```sdslv
TileA[p.row, p.col] =
    read AView[groupBaseRow + p.row, tileBaseK + p.col]
        when groupBaseRow + p.row < params.M and tileBaseK + p.col < params.K
        else 0.0;
```

## Immutability

Board values are immutable in M21. Local board values and helper returns are supported, but whole-board reassignment and field assignment are rejected. M22 does not change this. M23 adds a separate flow-owned mutable board-instance surface inside bounded `flow` / `state`, but ordinary M21 board values remain immutable snapshots.

## Lowering

Boards lower through VD-MIR as board declarations, board construction expressions, and ordinary field access. The HLSL backend emits board declarations as local/helper `struct` types and emits board literals through deterministic field assignment into temporary or local struct values.

The generated HLSL uses the VD-MIR representation; boards are not lowered directly from source AST to HLSL strings.

## Comptime And Guards

M21 does not add structured consteval board values. `comptime let p: LoadCoord = ...` is rejected.

Board literals may appear inside `comptime for` bodies after expansion. Board fields may be used in tile, `reg_tile`, and `matrix_view` indices, guarded reads/writes, and runtime guard `when` conditions or bodies.

## Future Work

Mutable board state, persistent board memory, `flow`/`state`, `goto`, `remember`, `resume`, `suspend`, and `when policy` remain deferred. The intended ladder is:

1. M21: immutable shader-local board values.
2. M22: bounded shader-local flow/state blocks.
3. M23: mutable board inside flow/state only.
