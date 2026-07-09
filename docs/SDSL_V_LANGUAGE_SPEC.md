# SDSL-V Language Specification

*Derived from the WyrmCoil (`src/Engine/shader/sdslv/`) and Aurelian (`src/Aurelian.Shaders/`) implementations. Authoritative source is the parser, validator, and emitter — not documentation.*

*Milestone markers (M0, M13, M55c, M56, M58, M59b, M60, M61, M62, M63, M64b/c, M65, M66b) appear where the implementation explicitly bounds a feature.*

---

## Overview

SDSL-V is a shader language that compiles to HLSL via DXC, targeting SPIR-V. Its design embeds the flow/board/state/when semantics of the Dominatus/Octomata behavioral model directly into the shader pipeline. A flow is a value-returning finite state machine — a stateful routing function that lowers to a standard HLSL helper. A when utility is a utility-scored branch expression — a ranked selection that lowers to an if/else-if chain.

The broad design pipeline is: `source → lex → parse → validate → template/config monomorphization → comptime expansion → lower to VD-MIR → emit HLSL → DXC → SPIR-V`.

Current GoOct M2 supports an opt-in DXC/SPIR-V/header generation lane for the compute-focused subset. Prometheus runtime wiring remains deferred.

---

## Module structure

A module is a single `.sdslv` file. Files have an optional `namespace` declaration and optional `use` imports, followed by top-level declarations. All top-level names participate in a flat namespace within the module — duplicate names are rejected at validation.

```sdslv
namespace WyrmCoil.Examples;

use WyrmCoil.Core;

// declarations follow
```

Paths use `.` as separator: `WyrmCoil.Core`, `IBaseColor`, `TMat`.

---

## Top-level declarations

Current GoOct SDSL-V accepts these top-level declaration kinds:

| Keyword | Form | Purpose |
|---|---|---|
| `type` | `type Name = TypeRef @space(...);` | Type alias, optionally space-annotated |
| `record` | `record Name { fields }` | Plain aggregate struct (no stage semantics) |
| `stream` | `stream Name { fields }` | Stage I/O struct (auto-assigned HLSL semantics) |
| `concept` | `concept Name { FIELD: Type; Group: { Field: Type = expr; }; require expr; }` | Compile-time config schema, grouped fields, defaults, and constraints |
| `config` | `config Name: ConceptName { FIELD: const_expr; Path.To.Field => const_expr; require expr; }` | Concrete compile-time config values |
| `interface` | `interface Name { fn signatures }` | Abstract method contract |
| `template` + `shader` | `template<C: Concept> shader Name { ... }` | Compile-time specialized shader template |
| `shader` | `shader Name { ... }` | Concrete shader program |
| `flow` | `flow Name(params) -> ReturnType { board? states }` | Value-returning FSM |
| `compile` | `compile TemplateShader<Config> as Alias;` | Monomorphize a template shader |
| `enum` | `enum Name { Variant; Variant { Field: Type; } }` | Tagged value enum with optional payload |

---

## Type system

### Primitive types

| SDSL-V | HLSL | Notes |
|---|---|---|
| `bool` | `bool` | Condition operands only |
| `i32` | `int` | Default integer literal type |
| `u32` | `uint` | |
| `uint2` | `uint2` | Compute vector |
| `uint3` | `uint3` | Compute vector |
| `uint4` | `uint4` | Compute vector |
| `f32` | `float` | Alias for `float`; interchangeable in non-space contexts |
| `float` | `float` | Same underlying as `f32` |
| `float2` | `float2` | Vector; constructor `float2(s, s)` |
| `float3` | `float3` | Vector; constructor `float3(s, s, s)` |
| `float4` | `float4` | Vector; constructor `float4(s, s, s, s)` |
| `float4x4` | `float4x4` | Matrix; constructor `float4x4(s×16)` |
| `string` | — | Recognized as a type name; no HLSL lowering |
| `Error` | — | Fallible error payload type |

Vector/matrix constructors require exact arity of numeric scalar arguments. Passing a wrong count or a non-numeric scalar (e.g. `bool`) is a validation error. Array literals (`[1.0, 2.0]`) cannot initialize vector/matrix types — use constructors instead.

### Array types

```sdslv
let weights: array<f32, 4>;
let weights: array<f32, 4> = [1.0, 2.0, 3.0, 4.0];
```

Array literals lower to sequential element assignments in HLSL. Literal length must match the declared size. Element types must match the array element type. Array parameters are immutable — element assignment via `weights[i] = v` is rejected on parameters; only local array variables may be mutated.

### Tile and matrix view types (GoOct M12)

M12 adds explicit 2D compute views:

```sdslv
workgroup TileA: tile<f32, 16, 16>;
let AView: matrix_view<f32> = row_major(A, params.M, params.K);
```

`tile<T, Rows, Cols>` is currently valid only for shader-scoped `workgroup` declarations. `Rows` and `Cols` must be positive compile-time integer expressions, and HLSL emits flat `groupshared` storage sized `Rows * Cols`.

`matrix_view<T>` is a lightweight local view over a resource array created by `row_major(buffer, rows, cols)`. It does not allocate storage. Access mode is inherited from the source resource: readonly views can be read but not assigned through; readwrite views can be read and written.

### Type aliases and coordinate spaces

```sdslv
type Color = float4;
type ClipPosition4 = float4 @space(clip.position);
type WorldPosition3 = float3 @space(world.position);
type WorldNormal3 = float3 @space(world.normal);
```

`@space(path)` annotates a type alias with a coordinate space. Space-annotated aliases are **semantically distinct** from their underlying type and from other space-annotated aliases with the same underlying type — `ClipPosition4` and `WorldPosition3` are both `float4` but are incompatible with each other and with plain `float4` in assignment, call argument, and return positions. Two space-annotated aliases with the same underlying type and the same underlying type *are* compatible only when one of the aliases resolves directly to the other's space annotation. The space annotation emits as a comment in HLSL.

### Enums

```sdslv
enum ShadowMode { None; Hard; Soft; }

enum LoadValue {
    Zero;
    Value { X: f32; }
}
```

Simple variants use `Variant;`.
Payload variants use `Variant { Field: Type; }`.

Construction uses a qualified variant name:

```sdslv
let z: LoadValue = LoadValue.Zero;
let v: LoadValue = LoadValue.Value { X: 1.0 };
```

Current GoOct M9 lowers enums to deterministic tagged carrier structs in HLSL. Tag constants emit in declaration order starting at `0`: `ShadowMode_None = 0`, `ShadowMode_Hard = 1`, ...

### Compatibility rules

- `f32` and `float` are interchangeable.
- `i32` and `u32` are not interchangeable with float types.
- A non-space-annotated type alias is compatible with its underlying type.
- Two space-annotated aliases are incompatible even if their underlying type is the same (e.g. `WorldVector3` vs `WorldNormal3`, both `float3`).
- A space-annotated alias is incompatible with its underlying primitive type.

---

## Records and streams

### Record

A record is a plain aggregate. Fields have no stage semantics. Fields cannot be assigned on parameters — use `with` to produce a modified copy.

```sdslv
record SurfaceData {
    WorldPos: float3;
    Normal: float3;
    BaseColor: float4;
    Roughness: f32;
}
```

Emits as a plain HLSL struct with no semantic annotations.

### Stream

A stream is a stage I/O aggregate. Fields are automatically assigned HLSL stage semantics on emission. A field typed with a `@space(clip.position)` alias maps to `SV_Position`; all other fields map to `TEXCOORD0`, `TEXCOORD1`, ... in declaration order. Only one `SV_Position` field per stream is valid.

```sdslv
stream VertexOut {
    Position: ClipPosition4;   // → SV_Position
    Color: float4;             // → TEXCOORD0
    WorldPos: WorldPosition3;  // → TEXCOORD1
}
```

Stream parameters are immutable. Field assignment on a stream parameter is rejected. Use `with` to produce a modified copy.

### Compute stream note (GoOct M3)

The original `stream` model above describes graphics stage I/O semantics.

GoOct SDSL-V M3 also supports compute-oriented streams:

- `ComputeThread`-style thread/system-value payloads;
- plain structured compute helper payloads;
- named resource bundles used by `resources BundleName;`.

Current M3 limitation: named resource bundles accept access-qualified `array<T>` resource fields. Mixed bundle streams that combine resources with plain payload fields remain deferred.

GoOct SDSL-V M4 adds shader-scoped `workgroup` declarations and compute barrier builtins on top of this compute-oriented subset.

---

## Interfaces and shaders

### Interface

An interface declares method signatures that shaders must implement.

```sdslv
interface IBaseColor {
    fn BaseColor(s: Surface) -> float4;
}
```

All methods in an interface are abstract (no body). Methods without a body in a shader are valid only for interface methods being fulfilled; all other shader methods must have a body.

### Concepts, configs, and template shaders (GoOct M5)

GoOct M5 adds a narrow compile-time specialization system for compute shader families.

```sdslv
concept TileCopyConfig {
    THREADS_X: u32;
    THREADS_Y: u32;
    TILE_SIZE: u32;
}

config Tile16x16: TileCopyConfig {
    THREADS_X: 16u;
    THREADS_Y: 16u;
    TILE_SIZE: 256u;
}

template<C: TileCopyConfig>
shader TileCopy {
    workgroup Tile: array<f32, C.TILE_SIZE>;
    stage compute [numthreads(C.THREADS_X, C.THREADS_Y, 1u)] fn CS() -> void {
        let tileElements: u32 = C.TILE_SIZE;
        return;
    }
}

compile TileCopy<Tile16x16> as TileCopy16x16;
```

Current M5 rules:

- concepts are compile-time config schemas only;
- configs provide concrete compile-time scalar values;
- template shaders accept exactly one concept-config parameter;
- `C.FIELD` is valid only inside the template shader that declares `C`;
- template shaders do not emit directly;
- `compile` declarations monomorphize template shaders before VD-MIR lowering.

Current M6 additions:

- `require expr;` inside concepts and configs for compile-time constraints;
- `static assert expr;` at shader scope inside template shaders;
- compile-time expression support for arithmetic, comparisons, boolean operators, modulo, and config-field references;
- loop attributes `[unroll]` and `[loop]`;
- resource binding attributes `[binding(n)]`.

Current M11 additions:

- structured concept field groups;
- fat-arrow config assignments with dotted paths;
- concept field defaults;
- dotted config references in requirements and template specialization;
- nonzero-by-default `u32` concept/config fields, with `u32!` as the explicit zero-permitted form.

Current M13 additions:

- constrained compile-time shader staging with `comptime let` and `comptime if`;
- comptime evaluation after template/config monomorphization and before VD-MIR lowering;
- function-body `static assert` for selected comptime branches;
- rejection of runtime parameters, resources, thread builtins, workgroup values, runtime locals, matrix views, tile reads, reductions, match payloads, and runtime function calls in comptime expressions.

Current M14 additions:

- constrained multi-way compile-time shader staging with `comptime match`;
- integer and bool literal arm patterns plus `else`;
- integer matches require `else`;
- bool matches are exhaustive with both `true` and `false`, otherwise they require `else`;
- duplicate literal patterns are rejected.

M13/M14 `comptime` is constrained shader staging, not arbitrary compile-time execution. HLSL emission remains VD-MIR-based and does not understand `comptime`.

### Shader

A shader is the core program unit. It may be generic, implement interfaces, and contain methods and stage methods.

```sdslv
shader FlatColor implements IBaseColor {
    material {
        Color: float4;
    }

    override fn BaseColor(s: VertexOut) -> float4 {
        return Color;
    }

    stage vertex fn VS(pos: float3, color: float4) -> VertexOut {
        let output: VertexOut;
        output.Position = float4(pos, 1.0);
        output.Color = color;
        return output;
    }

    stage pixel fn PS(input: VertexOut) -> float4 {
        return input.Color;
    }
}
```

**`material` block** — per-instance shader fields. Accessed directly by name within methods.

**`stage` methods** — the broad language design includes graphics stages, but the current GoOct compute subset centers on `stage compute [numthreads(...)] fn`. The compute entry point emits as `ShaderName_MethodName` and carries HLSL system-value parameters as needed. Full graphics-stage emission remains outside the current compute-focused milestone line.

**`workgroup` declarations (GoOct M4)** — shader bodies may declare backend-neutral workgroup/shared memory:

```sdslv
shader TileCopy {
    workgroup Tile: array<f32, 256>;
}
```

Current M4 rules:

- `workgroup` is shader-scoped only;
- current type shape is fixed-size `array<T, N>`;
- runtime-sized arrays and initializers are rejected;
- the HLSL backend lowers `workgroup` to `groupshared`.

GoOct M6 also adds backend-hint attributes in the compute subset:

```sdslv
stream TileCopyIO {
    [binding(0)] A: readonly array<f32>;
    [binding(1)] C: readwrite array<f32>;
}

[unroll]
for i in 0u..1u {
    return;
}
```

**Non-stage methods** — ordinary helper methods. Emit as `ShaderName_MethodName` HLSL functions. Not emitted as DXC entry points.

**`implements`** — lists interface names the shader fulfills. Methods satisfying interface contracts must be marked `override`. The signature (name, parameter types, return type) must match exactly. Methods marked `override` that do not appear in any implemented interface are a validation error.

### Compile declaration

GoOct M5 monomorphizes a template shader and assigns it an alias for entry point naming:

```sdslv
compile TileCopy<Tile16x16> as TileCopy16x16;
```

The emitted entry point is named `TileCopy16x16_CS` rather than `TileCopy_CS`. The template shader itself emits no entry points.

---

## Functions and bodies

### Signatures

```sdslv
fn Name(param: Type, ...) -> ReturnType
fn Name(param: Type) -> ReturnType ! ErrorType     // fallible
```

Fallible functions declare an error type with `! ErrorType`. Only `Error` is recognized as a valid error type in current milestones.

### Statements

| Form | Description |
|---|---|
| `let name: Type;` | Local variable declaration, zero-initialized |
| `let name: Type = expr;` | Local variable with initializer |
| `name = expr;` | Assignment to local variable |
| `expr.field = expr;` | Field assignment (locals only — parameters are immutable) |
| `expr[index] = expr;` | Array element assignment (local arrays only) |
| `return expr;` | Return |
| `if cond { ... }` | Conditional without else |
| `if cond { ... } else { ... }` | Conditional with else |
| `for i in start..end { ... }` | Bounded integer for loop |
| `for i in start..end step n { ... }` | Bounded for loop with step |
| `expr;` | Expression statement |
| `comptime let name: Type = const_expr;` | Compile-time local binding removed before VD-MIR |
| `comptime if const_bool { ... } else { ... }` | Compile-time branch selection before VD-MIR |
| `comptime match const_expr { pattern => { ... } else => { ... } }` | Multi-way compile-time branch selection before VD-MIR |
| `static assert const_bool;` | Compile-time assertion, including inside selected `comptime if` branches |

In the compute subset, workgroup arrays are also mutable assignment targets. Barrier builtins such as `WorkgroupMemoryBarrierWithSync();` are only valid as expression statements.

`comptime let`, `comptime if`, and `comptime match` are evaluated after templates/configs have been specialized. They may reference literals, resolved config fields, and prior comptime values. They may not reference runtime parameters, resources, thread builtins, workgroup memory, runtime locals, matrix views, tile reads/writes, reductions, match payload values, or runtime function results. No `comptime for` or comptime functions are supported in M14.

`comptime match` is a statement, not runtime branching. It supports integer literal patterns, bool literal patterns, and `else` in M14. Integer matches require an `else` arm. Bool matches require either both `true` and `false` arms or an `else` arm. Only the selected arm is spliced into the AST before VD-MIR lowering; non-selected arms do not fire `static assert` and do not reach HLSL. Runtime `match` remains a separate expression form that lowers through VD-MIR.

`while` loops are explicitly not supported. Use bounded `for` loops instead.

Nested `if/else { if/else }` ladders are not permitted — use `switch { case ... else ... }` instead.

`if` conditions must be `bool`. Non-bool conditions are validation errors.

`for` loop bounds must be integer (`i32` or `u32`). `step` must be a positive integer literal; zero or negative values are validation errors. The loop variable type is inferred from the bounds.

### Expressions

| Form | Description |
|---|---|
| `ident` | Variable / parameter reference |
| `42`, `3.14`, `true`, `false` | Literals |
| `[e, e, e]` | Array literal (typed local target only) |
| `expr.field` | Field access |
| `expr[index]` | Array index |
| `expr[row, col]` | M12 tile or matrix-view 2D index |
| `callee(args)` | Function call |
| `a + b`, `a - b`, `a * b`, `a / b` | Arithmetic |
| `a == b`, `a != b`, `a < b`, `a <= b`, `a > b`, `a >= b` | Comparison |
| `-expr` | Unary negation |
| `sum i in start..end { expr }` | Indexed additive reduction |
| `product i in start..end { expr }` | Indexed multiplicative reduction |
| `max i in start..end { expr }` | Reserved indexed max reduction (parsed, validator-rejected in M10) |
| `min i in start..end { expr }` | Reserved indexed min reduction (parsed, validator-rejected in M10) |
| `base with { field: expr, ... }` | Functional record/stream update |
| `switch { case cond => value ... else => value }` | Condition-switch expression |
| `switch subject { case value => result ... else => result }` | Subject-switch expression |
| `match subject { Enum.Variant => value ... }` | Exhaustive enum match |
| `match expr { ok(binding) => value err(binding) => value }` | Fallible match |
| `when utility { case value when guard score expr ... else value }` | Utility-scored selection |
| `expr?` | Fallible propagation (try) |
| `expr!` | Fallible unwrap |
| `error("msg")` | Error constructor (fallible return position only) |

### `with` expression

Produces a modified copy of a record or stream value with named fields overridden:

```sdslv
let adjusted: SurfaceData = surface with { Roughness: 0.5, BaseColor: surface.BaseColor };
```

The base expression and fields must be of the same record/stream type. Duplicate fields and unknown fields are validation errors. Field value types must match the field's declared type. Lowers to a copy followed by individual field assignments in HLSL.

### Indexed reduction expressions

M10a adds optional prefix loop attributes on explicit indexed reductions:

```sdslv
let acc: f32 = [unroll] sum kk in 0u..C.TILE_K {
    TileA[localRow, kk] * TileB[kk, localCol]
};
```

The attribute-free form remains valid:

```sdslv
let acc: f32 = sum kk in 0u..C.TILE_K {
    TileA[localRow, kk] * TileB[kk, localCol]
};
```

### 2D indexing and row-major matrix views

M12 supports explicit `value[row, col]` indexing for `tile<T, Rows, Cols>` and `matrix_view<T>` values only. Both indices must be integer expressions. Bounds checks are not inserted; callers remain responsible for guarding edges.

Tile indexing lowers through VD-MIR to flat workgroup storage:

```sdslv
TileA[localRow, kk]
```

lowers as `TileA[localRow * Cols + kk]` in HLSL. One-dimensional indexing of tile values is rejected.

Matrix views are created with:

```sdslv
let AView: matrix_view<f32> = row_major(A, params.M, params.K);
let CView: matrix_view<f32> = row_major(C, params.M, params.N);
```

`row_major` requires a readonly/readwrite resource array and integer row/column expressions. `AView[row, col]` lowers to `A[row * params.K + col]`; assigning through a readonly view is rejected, while assigning through a readwrite view is accepted.

Current M10 rules:

- `sum` and `product` are implemented;
- `max` and `min` are reserved and parsed, but still rejected during validation in M10;
- `[unroll]` and `[loop]` are accepted on reductions as backend hints only;
- reduction attributes are mutually exclusive on one reduction;
- bounds must be integer and `step` must be a positive integer literal;
- the index name is scoped only inside the reduction body;
- reductions are currently bounded to direct `let` initializer, assignment RHS, and `return` positions;
- `sum` and `product` use identity initialization and lower to deterministic HLSL loops through `VD-MIR`.

### `switch` expression

Two forms — condition switch and subject switch:

```sdslv
// condition switch: each case is a bool expression
let tier: i32 = switch { case weight < 1 => 1 case weight < 5 => 2 else => 3 };

// subject switch: cases are equality-tested against subject
let retries: i32 = switch code { case 408 => 3 case 429 => 5 else => 0 };
```

Both forms require at least one case and an `else` arm. Case conditions must be `bool` (condition switch) or the same type as the subject (subject switch). All arm values must be the same type. Lowers to `if / else if / else` chain in HLSL.

`switch` without any case (only `else`) is a validation error. `switch` is an expression; it can appear as a `let` initializer, assignment RHS, or `return` value — but not nested inside another expression (subject to M-series bounds).

### `match` expression

Two forms — enum match and fallible match:

```sdslv
// enum match: exhaustive over all variants
let quality: i32 = match mode { ShadowMode.None => 0 ShadowMode.Hard => 1 ShadowMode.Soft => 8 };

// payload enum match
let value: f32 = match load {
    LoadValue.Zero => 0.0
    LoadValue.Value(payload) => payload.X
};

// fallible match: over a fallible expression
let value: i32 = match Parse(raw) { ok(v) => v err(_) => 30 };
```

Enum match: subject must be an enum type. All variants must be covered exactly once. Arm types must be uniform. Variants from the wrong enum are rejected. Payload variants must bind exactly one payload name in M9; simple variants must not bind payload. Lowers to `if / else if / else` chains over the carrier struct `Tag`.

Fallible match: subject must be a fallible expression. Both `ok(binding)` and `err(binding)` arms are required. Binding names are scoped to their arm. `ok` arm receives the success value; `err` arm receives the error. Arm types must be uniform.

Match is bounded to let/assign/return expression positions in current milestones (M64c). Nested match in compound expressions is a validation error.

### `when utility` expression

A utility-scored ranked selection. Each case has a candidate value, a boolean guard, and a numeric score. The case with the highest score among those whose guard is true wins; if no case wins, the `else` fallback is returned. First-wins tie-breaking (strict `>`).

```sdslv
let result: i32 = when utility {
    case 100 when a > 0 score a
    case 200 when b > 0 score b
    else -1
};
```

Guards must be `bool`. Score expressions must be numeric. Stateful options (`hysteresis`, `min_commit`) are parsed but not lowered in M66b — they produce a diagnostic. `when utility` is bounded to let/assign/return positions; nested in compound expressions is a validation error (M66b).

`when policy` is reserved for `flow`/`state` bodies and is rejected in ordinary shader function bodies.

---

## Flow declarations

A `flow` is a value-returning finite state machine. It lowers to a regular HLSL helper function.

```sdslv
flow PickMode(useSoft: bool, quality: i32) -> i32 {
    board {
        SelectedMode: i32;
    }

    state Select {
        when {
            case useSoft -> goto Soft
            case quality > 2 -> goto Soft
            else -> goto Hard
        }
    }

    state Soft {
        board.SelectedMode = 2;
        return board.SelectedMode;
    }

    state Hard {
        board.SelectedMode = 1;
        return board.SelectedMode;
    }
}
```

### `board` block

The optional `board` block declares local state variables that persist across state transitions within a single invocation. Board fields are zero-initialized in the emitted HLSL function. Board fields may be read (`board.FieldName`) and written (`board.FieldName = expr`) within state bodies.

Valid board field types are the builtin scalar and vector types: `bool`, `i32`, `u32`, `f32`, `float`, `float2`, `float3`, `float4`, `float4x4`. User-defined types are not valid as board field types.

A board block must declare at least one field. Empty boards are a validation error. Duplicate field names are a validation error. At most one board block per flow. The board block must appear before all state declarations.

`board` is a reserved parameter name — a flow parameter named `board` is a parse error.

### States

Each state is a named list of statements. States may contain:

- `board.Field = expr;` — board field assignment
- `when { case cond -> action ... else -> action }` — conditional dispatch
- `goto StateName;` — unconditional state transition
- `return expr;` — return a value and exit the flow

Ordinary `let`, `if`, and other statement forms are not valid in state bodies. Their presence is a parse error.

State names must be unique within a flow. States may reference each other via `goto` but the resulting execution graph must be acyclic — the emitter performs a cycle check and rejects cyclic flows. Every state must have at least one statement. Every execution path through all reachable states must eventually reach a `return` — non-returning paths are an emission error.

The first declared state is the entry state.

### `when` in flow states

```sdslv
when {
    case cond -> goto StateName
    case cond -> return expr
    else -> goto StateName
}
```

`->` and `=>` are both accepted as case separators. Each case condition is a boolean expression. Guard conditions must resolve to `bool`. `when` must include an `else` arm — omitting it is a validation error.

Actions are either `goto StateName` or `return expr`.

### Flow lowering

A flow emits as an HLSL helper function. The emitter inlines state transitions by substituting `goto` targets inline (depth-first, cycle-checked). Board fields become local variable declarations at the top of the function, zero-initialized. `when` blocks lower to `if / else if / else` chains. The return type maps via the standard type mapping.

Flow functions are not emitted as DXC entry points. They appear in the HLSL output as regular callable functions. Entry point extraction skips them.

---

## Fallibility

Functions may declare a fallible return type with `! ErrorType`:

```sdslv
fn Parse(raw: i32) -> i32 ! Error { return raw; }
```

Within a fallible function, calling another fallible function produces a fallible expression. Fallible expressions must be explicitly handled with `?` (propagate) or `!` (unwrap):

```sdslv
fn G() -> i32 ! Error {
    let x: i32 = Parse(raw)?;    // propagate: return error if Parse fails
    let y: i32 = Parse(raw)!;    // unwrap: panic/undefined if Parse fails
    return x + y;
}
```

`?` may only appear inside a fallible function. `!` may appear anywhere. Using `?` or `!` on a non-fallible expression is a validation error. Leaving a fallible expression unhandled (as a statement, let initializer, or return value without `?` or `!`) is a validation error.

`error("msg")` constructs an error value. It is only valid in fallible return position (`return error("msg")`) within a fallible function. It may not appear in non-return position or in infallible functions.

HLSL emission of fallible modules is not supported in M58 — any module containing a fallible function signature produces an `EmitHlsl` diagnostic. The fallibility system is fully validated but not yet lowered.

---

## Testing language (`.sdslvtest`)

SDSL-V includes a test file format for unit-testing shader logic without a GPU. Test files use the `.sdslvtest` extension and are parsed by a separate parser.

```sdslvtest
namespace WyrmCoil.Tests;

[Fact]
fn BasicArithmetic() {
    let value: f32 = 1.0 + 1.0;
    Assert.True(value > 0.0, "value should be positive");
    Assert.Equals(value, 2.0, "value should equal two");
    Assert.Near(value, 2.001, 0.01, "value should be near two");
}

[Theory]
[InlineData(0.0, 0.0)]
[InlineData(0.5, 0.5)]
[InlineData(1.5, 1.0)]
fn SaturateClampsToUnit(input: f32, expected: f32) {
    Assert.Near(saturate(input), expected, 0.0001, "saturate should clamp into [0, 1]");
}
```

### Attributes

- `[Fact]` — parameterless test. Accepts no arguments. Function must have no parameters.
- `[Theory]` — parameterized test. Must have at least one `[InlineData(...)]` attribute. Function parameters must match InlineData arity and types.
- `[InlineData(values...)]` — row of inputs for a theory. Each row must match the test function's parameter count and types exactly.

A function cannot have both `[Fact]` and `[Theory]`. `[InlineData]` on a `[Fact]` is a validation error.

### Assert methods

| Method | Signature | Description |
|---|---|---|
| `Assert.True` | `(value: bool, message: string)` | Asserts value is true |
| `Assert.Equals` | `(actual: T, expected: T, message: string)` | Asserts exact equality |
| `Assert.Near` | `(actual: f32, expected: f32, tolerance: f32, message: string)` | Asserts within tolerance |

Non-assert expression statements are a validation error. Unsupported `Assert` methods (e.g. `Assert.Approximately`) are a validation error. The test runner evaluates tests in a CPU-side interpreter that supports: local variable declaration, arithmetic, comparison, function calls to known builtins (`saturate`, `clamp`), and all assert forms. Unrecognized function calls fail with an "unsupported function call" diagnostic. `[Theory]` tests produce one result per `[InlineData]` row, named `FunctionName[index]`. Failures accumulate — all tests in a file run even if earlier ones fail.

---

## Emission rules

### Entry point naming

Stage methods emit as `ShaderName_MethodName`. `compile` aliases emit as `AliasName_MethodName`. Generic shader stage methods do not emit directly.

### HLSL type mapping

| SDSL-V | HLSL |
|---|---|
| `f32` | `float` |
| `i32` | `int` |
| `u32` | `uint` |
| `bool` | `bool` |
| `float`, `float2`, `float3`, `float4`, `float4x4` | unchanged |
| `array<T, N>` | `T name[N]` (emitted as declaration; locals only) |
| `enum ShadowMode` fields | `int` |

### Semantic assignment

Stream fields: a field typed with a `@space(clip.position)` alias → `SV_Position`. All other fields → `TEXCOORD0`, `TEXCOORD1`, ... in declaration order. Only one `SV_Position` allowed per stream.

Pixel stage return type → `: SV_Target`.

### Determinism

Emission is fully deterministic — the same source always produces byte-identical HLSL output. Top-level declarations emit in source order. Enum constants emit in variant declaration order. Entry points emit in declaration order.

### Shader profile targets

| Stage | HLSL profile |
|---|---|
| `vertex` | `vs_6_0` |
| `pixel` | `ps_6_0` |

DXC is invoked with `-spirv` for SPIR-V output. Extra args (e.g. `-O3`) are configurable per compilation request.

---

## Validation summary

| Category | Rule |
|---|---|
| Top-level names | Must be unique across all declaration kinds |
| Record fields | Must be unique within the record |
| Shader material fields | Must be unique |
| Shader methods | Must be unique by name |
| Generic parameters | Must be unique per shader |
| Stage | Only `vertex` and `pixel` are supported |
| Interface contract | `override` required for fulfilled methods; signature must match exactly; `override` on non-interface method is an error |
| Compile | Target shader must be generic; type argument count must match |
| Flow states | Must be unique; graph must be acyclic; all paths must return |
| Flow `when` | Must include `else`; guards must be `bool` |
| Board | At least one field; no initializers; must precede states |
| Board reads | Must reference declared field on a flow with a board |
| Board assignments | Type of value must match field type |
| For loops | Bounds must be integer; step must be positive integer literal |
| `if` condition | Must be `bool` |
| Nested ladders | `if/else { if/else }` rejected — use `switch` |
| Switch cases | Condition must be `bool` (condition-switch) or match subject type (subject-switch); at least one case required; `else` required |
| Match (enum) | Subject must be enum type; all variants covered exactly once; arm types uniform |
| Match (fallible) | Subject must be fallible; both `ok` and `err` arms required; arm types uniform |
| Indexed reductions | Bounds must be integer; step must be positive integer literal; `sum`/`product` bodies must be numeric |
| Fallible handling | Every fallible expression in a body must be handled with `?` or `!` |
| `?` context | Only valid inside a fallible function |
| `error(...)` | Only valid as `return error(...)` in a fallible function |
| Immutability | Stream and record parameters are immutable; use `with` for modified copies |
| Array parameters | Immutable; element assignment rejected |
| Vector constructors | Exact arity; numeric scalar arguments only |
| Array literals | Target must be `array<T, N>`; length must match N; elements must match T |
| Coordinate spaces | Space-annotated aliases are incompatible with base type and other space-annotated aliases |
| Enum variants | Qualified references must use correct enum name |
| `when utility` options | Recognized but not lowered in M66b; emit diagnostic |
| `when policy` | Only valid in flow/state bodies |

---

## Reserved keywords

The following identifiers cannot be used as variable names, parameter names, or flow parameter names: `flow`, `board`, `state`, `when`, `step`, `sum`, `product`, `max`, `min`, `utility`, `policy`, `case`, `score`, `hysteresis`, `min_commit`, `goto`, `compile`, `interface`, `shader`, `stream`, `record`, `enum`, `match`, `ok`, `err`, `namespace`, `use`, `type`, `stage`, `implements`, `where`, `override`, `fn`, `let`, `return`, `with`, `if`, `else`, `switch`, `for`, `in`, `while`.

---

## Grammar summary (informal)

```
module       ::= ('namespace' path ';')? use* decl*
use          ::= 'use' path ';'
decl         ::= type-alias | record | stream | interface | shader | flow | compile | enum
type-alias   ::= 'type' IDENT '=' type-ref ('@space' '(' path ')')? ';'
record       ::= 'record' IDENT '{' field* '}'
stream       ::= 'stream' IDENT '{' field* '}'
interface    ::= 'interface' IDENT '{' fn-sig* '}'
shader       ::= 'shader' IDENT generic-params? implements? where-clause? '{' material? method* stage-method* '}'
flow         ::= 'flow' IDENT '(' params ')' '->' type-ref '{' board? state+ '}'
compile      ::= 'compile' path '<' type-args '>' 'as' IDENT ';'
enum         ::= 'enum' IDENT '{' enum-variant+ '}'
enum-variant ::= IDENT ';' | IDENT '{' field+ '}'

field        ::= IDENT ':' type-ref ';'
params       ::= (IDENT ':' type-ref (',' IDENT ':' type-ref)*)?
type-ref     ::= path | 'array' '<' type-ref ',' INT '>'
             | 'tile' '<' type-ref ',' const-expr ',' const-expr '>'
             | 'matrix_view' '<' type-ref '>'

board        ::= 'board' '{' (IDENT ':' type-ref ';')+ '}'
state        ::= 'state' IDENT '{' flow-stmt+ '}'
flow-stmt    ::= board-assign | when-flow | goto | return
board-assign ::= 'board' '.' IDENT '=' expr ';'
when-flow    ::= 'when' '{' case+ ('else' '->' flow-action) '}'
case         ::= 'case' expr ('->') flow-action
flow-action  ::= 'goto' path | 'return' expr

method       ::= 'override'? 'fn' IDENT '(' params ')' '->' type-ref ('!' type-ref)? body?
stage-method ::= 'stage' STAGE 'fn' IDENT '(' params ')' '->' type-ref body
STAGE        ::= 'vertex' | 'pixel'

body         ::= '{' stmt* '}'
stmt         ::= let | comptime-let | comptime-if | comptime-match | static-assert | assign | return | if | for | expr-stmt
let          ::= 'let' IDENT ':' type-ref ('=' expr)? ';'
comptime-let ::= 'comptime' 'let' IDENT ':' type-ref '=' expr ';'
comptime-if  ::= 'comptime' 'if' expr '{' stmt* '}' ('else' '{' stmt* '}')?
comptime-match ::= 'comptime' 'match' expr '{' comptime-match-arm+ '}'
comptime-match-arm ::= (expr | 'else') '=>' '{' stmt* '}'
static-assert ::= 'static' 'assert' expr ';'
assign       ::= expr '=' expr ';'
for          ::= 'for' IDENT 'in' expr '..' expr ('step' expr)? '{' stmt* '}'

expr         ::= ... (see expression table above)
index-expr   ::= expr '[' expr ']' | expr '[' expr ',' expr ']'
switch-expr  ::= 'switch' expr? '{' switch-case+ 'else' ('=>'|'->') expr '}'
match-expr   ::= 'match' expr '{' match-arm+ '}'
match-arm    ::= IDENT '.' IDENT ('(' IDENT ')')? '=>' expr
when-utility ::= 'when' 'utility' ('{' utility-opts '}')? '{' utility-case+ 'else' expr '}'
reduction    ::= reduction-attrs? ('sum' | 'product' | 'max' | 'min') IDENT 'in' expr '..' expr ('step' expr)? '{' expr '}'
reduction-attrs ::= ('[' 'unroll' ']' | '[' 'loop' ']')+
```
