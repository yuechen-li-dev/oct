# SDSL-V M32b.1: tensor backend materialization and rank-general layout

## Implemented lowering boundary

M32b consumes `validate.ValidatedTensorAssign` before specialization and keys
that compiler-owned handoff by its preserved source span. The backend never
reparses tensor syntax or infers tensor shapes. The lowering produces a
backend-neutral `vdmir.TensorAssign`, ordered `vdmir.TensorIndex` domains, and
`vdmir.TensorReductionExpr` for `Sum`.

| Source form | Initial lowering |
| --- | --- |
| elementwise tensor assignment | nested free-index loops |
| `Sum` reduction | ordered nested reduction loops and an accumulator |
| compound tensor `+=` | one destination read and one final write |
| rank-general fixed array | flattened linear storage plus ordered row-major offset |

The shared HLSL statement emitter owns tensor loop emission. It uses the
existing scalar expression, arithmetic, indexed-read, guarded-read, inline
HLSL, local-temporary, and type paths; no tensor-only HLSL emitter is added.
Generated loop and temporary names use the `__sdslv_tensor_` internal prefix.
Source markers cover the tensor statement, free/reduction loops, accumulator
initialization, reduction body, and final write.

`Sum` initializes `f32`, `i32`, and `u32` accumulators with `0.0`, `0`, and
`0u` respectively. Free domains follow M32a destination order and reduction
domains follow explicit `Sum[...]` order. Reordering is not permitted here.

## M32b.1 physical layout

Nested statically sized `array` values use one compiler-owned linear HLSL
array. For shape `[D0, D1, ..., Dn]` and logical indices
`[i0, i1, ..., in]`, VD-MIR carries the ordered extents and the backend emits:

```text
offset = (((i0 * D1 + i1) * D2 + i2) ...)
```

Rank one is the identity offset and rank two is the same row-major formula
already used by linear matrix storage. The compiler checks that each indexed
fixed-array axis has a positive static extent. Tiles, register tiles,
matrix views, runtime resource arrays, and compiler-owned test resources keep
their existing category-specific physical nodes and limits.

## M32b.1 materialization

The shared HLSL backend has one expression materialization path: an ordinary
VD-MIR expression emits zero or more prelude statements and yields a stable
value. It handles scalar/indexed expressions, calls, guarded reads, inline
HLSL, ordinary reductions, and tensor reductions. Operands with preludes are
visited left-to-right and use deterministic hygienic temporaries. Guarded
sources remain inside the true branch and inline HLSL remains owned by the
ordinary source-marker emitter.

Tensor destinations use a shared-backend materialized indexed-lvalue record.
It retains the resolved base, ordered address components, row-major offset
where applicable, value type, and tensor source span. Tensor `+=` computes
that address once, uses it for one old-value read, evaluates the RHS once, and
reuses the same address for the one final write. Tensor `=` uses the same
address boundary and keeps its single-write semantics.

## Validation status

Focused VD-MIR and shared-emitter tests cover ordered free/reduction domains,
typed zero initialization, source spans, rank-one through rank-four extents
and offsets, flattened declarations, hygienic generated names, left-to-right
preludes, one-shot compound destinations, guarded reads, inline HLSL, and the
absence of supported-expression placeholders.

`examples/SDSL-V/M32b1/TensorBackendMaterialization.sdslv` is the focused
real-path compiler proof. It combines a rank-four fixed tensor, row-major
indexed reads/writes, and an inline-HLSL tensor operand. On July 11, 2026 it
passed `sdslv check`, deterministic VD-MIR emission, shared HLSL emission, and
DXC `cs_6_0` SPIR-V generation (`vulkan1.0`, optimized output: 1,556 bytes).
This is a compiler/backend proof only; it deliberately does not claim GPU
execution.

The complete M32b hardware matrix, `.sdslvtest` tensor suite, and SGEMM parity
proof intentionally remain M32b.2 work. M32b.1 establishes the backend
prerequisite and does not claim those hardware results.
