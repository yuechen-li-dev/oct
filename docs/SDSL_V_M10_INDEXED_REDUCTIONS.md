# SDSL-V M10: Indexed Reductions

SDSL-V M10 adds explicit indexed reduction expressions for compute math without bypassing `VD-MIR`.

The pipeline remains:

```text
source -> lex -> parse -> validate -> lower to VD-MIR -> emit HLSL -> optional DXC/SPIR-V
```

## Syntax

M10a adds loop attributes on explicit reduction expressions while keeping reductions structured through `VD-MIR`.

Reduction syntax now supports an optional prefix loop attribute:

```sdslv
let acc: f32 = [unroll] sum kk in 0u..C.TILE_K {
    TileA[localRow * C.TILE_K + kk] * TileB[kk * C.TILE_N + localCol]
};
```

The attribute-free M10 syntax remains valid:

```sdslv
let acc: f32 = sum kk in 0u..C.TILE_K {
    TileA[localRow * C.TILE_K + kk] * TileB[kk * C.TILE_N + localCol]
};
```

Supported syntax shapes:

```text
reduce-expr ::= reduce-attrs? REDUCE_OP IDENT 'in' expr '..' expr ('step' expr)? '{' expr '}'
reduce-attrs ::= ('[' 'unroll' ']' | '[' 'loop' ']')+
```

Recognized reduction operators in M10:

- `sum`
- `product`
- `max`
- `min`

`max` and `min` are parsed and reserved in M10, but they are still validator-rejected with a clear diagnostic. `sum` and `product` are the working operators in this milestone.

## Validation

M10a validates:

- bounds must be integer (`i32` or `u32`);
- `step` must be a positive integer literal;
- the reduction index is scoped only inside the body expression;
- `sum` and `product` bodies must be numeric;
- only `[unroll]` and `[loop]` are accepted on reductions;
- `[unroll]` and `[loop]` are mutually exclusive on one reduction;
- reductions are currently supported only as a direct `let` initializer, assignment RHS, or `return` value.

Nested reductions such as `1.0 + sum i in 0u..N { ... }` are intentionally rejected in M10. This matches the existing `with` and `match` placement boundary and keeps HLSL lowering deterministic.

## Empty-range behavior

Current M10 behavior:

- `sum` uses the additive identity (`0`, `0u`, `0.0`) before the generated loop;
- `product` uses the multiplicative identity (`1`, `1u`, `1.0`) before the generated loop;
- `max` and `min` are deferred until the language has an explicit non-empty proof or identity story.

## VD-MIR

M10 keeps reductions as structured `VD-MIR` expressions rather than lowering directly from AST to HLSL text. The MIR node preserves:

- operator;
- index name and type;
- start/end/step expressions;
- typed body expression;
- result type.
- loop hint metadata (`none`, `unroll`, `loop`).

This keeps the backend boundary explicit and inspectable.

## HLSL lowering

HLSL lowers reductions to deterministic temp-plus-loop code.

For a direct `let` initializer:

```sdslv
let acc: f32 = [unroll] sum i in 0u..4u { A[i] };
```

the backend emits the canonical shape:

```hlsl
float acc = 0.0;
[unroll]
for (uint i = 0u; i < 4u; i += 1)
{
    acc = acc + (A[i]);
}
```

Assignment RHS and return-value reductions materialize a deterministic `__sdslv_reduce_*` temporary first, then assign or return it.

With M10a, reduction loop attributes lower through `VD-MIR` metadata rather than raw backend strings, and HLSL emits `[unroll]` / `[loop]` immediately before the generated reduction loop.

## Examples

- `examples/SDSL-V/M10/ReductionBasic.sdslv`
- `examples/SDSL-V/M10/ReductionTileShape.sdslv`
- `examples/SDSL-V/M10a/ReductionAttributes.sdslv`

These examples exercise:

- `sum` in `let`, assignment, and `return` positions;
- `product` in `let` and `return` positions;
- `[unroll] sum` and `[loop] product` on direct reduction positions;
- a tile-shaped compute expression that pressures the SGEMM-style math surface without changing Prometheus runtime dispatch.

## Limits and next steps

M10 intentionally does not add:

- implicit Einstein repeated-index inference;
- reduction identities for `max`/`min`;
- nested reduction placement inside arbitrary expression trees;
- tensor shapes or full tensor notation.

The production `sgemm_tile16x16_shared_fp32.sdslv` inner fixed accumulation loop was evaluated for a `[unroll] sum` refactor in M10a, but the repository fallback rule is to keep the explicit `[unroll] for` form if native correctness/performance lanes do not stay green. The outer runtime tile loop remains on explicit `[loop] for`.

M12 keeps reductions explicit and adds only tile/matrix view indexing. A reduction body may now read `TileA[localRow, kk] * TileB[kk, localCol]`, but repeated index names still do not imply Einstein notation or automatic reduction.

This milestone is the explicit-bounds bridge from manual scalar loops toward future tensor and Einstein-style compute notation.
