# SDSL-V language specification

## 1. Language status and authority

This document and the canonical corpus at `Language/SDSL-V/conformance/` define
SDSL-V. GoOct is the reference implementation. Behavior that conflicts with
this specification is a compiler defect; no particular parser, validator, or
emitter is independently authoritative.

SDSL-V is one statically typed shader language with a shared core and exactly
three execution profiles: compute, vertex, and pixel. Every feature described
as canonical below is implemented, validated, lowered to VD-MIR and HLSL, and
artifact-proven by the corpus. “Artifact-proven” means DXC emitted SPIR-V and
`spirv-val` accepted the module. File forms used only for host-side tests or
benchmarks are explicitly identified.

The canonical implementation pipeline is source → tokens → AST → validation →
VD-MIR → deterministic HLSL → DXC SPIR-V. Backend-neutral meaning ends at
VD-MIR. HLSL spellings and `SV_*` semantics are backend details.

## 2. Shared core

### 2.1 Files, declarations, and names

A module may declare `namespace`, `import`, `use`, type aliases, records, streams, enums,
concepts, configs, templates, shaders, flows, compile materializations, tests,
and benchmarks as permitted by its file profile. Names are statically resolved.
Imports do not redefine language semantics.

Canonical declarations have complete parser-to-artifact support:

| Surface | Status and meaning |
|---|---|
| `namespace`, `import`, `export` | module identity, local source dependencies, and explicit library visibility |
| `use` | legacy namespace metadata; does not load another source file |
| `type` | implemented/validated/lowered/artifact-proven alias, including coordinate spaces |
| `record`, `with` | implemented/validated/lowered/artifact-proven immutable aggregate value operations |
| `stream` | implemented/validated/lowered/artifact-proven compiler-owned typed boundary |
| `enum`, payload cases, `match` | implemented/validated/lowered/artifact-proven tagged values and exhaustive matching |
| `concept`, `config`, `template`, `derive`, `compile` | implemented/validated/lowered/artifact-proven closed static abstraction and materialization |
| `flow`, board/state forms | implemented/validated/lowered/artifact-proven bounded control planning |
| `shader`, `stage` | implemented/validated/lowered/artifact-proven compute, vertex, and pixel entry declarations |
| `material` | implemented/validated/lowered/artifact-proven graphics uniform sugar |
| `.sdslvtest`, `.sdslvbench` | implemented host profiles; they do not add shader runtime types |

Historical `interface`, `implements`, and `override` declarations are not part
of SDSL-V. Source using them is rejected with migration diagnostic
`SDSL-V4101`; use concepts, templates, and compile materialization.

Libraries declare a namespace and explicitly export their public declarations:

```sdslv
namespace Scientific.Precision;
fn PrivateScale(value: f32) -> f32 { return value * 2.0; }
export fn Scale(value: f32) -> f32 { return PrivateScale(value); }
```

An individual shader imports that source without copying its helpers:

```sdslv
import "../shared/Precision.sdslv" as Precision;
// Inside a shader function: Precision.Scale(value)
```

Relative paths resolve against the importing file. The `.sdslv` extension may
be omitted. `import Scientific.Precision;` resolves
`Scientific/Precision.sdslv` under the entry directory, or the explicit search
roots supplied to the loader. Its declared namespace must match. Without `as`,
the last namespace component is the qualifier. Resolution never scans arbitrary
directories or downloads packages.

Functions, aliases, records, boards, streams, enums, concepts, configs, and shader
templates can be exported. Types use the same qualifier as calls:
`Precision.Sample`, `Precision.Scale(value)`, and
`compile Kernels.Tile<Settings.Four> as Tile4;`. A type and a function may share
a name; exporting one does not export the other. Private helpers and their
transitive dependencies remain usable inside their owning library. Distinct
libraries may use the same private names.

Only declarations reachable from the entry module are emitted. Diamond imports
load one canonical source once. Imported libraries cannot declare concrete
shader entries, compile materializations, tests, or benchmarks; those are roots
owned by the entry file. Missing sources or members, private access, import
cycles, conflicting namespaces, and alias collisions are source diagnostics
(`SDSL-V4401`–`4408`). Import aliases cannot be shadowed by local declarations.
The loader bounds a graph to 256 files, 8 MiB per file, and 64 MiB in total.

Compute and graphics compilation, GPU tests, benchmarks, and generated headers
share this loader. Diagnostics and VD-MIR provenance retain each declaration's
owning source. A sorted source graph with SHA-256 hashes includes the entry and
every transitive import, including unused imports. Its identity participates in
test/benchmark replay IDs and graphics bundle identities; changing an imported
source invalidates those identities. Single-file sources keep their existing
entry names and artifact identities.

### 2.2 Types and values

Runtime scalar types are `bool`, `i32`, `u32`, `f16`, `f32`, and their accepted
aliases. Closed vector and matrix families, fixed arrays, records, payload
enums, resource handles, `ndarray`, and `tensor` retain their validated GoOct
contracts. Array/tensor dimensions and storage layout are static. Tensor and
ndarray meaning is coordinate-independent; graphics coordinate spaces do not
alter tensor semantics.

`string` is host/static data for tests, diagnostics, assertion reasons, and
compile-time metadata. It is not ordinary GPU storage. Runtime allocation,
dynamic reflection, and open object graphs are absent.

`let name: T = value;` creates an initialized immutable binding.
`var name: T = value;` creates an initialized mutable binding. Both require an
initializer. Mutation through `let` is rejected; mutation through `var` is
validated against its type. A `with` expression returns an updated record value
without mutating its operand.

Fallibility is deliberately absent. There is no shader `Error` type, fallible
return, postfix propagation `?`, postfix unwrap `!`, `ok`/`err` fallible match,
or runtime `error(...)`. Historical syntax receives `SDSL-V4100`. Logical
negation remains canonical. Programs represent recoverable GPU state with an
explicit payload enum or status/resource value.

### 2.3 Semantic-space aliases

A named vector alias may attach a dotted nominal semantic identity:

```sdslv
type ClipPosition4 = float4 @space(clip.position);
type WorldPosition3 = float3 @space(world.position);
type WorldNormal3 = float3 @space(world.normal);
```

The pair `(base type, space name)` is the semantic type. Aliases with compatible
base types and the same explicit space are compatible. Different spaces,
including `world.position` versus `world.normal`, are incompatible. A spaced
value does not implicitly convert to or from its unspaced primitive. A
constructor or function establishes a space only when its declared target or
return type supplies that space. This is a static rule with no runtime cost.
The compiler preserves useful alias comments in HLSL.

The same mechanism may name a bounded non-graphics semantic basis or domain,
for example `zimage.attention.query_head`. Such names must be dotted and remain
exact nominal strings; `@space` has no parameters, inheritance, pairing
declarations, or runtime identity. It is valid only on aliases whose resolved
physical base is `float2`, `float3`, or `float4`. Alias references may be used
as record or payload-enum fields, function parameters and returns, locals, and
array/`ndarray` element types. The space belongs to the element value, not to a
tensor axis.

Ordinary function signatures define legal transformations. A function that
accepts one space and returns another is the explicit establishment boundary.
Function argument mismatches involving non-graphics semantic spaces use
`SDSL-V4123` and name the operation, expected space, actual space, and the need
for an establishment function. Intrinsics that require plain vectors do not
silently erase a space; spaced code must extract ordinary scalar components and
explicitly establish its result.

`clip.position` on a vertex output is the graphics position builtin. The
compiler performs no automatic object/world/view/clip matrix transformation.
The established graphics roots `object`, `world`, `view`, and `clip` retain
their closed position/normal/vector vocabulary and existing diagnostics.

Semantic spaces do not type tensor axes, infer contractions, declare automatic
basis transformations, or add operator overloading. Those capabilities require
a separate future tensor-index design if a concrete kernel needs them.

Repetitive families may use the flat grouped spelling:

```sdslv
space zimage.attention {
    QueryHead: float4;
    KeyHead: float4;
    PositionedQueryHead: float4;
    Score: float4;
}
```

This is syntax sugar only. Each member is an ordinary lexical type alias whose
space is the group path plus a canonical member suffix: PascalCase ASCII names
become lower snake case, and consecutive uppercase acronym runs stay together.
Thus `QueryHead` becomes `zimage.attention.query_head`,
`PositionedQueryHead` becomes `zimage.attention.positioned_query_head`, and
`QKVHead` becomes `zimage.attention.qkv_head`. The expanded `Score` spelling is
`type Score = float4 @space(zimage.attention.score);`.

Members must use PascalCase ASCII identifiers. Groups are flat and
non-generic; nested groups and relation, conversion, or pairing declarations
are not supported. Duplicate generated type names use `SDSL-V1509`. Distinct
declarations that produce the same explicit semantic space identity use
`SDSL-V4124`, including a grouped member colliding with an explicit alias.
Ordinary function signatures remain the sole transformation and pairing
authority. Grouping is erased by front-end desugaring and has no VD-MIR,
HLSL, SPIR-V, ABI, or runtime representation beyond the aliases it generates.

### 2.4 Streams

A stream is a typed compiler-owned shader boundary. Validation assigns exactly
one role and VD-MIR records it explicitly:

1. `stage-value`: vertex inputs/outputs, pixel inputs/outputs, and varyings;
2. `resource`: storage arrays, uniforms, textures, samplers, and read/write
   resources;
3. `builtin`: compiler-provided invocation or stage values.

Resource attributes or resource field types establish the resource role;
`builtin` establishes the builtin role; graphics location/target use establishes
the stage-value role. Stage signatures provide remaining unambiguous context.
Mixed roles, ambiguous use, resource fields in varyings, stage attributes on
resources, and incompatible cross-profile reuse are rejected.

Supported bounded field attributes are `binding`, `builtin`, `location`,
`target`, and `interpolation`. They are not an open-ended attribute system.
Explicit binding values and source order are semantic authority and must survive
lowering unchanged.

### 2.5 Static abstraction and comptime

Choose the declaration by what it names:

| Declaration | Meaning | Example use |
| --- | --- | --- |
| `type` | An alias for a value type, optionally with a semantic space | Function parameters, locals, fields, resource elements |
| `record` | A concrete aggregate of value fields | A sample, a transform, or shader parameters |
| `concept` | A schema and requirements for compile-time configuration | Legal tile dimensions, feature switches, and static invariants |
| `config` | Named compile-time values declared against one concept | A particular kernel specialization |

```sdslv
type SampleValue = f32;
concept TileShape {
    WIDTH: u32;
    require WIDTH <= 64u;
}
config Tile4: TileShape { WIDTH: 4u; }
```

`SampleValue` can be stored in a buffer and passed to a function. `TileShape`
describes which configurations are valid; `Tile4` supplies one. The compiler
checks the configuration and substitutes its values into a shader template.
Concepts and configs are erased before GPU execution. They cannot be used as
runtime value types or alias targets (`SDSL-V4410`). A plain type alias does not
create a distinct nominal type; `@space` adds the semantic identity described
above. Use `record` when you need fields in a runtime value.

Concept satisfaction is explicit: a config declares its owning concept.
Identical fields in two different concepts do not make their configs
interchangeable. The current shader template parameter is a concept-config
parameter, not an arbitrary type parameter; concepts do not declare methods,
inheritance, or general predicates over unrelated types. The
[executable example](../Language/SDSL-V/Modules/TypeAndConcept.sdslvvalid)
combines a value alias with a checked static configuration.

Concepts describe required static facts; configs satisfy those facts; templates
reuse shader structure; `compile Template<Config> as Name;` creates a concrete
program. `derive` retains its existing closed derivation behavior. Constraints
are checked before lowering. Generic templates never emit entry points unless
materialized. Concrete names deterministically prefix stage entries, such as
`ForwardTextured_VS` and `ForwardTextured_PS`.

Put template parameters after the declared name, as in ordinary Oct:

```sdslv
concept TileConfig { WIDTH: u32; }
config Four: TileConfig { WIDTH: 4u; }
template shader Tile<C: TileConfig> {
    stage compute [numthreads(C.WIDTH, 1u, 1u)] fn CS() -> void { return; }
}
compile Tile<Four> as Tile4;
```

Shader templates take exactly one explicit concept-config parameter. The
`template` keyword is required; ordinary shader declarations cannot take
parameters. The older `template<C: TileConfig> shader Tile` spelling remains
accepted for existing sources and pinned artifacts. New source uses the
declaration-first spelling. This changes authoring syntax, not specialization,
concept satisfaction, bindings, or emitted entry names. It does not add the
host language's type-parameter templates to the GPU profile.

`comptime` is bounded static planning and specialization for immutable facts:
sizes, layouts, closed variants, unroll counts, resource counts, specialization
choices, and artifact structure. It provides no arbitrary I/O, mutable global
compile-time state, open-ended interpreter, runtime allocation, or reflection
subsystem.

### 2.6 Shared control and intrinsics

Functions, records, payload enums and exhaustive `match`, `switch`, bounded
loops, flows/boards/states, and ordinary expression materialization have the
same meaning in all stages. Graphics does not duplicate these systems.

The closed graphics math additions are `Cross`, `Normalize`, `Saturate`, `Lerp`,
and `Reflect`; existing shared intrinsics such as `Dot` remain available with
their validated scalar/vector contracts. Intrinsic names lower deterministically
to backend operations.

Shared scientific math includes `Exp`, `Sqrt`, `Rsqrt`, `Sin`, `Cos`, `Tanh`,
and `Pow`. Unary operations accept a plain `f32`, `float2`, `float3`, or
`float4` and return the same type, applying the operation to each vector lane.
`Pow(base, exponent)` requires two values of the same supported type. Integral
arguments, scalar/vector broadcasting, mismatched widths, and implicit erasure
of semantic spaces are rejected. Aliases resolve before these checks.

`Sin` and `Cos` use radians; `Rsqrt(x)` is reciprocal square root. These are
runtime shader operations with shader floating-point precision and domain
behavior, including non-finite results. They introduce no host error path or
compile-time evaluation. Use explicit domain guards where the model needs
them. Use the existing `Convert<T>` and `Bitcast<T>` for numeric conversion and
bit-preserving conversion respectively.

Ordinary math uses these typed operations instead of inline `HLSL` blocks.
An escape block needed by a real shader identifies a missing language
capability to specify and qualify. The
[frontend modernization report](../internal/sdslv/DevelopmentReport/SDSL_V_FRONTEND_MODERNIZATION.md)
records the remaining subgroup and packed-precision cases separately from
the deliberate foreign-language conformance proof.

### 2.7 Integer bit operations

Scalar `i32` and `u32` support `&`, `|`, `^`, unary `~`, `<<`, and `>>`.
AND, OR, and XOR require two operands of the same integer type; complement
preserves its operand type. There is no implicit signed/unsigned conversion
or admission of booleans, floating-point values, vectors, or arrays.

Shifts preserve the left operand's type and require a `u32` count. Counts use
their low five bits: 32 means zero positions and 33 means one. Left shift
discards bits beyond the 32-bit word. Unsigned right shift fills with zeros;
signed right shift extends the sign bit. Signed values use their 32-bit
two's-complement representation. Runtime and bounded constant evaluation use
the same rules.

Hexadecimal integer literals use `0x` or `0X`, with an optional unsigned
`u`/`U` suffix. The value must fit its declared literal type. For example,
`0x7fffffff` is an `i32`, and `0xffffffffu` is a `u32`. Decimal unsigned
constants also admit the full `u32` range. Source spans retain the authored
spelling while the AST normalizes hexadecimal values for downstream tools.

```sdslv
let bits: u32 = Bitcast<u32>(value);
let exponent: u32 = bits & 0x7f800000u;
let upperHalf: u32 = bits >> 16u;
let inverted: u32 = ~bits;
```

Binary precedence, from lower to higher, is `or`, `and`, comparison,
`|`, `^`, `&`, shifts, addition/subtraction, multiplication/division/modulo.
Each binary level associates left to right; unary `~` binds tightly.
Thus `bits & mask == mask` compares the masked value, and `1u << n + 1u`
shifts by `n + 1u`. Use parentheses to make compound bit expressions clear.
Inside an angle-delimited type extent, parenthesize right shifts:
`array<u32, (16u >> 2u)>`. Adjacent nested type closers still parse as `>>`.

## 3. Compute profile

Compute stages retain the established syntax and semantics:

```sdslv
shader Example {
    resources ComputeIO;
    stage compute [numthreads(8, 8, 1)]
    fn CS(thread: ComputeThread) { /* ... */ }
}
```

Existing readonly/readwrite storage arrays, bindings, compute builtin streams,
workgroup memory, barriers, ndarray/tensor lowering, concepts/config/templates,
flows, payload enums, tests, benchmarks, and cooperative-matrix intrinsic island
are unchanged. Canonical compute builtins remain dispatch thread ID, group
thread ID, group ID, and group index with their existing types and HLSL mapping.
Compute uses DXC compute profiles under the existing Vulkan target policy.

The conformance compute source and all production sources are regression
authorities. Graphics additions do not reassign bindings, entry names, shader
IDs, or production ownership.

## 4. Graphics profile

### 4.1 Programs and stages

Graphics supports exactly `stage vertex` and `stage pixel`, either in a concrete
shader or a concept-constrained materialized template. Helper functions are not
entry points. A graphics program may contain a paired vertex/pixel stage or a
single stage for bounded compilation and conformance.

Stage parameters may be stage-value, builtin, or resource streams. Ordinary
record/helper parameters are legal where statically representable. Stage-value
records and streams are lowered to HLSL interface structs. Resource-stream
parameters are compiler boundary syntax and lower to global resources, not HLSL
entry parameters.

### 4.2 Vertex and pixel interfaces

Vertex input fields use `[location(n)]`; locations must be unique. Varying
locations are explicit when supplied and otherwise assigned deterministically.
A vertex output has exactly one `float4 @space(clip.position)` field, lowered to
`SV_Position` and the SPIR-V Position builtin.

Paired vertex output and pixel input contracts are validated before DXC.
Locations, types (including coordinate spaces), and interpolation must agree.
Missing, extra, duplicate, or mismatched fields are diagnostics, not reflection
discoveries.

A pixel stage may return a scalar/vector shorthand for target zero or an output
stream whose fields use `[target(n)]`. Multiple render targets are supported.
Targets must be unique and deterministic. Pixel depth output is not canonical;
attempts are rejected as unsupported.

### 4.3 Graphics builtins

Canonical source uses backend-neutral names:

- vertex: `vertex_id: u32`, `instance_id: u32`;
- pixel: `position` with the canonical position type, `front_face: bool`.

The validator enforces stage legality, exact type, and uniqueness. HLSL maps
these to `SV_VertexID`, `SV_InstanceID`, `SV_Position`, and `SV_IsFrontFace`;
SPIR-V structural facts record the corresponding builtins. Raw `SV_*` names are
not source syntax.

### 4.4 Resources, material, texture, and sampling

Graphics preserves compute storage arrays and adds `uniform<T>`,
`texture2d<T>`, and `sampler`. Resources require readonly/readwrite access where
appropriate and explicit `[binding(n)]`; descriptor set/space is deterministically
set zero in the current bounded profile. Duplicate bindings are rejected and
source order is preserved through VD-MIR, HLSL, bundle metadata, and SPIR-V.

```sdslv
stream ForwardResources {
    [binding(0)] Albedo: readonly texture2d<float4>;
    [binding(1)] LinearSampler: readonly sampler;
}
```

`Sample(texture, sampler, coordinates)` is the canonical sampled lookup. The
first argument must be `texture2d<T>`, the second a sampler, and coordinates a
compatible `float2`; its result is `T`. It is supported in pixel and vertex
stages under DXC/Vulkan shader semantics. Raw HLSL object methods are not source
syntax. Derivatives, comparison samplers, texture arrays, storage textures, and
implicit LOD policy extensions are unsupported.

A shader-local material block is sugar for an immutable generated parameter
record and readonly constant-buffer resource:

```sdslv
material {
    Tint: float4;
    Roughness: f32;
}
```

The generated resource follows explicit HLSL-compatible 16-byte register
packing: scalars/vectors occupy their natural 4-byte component width when they
fit the current register; a value that would cross a 16-byte boundary begins at
the next boundary; aggregates are aligned conservatively; final size is rounded
to 16 bytes. The bundle records every field offset, size, alignment, total size,
and binding. Material values are immutable. Textures and samplers remain
explicit stream resources.

### 4.5 Graphics program bundle

`oct sdslv compile-graphics` emits schema
`sdslv.graphics-program-bundle.v1`. A bundle includes concrete program and
entry identities, source/compiler provenance and hashes, DXC profiles/flags,
HLSL and SPIR-V paths/hashes, validation evidence, normalized structural SPIR-V
facts, vertex layout, varyings, builtins, pixel targets, resource bindings,
material layout, capabilities, and a deterministic replay identity.

The bundle deliberately excludes topology, viewport/scissor, render target
formats, blend/rasterizer/depth state, and other future runtime pipeline state.

Vertex compilation uses `vs_6_0`, pixel uses `ps_6_0`, and compute retains its
existing `cs_6_0` policy. All use the repository’s established Vulkan SPIR-V
target. `spirv-val` acceptance and structural inspection are required evidence.

## 5. Testing and benchmark profiles

`.sdslvvalid` and `.sdslvinvalid` define source and diagnostic contracts.
`.sdslvtest` defines GPU correctness cases with the established result ABI.
`.sdslvbench` defines performance cases and stable benchmark identities; it
does not define correctness. Host strings/assertion reasons in these profiles
do not become shader runtime storage.

Tests and benchmarks share the same parser, type system, VD-MIR, and backend as
ordinary shaders. Stable identities derive from their canonical source and
manifest inputs.

## 6. Compilation artifacts and manifests

Compute artifacts retain existing manifests, production shader IDs, registry
ownership, source hashes, SPIR-V hashes, and replay identities. The graphics
bundle is the portable paired-stage artifact. Its normalized interface/resource
facts, rather than universal byte identity, are the cross-compiler contract.

The conformance schema is `sdslv.conformance.v1`. Valid records name source,
profile, entries, stages, stream roles, spaces, bindings, locations, builtins,
capabilities, reference hashes, and an optional bundle. Invalid records name a
stable diagnostic code, exact primary span, optional related span, and category.

Conformance tiers are: (1) acceptance/rejection, (2) semantic manifest,
(3) diagnostic code/span, (4) structural SPIR-V, (5) runtime behavior, and
(6) exact bytes only for explicitly canonical golden artifacts. Independent
compilers need not reproduce GoOct AST or VD-MIR.

## 7. Diagnostics and conformance

Diagnostics have stable `SDSL-Vnnnn` codes and source spans. Related spans are
provided for conflicts such as duplicate binding and immutable mutation. The
workspace checker verifies the portable manifest, valid and invalid fixture
paths, unique entry identities, expected diagnostics, reference hashes, and
bundle integrity. The canonical source of conformance truth is:

```text
Language/SDSL-V/conformance/manifest.json
```

## 8. Explicitly unsupported features

SDSL-V does not include shader fallibility; interfaces/implements/override;
uninitialized bindings; implicit coordinate conversion or automatic coordinate
transforms; geometry, hull, domain, mesh, task, amplification, ray-generation,
or hit stages; pixel depth output; derivatives; texture arrays; comparison
samplers; read/write textures; arbitrary attributes; runtime reflection or
allocation; arbitrary comptime I/O/state; a pipeline-state DSL; a Vulkan
graphics runtime; HLSL `SV_*` source names; GLSL, Slang, or direct SPIR-V
emission. Unsupported surface is rejected and is not validated-but-unlowered.
