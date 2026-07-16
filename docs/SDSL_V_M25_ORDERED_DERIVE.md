# SDSL-V M25: Ordered Immutable `derive`

M25 adds `derive`, an ordered immutable construction form for structured values.

`derive` exists to name dependent intermediate facts without forcing:

- repeated expressions
- a one-off helper function
- a mutable flow board
- extra `flow` / `state` ceremony

## Syntax

```sdslv
record LoadFacts {
    linear: u32;
    row: u32;
    col: u32;
    valid: bool;
}

let load: LoadFacts = derive {
    linear = localThreadLinear * 4u + lane;
    row = linear / tileK;
    col = linear % tileK;
    valid = row < params.M and col < params.K;
};
```

Fields evaluate in source order.
Each field becomes visible only after its initializer completes.
Later fields may reference earlier fields from the same derive block.

## Rules

- `derive` is an expression.
- M25 requires an explicit record or immutable-board target type from context.
- Valid targets are `record` and immutable local `board` values.
- Invalid targets include flow-owned mutable board initializers, streams, resources, `matrix_view`, `tile`, `reg_tile`, scalars, and arrays.
- Unknown, duplicate, and missing fields are rejected.
- Field types must match the target declaration exactly.
- A field may not reference itself.
- A field may not reference a later field.
- Derive bindings are immutable and scoped only to the derive block.
- The completed value is immutable after construction.
- `derive` is not available in compile-time structured evaluation in M25.

## Record / Board / Flow Distinction

- `record` is ordinary immutable structured data.
- `board` remains the execution scratch/state noun associated with flow logic.
- `derive` is ordered immutable construction for structured values.

M25 does not add mutation to derived values.
M25 does not change board mutation rules.
M25 does not change flow/state semantics.

Flow-owned mutable boards still require explicit board initializers:

```sdslv
flow TileLoad {
    board Load: LoadCoord = LoadCoord {
        linear: 0u;
        row: 0u;
        col: 0u;
    };
}
```

Using `derive` there is rejected because `derive` constructs immutable values while M23 flow-owned boards are the mutable scratch surface.

## Lowering

`derive` lowers through VD-MIR, not directly to HLSL source strings.

Conceptually:

```text
tmp_linear = ...
tmp_row = tmp_linear / ...
tmp_col = tmp_linear % ...
result = LoadCoord { linear = tmp_linear, row = tmp_row, col = tmp_col }
```

HLSL emission preserves this source-ordered evaluation with deterministic temporary names.

## Comptime Interaction

`comptime for` expands first.
After expansion, `derive` remains runtime ordered construction.
This makes per-lane coordinate derivation readable without introducing mutable flow state.

## M26 Follow-Up

M26 validates `derive` on a real Prometheus SGEMM kernel:

- source: `internal/prometheus/shaders/sdslv/production/sgemm/sgemm_reg2x2_tile16x16_derive_fp32.sdslv`
- report: `internal/prometheus/DevelopmentReport/SDSL_V_M26_DERIVE_SGEMM_COMPARISON.md`

The key outcome is:

- `derive` is a strong fit for immutable coordinate/fact bundles in otherwise linear GPU kernels
- it removes most of the repeated M20 coordinate algebra without requiring the M24 flow/state/board ceremony
- backend lowering stays deterministic and sane

Important limitation carried forward from M24:

- guarded `read ... when ... else 0.0` feeding a shared-tile assignment still needs inspection in generated HLSL
- the final M26 shader uses derive-based coordinates plus explicit fallback-zero temporaries before writing `TileA` / `TileB`
- that is a source-level correctness discipline under current lowering, not a new language feature
