# Concept attention score execution

Date: 2026-11-05

Status: **MILESTONE VERIFIED — Success (end state A).**

The Gemma 4 E2B raw-score executor now lives in
`reactors/PrometheusRmsNorm.concept`, sharing the attachment that owns its Q/K
producers. The existing public entrypoint still prepares normalized, BF16,
positionally encoded Q and K, then computes raw scores. The complete chain
avoids the shared legacy model state and intermediate host readbacks.

## Ownership and execution

The score pipeline is created lazily from package variant 69 using the existing
Concept Vulkan module-to-pipeline API. Its shared output buffer persists across
calls. The existing closed numerical contract remains 15 tokens, eight query
heads, one key head, width 256, and scale 0.0625. Masking and softmax remain
separate operations. No shader source or public request/result layout changed.

One owned score submission borrows the Concept-owned resident Q/K buffers.
On successful final readback it retires and consumes their resident authority.
Record/submit failures preserve Q/K for retry. An uncertain observation retains
the submission and all of its input/output storage; retry, producer replacement,
or destruction must drain it before reuse. The internal one-shot fault seam
exercises these states without adding fields to the public configuration ABI.

The Q/K producers and score consumer use separate recordings on the same queue.
The score recording explicitly declares the prior compute-write → compute-read
dependency through Concept Vulkan's existing barrier primitive; a recording's
local hazard tracker has no history of previous submissions. Declared-access
dispatch and final host visibility use the foundational implementation.

The C++ adapter validates the foreign packet and serializes the shared owner.
The C11 facade projects packets and diagnostics while keeping generated class
layouts private. Concept owns pipeline/buffer creation, recording, submission,
failure retention, readback, and retirement. Score IDs use the same diagnostic
namespace as their producer slots. The former legacy-borrow bridge and its
device-idle waits are removed; consumer completion now follows its owned fence.

Calls on one owner, including teardown, remain externally serialized. The shared
queue callback still allows legacy SGEMM and Concept reductions to coexist on
the original device. General model-block/Z-Image state and standalone legacy
RoPE remain outside this milestone.

## Compiler defect found by the port

The first native package generation rejected valid `PrepareScores` with
`CV4516: MIR cleanup ... is not a unique ordered drop obligation`. Its two
independent branches each declared an owning `entry`. The MIR cleanup collector
used one name-keyed type/move-state table for all scopes. Reducing the case also
found a nested-shadow bug: C emission retired the inner owner and lost the
outer owner's drop, because both used the same live-owner key.

Both fixes belong to Concept. Cleanup collection now resolves lexical
declarations, including non-owning shadows, and gives repeated owning names
stable identities. An optional `source_name` retains the source-name connection
for planner replacement lookup; that metadata is omitted for unique owning
names. Existing checked outputs pass without regeneration. C emission assigns
distinct storage names to shadows. The MIR uniqueness/order
validator remains enforced.

The authoritative runtime witness is Concept's
`tests/runtime/scoped_cleanup.concept_test`: independent conditional scopes,
different resource types with the same name, nested shadowing, and moved/live
owners. The owning Go test inspects cleanup identities, performs three fresh
deterministic generations, and independently compiles and executes strict C11.
The initial diagnostic and failing nested-drop witness were retained as defects,
then rerun after the fixes; renaming the consumer's variables was not the fix.
The rejected package log is `.tmp/attention-m3-package.log`; the generated C
witness that omitted the outer drop is under `.tmp/attention-scoped-cleanup/`.

## Verification

```powershell
cmd /d /c internal\prometheus\native\build_windows.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptRmsNorm
out/prometheus/native/marionette_tests.exe PrometheusConceptAttention_
out/prometheus/native/marionette_tests.exe PrometheusM46
out/prometheus/native/marionette_tests.exe PrometheusM49aMatchedInputM46HardwareProof
out/prometheus/native/marionette_tests.exe PrometheusReduction_
out/prometheus/native/marionette_tests.exe PrometheusFrM0
out/prometheus/native/marionette_tests.exe PrometheusReactor_Sgemm
out/prometheus/native/marionette_tests.exe PrometheusStage0
go run ./tools/prometheus_concept -check
go test ./tools/prometheus_concept ./tools/prometheus_native_manifest
go vet ./tools/prometheus_concept ./tools/prometheus_native_manifest
go run ./tools/prometheus_native_manifest
```

GPU checks compare both public preparation orders and optional positional
readback against the existing independent BF16/double-sum/dot-product oracle.
They require no legacy model state, one reusable score pipeline, allocation-free
warm score execution, correct score/producer authority, coexistence with SGEMM
and reductions, and balanced resources on destruction. Fault checks cover
malformed/missing inputs, record/submit/observation failures, retry without
reupload, producer reuse, and direct detach with retained score work.

Windows/RTX 3070 results: all 34 selected native facts pass with validation
enabled, zero skips, and zero failures. The resident score oracle's maximum
absolute error is `5.96046e-8`; RMSNorm remains at `1.19209e-7`. Warm score calls
allocate no buffers; score and producer resources return to their starting
counters on detach/destruction. Native package, DLL, test/benchmark binaries,
and SDSL-V host build successfully. Tooling tests/vet, manifest parity, ignored
generated-output checks, and fresh deployment comparison pass. The four existing
Concept reduction facts also pass in Normal and Verify.

Concept validation includes focused ownership/failure/C11 regressions, the new
runtime contract in Normal and Verify, checked outputs without regeneration,
and full Go tests and vet, all passing. The compiler package completed in
1,577.284 seconds and the CLI package in 345.768 seconds. The owning cleanup
test passed three fresh deterministic emissions and independent native C11
execution. The emitted production module also passes
`clang -std=c11 -pedantic-errors -fsyntax-only` with the Vulkan include paths.
Frozen Zig is skipped because that compiler and its
infrastructure did not change. This is ordinary milestone verification; no
new full phase-closeout or Linux GPU qualification is claimed.

The final logs use `.tmp/attention-m3-*`. Generated C, headers, provenance,
objects and archives remain ignored build outputs. The authored manifest and
Concept modules remain the build authority.

## Next boundary

Follow-up: [M4](prometheus_concept_rope_m4.md) completes the standalone RoPE
port described below. This report preserves the M3 verification boundary.

`reactor_vulkan_gemma4e2b.c` now contains the pure RMSNorm planner/oracles and
standalone legacy RoPE. Removing the standalone RoPE executor can reuse the
already owned Concept pipeline and complete this file's GPU execution port.
