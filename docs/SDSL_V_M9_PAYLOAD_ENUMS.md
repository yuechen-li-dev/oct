# SDSL-V M9: Payload Enums and Match

SDSL-V M9 adds tagged payload enums and exhaustive enum `match` expressions to the compute-focused language subset.

The compiler pipeline remains:

```text
source -> lex -> parse -> validate -> lower to VD-MIR -> emit HLSL -> optional DXC/SPIR-V
```

M9 keeps `VD-MIR` as the backend boundary. HLSL emission does not inspect raw AST nodes for enum payload or `match` semantics.

## Syntax

Simple enums remain valid:

```sdslv
enum ShadowMode {
    None;
    Hard;
    Soft;
}
```

Payload variants add record-like fields:

```sdslv
enum LoadValue {
    Zero;
    Value { X: f32; }
}
```

Construction uses a qualified variant:

```sdslv
let z: LoadValue = LoadValue.Zero;
let v: LoadValue = LoadValue.Value { X: 1.0 };
```

Enum `match` is expression-oriented and exhaustive:

```sdslv
let out: f32 = match value {
    LoadValue.Zero => 0.0
    LoadValue.Value(payload) => payload.X
};
```

The payload binding is required for payload variants in M9 and is scoped only to that arm.

## Validation

M9 validates:

- duplicate enum variants;
- duplicate payload fields;
- unsupported payload field types;
- missing, extra, duplicate, or mistyped payload initializers;
- payload construction on simple variants;
- missing payload construction on payload variants;
- `match` subject must be an enum;
- `match` arms must cover each variant exactly once;
- wrong-enum arms and duplicate arms;
- payload binding only on payload variants;
- uniform arm result types.

Current placement boundary:

- `match` is supported as a direct `let` initializer;
- as a direct assignment RHS;
- and as a direct `return` expression.

Nested `match` inside deeper expression trees is still rejected in M9 with a clear diagnostic.

## VD-MIR

VD-MIR now models:

- enum variants with optional payload fields;
- enum construction expressions;
- enum `match` expressions;
- per-arm payload binding type information.

This keeps enum lowering deterministic and inspectable before backend emission.

## HLSL lowering

Payload enums lower to a simple tagged carrier struct plus one payload struct per payload variant.

For:

```sdslv
enum LoadValue {
    Zero;
    Value { X: f32; }
}
```

HLSL shape is:

```hlsl
static const int LoadValue_Zero = 0;
static const int LoadValue_Value = 1;

struct LoadValue_ValuePayload { float X; };

struct LoadValue {
    int Tag;
    LoadValue_ValuePayload Value;
};
```

The backend also emits deterministic constructor helpers for expression positions and lowers `match` to `if / else if` chains over `Tag`.

## Limits in M9

- payload field types are currently limited to scalar and vector value types already supported by the compute subset;
- arrays, resources, and workgroup storage are not valid payload field types;
- nested destructuring patterns, guards, and generic algebraic data types are deferred;
- `.sdslvtest` payload-enum evaluation remains deferred in M9.

## Compute use

The intended use is small helper data that makes shader control/data shape explicit without pushing SDSL-V toward CUDA-style ad hoc branching boilerplate.

The M9 examples under `Examples/SDSL-V/M9/` show:

- basic payload enum construction and exhaustive match;
- a tile-load-style helper that returns either a loaded value or an explicit zero case without changing Prometheus runtime dispatch.
