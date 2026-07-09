# SDSL-V M11: Structured Configs, Defaults, and Nonzero-by-Default Fields

SDSL-V M11 reduces concept/config boilerplate without changing the backend boundary:

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

Concepts and configs remain compile-time only. VD-MIR remains the boundary. Runtime shader semantics do not change unless source is migrated.

M13 keeps this separation: structured configs choose concrete variants, while constrained `comptime` shapes code inside those variants after monomorphization. M14 adds `comptime match` for multi-way structural selection over resolved config fields and prior comptime values. M14a adds `comptime when utility` for utility-scored compile-time arbitration over those same resolved values.

## What M11 adds

- structured concept field groups;
- dotted config field paths;
- fat-arrow config assignments: `Path.To.Field => expr;`;
- concept field defaults;
- nonzero-by-default `u32` concept/config fields;
- explicit zero-permitted `u32!` concept/config fields;
- dotted config references in defaults, `require`, and template specialization.

## Structured concept fields

```sdslv
concept SharedTileSgemmConfig {
    Threads: {
        X: u32;
        Y: u32;
    };

    Tile: {
        K: u32;
    };
}
```

Groups are compile-time namespaces, not runtime records and not enums. Dotted leaf paths are formed from group names plus the leaf field name, such as `Threads.X` and `Tile.K`.

## Fat-arrow config assignments

```sdslv
config Tile16x16: SharedTileSgemmConfig {
    Threads.X => 16u;
    Threads.Y => 16u;
    Tile.K => 16u;
}
```

Legacy flat `FIELD: value;` config syntax remains accepted for existing flat concepts, but M11 syntax should prefer `=>`. Mixing `:` and `=>` within one config is rejected.

## Defaults

```sdslv
OutputsPerInvocation: {
    M: u32 = 1u;
    N: u32 = 1u;
};

Tile: {
    M: u32 = Threads.X * OutputsPerInvocation.M;
    N: u32 = Threads.Y * OutputsPerInvocation.N;
    K: u32;
};
```

Defaults must be compile-time expressions. They may reference earlier fields by dotted path. Configs may omit fields with defaults; after assignments plus defaults, every field must resolve to a concrete constant.

## Nonzero-by-default `u32`

Inside concept/config field declarations only:

- `u32` means zero is rejected by default;
- `u32!` means zero is explicitly allowed.

```sdslv
Padding: {
    K: u32! = 0u;
};
```

This rule applies only to compile-time concept/config fields. Ordinary runtime shader variables still use normal `u32` semantics and may be zero.

## Dotted references

Concept requirements and template specialization use dotted paths:

```sdslv
require Threads.X * Threads.Y <= 1024u;

stage compute [numthreads(C.Threads.X, C.Threads.Y, 1u)] fn CS() -> void {
    let tileM: u32 = C.Tile.M;
    return;
}
```

## Metadata and generated headers

Config expansion still produces a deterministic flat constant map before VD-MIR lowering. Structured dotted paths are flattened deterministically for metadata and generated headers:

- `Threads.X` -> `THREADS_X`
- `OutputsPerInvocation.M` -> `OUTPUTS_PER_INVOCATION_M`
- `Tile.K` -> `TILE_K`

This keeps Prometheus-native metadata naming stable while allowing more declarative source configs.

## Current limits

M11 does not add:

- config sets / compile-each;
- tensor notation;
- implicit Einstein notation;
- Prometheus runtime dispatch changes;
- selector retuning;
- P15 changes;
- FFT/P16 changes;
- any Oct MIR routing for SDSL-V.

Future configset / compile-each work can build on the structured config model, but it is intentionally deferred.
