# SDSL-V M1: VD-MIR

SDSL-V M1 introduces `VD-MIR` as the backend-neutral compiler boundary for the GoOct shader-authoring experiment.

The intended pipeline is now:

```text
SDSL-V source
  -> lex
  -> parse
  -> validate
  -> lower to VD-MIR
  -> emit HLSL
  -> later: DXC / SPIR-V
```

## What VD-MIR is

`VD-MIR` is a typed, deterministic middle layer for the current M0 compute subset.

For now, the name means exactly `VD-MIR`. Future documentation may expand it as Vulkan/Dominatus MIR or Vector/Data MIR, but the compiler and docs should use the exact term `VD-MIR`.

VD-MIR is deliberately:

- backend-neutral
- structured rather than SSA-based
- typed at declaration, statement, and expression boundaries
- deterministic to dump and deterministic to emit from

HLSL is no longer treated as the internal truth of SDSL-V. HLSL is now only the first backend consuming VD-MIR.

## M1 supported subset

M1 keeps scope aligned with the GoOct SDSL-V M0 compute subset:

- `namespace`
- `type` aliases
- `record`
- `enum`
- `shader`
- `resources`
- ordinary `fn`
- `stage compute [numthreads(...)] fn`
- M0 expressions and statements, including `when utility`

The MIR models:

- module namespace, aliases, records, enums
- resource table with read-only vs read-write access
- functions with emitted names, parameters, locals, and structured bodies
- compute entry points with `numthreads` and builtin metadata
- typed expressions including `when utility`

As of M9, the enum/match portion of that boundary also includes:

- payload enum variants;
- enum construction expressions;
- exhaustive enum `match` expressions with per-arm payload binding metadata.

M3 later extends this boundary with compute streams, named resource bundles, and `with` expressions without routing through Oct MIR or bypassing VD-MIR.
M4 further extends it with workgroup storage declarations and backend-neutral barrier intrinsics.
M5 further extends it with template-shader monomorphization before lowering, so VD-MIR still receives only concrete shaders.
M6 further extends it with loop-hint metadata and explicit-vs-implicit resource binding metadata, still without leaking raw backend strings into the AST.
M10 further extends it with structured indexed reduction expressions so compute-math lowering still crosses the backend boundary through typed MIR rather than raw emitted loops.

## `when utility` in VD-MIR

M1 keeps `when utility` as a first-class VD-MIR expression instead of erasing it during lowering.

Each case retains:

- candidate value
- guard expression
- score expression

plus the `else` expression.

Tie semantics remain deterministic first-wins because the HLSL backend only replaces the current best value when a later score is strictly greater.

## HLSL backend boundary

GoOct M1 routes `oct sdslv emit-hlsl` through:

```text
parse -> validate -> lower to VD-MIR -> emit HLSL
```

The HLSL backend no longer emits directly from the raw SDSL-V AST.

This gives room for future backends such as:

- `VD-MIR -> HLSL`
- `VD-MIR -> Slang`
- `VD-MIR -> PTX`

DXC, SPIR-V generation, and runtime dispatch wiring are still deferred.

## Dumping VD-MIR

Use:

```powershell
go run ./cmd/oct sdslv emit-vdmir Examples/SDSL-V/M0/VectorAdd.sdslv
```

The dump is human-readable and deterministic, but it is not yet a stable external serialization format.

Example shape:

```text
vdmir module Prometheus.Kernels
resource readonly A: array<f32>
resource readwrite C: array<f32>
entry compute VectorAdd_CS numthreads(16,16,1)
  param params: VectorParams
  builtin DispatchThreadID: uint3 semantic SV_DispatchThreadID referenced=true
```

## Diagnostics and provenance

The current SDSL-V AST does not yet carry full source spans. M1 therefore preserves file-level provenance on VD-MIR nodes and keeps the model ready for richer declaration/statement/expression spans in a follow-up milestone.

## `.sdslvtest`

`.sdslvtest` remains AST-interpreter-based in M1.

This is intentional. The goal of M1 is to insert a real backend boundary for shader emission, not to force the test interpreter through VD-MIR before that becomes useful.

## Non-goals

M1 does not add:

- SSA
- optimization passes
- DXC invocation
- SPIR-V generation
- Slang or PTX backends
- SGEMM kernel generation
- Prometheus runtime dispatch changes
- FFT/P16 changes
