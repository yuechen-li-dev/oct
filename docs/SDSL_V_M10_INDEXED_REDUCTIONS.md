# SDSL-V M10: Indexed Reductions

SDSL-V M10 adds explicit indexed reduction expressions for compute math without bypassing `VD-MIR`.

The pipeline remains:

```text
source -> lex -> parse -> validate -> lower to VD-MIR -> emit HLSL -> optional DXC/SPIR-V
```

## Syntax

M10 adds explicit reduction expressions with visible bounds:

```sdslv
let acc: f32 = sum kk in 0u..C.TILE_K {
    TileA[localRow * C.TILE_K + kk] * TileB[kk * C.TILE_N + localCol]
};
```

Supported syntax shape:

```text
reduce-expr ::= REDUCE_OP IDENT 'in' expr '..' expr ('step' expr)? '{' expr '}'
```

Recognized reduction operators in M10:

- `sum`
- `product`
- `max`
- `min`

`max` and `min` are parsed and reserved in M10, but they are still validator-rejected with a clear diagnostic. `sum` and `product` are the working operators in this milestone.

## Validation

M10 validates:

- bounds must be integer (`i32` or `u32`);
- `step` must be a positive integer literal;
- the reduction index is scoped only inside the body expression;
- `sum` and `product` bodies must be numeric;
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

This keeps the backend boundary explicit and inspectable.

## HLSL lowering

HLSL lowers reductions to deterministic temp-plus-loop code.

For a direct `let` initializer:

```sdslv
let acc: f32 = sum i in 0u..4u { A[i] };
```

the backend emits the canonical shape:

```hlsl
float acc = 0.0;
for (uint i = 0u; i < 4u; i += 1)
{
    acc = acc + (A[i]);
}
```

Assignment RHS and return-value reductions materialize a deterministic `__sdslv_reduce_*` temporary first, then assign or return it.

M10 does not attach loop attributes such as `[unroll]` to reductions yet. That is why the production tile16 SGEMM shader remains on its explicit `[unroll] for` loop in this milestone even though the language now supports reduction syntax.

## Examples

- `examples/SDSL-V/M10/ReductionBasic.sdslv`
- `examples/SDSL-V/M10/ReductionTileShape.sdslv`

These examples exercise:

- `sum` in `let`, assignment, and `return` positions;
- `product` in `let` and `return` positions;
- a tile-shaped compute expression that pressures the SGEMM-style math surface without changing Prometheus runtime dispatch.

## Limits and next steps

M10 intentionally does not add:

- implicit Einstein repeated-index inference;
- reduction identities for `max`/`min`;
- nested reduction placement inside arbitrary expression trees;
- reduction attributes such as `[unroll] sum ...`;
- tensor shapes or full tensor notation.

This milestone is the explicit-bounds bridge from manual scalar loops toward future tensor and Einstein-style compute notation.
