# Prometheus Concept session streams — M9

Status: **MILESTONE VERIFIED** for the bounded session/stream port on Windows
Vulkan. NoiseRefiner, ContextRefiner and MainTransformer now share a Concept-owned
stream lifetime and publication protocol through the existing production API.

## Ownership and architecture

`PrometheusModelBlock.concept` adds one `SessionOwner` with three owned buffers:
PreparedImage (15,728,640 bytes), PreparedContext (491,520 bytes), and JointWorking
(16,220,160 bytes). It owns capture/composition recordings, pending submissions,
stream generations, producer identities and joint provenance. `StreamTransfer`
is a tagged capture/composition plan. Joint publication uses checked
begin/commit/abort transitions around Main's borrowed joint write.

C retains lock/profile/producer admission, the closed retarget sequence,
weight-window/prefetch policy, model-output counters and public evidence/replay
transport. Its stream packets are read-only projections of Concept-owned
resources and versions. The old C session buffer construction, command buffer,
fence, capture/composition recorder and submit/wait executor are deleted.
The existing C++23 package adapter remains useful and unchanged.

This is an intent port using existing Concept/Vulkan operations. No compiler,
shared-library or language thaw was needed. The three existing stream buffers
are transferred to Concept ownership; no additional persistent buffer or
full-size activation scratch is introduced. Capture/composition and their
retries allocate no new buffers. Recording/submission resources still use the
existing Concept library lifecycle; persistent command/fence reuse and a
performance improvement are not claimed.

## Publication and retirement

- Recording or injected queue-submit failure occurs before submission and keeps
  the previous stream valid. It advances neither generation nor composition count.
- Immediately before a real submission, the destination becomes invalid.
  Unobserved completion retains owned pending work and cannot publish a version.
- Draining waits for safe reuse/release. It does not retroactively publish an
  unobserved copy. A successful fresh operation publishes exactly one version.
- Joint validity is derived from current valid Image/Context versions and the
  versions used for composition. A new capture invalidates an older joint's
  provenance. Reset invalidates all streams while preserving monotonic versions.
- Main chain publication commits only after completed work and successful
  measurement resolution. A known pre-submit abort restores the old joint;
  uncertain execution leaves it invalid. Old generations cannot replay.
- Session capture borrows model output; Main chain execution borrows the session
  joint. Model retirement/cleanup drains session work, and session destruction
  retires model work. Both orders discharge submitted borrows before freeing
  their buffers. Ordinary public calls remain externally serialized.

FB-125 records the removed C executor's conflation of rejected submission and
uncertain completion, plus stale destination validity and construction diagnostic
loss. FB-126 records the cross-owner retirement dependency. Both are resolved
with explicit native failure tests. The old unsignaled-fence hang and a GPU
freed-buffer access were inspection findings, not deliberately reproduced faults.

## Real-path witnesses

The existing Main witness now injects recording, submission and completion
failures into Image capture, overwrite of a valid Context stream, and overwrite
of a valid joint. It checks phase-specific diagnostics, quarantine, unchanged
failed-operation generations, exact successful-publication increments, rejection
of uncertain bytes after drain, stale-source rejection and buffer reuse.

The same test still executes the actual Noise0 → Noise1 capture → Context0 →
Context1 capture → joint composition → Main chain path. It covers Serial,
SubgroupOwned32 and Auto, compound and standalone Main owners, two successive
layers, independent CPU contractions, static audit, failure recovery and legacy
SGEMM coexistence. All 4,055,040 Main elements agree with the independent
reference: maximum absolute error 4.06713e-6 for the first layer and 4.92126e-6
for the successor in both Normal and Verify runs. No numerical tolerance changed.

The Context witness retains an unobserved capture while destroying the model
first. The Main witness retains an unobserved chain submission while destroying
the session first. Both leave zero pending submissions; buffers/pipelines return
to baseline. The standalone partial-construction fact fails after each of three
buffer acquisitions, proves buffer/context cleanup and no double destruction,
and successfully creates/destroys a new session on the same device after each.
Compound reset also preserves all stream generations while rejecting old
publication inputs.

## Qualification

The compiler/library checkout is unchanged at Concept `b3cc8320c52eb7ce`;
Oct started at `0bf07c9814192ab5`. GPU: NVIDIA GeForce RTX 3070, driver 610.88;
Vulkan SDK 1.4.350.0, MSVC 14.51.36231 and Clang 22.1.3.

- Three fresh emissions match all six C/header/manifest/map/MIR artifacts in
  `.tmp/m9-emission-{1,2,3}/`. Normal and Verify generated/public headers match.
  Both emitted C files compile with `clang -std=c11 -pedantic-errors`.
- All **seven focused Normal** model/session cases pass, zero skips/failures,
  with `PROMETHEUS_VK_VALIDATION=1` and `VK_LAYER_VALIDATE_SYNC=1`.
  Log: `.tmp/m9-focused-final.log`.
- All **seven focused Verify** cases pass under the same validation settings,
  with no VUID, synchronization hazard or validation error. The Verify
  model/session object replaces `006.o` in a copied consumer archive; companions
  retain their Normal build. This qualifies the changed module, not a Verify
  rebuild of every dependency. Log: `.tmp/m9-verify-native.log`.

## Final regression closeout

- The canonical Windows launcher passes in **150.164 s**, building the DLL,
  normal/slow native harnesses and benchmark host. Existing native warning
  categories remain C4996, C4505, C4244 and C4456. Log:
  `out/test-artifacts/prometheus_native_windows_build.log`.
- The **35-case Normal GPU lane passes**, zero skips/failures and no VUID,
  synchronization hazard or validation error. It includes all previous 34 M8
  cases plus partial session construction, with expanded capture/composition,
  reset and both cross-owner teardown checks in the existing model witnesses.
  Counts/logs: `out/test-artifacts/prometheus_concept_session_m9/` and
  `.tmp/m9-final-native.log`.
- All four explicit M5b/M6a research C configurations compile after the private
  session-state change. Their GPU execution remains unqualified.
  Log: `.tmp/m9-experiments-compile.log`.
- Fresh generation/deployment provenance, native manifest parity and SDSL-V
  workspace checks pass. Targeted Go tests and vet pass for Prometheus and the
  owning generation/manifest/workspace/package tools. All three SDSL-V module
  corpus cases pass through the real shader host. Logs:
  `.tmp/m9-check-{generation,manifest,workspace,go,vet,modules}.log`.
- Both repositories pass `git diff --check`; Concept is unchanged. The issue
  tracker has 126 unique IDs: 7 open, 11 deferred, 107 resolved and 1 superseded.
  New resolved entries are newest first and the next identifier is FB-127.

## Reproduction and limits

From the Oct checkout:

```powershell
go run ./tools/prometheus_concept -build-package
cmd /d /c internal\prometheus\native\build_windows_launcher.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
$env:VK_LAYER_VALIDATE_SYNC = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptSessionUnwindsEveryPartialConstruction
out/prometheus/native/marionette_tests.exe PrometheusConceptContextRefinerHasIndependentNonzeroWitnessesAndSharedPortfolios
out/prometheus/native/marionette_tests.exe PrometheusConceptMainTransformerRunsIndependentJointWitnessesAndChains
```

Generated C, headers, JSON/provenance, archives, binaries and logs remain ignored
build products. The Concept sibling has no source changes. Full Oct/Concept Go
and frozen Zig suites were skipped for this application-only change; the focused
Go, native execution and compiler-output checks above cover the affected seams.

The original full Z-Image payload/model, prefetch GPU execution, Linux/native
platform parity, actual device-loss behavior and experimental Main GPU routes
remain unqualified. No throughput/latency comparison or whole-library C migration
is claimed. Remaining handwritten C policy and other reactor families retain
their own migration and qualification boundaries.
