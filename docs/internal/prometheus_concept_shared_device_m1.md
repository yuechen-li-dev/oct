# Concept reductions on the shared Prometheus device

Date: 2026-11-05

Status: **MILESTONE VERIFIED — Success (end state A).**

The accepted intent-port capability works through the production Vulkan path,
and the scoped regressions pass. The scope is standalone row-wise FP32 sum, max, and stable softmax, their public
ABI, shared-device lifetime, and a working native consumer package.

## Build-output cleanup follow-up

The M1 evidence below originally used checked generated deployment files.
The subsequent cleanup removes those six C/header/provenance snapshots from
Git (5,790 lines, 242,520 bytes) and generates them under ignored
`internal/prometheus/.native-build/generated/`. Native helpers emit and verify
them before any C consumer compiles. Source manifests, integrity checks, shader
build inputs, and historical numerical witnesses remain authoritative.

The clean package entrypoint is now
`go run ./tools/prometheus_concept -build-package`: it emits Concept source,
checks source/output digests, and invokes the native manifest build. Direct
`concept package build` consumes those emitted inputs. A clean-checkout source
inventory check does not require disposable outputs to exist; the real build
still requires and validates them.

Cleanup verification: the Windows native build succeeds with the old package
cache moved aside. All four freshly emitted C/header files are byte-identical
to the prior checked deployment. The 26 reduction/softmax/SGEMM Vulkan tests,
three SDSL-V module GPU cases, tooling tests, and tooling vet pass. The rebuilt
C and provenance are ignored, including the retired output location. Historical
numerical evidence and required shader fixtures are retained.

The subsequent [RMSNorm M2 port](prometheus_concept_rmsnorm_m2.md) removes that
operation's dependence on the legacy model state and includes its resident RoPE
tail. Standalone legacy RoPE and the score consumer remain future port slices.

## Ownership after the port

| Responsibility | Owner |
| --- | --- |
| Shape admission, strategy, dispatch topology, temporary requirements | `reactors/PrometheusReduction.concept` |
| Ready slots, reusable buffers/pipelines, recording, completion, quarantine, retirement | `reactors/PrometheusExecution.concept` |
| FIFO ready queue | `DragonGod.Scheduling.Queue` |
| Spans and finite-value checks | Standard library |
| Borrowed device attachment, per-device dispatch, Vulkan resource mechanics | Concept `libraries/Vulkan` |
| Public packet projection and opaque generated-state storage | small C11 ABI facade |
| Shader package module loading and public/native adaptation | C++23 adapter |
| Device/queue authority and legacy SGEMM | existing Prometheus runtime |
| Queue host synchronization across old and new operation families | one short native queue API boundary |

The old standalone C executor, its buffer preparation, command recording, and
public execution/diagnostic implementation were removed (over 300 lines).
`reactor_vulkan_fused_reduction.c` retains plan serialization, CPU oracles,
softmax public admission, and model resource support still used by legacy model
operations. Ordinary reductions never initialize that model state.

This is an intent port. It retains public numerical, failure, output-freshness,
request identity, ring reuse, and resource-lifetime contracts. Buffers and seven
pipelines persist across calls; command buffers, fences, and optional queries
use the existing owned Vulkan recording/submission API. Persistent command
object pooling and performance crossover measurements are not claimed here.

## Shared-device contract

The owner queries and actually enables synchronization2 and pushDescriptor in
its device feature chain, preserving its existing model/ray feature chain.
`DeviceBorrow` records physical device, device, queue, family, and enabled bits.
A borrower creates its own pool and cache and never destroys the owner's device,
instance, or queue. Dispatch registrations are per device and reference-counted.

All legacy production queue submissions and Concept callbacks use the same
host synchronization boundary. GPU completion waits do not hold its submission
lock. This follows Vulkan's
[external queue synchronization requirement](https://docs.vulkan.org/refpages/latest/refpages/source/vkQueueSubmit.html).
Unknown completion retains a slot and its resources. Detach drains retained
submissions before dropping slots, pipelines, pool, and cache. Observed device
loss is terminal for the reduction executor; Vulkan's
[device-loss lifetime rule](https://docs.vulkan.org/spec/latest/chapters/devsandqueues.html)
allows pending-resource retirement after that result. Physical device-loss
recovery was not induced or qualified in this milestone.

The native C++ boundary does not include generated private class layouts or
C11 atomic representations. It adapts recoverable allocation/mutex failures to
public errors and keeps exceptions inside the C++ boundary.

## Concept defects fixed at their owner

The real port exposed and reproduced these failures:

1. A fallible numeric conversion inside a record update omitted its generated
   Result type. Failure collection and panic detection now use the common
   typed syntax walker rather than partial hand-maintained expression lists.
2. Borrowed matches copied and destroyed owned payloads, freeing the live
   buffer before upload. Semantic and C lowering now project borrowed payloads
   with the carrier's mutability.
3. Structural owned payloads inside carriers were not recursively dropped,
   and replacement through a named mutable reference omitted the old drop.
   Carrier cleanup now reuses the existing structural lowerer, with forward
   declarations for nested carrier cleanup; reference replacement evaluates the
   incoming value before retiring the old value.

Authoritative new runtime contracts are Concept's
`tests/runtime/record_failure.concept_test`,
`tests/runtime/borrowed_owned_match.concept_test`, and the Vulkan library's
borrowed-buffer replacement fact. They pass through strict C11 and in Normal
and Verify. See [FB-111](../../FEEDBACK.md#fb-111--concept-reduction-port-exposes-incomplete-generated-ownership-cleanup).
The separate contextual Result argument gap is recorded in
[FB-112](../../FEEDBACK.md#fb-112--concept-call-arguments-do-not-supply-context-for-result-constructors).

## Manifest and starter package

`internal/prometheus/manifest.concept` builds the deployed Concept modules,
C11 facade, Vulkan dependency source, and C++23 adapters into two static
archives. Both native launchers consume these archives through the real Concept
package builder. The native source manifest records which sources the Concept
package owns. Windows source/object response files keep commands within shell
length limits. Generated deployment provenance includes source/output hashes,
the compiler checkout revision, and a hash of actual compiler/library inputs,
including uncommitted implementation files.

The Vulkan native implementation stays in Concept and is compiled from the
installed dependency. Native builds require that distribution's libraries plus
a current Concept driver (or the sibling compiler checkout for bootstrap),
Clang/LLVM, a C++23 standard library with `std::expected`, and the Vulkan SDK.
These cross-repository changes must be published together before a clean remote
checkout can use the new dependency contract. No repository was committed or
pushed by this milestone.

```powershell
$env:CONCEPT_LIBRARY_ROOT = 'C:\Users\yuech\source\repos\Concept\libraries'
$env:VULKAN_SDK = 'C:\VulkanSDK\1.4.350.0'
# Emit source and build the manifest-owned archives from a clean checkout:
go run ./tools/prometheus_concept -build-package
```

Concept also supports a real default starter:

```console
concept init MySimulation --author "Your name"
concept package build MySimulation
concept package test MySimulation
concept package test MySimulation --verify
```

The initializer creates a manifest, source module, and executable fact, and
rejects existing files. `Standard.Build.Metadata` now lives at its resolvable
module path. Manifest imports use ordinary semantic module resolution.
Native inputs can explicitly name `@Vulkan/native/...` and
`@VulkanSDK/include`; their source/header bytes enter build identity, while
outputs remain inside the consumer. Empty native test lists allow archive
builds; requesting tests without test functions remains an error.

## Evidence ledger

- Complete Windows native build: PASS on the final source, including DLL,
  Marionette tests, benchmark executable, and SDSL-V host.
- Real Vulkan on NVIDIA GeForce RTX 3070 with validation enabled: nine
  reduction tests PASS, zero skips.
  Includes alternating SGEMM/reductions across two independent devices,
  borrowed-resource detach and reattach, preserved device/queue identity,
  quarantine drain, no legacy reduction/model state, and balanced live-resource
  counters. Two existing FrM0 tests and fifteen legacy SGEMM regressions also
  PASS, zero skips: 26 native tests total. The FrM0 witness records maximum
  absolute error `1.49011611938e-8`, maximum row-sum error
  `1.09954271466e-7`, zero ordering violations, and zero validation errors.
- Manifest-built archives independently linked into the existing reduction
  harness: all nine reduction tests PASS.
- Concept Vulkan package: fifteen facts PASS in Normal and Verify.
- Standard: 44 facts PASS in Verify, plus three bounded benchmark cases.
  DragonGod: 23 facts PASS in Verify, plus one bounded benchmark case.
- Two new Concept ownership/failure facts: PASS in Normal and Verify; focused
  strict-C11, numeric/failure, package initializer, dependency hash/invalidation,
  and ABI mismatch regressions PASS.
- Default package CLI: initialize/build/test/Verify PASS; second initialization
  rejects with `PACKAGE_INIT_EXISTS`.
- `./test.ps1`: PASS (Concept runtime/backend facts in Normal/Verify and CLI
  workflow checks). Existing checked compiler outputs: PASS without regeneration.
- Oct `go test -timeout 30m ./...`: PASS. Oct and Concept `go vet ./...`: PASS.
- SDSL-V module GPU corpus: three cases PASS. No shader source changes required.
- Deterministic Concept deployment: three fresh current-source generations
  match the checked output bytes and provenance. Final generated executor also
  compiles with `clang -std=c11 -pedantic-errors`.
- Concept `go test -timeout 75m ./...`: PASS. The compiler package completed
  in 1,509.361 seconds and CLI package in 318.707 seconds. The initial attempt
  reached Go's default ten-minute timeout without an assertion failure before
  timeout; that failed run is retained separately. This is ordinary milestone
  verification, not the complete 100-trial phase-closeout qualification.
- Frozen Zig suites skipped: the frozen compiler and its infrastructure did not
  change. Linux native compilation was not executed on this Windows host.

Earlier crash/numerical failure probes and the initial timeout remain in the
ignored local `.tmp` evidence. The passing runs above rerun the affected paths
with the fixes; prior failures are not relabeled as passes. FB-104 workspace
checker and FB-110 top-level benchmark builtin scope remain outside this port.
