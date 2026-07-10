# SDSL-V M29 — `.sdslvtest` design authority

## Scope and ownership

M29 is a test-only compute contract. Discovery, compilation, manifest
creation, process orchestration, timeout enforcement, Vulkan preflight,
readback, and formatting belong to the host. Test bodies and assertion
evaluation belong to GPU code. The GPU writes a fixed record per invocation;
the host chooses the lowest failing invocation deterministically.

The fixed generated interface is descriptor set `0`, binding `0`, a compiler
owned structured result buffer. Its record is version `1`, fixed width, and
contains failure state, assertion/source identity, invocation coordinates,
kind/component metadata, and expected/actual/tolerance bit lanes. It has no
pointers, strings, variable payloads, or user-visible resource declaration.

Facts create one case. Theories create one case for each `InlineData` row;
their values are compile-time constants and rows share a compiler group by
workgroup size. The manifest is the durable artifact: stable IDs are SHA-256
prefixes of canonical source path, function name, case kind, and row identity.
`--case` addresses that ID and is independent of transient ordering.

Assertions keep a local first-failure state and never early-return. Generated
epilogues write the final invocation record. Initial meaningful assertions are
`True`, `False`, `Equal`, `NotEqual`, and `Near` for Bool/Int/UInt/Float and
unambiguous small vectors; operands must be evaluated once.

The eventual native `sdslv_test_host` runs one selected case per process. The
parent compiles once, supplies the manifest and selected case, applies a
conservative deadline, and classifies abnormal exits, timeout, and device loss.
`PROMETHEUS_REQUIRE_VULKAN_HARDWARE=1` converts absent loader/device/compute
queue into an explicit failure; normal CI may compile and validate manifests
while honestly skipping execution.

## Current implementation boundary

This pass adds a first executable narrow path for the current M29 inline-HLSL
fixture. The compiler groups cases by workgroup size, emits a compiler-owned
HLSL dispatcher, compiles it through DXC, and invokes the separate
`sdslv_test_host` process per case. The host owns a set-0/binding-0 storage
buffer, four-word push constants (`case`, `row`, `width`, `height`), checked
result allocation, pipeline creation, polling, ABI readback, and JSON output.
The host is built by the canonical native manifest/build scripts but is not
linked to the reactor API or shader registry.

On this Windows checkout, `InlineHlslFacts.sdslvtest` produced four PASS JSON
results through real Vulkan after DXC SPIR-V generation: the inline `asuint`
Fact, two independently dispatched Theory rows, and the explicit 32-thread
launch Fact. This is hardware execution evidence for the bounded foreign HLSL
path, not a claim that arbitrary SDSL-V test bodies are supported.

## Explicitly deferred

Arbitrary user resources, descriptor schemas, textures/samplers, graphics
tests, whole-buffer host scripting, production registry IDs, arbitrary public
compute dispatch, pools/parallel test execution, and async/batch redesign are
all deferred. These limits keep M29 test-owned and prevent pollution of the
production shader registry.

## M28 handoff

M28 remains accepted for compiler/toolchain scope. M29 now supplies real
hardware dispatch/readback evidence for its `asuint` inline-HLSL fixture.

## Acceptance status

**SDSL-V M29 BLOCKED.** The fixed execution path works for the current bounded
inline-HLSL fixture, but completion requires real SDSL-V AST/VD-MIR lowering of
assertions and bodies (rather than the fixture-specific emitter), intentional
failure and invalid-ABI fixtures/tests, M27 guarded-read execution without
introducing user resources, robust source locations, process-level timeout
enforcement by the parent runner, and the requested committed evidence files.

## M29a first-class declaration boundary

Regex extraction is no longer discovery authority. The ordinary SDSL-V lexer
and parser now parse function-level attributes into `ast.FunctionDecl.Attributes`
using the existing typed `ast.Attribute` expression arguments. `.sdslvtest`
discovery loads the file through `source -> lex -> parse`, derives Fact/Theory
metadata from real declarations, validates launch constants and typed scalar
theory literals, and then derives the existing stable IDs and manifest cases.
Comment and string text cannot create test cases because lexer ownership comes
before parser attribute construction.

Function test attributes are rejected by normal validation in non-`.sdslvtest`
sources. The fixture-specific HLSL emitter remains a temporary M29 bootstrap,
but it now consumes AST-derived cases and must not gain new fixture branches.
M29b owns its deletion once Assert intrinsics, test-body lowering, source spans,
and epilogue state are represented in VD-MIR.

**M29a BLOCKED.** The AST/parser authority is established, but full M29a still
needs validator-owned duplicate/placement diagnostics, precise typed metadata
source spans, normal body validation with unlowered Assert intrinsics, and the
requested parser/validator coverage before it can be accepted independently.

### M29a validator follow-up

The normal validator now owns core function-attribute conflicts and test
function contracts, and `.sdslvtest` discovery invokes normal module
validation before deriving cases. `Assert.True`, `Assert.False`, `Assert.Equal`,
`Assert.NotEqual`, and `Assert.Near` are recognized only in test source and
have front-end arity/type validation; they remain `void` intrinsics with no
VD-MIR, HLSL, or GPU behavior. M29b replaces the bootstrap emitter by lowering
these validated call sites into local failure state and the fixed result ABI.

**SDSL-V M29a BLOCKED.** Exact attribute/argument source spans in typed
metadata, validator-owned launch/InlineData constant diagnostics, a single
exported validated-declaration handoff model, and the requested dedicated
coverage remain incomplete.
