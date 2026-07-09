# SDSL-V M4: Workgroup Memory, Barriers, and Full Compute Builtins

SDSL-V M4 adds the core GPU-compute execution primitives needed before real tiled kernels such as Prometheus SGEMM:

- shader-scoped `workgroup` memory declarations;
- backend-neutral workgroup/barrier builtins;
- complete compute system-value coverage for both `ComputeThread` and legacy builtin style.

The pipeline remains:

```text
SDSL-V source
  -> lex
  -> parse
  -> validate
  -> lower to VD-MIR
  -> emit HLSL
  -> optional DXC / SPIR-V / generated header
```

## `workgroup` keyword

SDSL-V source uses:

```sdslv
workgroup Tile: array<f32, 256>;
```

It does not expose HLSL's `groupshared` spelling directly. `workgroup` is the backend-neutral source keyword so the same program shape can lower to:

- HLSL `groupshared`
- VD-MIR workgroup storage
- future SPIR-V `Workgroup`
- future CUDA `__shared__`
- future Metal `threadgroup`

Current M4 rules:

- workgroup declarations are shader-scoped only;
- current M4 type is `array<T, N>` only;
- `N` must be a positive integer literal;
- runtime-sized workgroup arrays are rejected;
- initializers are rejected;
- workgroup storage is mutable inside compute code.

## Compute builtin model

M4 completes the compute builtin surface:

- `DispatchThreadID : uint3`
- `GroupID : uint3`
- `GroupThreadID : uint3`
- `GroupIndex : u32`

Two source styles are supported:

1. `ComputeThread` stream style such as `thread.DispatchId.x`
2. legacy builtin style such as `DispatchThreadID.x`

The HLSL backend emits the full deterministic system-value parameter set once and reuses it for either source style.

## Barrier builtins

Required M4 builtins:

```sdslv
WorkgroupBarrier();
WorkgroupMemoryBarrier();
WorkgroupMemoryBarrierWithSync();
```

Current HLSL backend mapping:

- `WorkgroupBarrier()` -> `GroupMemoryBarrierWithGroupSync()`
- `WorkgroupMemoryBarrier()` -> `GroupMemoryBarrier()`
- `WorkgroupMemoryBarrierWithSync()` -> `GroupMemoryBarrierWithGroupSync()`

`WorkgroupBarrier()` is currently a safe alias for full group memory barrier plus synchronization in the HLSL backend. A more precise backend split can be introduced later if needed.

Barrier builtins:

- return `void`;
- require zero arguments;
- may only be used as expression statements;
- are intended for compute entry points and compute helpers, not non-compute stage code.

## VD-MIR boundary

M4 keeps the boundary explicit:

- workgroup memory lowers to a real `VD-MIR` workgroup storage declaration;
- barrier calls lower to `VD-MIR` intrinsic calls;
- HLSL is still emitted from `VD-MIR`, not directly from the AST.

That keeps the future SGEMM path clean without routing SDSL-V through Oct MIR or leaking backend-specific spellings into source.

## Current limitations

M4 intentionally does not add:

- templates or concepts;
- payload enums;
- tensor notation;
- SGEMM kernel wiring;
- Prometheus runtime dispatch changes;
- selector changes;
- FFT/P16 changes.

## Examples

- `examples/SDSL-V/M4/WorkgroupTileCopy.sdslv`
- `examples/SDSL-V/M4/BarrierSmoke.sdslv`

These examples keep scope small while proving the source model needed for future tiled kernels.
