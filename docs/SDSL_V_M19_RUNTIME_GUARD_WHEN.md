# SDSL-V M19: Oct Runtime Guard `when`

M19 adds the shader-safe subset of Oct guard `when` for bounded runtime control flow.

SDSL-V follows Oct guard `when` syntax for bounded shader flow. This is not a new dialect. Full Octomata-style flow/state support is future work and must not be faked.

Reference files used:

- `Language/reference/language/04-control-flow.md`: decision-surface split, `switch`/`when` arrow spelling, semantic boolean operators, and ordinary-vs-flow context.
- `Language/reference/runtime/21-octomata.md`: guard `when`, ordered first-true semantics, action blocks, `flow`/`state`, `goto`, `remember`, `resume`, `suspend`, board state, `when policy`, `hysteresis`, and `min_commit`.
- `Language/Expressions/UtilityWhen/M80_REPORT.md`: standalone `when utility` remains expression-level ranked choice, while `when policy` remains controller-bound and requires explicit policy fields.

## Syntax

```sdslv
when {
    case <bool_expr> -> {
        <bounded statements>
    }
    case <bool_expr> -> {
        <bounded statements>
    }
    else -> {
        <bounded statements>
    }
}
```

`else` is optional in SDSL-V M19 guard `when`. If no case matches and no `else` is present, no arm body executes. At least one `case` is required.

Both Oct arrow spellings are tokenized by the existing parser, but examples use `->`.

## Semantics

- statement form only;
- cases are evaluated in source order;
- the first true guard executes;
- no fallthrough;
- guards must typecheck as `bool`;
- bodies are bounded SDSL-V statement blocks;
- nested guard `when` is allowed;
- guarded `read ... when ... else ...` and `write ... when ...` may appear inside arm bodies.

This is not `when utility`. Guard `when` does not score candidates and does not select the highest score.

## Lowering

Runtime guard `when` lowers through VD-MIR as an ordered `IfStmt` chain. HLSL then emits deterministic `if / else if / else`.

The source spelling does not reach generated HLSL:

```hlsl
if (fullTile) {
    ...
}
else if (!fullTile) {
    ...
}
else {
    ...
}
```

`comptime when utility` remains compile-time-only. It expands before VD-MIR and is unchanged by M19.

## Relationship To `if`

Use `if` for ordinary binary local branching.

Use guard `when` when ordered guarded actions are clearer as one visible decision surface, especially tile/tail shader paths where source-order first match matters.

## Relationship To Guarded Memory Access

Guarded memory access remains expression/statement-level memory protection:

```sdslv
read AView[row, k] when row < params.M and k < params.K else 0.0
write CView[row, col] = value when row < params.M and col < params.N;
```

Runtime guard `when` is statement-level control flow:

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

## Unsupported Octomata Actions

M19 supports bounded shader statement bodies only. It does not implement persistent Octomata controller state.

Rejected actions include:

- `goto`
- `remember`
- `resume`
- `suspend`
- persistent board field assignment
- full `flow` / `state` controllers

M21 adds a different, shader-local immutable `board` value type for derived coordinate bundles. It does not change this M19 restriction: mutable or persistent board state remains unsupported outside future flow/state controllers.

Representative diagnostics:

```text
SDSL-V M19 does not support `goto` in shader flow
SDSL-V M19 supports bounded guard `when` bodies only
SDSL-V flow/state controllers are planned but not supported in M21
```

## `when policy`

`when policy` is not implemented in SDSL-V M19.

The Oct reference requires controller-bound policy state with both:

```sdslv
hysteresis: <int>
min_commit: <int>
```

Those fields are meaningful only if the implementation preserves policy memory across controller steps. Shaders in M19 do not have persistent policy state, so SDSL-V rejects `when policy` instead of parsing it and silently ignoring `hysteresis` or `min_commit`.

Diagnostic:

```text
when policy requires persistent policy state; SDSL-V M19 does not support it yet
```

## Examples

- `examples/SDSL-V/M19/GuardWhenBasic.sdslv`
- `examples/SDSL-V/M19/GuardWhenTilePath.sdslv`
- `examples/SDSL-V/M19/GuardWhenWithComptimeFor.sdslv`
- `examples/SDSL-V/M21/BoardGuardWhenTileLoad.sdslv`

## Prometheus M20 Usage

`internal/prometheus/shaders/sdslv/sgemm_reg2x2_tile16x16_exacttail_fp32.sdslv` uses runtime guard `when` to split exact-tile SGEMM loads/stores from tail-safe guarded memory access. The generated HLSL keeps direct exact-path A/B loads outside the guarded-read fallback temp blocks while preserving guarded fallback behavior in the tail path.
