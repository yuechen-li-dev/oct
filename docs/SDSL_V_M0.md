# SDSL-V M0

SDSL-V M0 adds a GoOct-native compiler scaffold for shader authoring experiments before Prometheus receives new SGEMM kernels. It is intentionally a runway milestone: parse, validate, test, and emit deterministic HLSL for a compute-focused subset, without changing Prometheus runtime dispatch or replacing existing SPIR-V headers.

## Compute extension

The supplied SDSL-V spec documents only `vertex` and `pixel` stages. GoOct SDSL-V M0 adds an explicit Prometheus compute extension:

```sdslv
shader VectorAdd {
    resources {
        A: readonly array<f32>;
        C: readwrite array<f32>;
    }

    stage compute [numthreads(16, 16, 1)] fn CS(params: VectorParams) -> void {
        let index: u32 = DispatchThreadID.x;
        return;
    }
}
```

Compute entry points emit as `ShaderName_MethodName` and automatically receive HLSL system-value parameters:

```hlsl
[numthreads(16, 16, 1)]
void VectorAdd_CS(VectorParams params, uint3 DispatchThreadID : SV_DispatchThreadID, ...)
```

`DispatchThreadID`, `GroupThreadID`, `GroupID`, and `GroupIndex` are reserved builtin names. M0 supports `DispatchThreadID.x/y/z` and reserves the rest for SGEMM follow-up work.

## Implemented subset

M0 supports `namespace`, no-op `use`, `type` aliases, `record`, `enum`, `shader`, `resources`, ordinary `fn`, and `stage compute [numthreads(...)] fn`.

Types: `void`, `bool`, `i32`, `u32`, `f32`, `float`, `float2`, `float3`, `float4`, `array<T>`, `array<T, N>`, and record types.

Statements: `let`, assignment, `return`, `if/else`, bounded `for i in start..end step n`, and expression statements for calls.

Expressions: identifiers, integer/float/bool/string literals, field access, array index, calls, arithmetic, comparisons, unary negation, and `when utility`.

Resources use the M0 syntax:

```sdslv
resources {
    A: readonly array<f32>;
    C: readwrite array<f32>;
}
```

They emit to `Buffer<T>` or `RWBuffer<T>`. Binding/register policy is deferred to the Prometheus SGEMM integration milestone.

## Deferred

Interfaces, generic shaders, compile declarations, streams, vertex/pixel lowering, coordinate spaces, fallibility, `flow`, `switch`, `match`, full `.sdslvtest` theory support, DXC invocation, SPIR-V output, and runtime Prometheus dispatch changes are deferred. Unsupported top-level features produce a clear `not implemented in GoOct SDSL-V M0` diagnostic.

## CLI

```powershell
go run ./cmd/oct sdslv check examples/SDSL-V/M0/VectorAdd.sdslv
go run ./cmd/oct sdslv emit-hlsl examples/SDSL-V/M0/VectorAdd.sdslv -o out/sdslv/vector_add.hlsl
go run ./cmd/oct sdslv test examples/SDSL-V/M0/basic.sdslvtest
```

## Prometheus path

Prometheus SGEMM Px16 showed that one-off SPIR-V patching is not enough. SDSL-V M0 creates a source-backed shader loop so later milestones can generate coherent compute kernel families, validate shader-helper decisions with `.sdslvtest`, emit deterministic HLSL, and then add DXC/SPIR-V and runtime wiring intentionally.
