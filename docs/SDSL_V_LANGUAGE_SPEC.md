# SDSL-V Language Specification

## M35a compiler-owned packed/vector intrinsics

SDSL-V supports a closed set of compiler-defined generic intrinsics:
`Pack<Format>`, `Unpack<Format>`, `Bitcast<T>`, and `Convert<T>`. These are not
user-defined generics; concepts, configs, and template shaders remain the user
specialization mechanism. `F16x2` maps low/high binary16 lanes of `u32` to
`.x/.y` of `float2`. First-class component reads support `.x` through `.w` as
permitted by `float2`–`float4` and `uint2`–`uint4`; `Dot` accepts matching
float vectors and returns `f32`.

Closed validator-owned matrix:

- component reads:
  - `.x` on width `2..4`
  - `.y` on width `2..4`
  - `.z` on width `3..4`
  - `.w` on width `4`
  - base types: `float2|float3|float4|uint2|uint3|uint4`
- `Dot(float2|float3|float4, same) -> f32`
- `Unpack<F16x2>(u32) -> float2`
- `Pack<F16x2>(float2) -> u32`
- `Bitcast<u32>(f32|i32)`, `Bitcast<f32>(u32)`, `Bitcast<i32>(u32)`
- `Convert<f32>(u32|i32)`, `Convert<u32>(f32|i32)`,
  `Convert<i32>(f32|u32)`

`F16x2` is valid only as a compiler-known packed-format descriptor inside
`Pack` and `Unpack`. User functions do not accept generic call syntax, and
ordinary user code cannot define new intrinsic families, new packed formats, or
arbitrary conversion hooks.

## Status and authority

This is the current-state specification for the in-repository SDSL-V shader
language. The authoritative implementation is the Go front end and lowering in
`internal/sdslv/`; its fixtures and executable test corpus are the behavioral
contracts. This document is an inventory of that implementation, not a roadmap
or a milestone changelog. The authoritative Oct language reference remains
`Language/reference/`; it is not a specification for SDSL-V unless the SDSL-V
implementation and its tests also support the construct.

SDSL-V currently has a compute compiler path: SDSL-V source is parsed,
validated, lowered to VD-MIR, emitted as HLSL, compiled by DXC to SPIR-V, and
used by bounded Vulkan compute/test routes. Graphics syntax tokens exist, but
there is no graphics-stage lowering, graphics pipeline, or graphics runtime
proof. A parsed spelling is not, by itself, implementation support.

## Language overview

SDSL-V has a small shared expression and declaration language, a deliberately
compute-oriented resource and execution model, and an explicitly unimplemented
graphics surface. Its fixed-shape data concepts are distinct:

- `ndarray<T, [shape...]>` is shaped, row-major value storage.
- `Fill` and `Generate` construct ndarray values.
- `tensor` and `Sum` describe indexed computation and reduction.

## Implementation status legend

| Label | Meaning |
|---|---|
| `[IMPLEMENTED]` | End-to-end for its stated scope, with implementation and tests. |
| `[PARTIAL]` | Some layers or restricted forms exist; the intended scope is not complete. |
| `[PLANNED]` | Repository-backed intent exists, but no authoritative implementation exists. |
| `[LEGACY]` | Supported for compatibility or older fixtures; not the preferred modern form. |
| `[DEFERRED]` | Intentionally postponed beyond the current scope. |
| `[OUT OF SCOPE]` | Explicitly not part of SDSL-V's intended design. |

“Backend” below means the current VD-MIR → HLSL emitter. “Hardware proof” means
the repository contains DXC/SPIR-V and Vulkan execution evidence for the stated
scope; it never means every syntactically accepted program has been executed on
hardware.

# Part I — Shared language

| Feature | Status | Parser | Validator | Backend | Hardware proof |
|---|---|---:|---:|---:|---:|
| Declarations, records, enums, functions | IMPLEMENTED | Yes | Yes | Yes | Compute fixtures |
| Scalars, literals, operators, calls | IMPLEMENTED | Yes | Yes | Yes | Compute fixtures |
| Arrays and indexing | IMPLEMENTED | Yes | Yes | Yes | Compute fixtures |
| `ndarray` value types | IMPLEMENTED | Yes | Yes | Yes | M33a Vulkan proof |
| `Fill` / `Generate` | IMPLEMENTED | Yes | Yes | Yes | M33b fixtures |
| `if`, runtime `for`, comptime control | IMPLEMENTED | Yes | Yes | Yes | Compute fixtures |
| Enum `match` | IMPLEMENTED | Yes | Yes | Yes | HLSL regression coverage |
| General `switch` | PLANNED | No | No | No | No |
| Imports/modules across files | DEFERRED | `use` parses | No linking | No | No |
| Inline HLSL scalar/vector escape | IMPLEMENTED | Yes | Yes | Yes | M29 Vulkan proof |

## Lexical structure and source mapping

`[IMPLEMENTED]` SDSL-V has identifiers, integer, floating-point, boolean and
string literals; punctuation and operators are tokenized by
`internal/sdslv/lex` and `internal/sdslv/token`. Line and block comments are
accepted by the lexer. Compiler-owned source spans are retained from tokens
through AST, validation diagnostics, VD-MIR provenance, and inline-HLSL source
markers. Diagnostics therefore identify SDSL-V source rather than generated
HLSL whenever the relevant layer owns the error.

Strings parse as expressions, but they are not a general shader-data type or
HLSL value type. Their current useful scope is diagnostic/test metadata and
attribute-related validation, not arbitrary GPU computation.

## Modules and declarations

`[PARTIAL]` A file may begin with `namespace Name;` and may contain `use path;`.
The AST records both. The current compiler does not implement imported-module
resolution or cross-file linking, so `use` must not be presented as an available
module system.

`[IMPLEMENTED]` Supported top-level declarations are type aliases, `record`,
`board`, `stream`, `enum`, `concept`, `config`, `shader`, `compile`, and
ordinary `fn` declarations. `concept`/`config`/template shaders are a
compile-time specialization facility, not runtime polymorphism. Function and
shader method parameters are immutable; local `let` variables are mutable in
the current language despite the name. Assignment is permitted only to a
validated mutable local, writable resource element, board field, or supported
tile/ndarray element.

```sdslv
namespace Demo;

record Params { Count: u32; }
enum Mode { Fast; Safe; }

fn Scale(x: f32) -> f32 { return x * 2.0; }
```

`[IMPLEMENTED]` Records have named fields. `stream` is a record-like type whose
fields may carry resource access annotations; `ComputeThread` is the built-in
compute stream convention. `board` is a restricted scalar/vector record for
compute flow-local state. `with` copies a record/stream value and replaces
named fields; `derive { Field: value ... }` constructs an ordered immutable
coordinate/value record. Both require declared fields, no duplicates, and
matching field types.

## Types, literals, and expressions

`[IMPLEMENTED]` Built-in scalar types are `bool`, `i32`, `u32`, `f32`/`float`,
with supported HLSL vector aliases `float2`, `float3`, `float4`, `uint2`,
`uint3`, and `uint4`. Numeric operators, comparisons, `and`/`or`/`not`, and
ordinary calls type-check and lower. Exact accepted operand combinations are
validator-owned; no implicit cross-kind numeric conversion should be assumed.

`[IMPLEMENTED]` Fixed arrays use `array<T, N>` and can nest, for example
`array<array<f32, 4u>, 4u>`. Arrays require compile-time positive extents. They
remain a supported compatibility and resource representation; nested arrays
are not silently interchangeable with `ndarray`.

`[IMPLEMENTED]` `ndarray<T, [D0, D1, ...]>` is a first-class fixed-shape value
type with rank at least one, positive integer constant extents, supported
numeric element kinds, exact shape equality, and no implicit conversion to or
from nested arrays. Dense literals are flat and target-typed. Index count must
equal rank; each index is an integer. Its logical layout is row-major, with the
last axis varying fastest.

```sdslv
let weights: ndarray<f32, [2u, 3u]> = [1.0, 2.0, 3.0, 4.0, 5.0, 6.0];
weights[1u, 2u] = 9.0;
```

`[IMPLEMENTED]` `Fill(value)` constructs every element of a contextually typed
ndarray. Its argument is evaluated exactly once, then used for every element.
`Generate[i, j](body)` constructs every element of a contextually typed ndarray
with one immutable `u32` binder per axis. Binders are ordered outermost to
innermost and the conceptual traversal is row-major. Binder count must equal
rank and the body must produce the element type.

```sdslv
let zero: ndarray<f32, [16u, 16u]> = Fill(0.0);
let grid: ndarray<u32, [2u, 3u]> = Generate[i, j](i * 10u + j);
```

## Ordinary and compile-time control flow

`[IMPLEMENTED]` `if condition { ... } else { ... }` is ordinary conditional
control. Conditions must be boolean. `for i in start..end step n { ... }` is
the ordinary bounded loop form; loop bounds, step, and index typing are
validated before lowering. The backend emits ordinary HLSL control flow.

`[IMPLEMENTED]` `comptime let`, `comptime if`, `comptime match`, `comptime when
utility`, and `comptime for` run during specialization/lowering. They may use
literals, resolved configuration fields, and earlier compile-time values; they
may not inspect runtime parameters, resources, builtins, local runtime values,
tile/ndarray reads, reductions, or runtime function results. Only the selected
compile-time arm reaches VD-MIR. `comptime match` accepts integer/bool literal
patterns; integer selection requires `else`, while bool selection requires
both boolean cases or `else`.

`[IMPLEMENTED]` `when utility` is a deterministic scored selection expression,
not `switch`: each case has a value, boolean `when` condition, and score. Its
validator enforces the bounded form and uniform result type. `when { case cond
=> { ... } else => { ... } }` is instead a compute guard statement; see Part II.

## `match` and the explicit `switch` audit

`[IMPLEMENTED]` Runtime `match` is enum decomposition. It requires an enum
subject, covers every variant exactly once, rejects variants from other enums,
and requires uniform arm result types. Payload variants bind exactly one payload
name; non-payload variants cannot bind one. It is currently valid only as a
direct `let` initializer, assignment right-hand side, or return value; nesting
inside a compound expression is rejected. VD-MIR materializes it and HLSL
emits tag tests and branches.

```sdslv
let cost: f32 = match mode {
    Mode.Fast => 1.0
    Mode.Safe => 2.0
};
```

`[PARTIAL]` The historical specification claimed fallible `ok(...)`/`err(...)`
match syntax. The current SDSL-V lexer/parser/AST match form contains only
qualified enum variant arms. No current SDSL-V fallible-match parser path or
fixture establishes that spelling. Treat fallible matching as absent from this
SDSL-V inventory until implemented and tested.

`[PLANNED]` General `switch` is not implemented. There is no `switch` token,
no parser case, no AST node, no validation rule, no lowering, no VD-MIR node,
no emitted source-level switch lowering, and no SDSL-V fixture for any of:

- condition switch: `switch { case condition => value else => value }`;
- subject switch: `switch value { case literal => value else => value }`;
- statement-style or enum switch.

The HLSL emitter's internal `switch` for flow dispatch and generated test
dispatch are backend implementation details, not SDSL-V `switch` syntax.
`match` is the implemented closed enum-decomposition construct. A future
general `switch`, if adopted, would be for general value/condition selection;
this document does not define its syntax or semantics.

## Attributes and test surface

`[IMPLEMENTED]` Attribute parsing and placement validation support function,
statement, expression, field, resource, and test declarations where the
specific feature permits them. Known shader/test attributes include loop hints,
test launch metadata, `Fact`, `Theory`, and `InlineData`. Unsupported placement
or arguments are diagnostics, not ignored metadata.

`[PARTIAL]` `.sdslvtest` is a bounded compute test language. `Fact` and
`Theory` tests, typed `InlineData`, `Assert.True`, `False`, `Equal`,
`NotEqual`, and `Near` are validated. The compiler owns a set 0/binding 0
result buffer and a fixed ABI; test input resources are restricted to the
implemented scalar/vector payload kinds. The test harness has real Vulkan proof
for the committed inline-HLSL route, while arbitrary source-body lowering and
arbitrary user descriptors remain deliberately bounded/deferred.

Every assertion requires a final nonempty string-literal reason. The reason
describes the invariant under test, is retained as compiler-owned manifest
metadata, and is reported by the host on failure. It is not a shader runtime
string and does not change the GPU result ABI. Empty and ASCII-whitespace-only
literals, variables, and other expressions are rejected.

# Part II — Compute language

| Feature | Status | Parser | Validator | Backend | Hardware proof |
|---|---|---:|---:|---:|---:|
| Compute entry points and dispatch builtins | IMPLEMENTED | Yes | Yes | Yes | Yes |
| Structured-buffer resources and bindings | IMPLEMENTED | Yes | Yes | Yes | Yes |
| Workgroup arrays/tiles and barriers | IMPLEMENTED | Yes | Yes | Yes | SGEMM/examples |
| Guarded reads/writes | IMPLEMENTED | Yes | Yes | Yes | Compute fixtures |
| Matrix views and register tiles | IMPLEMENTED | Yes | Yes | Yes | SGEMM evidence |
| Flow states and stack transitions | IMPLEMENTED | Yes | Yes | Yes | `.sdslvtest` / HLSL coverage |
| Indexed `tensor` / `Sum` | IMPLEMENTED | Yes | Yes | Yes | M32b Vulkan proof |
| Subgroup/wave operations | DEFERRED | No | No | No | No |
| Persistent reactors/rings | OUT OF SCOPE | No | No | No | Host runtime only |

## Entry points, dispatch, and resources

`[IMPLEMENTED]` A compute entry is a shader method of this form:

```sdslv
shader VectorAdd {
    resources {
        A: readonly array<f32>;
        B: readonly array<f32>;
        C: readwrite array<f32>;
    }
    stage compute [numthreads(16, 16, 1)] fn CS(params: Params) -> void {
        let i: u32 = DispatchThreadID.x;
        if i < params.Count { C[i] = A[i] + B[i]; }
        return;
    }
}
```

`numthreads` dimensions are positive compile-time integers (or specialized
template constants). `DispatchThreadID`, `GroupThreadID`, `GroupID`, and
`GroupIndex` are the established compute builtins, including through the
`ComputeThread` stream convention. The current resource surface is readonly or
readwrite runtime `array<T>` structured-buffer storage. Binding ownership is
compiler/backend controlled and emitted using Vulkan HLSL binding attributes;
the user does not write raw register declarations.

`[PARTIAL]` Runtime arrays are resource-oriented storage, not general shaped
values. Arbitrary descriptor schemas, storage-qualified ndarrays, textures,
samplers, and images are not supported by this surface.

## Workgroup storage, barriers, and guarded access

`[IMPLEMENTED]` `workgroup` declarations accept fixed `array<T, N>` storage or
`tile<T, Rows, Cols>` storage with supported scalar/vector elements and positive
compile-time dimensions. Tiles and `matrix_view` provide row-major 2-D views
over compatible backing storage. `reg_tile<T, Rows, Cols>` is per-invocation
local storage, not workgroup storage; current supported local use is bounded to
the validated f32-oriented SGEMM form.

`[IMPLEMENTED]` Compute barriers are recognized only in permitted compute
contexts and flow analysis records barrier-bearing states. Uniformity and flow
stack constraints are validated; an ambiguous stack path across a barrier is
rejected. The emitted barrier is current HLSL backend behavior, not a promise
of cross-stage synchronization.

`[IMPLEMENTED]` A guarded read has `read target when condition else fallback`;
the condition guards the load and the fallback has exactly the target element
type. A guarded write has `write target = value when condition`; it evaluates
the validated address/value according to the lowering's once-only materialized
address boundary and performs the store only when true. These forms protect
tail accesses without making unchecked indexing safe.

## Flows and state transitions

`[IMPLEMENTED]` `flow` is compute-local phase/state control, not a general
shared-language branch construct. A flow contains named states and optional
scalar/vector `board` locals. State bodies may finish, fall through, `goto`,
`push`, or `pop`. Transitions are not function calls. The validator resolves
targets, rejects duplicate/cross-flow targets, prevents illegal statements
after terminal transitions, checks reachability and stack depth, prohibits
unsafe nested use, and preserves barrier safety. `push`/`pop` uses bounded LIFO
return state; `finish` terminates the flow even with a nonempty stack.

Older linear state fallthrough remains supported in fixtures but is
`[LEGACY]`; explicit transitions are the preferred form. Flow lowering uses an
internal state dispatcher in VD-MIR/HLSL. That generated HLSL `switch` is not
SDSL-V source syntax.

## Shaped construction and indexed tensor computation

`[IMPLEMENTED]` `ndarray`, `Fill`, and `Generate` retain the shared semantics
in Part I when used in compute code. Current lowering materializes row-major
flat storage and static binder-ordered loops. They are values; they do not
declare storage class or resource placement.

`[IMPLEMENTED]` Tensor statements are separate indexed computation syntax:

```sdslv
tensor C[i, j] = Sum[k](A[i, k] * B[k, j]);
```

Free indices on the destination identify the output iteration space; `Sum[k]`
introduces reduction indices scoped to the expression. Validator rules require
compatible extents, unique free/reduction indices, valid indexing, a mutable
destination, and safe source/destination aliasing. Current `Sum` is the
implemented tensor reduction; additional reduction operators, affine index
forms, and broad alias transformations are not implied. VD-MIR lowers the
statement into explicit materialized loops, and the HLSL emitter emits those
loops. M32b test execution supplies bounded DXC/SPIR-V/Vulkan proof.

## Inline HLSL

`[IMPLEMENTED]` `HLSL { ... }` is a statement escape hatch and
`HLSL<T> { return ...; }` is an expression escape hatch. Raw contents are not
SDSL-V-tokenized. Captures must be explicitly named when required and must be
supported scalar/vector values; expression result types have the same bound.
The validator rejects interface-shaping HLSL such as preprocessor directives,
resource declarations, `register`, `cbuffer`, texture/sampler/buffer
declarations, `[numthreads]`, structs, and namespaces. This is a portability
boundary, not a general foreign-language embedding system.

The backend inserts scoped source markers and materializes an expression result
through a compiler-owned local. The established M29 Vulkan proof covers the
bounded inline-HLSL test path, not arbitrary injected HLSL or graphics use.

## Production evidence and non-language runtime systems

`[IMPLEMENTED]` The Prometheus production shader directory contains compiled
SDSL-V SGEMM variants using resources, guarded tail accesses, workgroup tiles,
register tiles, flows/boards, derives, and inline HLSL. These are production or
benchmark evidence for those *existing restricted forms*, not a claim of a
general tensor optimizer or automatic tiler.

`[OUT OF SCOPE]` Prometheus reactors, submission rings, task lifecycle,
dispatch selection, asynchronous readback, and persistent host state are host
runtime systems. They consume shader artifacts but are not SDSL-V syntax or
semantic constructs.

# Part III — Graphics language

| Feature | Status | Parser | Validator/lowering | Backend/runtime | Evidence |
|---|---|---:|---:|---:|---|
| `stage vertex` / `stage pixel` tokens | PARTIAL | Yes | No end-to-end path | No | Parser and rejection tests |
| Vertex/fragment interfaces and varyings | PLANNED | No | No | No | Graphics-boundary report |
| Mesh/task/geometry/tessellation stages | PLANNED | No | No | No | No implementation |
| Textures, samplers, image load/store | DEFERRED | No | No | No | Explicit test/HLSL boundary |
| Render targets, blend, depth/stencil | PLANNED | No | No | No | Graphics-boundary report |
| Graphics pipeline linkage/draw | PLANNED | No | No | No | Graphics-boundary report |

SDSL-V is not presently a graphics shading language implementation. The lexer
reserves `vertex` and `pixel`, and the parser can record them after `stage`, but
VD-MIR models only `ComputeEntryPoint`; HLSL emission writes compute
`[numthreads]` entry points; and no graphics pipeline or Vulkan draw path is
implemented. The validator test that parses a vertex spelling is evidence of a
front-end boundary, not support for a vertex shader.

`[PLANNED]` Repository design notes require a future graphics implementation to
keep graphics artifacts distinct from compute dispatch facts. It must define
stage interfaces, vertex layout, varying/interpolant rules, render targets,
rasterization, depth/stencil, blend state, compatible shader-stage linkage,
and mutable graphics pipeline instances. None of those is exposed by SDSL-V
today. Machina/Prometheus consumers do not supply an alternate SDSL-V graphics
language surface.

`[DEFERRED]` Texture, sampler, and image declarations are deliberately outside
the current inline-HLSL and `.sdslvtest` ABI boundaries. The inline-HLSL
validator rejects those interface declarations; that rejection must not be
misread as native SDSL-V texture support.

# Part IV — Backend and compilation model

`[IMPLEMENTED]` The supported compilation chain is:

```text
SDSL-V source → lexer/parser → AST → validator → lowering/specialization
             → VD-MIR → HLSL emitter → DXC → SPIR-V → bounded Vulkan compute
```

VD-MIR owns backend-neutral scalar/data/control representations, resources,
workgroup memory, compute entry points, flows, and source provenance. The HLSL
emitter owns HLSL spelling, generated helpers, Vulkan binding attributes,
row-major address materialization, compute entry semantics, and source markers.
No language rule is defined solely by emitted HLSL; validator rules and corpus
fixtures define SDSL-V's accepted scope.

`[PARTIAL]` DXC/SPIR-V is the current target path and Vulkan evidence exists
for representative compute shaders, tensors, and bounded tests. There is no
claim of a portable graphics backend, a second code generator, or a proof that
every accepted feature combination has hardware coverage.

Resource and test ABI boundaries are compiler-owned. `.sdslvtest` fixes the
test result buffer at descriptor set 0/binding 0 and uses compiler-generated
push constants and manifest identity. User code cannot redefine that ABI.

# Part V — Known limitations and planned work

Only items supported by repository reports, comments, or current explicit
boundaries appear here.

## Shared

- `[PLANNED]` General condition/value `switch`; no syntax is specified here.
- `[DEFERRED]` Imported-module resolution and cross-file SDSL-V linking.
- `[PLANNED]` Broader nested-expression placement for `match` and richer
  whole-value operations, subject to an implementation/design pass.
- `[DEFERRED]` A broader fallible-value surface and fallible `match` in SDSL-V;
  the old spec's spelling is not current implementation evidence.

## Compute

- `[PLANNED]` Storage-qualified ndarray categories (register/workgroup/resource)
  rather than treating ndarray values as an implicit storage abstraction.
- `[PLANNED]` Tensor optimization, cooperative matrices, automatic tiling, and
  additional reductions beyond the validated current `Sum` form.
- `[DEFERRED]` A reusable arbitrary compute-dispatch/readback harness; current
  `.sdslvtest` remains test-owned and deliberately restricted.
- `[PLANNED]` Production SGEMM migration only where a measured, validated
  replacement improves its existing explicit variants.
- `[OUT OF SCOPE]` Encoding Prometheus host reactors/rings as shader language.

## Graphics

- `[PLANNED]` A real stage model, stage I/O records, builtin semantics, and
  shader interface validation.
- `[PLANNED]` Vertex inputs, interpolants, render-target/depth outputs, and
  graphics pipeline linkage.
- `[DEFERRED]` Textures/samplers/images until resource and pipeline ownership is
  designed; current rejection is intentional.

# Appendix A — Legacy syntax and compatibility

| Item | Status | Preferred form / note |
|---|---|---|
| Nested fixed arrays as rank-N tensor proxies | LEGACY | Use `ndarray<T, [shape...]>` for shaped fixed values; no implicit conversion. |
| Array `Index`/`Index2` AST adapters | LEGACY implementation adapter | Source indexing is the ordered `Indices` form; this is not user syntax. |
| Linear flow-state fallthrough | LEGACY | Use explicit `goto`, `push`, `pop`, or `finish` for phase transitions. |
| Older M12/M15 tile spellings/examples | LEGACY where accepted | Preserve current validated `tile`, `matrix_view`, and `reg_tile` restrictions. |
| Historical runtime `switch` prose | Removed stale documentation | No supported replacement; use `if`, `when utility`, or enum `match` according to intent. |

Supported compatibility syntax remains implemented. This appendix does not
deprecate it by itself, and production sources must not be changed merely to
modernize documentation.

# Appendix B — Milestone history

Historical milestones are evidence, not current status labels. In broad order:
M0–M9 introduced compute resources, workgroup memory, concepts/configs,
records/streams, and enums/match; M10–M16 added reductions, views, register
tiles, compile-time staging, and guarded access; M19–M28 added guards,
board/flow forms, derive, and inline HLSL; M29–M31 added bounded GPU test
contracts and flow stack lowering; M32–M33 added tensor lowering, ndarray, and
construction. Consult the individual `docs/SDSL_V_M*.md` and
`internal/prometheus/DevelopmentReport/SDSL_V_M*.md` reports for historical
acceptance evidence. Their milestone wording must not override this
implementation inventory.

# M36a benchmark declarations

`.sdslvbench` is a tooling-only source type for repeatable GPU performance
experiments. A benchmark is a top-level function with `[Benchmark]`, required
`[DispatchGroups(X, Y, Z)]`, and optional `[Warmup(N)]`, `[Iterations(N)]`, and
`[WorkgroupSize(X, Y, Z)]`. Warmup defaults to 10 and iterations to 100.
Benchmark functions currently take no parameters and return `void`; test
attributes and `Assert.*` are invalid in benchmark files.

`oct sdslv bench file.sdslvbench --list` reads and validates declarations
without Vulkan execution. `--case <stable-id>` selects one stable declaration,
`--json` emits schema-versioned deterministic manifest data, and `--backend
<auto|godot|kaiju>` selects the execution witness. `auto` prefers the optional
typed Kaiju Octxiliary Vulkan sidecar when installed and capability-compatible,
then falls back to the Godot benchmark host when Kaiju is unavailable. Stable
IDs derive from normalized source identity, declaration name, and dispatch
metadata, not device or timing values. Benchmark results are performance
observations, never correctness proofs; correctness remains in `.sdslvtest`.
