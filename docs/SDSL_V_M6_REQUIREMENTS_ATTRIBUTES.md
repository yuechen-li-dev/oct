# SDSL-V M6: Config Requirements, Static Asserts, and Backend Attributes

SDSL-V M6 adds the compile-time safety and backend-hint layer needed before real specialized compute kernels such as Prometheus SGEMM families.

The pipeline remains:

```text
SDSL-V source
  -> lex
  -> parse
  -> validate
  -> monomorphize compile declarations
  -> expand constrained comptime staging
  -> lower concrete shaders to VD-MIR
  -> emit HLSL
  -> optional DXC / SPIR-V / generated header
```

## What M6 adds

- concept-level `require` constraints;
- optional config-level `require` constraints;
- shader-scope `static assert` inside template shaders;
- loop attributes: `[unroll]`, `[loop]`;
- resource binding attribute: `[binding(n)]`;
- richer compile-time constant evaluation for requirement/assert expressions.

## `require`

M6 uses statement syntax:

```sdslv
concept TileConfig {
    TILE_SIZE: u32;
    require TILE_SIZE > 0u;
}
```

Rules:

- concept `require` is checked against every `config` implementing that concept;
- config-local `require` is also allowed and checked against that config only;
- requirement expressions must be compile-time evaluable and must produce `bool`;
- requirements do not emit to VD-MIR or HLSL.
- M11 also allows dotted concept/config paths inside requirements, such as `Threads.X * Threads.Y <= 1024u`.

## `static assert`

M6 uses shader-scope statement syntax:

```sdslv
template<C: TileConfig>
shader TileCopy {
    static assert C.TILE_SIZE <= 1024u;
}
```

Rules:

- current support is shader scope inside template shaders;
- config-dependent static asserts are evaluated during `compile Template<Config> as Alias`;
- failing asserts stop monomorphization before VD-MIR lowering;
- static asserts do not emit to VD-MIR or HLSL.
- M13 also allows `static assert` inside shader function bodies when used with constrained `comptime if`; assertions in non-selected branches do not fire.

## Constant expressions

M6 compile-time expressions support:

- integer literals;
- bool literals;
- unary `-` and `not`;
- `+`, `-`, `*`, `/`, `%`;
- `==`, `!=`, `<`, `<=`, `>`, `>=`;
- `and`, `or`;
- parentheses;
- bare config-field references such as `TILE_SIZE` inside concept/config requirements;
- template config references such as `C.TILE_SIZE` inside template static asserts and other M5 compile-time positions.

Float constant expressions remain deferred.

`!=` remains the comparison spelling. SDSL-V source rejects logical `&&`, `||`, and unary logical `!` in favor of `and`, `or`, and `not`.

M13 reuses this constant-expression machinery for `comptime let` initializers and `comptime if` conditions, with additional rejection of runtime shader values.

## Attributes

Loop attributes apply to `for` statements and, as of M10a, indexed reduction expressions in direct reduction positions:

```sdslv
[unroll]
for i in 0u..C.TILE_SIZE { ... }

[loop]
for i in 0u..params.Count { ... }

let tileAcc: f32 = [unroll] sum kk in 0u..C.TILE_K {
    TileA[rowBase + kk] * TileB[kk * C.TILE_N + localCol]
};
```

Resource binding attributes apply to resource fields:

```sdslv
stream TileCopyIO {
    [binding(0)] A: readonly array<f32>;
    [binding(1)] C: readwrite array<f32>;
}
```

Rules:

- `[unroll]` and `[loop]` are mutually exclusive on one loop;
- reductions accept `[unroll]` and `[loop]` only on a direct `let` initializer, assignment RHS, or `return` value;
- reduction attributes are backend hints only; they do not change reduction semantics;
- `[binding(n)]` requires a non-negative integer literal;
- duplicate explicit bindings in one shader resource set are rejected;
- explicit bindings use descriptor set `0`;
- unannotated resources receive deterministic implicit bindings using the next free binding number;
- unknown attributes are validation errors.

## VD-MIR and HLSL lowering

M6 keeps the backend boundary explicit:

- loop attributes lower to `VD-MIR` loop-hint metadata, not raw HLSL strings;
- reduction loop attributes also lower to `VD-MIR` loop-hint metadata;
- resource bindings lower to `VD-MIR` binding metadata with `set`, `binding`, and explicit-vs-implicit tracking;
- HLSL emits `[unroll]` / `[loop]` from `VD-MIR`;
- HLSL emits `[[vk::binding(binding, 0)]]` from `VD-MIR`.

## Example

- `examples/SDSL-V/M6/ConfigCheckedTileCopy.sdslv`

This example combines concept requirements, config validation, template static asserts, workgroup sizing from config, explicit resource bindings, a loop hint, and `compile ... as ...` specialization.
