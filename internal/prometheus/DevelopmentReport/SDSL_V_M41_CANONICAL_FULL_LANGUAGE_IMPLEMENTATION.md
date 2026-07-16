# SDSL-V M41 — canonical full language implementation

## Outcome and authority

Convergence outcome: **SUCCESS**. Milestone state: **COMPLETE**.

SDSL-V is now one shared language with compute, vertex, and pixel profiles. The
normative authority is `docs/SDSL_V_LANGUAGE_SPEC.md` together with
`examples/SDSL-V/conformance/`; GoOct is the reference implementation. A parser,
validator, or emitter that disagrees with those authorities is defective.

The historical Wyrmcoil/Aurelian graphics document contributed useful stage,
material, resource, and coordinate-space ideas, but conflicted with GoOct’s
modern compute language. No Wyrmcoil or Aurelian repository was modified.

## Reconciliation

The human migration guide is `docs/SDSL_V_GRAPHICS_RECONCILIATION.md`; the
complete machine-readable feature ledger is
`docs/SDSL_V_GRAPHICS_RECONCILIATION.json`. It covers declarations, records,
streams, payload enums/match, arrays/tensors, coordinate aliases, static
abstraction, flows, all three stages, materials/resources/builtins,
texture/sampler, mutability, removed systems, and host test/benchmark profiles.

Canonical declarations are namespace/use, aliases, records, streams, enums,
concepts, configs, templates, derive, shaders/stages, flows/boards/states,
compile materializations, materials, and file-specific tests/benchmarks.
Historical interfaces are replaced by concepts/templates. Fallibility is
removed rather than deferred. `SDSL-V4101` and `SDSL-V4100` provide explicit
migration diagnostics.

## Shared semantic model

`stream` has one meaning: a typed compiler-owned shader boundary. Validation
and VD-MIR assign exactly one role: stage-value, resource, or builtin. Mixed
role use is rejected. Compute stream spelling and binding semantics are
preserved; resource lowering now also preserves explicit source order through
graphics projection.

Coordinate aliases carry `(base type, canonical space)` identity. Same-base,
same-space aliases are compatible. Primitive/spaced and differently spaced
values are not implicitly compatible. Constructors/functions establish a space
only through a declared target or return type. `clip.position` maps to the
vertex position builtin without runtime representation cost or automatic
coordinate transformation.

Shared records/`with`, payload enums/exhaustive `match`, initialized immutable
`let`, initialized mutable `var`, concepts/configs/templates, comptime, and
flow/board/state lowering operate unchanged inside graphics functions. The
canonical graphics proof exercises all of these rather than adding parallel
graphics-only systems.

## Graphics profiles

The parser, AST, validator, VD-MIR, lowerer, HLSL emitter, DXC driver, and SPIR-V
inspection path support exactly vertex and pixel in addition to compute.
Concrete and materialized-template programs receive deterministic stage entry
names; generic templates do not emit entries.

Vertex inputs and varyings carry deterministic locations. Vertex output
requires exactly one `float4 @space(clip.position)`. Paired vertex-output and
pixel-input location/type/interpolation parity is validated before DXC. Pixel
supports target-zero shorthand and explicit target streams, including multiple
render targets.

The bounded graphics builtin set is vertex ID, instance ID, pixel position, and
front-face. It uses backend-neutral source names and projects to HLSL/SPIR-V
semantics with stage/type/duplicate validation. Compute builtins are unchanged.

## Resources, material, and intrinsics

Graphics resources extend existing streams with `texture2d<T>`, `sampler`, and
uniform data. Explicit set-zero bindings remain source authority; collisions
produce stable diagnostics with related spans. `Sample(texture, sampler,
float2)` is typed and lowers deterministically. The closed useful graphics math
surface adds Cross, Normalize, Saturate, Lerp, and Reflect while retaining Dot.

`material` is sugar for a generated immutable parameter record and readonly
constant buffer. Layout uses deterministic 16-byte register packing and the
bundle records field offset/size/alignment, total size, and binding. Textures and
samplers remain explicit resources; no engine parameter-key or runtime
reflection subsystem was added.

## Bundle and conformance proof

`oct sdslv compile-graphics` emits
`sdslv.graphics-program-bundle.v1`: program/stage identities, source and compiler
provenance, DXC flags/profiles, HLSL/SPIR-V hashes, validation evidence,
normalized SPIR-V interface facts, locations, targets, builtins, resources,
material layout, capabilities, and deterministic replay identity. Pipeline
state and runtime objects are intentionally absent.

The permanent corpus is `examples/SDSL-V/conformance/` with 6 valid fixture
records and 21 invalid fixture records. The canonical graphics source subsumes
paired varyings, clip position, explicit inputs, texture/sampler, material,
concept specialization, payload match, record/with, flow, comptime, spaces,
builtins, and graphics math. Separate minimal vertex, minimal pixel, MRT, and
compute regression sources keep individual contracts legible.

Reference artifacts:

| Artifact | SHA-256 |
|---|---|
| ForwardTextured vertex HLSL | `adbec3af8425d50b51bba7b17d3bd5d7620e3f76d0551b618856bac92fd08a76` |
| ForwardTextured vertex SPIR-V | `b5da74f59d6985fed9cfa4d34430dc86b7e3dc66bdcdc9351457cf26db43faa2` |
| ForwardTextured pixel HLSL | `adbec3af8425d50b51bba7b17d3bd5d7620e3f76d0551b618856bac92fd08a76` |
| ForwardTextured pixel SPIR-V | `7ada711827920e644d5a33167fae348e5ace42d421994da23b766160d060afe7` |

DXC selected `vs_6_0` and `ps_6_0`; `spirv-val` accepted all committed graphics
modules. Structural inspection proves vertex locations 0/1, Position,
VertexIndex, InstanceIndex; pixel locations 0/1, FragCoord, FrontFacing; and
resource bindings 0/1/2. The canonical bundle replay identity is
`2913f2c0639468909ef18971d0e522b18d9b8d04a14d52e04e2cf66429a6eab2`.

The portable manifest defines six future independent-compiler tiers. GoOct AST
or VD-MIR parity and universal byte-identical SPIR-V are not requirements.

## Compute and stable-identity impact

M28–M40 contracts were retained: bounded intrinsic islands, M29 test ABI and
case IDs, fixed test resources, flow-stack execution, tensor/ndarray type and
layout, packed intrinsics, benchmark language, let/var, workspace ownership,
production reductions, and cooperative-matrix proof. Production shader source,
registry IDs, manifests, and decoded SPIR-V hashes did not change after
regeneration. Existing test, benchmark, reduction, and M40 replay-identity
derivation code was not modified.

## Validation

The required Go unit/integration matrix, manifest checks, canonical corpus,
vertex/pixel HLSL and SPIR-V compilation, `spirv-val`, structural inspection,
deterministic second generation, production shader regeneration, removed-syntax
search, Windows build, Linux shell syntax check, and `git diff --check` passed.
No graphics hardware execution was required or performed.

## Unsupported graphics surface

The exact exclusions are geometry/tessellation/mesh/task/amplification/ray
stages, pixel depth output, derivatives, texture arrays, comparison samplers,
read/write textures, arbitrary attributes, implicit coordinate transforms,
pipeline-state DSL, Vulkan graphics runtime/window/swapchain/render pass,
engine integration, GLSL/Slang/direct-SPIR-V backends, and runtime fallibility.
Unsupported syntax is rejected; it is not validated-but-unlowered.

## Return point

Future Wyrmcoil/Aurelian work should implement the portable conformance tiers
against the canonical spec without importing GoOct internals. With M41 closed,
the exact ML return point is M40b's recorded next workload: the
128x1024x1024 attention-score case with packed-f16 A from a real upstream
Vulkan operator, persistent packed B, cooperative SGEMM, row-wise softmax, and
no synthetic A residency or timed intermediate/final readback. M41 did not
begin that workload, attention execution, convolution, or another reactor.
