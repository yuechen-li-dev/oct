# Prometheus Concept model tail — M6

Status: **MILESTONE VERIFIED** for standalone NoiseRefiner attention/FFN on
Windows Vulkan. This is a bounded consumer migration, not a project-wide
qualification or a performance claim.

## Purpose and boundary

This is an intent port for maintainability. Handwritten Vulkan C had become
difficult to extend and keep correct even with substantial tooling and frontier
LLM assistance. Concept supplies typed operations and ownership; Concept-shaped
C++23 remains useful for host adapters and existing integrations. Success means
a clearer implementation that works through the production path, rather than
eliminating every foreign-language line.

M6 completes the standalone NoiseRefiner attention and FFN execution in
`internal/prometheus/reactors/PrometheusModelBlock.concept`. It retains the
public ABI, admission policy, generation bookkeeping, and larger model/session
executors. ContextRefiner and MainTransformer execution are later slices.

## Implementation

- The existing owner now holds all thirteen production model pipelines. Its
  fourteen slots include the separate closed-block identity pipeline. The same
  borrowed Vulkan device and owned buffer portfolio serve every stage.
- A closed `BlockStage` and short fixed schedules express attention aggregation,
  projection, residual, FFN normalization, hidden projections, gating, and output.
  Typed capture plans select producer boundaries before storage is overwritten.
  The three physical W3 fragments are concatenated into the requested audit view.
- Static audit replay borrows these exact Concept stage implementations through
  an opaque export. Its numeric transport roles are decoded by Concept; C no
  longer constructs private compiler enum layouts.
- The old C attention/FFN recorders and their pipeline factories are deleted.
  Thirteen legacy audit descriptor sets remain; the model interfaces use Concept
  descriptor pushes. The cold portfolio still creates twenty-six pipelines in
  total, including the legacy audit projections.
- Resident timing now sums the Concept recordings that actually ran. It no
  longer waits for unissued queries in the old C pool. Uninstrumented per-submit
  and per-fence CPU fields are cleared instead of carrying stale upload timings.

This slice does not add persistent recording/fence reuse or claim a speedup.

## Evidence

The initial canonical Windows native build and the existing 29-test selected
GPU lane pass with synchronization validation enabled, zero skips and no Vulkan
validation findings. The new independent nonzero attention/FFN witness also
passes in Normal and Verify model-block output on an NVIDIA GeForce RTX 3070
(driver 610.88). Its maximum absolute errors are:

| Boundary | Elements compared | Maximum absolute error |
| --- | ---: | ---: |
| Attention | 3,932,160 | 1.28602e-6 |
| Attention projection | 3,932,160 | 3.21506e-7 |
| Attention residual | 3,932,160 | 1.67718e-7 |
| FFN norm | 3,932,160 | 2.97806e-5 |
| FFN modulated | 3,932,160 | 4.45517e-5 |
| W1 | 10,485,760 | 4.45517e-5 |
| Split W3 | 10,485,760 | 4.41664e-5 |
| Gated hidden | 10,485,760 | 5.11641e-5 |
| W2 | 3,932,160 | 2.48413e-5 |
| Final output | 3,932,160 | 1.17838e-5 |
| Block-1 resident final, two replays | 3,932,160 each | 7.40471e-6 |

The witness uses sparse FP16 weights, five BF16 input row classes, and an
independent double-precision closed-form oracle. It compares every element of
all ten tail boundaries, distinguishes all three physical W3 fragments, checks
static-audit projection packets, exercises resident replay and its GPU timings,
and injects record/submit/completion failures for both attention and FFN with
recovery and legacy SGEMM on the shared device. The resident run performs the
required block-1 rebind before consuming its predecessor's FP32 boundary. The
bound is 3e-4 absolute, with no nonfinite outputs accepted. Static tail audit
packets retain their stage IDs, finite counts, projection keys and values.
Teardown returns Concept buffer/pipeline live counts to baseline.

Three fresh Normal emissions produce identical C, generated/public headers,
manifest, map and MIR. This small sample fits an application-only schedule and
capture change; the compiler and shared Vulkan library are unchanged. Verify C
compiles with `clang -std=c11 -pedantic-errors`. For its native witness, the
Verify object replaces only `006.o` (the model-block module) in a copied C
archive; the production native objects and companion archives stay identical.
Normal and Verify headers are byte-identical and both native witnesses pass the
same oracle and failure matrix. This qualifies the changed module, rather than
claiming every dependency was rebuilt in Verify mode.

Evidence: `.tmp/m6-nonzero-rebound.log`, `.tmp/m6-verify-nonzero.log`,
`.tmp/m6-emission-{1,2,3,verify}/`, and the final regression logs below. The first
new host test did not compile because its near assertion mixed float and double;
the next run exposed its omitted required rebind. Both harness errors were
corrected without weakening production admission or numerical checks. Failed
logs remain in `.tmp/m6-native-build-with-oracle.log` and `.tmp/m6-nonzero.log`.
All generated C, headers, native binaries and test outputs remain ignored build
products. Oct base: `7c8e84cc`; Concept compiler/library: `b3cc832`.

The original O19 captured image input remains absent. The synthetic nonzero
witness and retained real-weight AdaLN witness do not replace its pinned capture
hash gate. See `docs/EVT2_LOCAL_PAYLOADS.md` and the M5 report for that boundary.

## Final regression closeout

- The canonical Windows launcher passes in 137.592 s. The host uses MSVC
  14.51.36231; the manifest archives use Clang C11/C++23. Existing native warnings
  remain. Log: `out/test-artifacts/prometheus_native_windows_build.log`.
- All **30 selected GPU tests pass**, zero skipped or failed, with
  `PROMETHEUS_VK_VALIDATION=1` and `VK_LAYER_VALIDATE_SYNC=1`; no VUID, validation
  error or synchronization hazard appears. This includes model ownership,
  replacement/recovery, resident models, BF16 ingress, shader packages, M2C
  admission, reductions, RMSNorm, attention, RoPE and FR-M0 regressions. Logs:
  `out/test-artifacts/prometheus_concept_model_block_m6/` and
  `.tmp/m6-final-native.log`. The real-weight AdaLN witness's maximum absolute
  difference is zero.
- Fresh `prometheus_concept -check`, native manifest parity, SDSL-V workspace
  ownership, targeted Go tests and vet pass. The SDSL-V module corpus passes all
  three cases through the actual native shader test host. Logs:
  `.tmp/m6-check-{generation,manifest,workspace,go,vet,modules}.log`.
- Both workspaces pass `git diff --check`; Concept has no source changes.
  Generated C/provenance, binaries, payloads and test logs remain ignored.
  FB-119 records the repaired resident timing defect. The tracker has 119 unique
  IDs: 7 open, 11 deferred, 100 resolved, 1 superseded.

The compiler and shared library were unchanged, so a new full Concept Go/native
checkpoint and frozen-Zig lanes were not run. Linux, actual device loss,
experimental kernels, ContextRefiner/MainTransformer GPU execution and original
O19 full-capture equivalence remain outside this milestone's evidence.

The normal reproduction entry points are:

```powershell
cmd /d /c internal\prometheus\native\build_windows_launcher.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
$env:VK_LAYER_VALIDATE_SYNC = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptModelBlockAttentionAndFfnHaveIndependentNonzeroWitnesses
go run ./tools/prometheus_concept -check
go run ./tools/prometheus_native_manifest -check
go run ./tools/sdslv_workspace_check
go test ./internal/prometheus ./tools/prometheus_concept ./tools/prometheus_native_manifest ./tools/sdslv_workspace_check ./internal/prometheus/shaderpackage -count=1
go vet ./tools/prometheus_concept ./tools/prometheus_native_manifest ./tools/sdslv_workspace_check ./internal/prometheus/...
go run ./cmd/oct sdslv test Language/SDSL-V/Modules
```

The standalone nonzero witness needs no external weights. For the real AdaLN
case in the wider `PrometheusConceptModelBlock` filter, set
`OCT_PROMETHEUS_M5_ADALN_REAL=1`, `OCT_EVT2_CACHE` and `OCT_EVT2_ORACLE` using the
validated local payload setup described in M5.

## Next slice

Move the model/session execution that still owns handwritten ContextRefiner and
MainTransformer Vulkan recordings when its producer and generation boundaries
can be proved. Keep the useful public/package adapters and use the existing
Concept owner and libraries instead of adding another scheduler or shader path.
