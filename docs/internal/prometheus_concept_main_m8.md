# Prometheus Concept MainTransformer — M8

Status: **MILESTONE VERIFIED** for production MainTransformer in standalone and
compound owners on Windows Vulkan. This is a bounded execution/ownership port,
not full-model, cross-platform or performance qualification.

## Scope and ownership

Production MainTransformer pipeline ownership and GPU execution now use the
existing Concept model owner. Three disjoint fourteen-slot banks retain thirteen
Noise, eleven Context and thirteen Main pipelines together. Main also works in
a standalone owner while the session retains its prepared streams.

`BlockDomain` replaces the old two-family boolean. Normalization, projection,
residual, attention binding and FFN stages share one implementation across the
three token shapes. Main adds timestep widening, its 24-byte mixed-position
Q/K contract, and a fixed thirteen-stage schedule. It copies the lock-owned
joint into working input and, in chain mode, publishes the final candidate back
to JointWorking before completion. The C adapter retains admission, package
selection, rebinding, generation bookkeeping and explicit audit readback.

This is an intent port for maintainability. Existing Concept/Vulkan operations
were sufficient; no compiler or shared-library changes were needed. The native
C++23 package adapter remains useful and unchanged. Production does not fall
back to a C executor.

## Attention and research boundaries

Production route selection still owns admission. Serial uses shader 41 and
31,680 workgroups; subgroup-owned routes use the admitted production payload
and 3,960 workgroups. The loader validates the selected shader/interface/group
combination before constructing owned pipelines. The existing API maps forced
subgroup and supported Auto to builtin-topology shader 49; shader 47 remains an
admitted production interface. This milestone executes 41 and 49 on the GPU.

The Main RoPE reference deliberately preserves the pinned source's sequential
even/odd write behavior. Its odd lane observes the rotated even lane. Context's
independent pair rotation is a different contract; the port does not silently
replace Main's numerical authority with it.

The old research factory/recorder is isolated in
`internal/prometheus/native/reactor_main_experiments.inc`. Only explicit M5b
subgroup/Gemini or M6a cooperative build flags include it. Those four C
configurations compile; their GPU numerics and performance were not qualified.
They preserve the existing research allocation path and are never automatic
fallbacks from the production Concept executor.

## Protocol repairs

- **FB-122:** Main's quarantine check ran before retirement. Non-chain retries
  now reach the existing owner/reap path and observe completion before reuse.
- **FB-123:** A failed completion observation could leave changed joint bytes
  valid under an old generation. Uncertain chain publication now invalidates the
  joint. Explicit recomposition creates a fresh input generation. Post-compute
  measurement failure also invalidates a chain that cannot commit publication.
  Arbitrary later-layer rollback or automatic restart is not claimed.
- **FB-124:** Static replay retained the preceding execution's GPU timestamps.
  It now resolves its own recording before readback. Query ownership stays with
  the Concept submission; C projects the existing public raw-tick/nanosecond ABI
  through a short-lived borrowed query-pool handle. The old C query pool is not
  used as a proxy for production Concept work.

The first numerical witness failed precisely at completion retry. After that
repair, all three policies exposed stale static timestamps and accepted reuse
after uncertain chain publication: six assertions failed. Failed logs remain
in `.tmp/m8-main-{first,recovery}.log`. The repaired witness passes in
`.tmp/m8-main-publication.log`.

## Independent witness

`PrometheusConceptMainTransformerRunsIndependentJointWitnessesAndChains` needs
no external payload. It prepares image and context through the actual Noise and
Context producer APIs, captures them in the retained session slots, and composes
the joint in Image-then-Context order. Sparse nonzero FP16 weights and a nonzero
BF16 timestep then exercise widening, AdaLN, mixed image/text RoPE, full softmax
attention, residuals and SiLU-gated FFN.

An independent double-precision CPU reference computes all 1,056 token rows.
Every one of the **4,055,040 final elements** is compared on serial, forced
subgroup and Auto policies, including standalone Main and the compound owner.
The absolute bound is 2e-4 with no nonfinite values accepted. Observed maximum
absolute errors are 4.06713e-6 for the first layer and 4.92126e-6 for the second
layer computed from the observed first-layer boundary. All policies give the
same reported maxima.

The witness also checks thirteen nonzero GPU stage intervals and their shared
raw timestamp boundaries, stale retained-generation rejection, record/submit/
completion failure diagnostics, output invalidation, legacy SGEMM coexistence,
explicit recomposition after uncertain chain publication, successor rebinding,
two-layer GPU publication and exact resource-counter teardown. Warm computation
and recovery allocate no replacement buffers/pipelines or weight uploads.

Standalone Main has thirteen Concept pipelines and **zero persistent descriptor
sets**. The compound owner has 63 total pipelines, of which 37 are Concept-owned,
and 26 remaining legacy audit descriptor sets (down from 39 in M7). No persistent
buffer is added and the M7 memory reservation remains unchanged.

## Timing and qualification limits

Concept timestamps use its existing `ALL_COMMANDS` completion scope. The sixteen
boundaries retain the public transfer/compute/stage/publication roles, raw ticks
and nanoseconds. Unmeasured legacy CPU reset/begin/record/end/submit/wait fields
are cleared on ordinary Main execution. This is not a performance comparison
with the old narrower C timestamp-stage masks.

Normal numerical/lifecycle execution passes with synchronization validation.
Three fresh emissions match all six model-block artifacts. Both Normal and
Verify C compile with `clang -std=c11 -pedantic-errors`; their generated and
public ABI headers are identical. All six focused Verify GPU tests pass with
zero skips, failures or validation findings, including the new Main witness,
the Context shared-owner witness and the retained Noise proofs. The Verify
model-block object replaces `006.o` in a copied consumer archive and links with
the same native companions; this qualifies the changed module without claiming
every dependency was rebuilt in Verify. Log: `.tmp/m8-verify-native.log`.

The three fresh generations match all six C/header/manifest/map/MIR artifacts
in `.tmp/m8-emission-{1,2,3}/`. This sample fits an application-only change with
unchanged compiler and shared libraries. The research compile checks are in
`.tmp/m8-experiments-compile.log`; default preprocessing excludes both legacy
research functions (`.tmp/m8-default-preprocessed.i`).

## Final regression closeout

- The canonical Windows native launcher passes in 137.488 s. MSVC 14.51.36231
  builds the common host; the consumer archives use Clang C11/C++23. Existing
  native warnings remain. Log: `out/test-artifacts/prometheus_native_windows_build.log`.
- The **34-test Normal native GPU lane passes**, zero skips/failures and no VUID,
  synchronization hazard or validation error. Both
  `PROMETHEUS_VK_VALIDATION=1` and `VK_LAYER_VALIDATE_SYNC=1` are enabled on an
  NVIDIA GeForce RTX 3070, driver 610.88. This includes the previous 31 M7 cases,
  the Main witness, strict subgroup/Auto selection and ordinary-process forced
  route rejection. Logs and counts:
  `out/test-artifacts/prometheus_concept_main_m8/` and `.tmp/m8-final-native.log`.
- All six focused Verify model cases pass under the same validation settings.
  The first-layer and second-layer numerical maxima remain 4.06713e-6 and
  4.92126e-6 in both modes. The retained real-weight AdaLN witness is exact.
- Fresh deployment/provenance verification, native manifest parity and SDSL-V
  workspace checks pass. Targeted Go tests and vet pass for Prometheus and its
  owning generation/workspace tools. All three SDSL-V module corpus cases pass
  through the real native shader host. Logs:
  `.tmp/m8-check-{generation,manifest,workspace,go,vet,modules}.log`.
- Both repositories pass `git diff --check`. Concept has no source changes.
  Generated C, private/public headers, provenance, binaries and logs remain
  ignored build products. The handwritten research include is ordinary source.
  Oct base: `9c34fcfb`; Concept compiler/library: `b3cc832`.
- FEEDBACK has 124 unique IDs: 7 open, 11 deferred, 105 resolved, 1 superseded.
  FB-122, FB-123 and FB-124 close the reproduced protocol defects above.

From the Oct checkout, the focused reproduction path is:

```powershell
cmd /d /c internal\prometheus\native\build_windows_launcher.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
$env:VK_LAYER_VALIDATE_SYNC = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptMainTransformerRunsIndependentJointWitnessesAndChains
out/prometheus/native/marionette_tests.exe PrometheusSubgroupOwned32RouteSelectionIsStrictAndFallsBackOnlyForAuto
out/prometheus/native/marionette_tests.exe PrometheusForcedAttentionRouteRejectionIsAnOrdinaryProcessFailure
go run ./tools/prometheus_concept -check
go run ./tools/prometheus_native_manifest -check
go run ./tools/sdslv_workspace_check
go test ./internal/prometheus ./tools/prometheus_concept ./tools/prometheus_native_manifest ./tools/sdslv_workspace_check ./internal/prometheus/shaderpackage -count=1
go vet ./tools/prometheus_concept ./tools/prometheus_native_manifest ./tools/sdslv_workspace_check ./internal/prometheus/...
go run ./cmd/oct sdslv test Language/SDSL-V/Modules
```

No fresh full Concept checkpoint or frozen-Zig lane was run: neither compiler
nor shared libraries changed. No earlier project-wide qualification is inherited.

Full original-capture/model evaluation, prefetch GPU execution, arbitrary
later-layer recovery, actual device loss, Linux and research-kernel performance
remain outside this milestone's evidence. Independent sparse contractions do
not replace the pinned payload hash gates in `docs/EVT2_LOCAL_PAYLOADS.md`.

## Next boundary

The remaining production C in the model-block layer is chiefly model/session policy, retained stream
capture/composition, weight-window retargeting/prefetch and explicit audit
transport. Move those protocols in bounded slices where Concept clarifies their
ownership and failure transitions. Keep useful host/package adapters rather
than replacing them to meet a language-purity or line-count target.
Other native reactor families, including SGEMM and FFT, remain separate porting
work; this milestone does not claim that all of libprometheus has migrated.
