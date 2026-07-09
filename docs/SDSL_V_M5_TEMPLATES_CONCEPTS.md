# SDSL-V M5: Templates and Concepts M0 for Compute Shader Specialization

SDSL-V M5 adds a narrow compile-time specialization system for compute shader families.

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

## What M5 adds

- `concept` declarations as compile-time config schemas;
- `config` declarations as concrete compile-time values;
- single-parameter `template<Param: Concept>` shader declarations;
- `compile TemplateShader<Config> as Alias;` monomorphization;
- narrow constant-expression support for:
  - workgroup array sizes
  - `numthreads(...)`
  - ordinary shader expressions

## Supported forms

```sdslv
concept SgemmTileConfig {
    TILE_M: u32;
    TILE_N: u32;
    TILE_K: u32;
}

config Tile16x16x16: SgemmTileConfig {
    TILE_M: 16u;
    TILE_N: 16u;
    TILE_K: 16u;
}

template<C: SgemmTileConfig>
shader SgemmTile {
    workgroup TileA: array<f32, C.TILE_M * C.TILE_K>;
    stage compute [numthreads(C.TILE_M, C.TILE_N, 1u)] fn CS() -> void {
        let tileElements: u32 = C.TILE_M * C.TILE_N;
        return;
    }
}

compile SgemmTile<Tile16x16x16> as SgemmTile16;
```

## Constant expressions

Current M5 constant expressions support:

- integer literals
- bool literals
- unary `-` on integers
- `+`, `-`, `*`, `/`, `%`
- `==`, `!=`, `<`, `<=`, `>`, `>=`
- `&&`, `||`
- parentheses
- `C.FIELD` template-config field references

`f32` concept fields are still intentionally narrow in this milestone. Integer and bool config values are the intended path for compute kernel families.

## Monomorphization model

M5 monomorphizes before VD-MIR lowering.

- Template shaders do not emit on their own.
- Each `compile` declaration clones the template shader into a concrete shader alias.
- `C.FIELD` references are substituted with concrete config literals.
- Workgroup sizes and `numthreads` become concrete integers before VD-MIR.
- Entry points use the compile alias name, such as `TileCopy16x16_CS`.

This keeps template logic out of both VD-MIR and the HLSL backend.

M13 adds a separate `comptime` pass after monomorphization. Templates/configs still choose concrete variants; `comptime let` and `comptime if` only shape code inside the concrete specialized shader. See `docs/SDSL_V_M13_CONSTRAINED_COMPTIME.md`.

## Generated metadata convention

When a concrete compute shader is generated through `compile Template<Config> as Alias;`, the specialized config values are also available to later toolchain stages.

- `numthreads(...)` always becomes concrete entry metadata.
- Integer config fields can be emitted into generated headers as deterministic constants.
- For SGEMM-family kernels, the current metadata convention uses config fields such as:
  - `OUTPUTS_PER_INVOCATION_M`
  - `OUTPUTS_PER_INVOCATION_N`
  - `TILE_M`
  - `TILE_N`
  - `UNROLL_K`

This lets future `TILE16x16` or rectangular kernels generate both shader code and host-consumed dispatch metadata from the same compile-time config.

## Current limits

M5 intentionally does not add:

- runtime generics
- interface dispatch
- multiple template parameters
- generic functions, records, or streams
- concept methods or inheritance
- payload enums
- tensor notation
- SGEMM kernels themselves

M6 extends this model with:

- concept/config `require` constraints;
- shader-scope `static assert`;
- backend attributes such as `[unroll]`, `[loop]`, and `[binding(n)]`.

See `docs/SDSL_V_M6_REQUIREMENTS_ATTRIBUTES.md`.

M11 extends the same compile-time model with structured config groups, fat-arrow assignments, defaults, dotted references, and nonzero-by-default `u32` config fields. See `docs/SDSL_V_M11_STRUCTURED_CONFIGS.md`.

M13 adds constrained compile-time shader staging with `comptime let` and `comptime if`. See `docs/SDSL_V_M13_CONSTRAINED_COMPTIME.md`.

## Examples

- `examples/SDSL-V/M5/TemplateTileConfig.sdslv`
- `examples/SDSL-V/M5/TemplateWorkgroupTileCopy.sdslv`
