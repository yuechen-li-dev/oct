# Prometheus Concept ContextRefiner — M7

Status: **MILESTONE VERIFIED** for standalone and shared-owner ContextRefiner on
Windows Vulkan. This is a bounded consumer port, not full-model qualification
or a performance claim.

## Scope

ContextRefiner's eleven model pipelines, buffers and ordinary execution now
belong to the existing Concept block owner. The handwritten C pipeline factory
and full execution recorder are deleted. The C static-audit adapter records
the same Concept stages and retains its generated packet schedule, readback and
public generation policy. MainTransformer's executor remains C.

This is an intent port for practical maintenance. Reuse the existing stage and
ownership machinery where the operations are the same; retain useful native
adapters and repair actual lifetime problems rather than preserving C's shape.
No Concept compiler or shared-library changes were needed.

## Design

- `RecordStage` supports the existing 1024-token NoiseRefiner and the 32-token
  ContextRefiner shapes. Normalization, contractions, residuals, attention and
  FFN share one implementation. Context-specific head stages carry the distinct
  20-byte Q/K push contract and `[token + 1, 0, 0]` text-coordinate RoPE.
- `ContextFrame` projects the eleven package-local weight roles onto that stage
  vocabulary and supplies identity modulation from the existing unit buffer.
  The native borrowed-weight accessor reuses the authoritative mapping for
  shared physical slots; it does not duplicate a second tensor-role table.
- The owner has two disjoint fourteen-slot pipeline banks. A standalone Context
  portfolio uses eleven slots; the compound owner retains thirteen Noise and
  eleven Context pipelines together. Creating Context cannot replace a live
  Noise pipeline. MainTransformer's legacy portfolio is unchanged.
- Ordinary Context execution uses the same owned recording, submission,
  completion, quarantine and timing path as NoiseRefiner. Static audit borrows
  a recording and decodes bounded transport roles into the same typed stages.
- Package/interface validation remains in the C++23 adapter. C remains useful
  for public admission, rebinding, generated audit-packet orchestration and
  model/session integration.

## Defects repaired during the port

**FB-120: recovery admission ran before retirement.** Context execution and its
static audit rejected `quarantined` before reaching the existing reap operation.
An injected uncertain completion could therefore prevent recovery even after
the GPU finished. Admission now drains pending work through the existing owner
before reuse; failures still invalidate output and preserve their phase code.
The new regression failed precisely at completion retry before the fix.

**FB-121: compound-owner aliases had overlapping lifetimes.** Shared W3 aliased
the audit arena, overwriting packet headers and projections from earlier stages.
The pinned predecessor aliased the attention residual, so its second resident
replay consumed a different input. The independent test observed 133 assertion
failures, corrupted audit keys and a second-replay maximum output error of
0.852277, despite no Vulkan validation finding.

Both resources now have distinct Concept-owned device-local allocations and
exactly-once cleanup. Their combined cost is **17,039,360 bytes (16.25 MiB)**:
15,728,640 for the largest predecessor and 1,310,720 for Context W3. The compound
owner explicitly reserves those bytes beyond its Noise-only plan; the resulting
reservation is 671,931,136 bytes, compared with 654,891,776 before this fix.
Memory evidence includes both allocations. The measured compound committed
plan is 660,611,076 bytes; the test asserts that it fits the new reservation.
The standalone Context committed plan is 496,318,464 bytes. This is a deliberate
memory-for-correctness tradeoff; it adds no host activation bounce and changes
no frozen lock or payload identity. Warm execution creates no replacement
buffers, pipelines or weight uploads.

The host test also caught two implementation/setup mistakes during iteration:
standalone Context initially lacked its Concept attachment, and the shared
session request omitted the required explicit execution profile. Both were
corrected; invalid profiles remain rejected. Failed logs are retained in
`.tmp/m7-context-{first,owned,recovery,shared}.log`.

## Numerical and lifecycle witness

`PrometheusConceptContextRefinerHasIndependentNonzeroWitnessesAndSharedPortfolios`
needs no external payload. Sparse FP16 contractions and nonzero FP32 input feed
an independent double-precision CPU reference for Q/K normalization, text RoPE,
stable softmax, attention projection, residuals and SiLU-gated FFN.

It checks every final element (122,880 values per execution), all sixteen
static-audit boundaries' stage IDs, finite counts, keys and numerical projections,
two resident replays of one pinned predecessor, malformed ingress and stale
generation rejection, and record/submit/completion failure recovery. Legacy
SGEMM computes its own independent result on the same failure device. The shared
case executes Noise after both Concept portfolios have been created, retargets
the physical weights to Context, and checks shared audit and resident execution.
Explicit teardown returns Concept buffer/pipeline counters to their baselines.

The absolute numerical bound is 1e-4 and accepts no nonfinite values. Observed
errors are identical in standalone and shared cases:

| Observation | Maximum absolute error |
| --- | ---: |
| Every final element, external input | 1.7726e-5 |
| Every final element, each resident replay | 1.63384e-5 |
| Q norm projections | 1.04455e-7 |
| K norm projections | 1.84381e-7 |
| Positioned Q projections | 2.16922e-7 |
| Positioned K projections | 6.17971e-7 |
| Largest static stage projection error | 1.62156e-5 |

Standalone creation reports 24 pipelines and 13 persistent descriptor sets;
eleven Concept model interfaces use descriptor pushes. The compound portfolio
reports 63 pipelines, 39 descriptor sets, and 24 live Concept model pipelines.

## Qualification

The focused Normal GPU witness passes with Vulkan synchronization validation
on an NVIDIA GeForce RTX 3070, driver 610.88.
Three fresh Normal emissions match for all six model-block artifacts: C,
generated/public headers, manifest, source map and MIR. Verify C compiles with
`clang -std=c11 -pedantic-errors`; Normal and Verify ABI headers are identical.
For GPU parity, the Verify model-block object replaces `006.o` in a copied
consumer C archive, linked with unchanged native companions. This tests the
changed module in Verify without claiming every dependency was rebuilt in that
mode. All **five** focused Verify model tests pass, including the new Context
witness, the independent Noise tail/prefix witnesses, ownership recovery and
the retained real-weight AdaLN witness. Log: `.tmp/m7-verify-native.log`.

Final regression evidence:

- The canonical Windows native launcher passes in 144.637 s. MSVC 14.51.36231
  builds the common host; the manifest archives use Clang C11/C++23. Existing
  native warnings remain. Log: `out/test-artifacts/prometheus_native_windows_build.log`.
- All **31 selected Normal GPU tests pass**, zero skips/failures and no VUID,
  synchronization hazard or validation error. Both
  `PROMETHEUS_VK_VALIDATION=1` and `VK_LAYER_VALIDATE_SYNC=1` are enabled. The
  lane includes the new Context case, all previous thirty M6 regressions, shader
  package admission, resident model recovery, M2C invalid-input guards,
  reductions, RMSNorm, attention, RoPE and FR-M0. Logs and counts:
  `out/test-artifacts/prometheus_concept_context_m7/` and `.tmp/m7-final-native.log`.
- Fresh deployment/provenance checks, native manifest parity and SDSL-V workspace
  ownership checks pass. Targeted Go tests and vet pass for `internal/prometheus`
  and the owning generation/workspace tools. The SDSL-V module corpus passes
  all three cases through the real native test host. Logs:
  `.tmp/m7-check-{generation,manifest,workspace,go,vet,modules}.log`.
- Three fresh emissions match all six artifacts. This bounded sample fits the
  application-only stage/transport change; compiler and shared-library behavior
  are unchanged. Artifacts: `.tmp/m7-emission-{1,2,3,verify}/`.
- Both repositories pass `git diff --check`. Concept has no source changes;
  all generated C/provenance, native binaries and test logs remain ignored.
  Oct base is `99570e15`; Concept compiler/library revision is `b3cc832`.
  FEEDBACK has 121 unique IDs: 7 open, 11 deferred, 102 resolved, 1 superseded.

The ordinary reproduction path is:

```powershell
cmd /d /c internal\prometheus\native\build_windows_launcher.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
$env:VK_LAYER_VALIDATE_SYNC = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptContextRefinerHasIndependentNonzeroWitnessesAndSharedPortfolios
go run ./tools/prometheus_concept -check
go run ./tools/prometheus_native_manifest -check
go run ./tools/sdslv_workspace_check
go test ./internal/prometheus ./tools/prometheus_concept ./tools/prometheus_native_manifest ./tools/sdslv_workspace_check ./internal/prometheus/shaderpackage -count=1
go vet ./tools/prometheus_concept ./tools/prometheus_native_manifest ./tools/sdslv_workspace_check ./internal/prometheus/...
go run ./cmd/oct sdslv test Language/SDSL-V/Modules
```

A fresh full Concept checkpoint and frozen-Zig lanes were not run because
neither compiler nor shared libraries changed. No full-project green claim is
carried forward from earlier milestones.

The original full O19 captured image input remains absent, as documented in
M5/M6 and `docs/EVT2_LOCAL_PAYLOADS.md`. This independent Context witness does
not replace any original model-capture hash gate. Original real-weight Context
replay, full-model/prefetch execution, Linux, experimental MainTransformer
kernels and actual device loss remain outside this milestone's qualification.

## Next slice

MainTransformer's ordinary GPU recorder and package pipelines are the next
consumer boundary. Preserve its selected attention route, retained joint-stream
generation contract and experimental-route boundaries while moving the default
execution onto the existing Concept owner. A second scheduler or shader copy is
unnecessary.
