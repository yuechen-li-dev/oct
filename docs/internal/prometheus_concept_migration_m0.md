# Prometheus Concept reactor migration: modules and first production capability

This is the M0 historical checkpoint. The subsequent
[shared-device execution milestone](prometheus_concept_shared_device_m1.md)
replaces the standalone C reduction executor and resolves the device blocker.

Date: 2026-11-05

## Accepted direction

This is an intent port. Preserve public contracts, numerical requirements,
resource lifetime, diagnostics, and observable failure behavior. Improve the
implementation instead of translating source lines or copying every old test.
Use independent observable acceptance cases for the new design.

Concept provides the language and foundational Vulkan library. Prometheus
reactors own specialized execution, buffering, scheduling, model mechanisms,
and their implementation details. A missing reactor capability does not imply
it belongs in Concept's foundational Vulkan library. Compiler defects exposed
by the port belong in Concept.

The C ABI remains the consumer boundary. Concept-authored code may deploy as
generated C11. Any handwritten C++23 bridge follows the current upstream
primer, pinned at `648212bf6b445d3910a7d3941961c47b0140ca91`:
<https://github.com/yuechen-li-dev/primer/tree/648212bf6b445d3910a7d3941961c47b0140ca91>.
Read its `common-primer.yaml`, `cpp-primer.yaml`, and `vulkan-primer.yaml`.

## Working baselines

- Oct: `0b03bca7`, branch `codex/sdslv-imports-prometheus`.
- Concept: current integration main `f3bff84`, branch
  `codex/prometheus-reactor-port`. The previously clean checkout was
  fast-forwarded from `7daab14` after fetching main.

## Execution order

1. Implement real SDSL-V imports through one module loader and declaration
   binding pass shared by checks, compute/graphics compilation, GPU tests,
   benchmarks, and packaging. Preserve per-file diagnostics and dependency
   identities. Keep existing single-file entry commands.
2. Extract shared BF16 shader code and exercise imported private helper closure,
   qualified declaration identity, dependency changes, and native GPU execution.
3. Implement the first Concept-owned production reduction capability. Share the
   existing runtime/device authority and public ABI. Keep reactor implementation
   details in Prometheus. Use C++23 only at explicit mechanical FFI boundaries.
4. Verify public behavior, numerical results, failure cleanup, repeated use, and
   real GPU execution. Remove replaced production authorities when the new path
   owns them; preserve useful old evidence without requiring a mechanical port.

## Acceptance evidence

Module behavior belongs in `Language/SDSL-V/` source and GPU contracts. Go
tests orchestrate module loading, emitted artifacts, source provenance, and
dependency identity. Concept behavior belongs in `.concept_test` contracts;
compiler boundary checks use the same fixtures. Current Concept qualification
policy governs compiler fixes. The frozen Zig compiler remains unchanged.

## Result: meaningful progression

The import prerequisite and first production Concept capability are implemented
and verified. The full reactor migration remains open. This is **MILESTONE
VERIFIED** for the bounded planner seam, not full Concept compiler qualification
or a completed Vulkan executor port.

### Modules and static declarations

One loader serves checks, compute/graphics compilation, GPU facts, benchmarks,
and source provenance during package staging. It resolves explicit exports,
transitive private closure, diamond dependencies, relative and namespace paths,
and qualified types/configs/templates. Imported functions and types retain
separate identity even when their source names match. Unreferenced namesake
functions do not become roots merely because a type is used.

The graph owns per-file spans and sorted source hashes. Dependency edits change
test/benchmark replay IDs and graphics bundle identity. Package staging refuses
stale imported headers. Single-file conformance and existing pinned identities
remain green. Five module acceptance cases and twelve diagnostic cases are
registered in the conformance manifest; three module GPU facts exercise actual
imports, payloads, and private helper closure.

The language specification now explains `type`, `record`, `concept`, and
`config` together. A concept is a static configuration schema; a config supplies
values against that schema. Neither is GPU value storage. The validator formerly
accepted concepts as runtime types and alias targets; those paths now produce
`SDSL-V4410`, covered by permanent negative contracts.

### Shared production BF16

`production/shared/Bf16.sdslv` owns round-to-nearest-even conversion and exact
packed-lane widening. The Gemma roundtrip and RoPE shaders and Z-Image ingress
import it. The permanent precision GPU contracts also import that production
authority instead of carrying a second implementation.

All three complete production modules retain **byte-identical SPIR-V**, including
before stripping debug names. Their generated headers gain source graph
provenance; shader IDs, entries, bindings, and numerical policy stay stable.
All 31 modernization GPU cases and the three module facts pass.

### First production Concept owner

`internal/prometheus/reactors/PrometheusReduction.concept` now owns numeric
admission, packed/fused/composed strategy, stage topology, and temporary-region
requirements for sum, max, and stable softmax. One row-reduction combinator
expresses the partial and finishing passes; the old C planner, crossover, and
temporary-assignment implementations were removed.

The implementation uses a mutable internal builder and an immutable `repr(C)`
schedule. The adapter checks foreign pointers, maps stage roles to the existing
catalog, serializes the public plan, and hashes that wire packet. Public ABI
constants are checked at compile time. There is no second planner or alternate
runtime selection path.

The official generator is `go run ./tools/prometheus_concept`; `-check` compares
fresh output. Generated C11/header/provenance files are intentional clean-clone
build inputs, listed in the native manifest. Ordinary builds require no sibling
Concept checkout. Go tests reject stale source or edited generated outputs.
Regeneration records compiler revision `f3bff8463d00dad231936c4c2e1cf228d1aded5b`
and source/output hashes. LF rules pin Concept and primer bytes; the Windows
build fragment explicitly checks out with its generator's CRLF convention.

| Responsibility | Current owner |
| --- | --- |
| Numeric request admission, strategy, topology, temporary sizing | Concept reactor |
| Foreign pointer admission, public packet serialization, replay hash | C adapter |
| Device and queue authority | Existing shared Prometheus runtime |
| Buffers, descriptor sets, recording, slots, retirement, transformer resources | Existing C executor; not ported in this checkpoint |
| Shader programs | SDSL-V, including the shared precision module |

### Evidence

- Four Concept facts pass through strict C11 in Normal and Verify modes.
- Independent public-ABI comparison against `0b03bca7` passes 5,950 requests:
  identical status and plan bytes, including invalid shapes and replay IDs.
- Rebuilt production Marionette: seven reduction tests and two FR-M0 softmax
  tests pass, with zero skips. They cover GPU numerical results, repeated use,
  strategy agreement, nonfinite rejection, failure cleanup, and output freshness.
- Imported compute templates and a vertex/pixel graphics bundle compile with
  DXC and `spirv-val`. An imported benchmark executes via Kaiju Vulkan on the
  RTX 3070 with GPU timestamps. This is path qualification, not a speedup claim.
- Generated Concept deployment compiles as strict C11 with Clang and GCC and
  through the real MSVC native build. The public C header itself still has two
  historical enum values outside C11's `int` range; a pedantic compile of the
  entire adapter exposes those existing declarations. This does not qualify the
  whole legacy library as pedantic C11.
- Full fast Go regression, focused SDSL-V/conformance/package tests, reference
  link validation, native manifest parity, and deterministic regeneration pass.
  Unchanged frozen Zig and broad Concept backend qualification are skipped.

Initial native execution failed before shader packages were staged alongside
the executable. The initial failure log was retained; staging the real package
and rerunning the affected lanes produced the passing native results above.

The broader workspace checker still fails its preexisting source-bearing native
registry assumption ([FB-104](../../FEEDBACK.md#fb-104--workspace-ownership-check-still-assumes-a-source-bearing-native-registry)).
Imported benchmark qualification also exposed the independent top-level builtin
scope defect [FB-110](../../FEEDBACK.md#fb-110--top-level-gpu-benchmarks-admit-invocation-builtins-without-emitting-them).
Neither gate is represented as fixed by this milestone.

## Next isolated blocker: shared-device execution

Directly substituting Concept's existing Vulkan executor is unsafe with the
current shared runtime:

1. `libraries/Vulkan/native/device_runtime.c` initializes its process-global
   `cmdPipelineBarrier2`, `cmdPushDescriptorSet`, and timestamp entry points only
   inside `impl_CreateContext`. It has no borrowed-device attachment operation.
2. Its `Context` destruction path destroys the device and instance. The
   Prometheus device is shared with other reactor families and must remain owned
   by the existing runtime.
3. Concept's device creation explicitly enables `synchronization2` and
   `pushDescriptor`. Prometheus's current feature chain enables other optional
   model capabilities but neither of those features. A Vulkan 1.4 API version
   alone does not enable them.

The next execution change needs an explicit borrowed-device capability contract
and entry-point lifetime, with feature admission at the owning device-creation
seam. Reactor-specific buffering, package pipelines, recording, and retirement
remain Prometheus responsibilities. Creating a second context or relabeling the
existing C executor would bypass the ownership problem rather than remove it.
