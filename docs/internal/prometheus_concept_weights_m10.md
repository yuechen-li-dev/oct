# Prometheus Concept synchronous weights — M10

Status: **MILESTONE VERIFIED** for the bounded synchronous binding port on Windows Vulkan.

## Scope and architecture

Concept now owns one synchronous weight-binding lifecycle for initial upload,
standalone Noise/Context/Main replacement, and minimum-memory compiled-model
retarget. `WeightChange` distinguishes Initial, Replacement and InPlace;
`WeightPhase` records Empty, Bound, Uploading, Ready, Failed and Uncertain.
The binding holds active/candidate identities, physical slot mappings, completed
transfer count, validity and binding/descriptor generations. The existing block
owner retains the actual buffers and submissions.

C validates public packets, pinned lock/descriptor sizes and the closed model
transition sequence. It stages caller bytes into the existing mapped upload
buffer and adapts legacy audit descriptor packets. Every family calls the same
synchronous helper; Concept checks transfer ordering, submits GPU copies,
controls readiness, authorizes commit and retires failed candidates. Private
Concept types remain inside C11 transport; the useful C++23 shader-package and
host adapters are unchanged.

The old C synchronous upload recorder, native pending-weight owner and three
duplicated candidate allocation/upload/swap/cleanup implementations are removed.
The physical Context slot map has one C admission authority and is passed into
the binding plan. Model buffers, pipelines and prepared session streams continue
to use the M5–M9 Concept owners.

No compiler/library change was needed. The application uses explicit read-only
borrows for candidate metadata and Option for absent versions. Public native
packets use concrete referenced records rather than exposing internal views.

## State and failure contracts

- Buffer creation alone publishes no weight binding. The first complete initial
  upload commits generation 1. A partial initial set is not executable.
- Replacement candidates are separate owned buffers. Failure preserves the old
  active set and its generations. The public Bound state describes that usable
  active set; the operation's detail code reports its failed transaction phase.
- In-place retarget reuses the existing physical window. The first submitted
  overwrite invalidates the old set. Partial upload or descriptor failure cannot
  restore validity by draining; a fresh complete transfer repairs the same closed
  transition. No second weight window is allocated for this mode.
- Only completed transfers make a transaction Ready. Native descriptor admission
  occurs before the serialized commit. Only commit advances binding and
  descriptor generations and replaces active identities. Failed attempts do not
  advance the session's retarget position.
- Known-complete failed candidates are released immediately. Unobserved submitted
  candidates and staging resources remain owned until retirement. Their retained
  candidate bytes are included in native memory evidence. Retirement discards
  failed candidates; it never publishes an unobserved transaction.
- Initial upload and Context/Main execution now retire pending work before testing
  cached binding/quarantine state. Candidate failure therefore cannot permanently
  prevent repair or execution of an intact old binding.

Native descriptor updates are a small C integration seam over already completed
candidates. They are not a second owner of binding validity or generation. Host
calls remain externally serialized. Device loss is terminal in the existing
owner; actual device-loss recovery is not qualified here.

## Executable witnesses

`PrometheusConceptWeightsInitialPartialUploadsCannotExecuteAndRetryCleanly`
fails after three completed transfers in a small real closed program. Record,
submit and completion faults leave an incomplete non-executable set. A fresh
complete upload reuses every buffer, executes through the GPU path, and returns
all buffers/submissions to baseline at teardown.

The existing independent Noise witness now uploads replacement bias bytes that
differ from the active set, then injects record, submit, completion and descriptor
failures. Generations stay unchanged. After retirement, all 3,932,160 attention
residual elements still match the old independent reference (maximum absolute
error 1.67718e-7). Complete replacement then publishes normally. The Context and
Main witnesses also prove old-binding execution and independent numerics after
completion/descriptor failure, then execute their rebound successors.

The minimum-memory Main-chain witness injects the same four failures into the
first Noise retarget after three earlier tensors were overwritten. It rejects
execution of the incomplete binding, preserves both generations and allocates
no new buffers. Full retry repairs the transition and continues the actual
Noise0 → Noise1 → Context0 → Context1 → joint composition → Main chain. The
existing Serial, SubgroupOwned32 and Auto paths, standalone/shared owners, two
Main layers, session failure matrix, reset, both teardown orders and SGEMM
coexistence remain in the regression lane.

The first expanded run failed four assertions because Context/Main tested cached
quarantine before retirement. That failure remains in `.tmp/m10-fault-native.log`.
The corrected entry points pass their focused reruns in
`.tmp/m10-recovery-PrometheusConcept{ContextRefinerHasIndependentNonzeroWitnessesAndSharedPortfolios,MainTransformerRunsIndependentJointWitnessesAndChains}.log`.
FB-127 and FB-128 record the repaired replacement and in-place/initial protocols.

## Qualification and limits

Oct base: `90753116`; Concept compiler/library: unchanged at `b3cc8320c52eb7ce`.
The target remains Windows AMD64, NVIDIA GeForce RTX 3070, driver 610.88,
Vulkan SDK 1.4.350.0, MSVC 14.51.36231 and Clang 22.1.3. Validation runs enable
`PROMETHEUS_VK_VALIDATION=1` and `VK_LAYER_VALIDATE_SYNC=1`.

- The canonical native launcher passes in **148.497 s**, building the DLL,
  normal/slow test harnesses and benchmark host. Existing warning categories
  remain C4996, C4505, C4244 and C4456; no new category was introduced. Log:
  `out/test-artifacts/prometheus_native_windows_build.log`.
- **36 Normal native GPU cases pass**, zero skips/failures and no VUID,
  synchronization hazard or validation error. This includes the previous 35 M9
  cases plus the new partial-initial-upload fact, with replacement/retarget
  failure matrices added to the existing independent witnesses. Logs/counts:
  `out/test-artifacts/prometheus_concept_weights_m10/` and `.tmp/m10-final-native.log`.
- **Eight focused Verify model/weight/session cases pass** under the same
  validation settings, zero skips/failures/findings. The changed model/weights
  module replaces `006.o` in a copied consumer archive; its companions remain
  Normal. This is not a Verify rebuild of every dependency. Log:
  `.tmp/m10-verify-native.log`.
- The Main witness still checks all 4,055,040 output elements, with first- and
  second-layer maximum absolute errors 4.06713e-6 and 4.92126e-6 in Normal and
  Verify. No oracle or numerical tolerance was weakened. The retained real-weight
  AdaLN witness also remains exact.
- Three fresh emissions match all six C/header/manifest/map/MIR artifacts.
  Normal and Verify generated/public ABI headers match. Both emitted C files
  compile with `clang -std=c11 -pedantic-errors`. Logs/products:
  `.tmp/m10-emission-final.log` and `.tmp/m10-emission-{1,2,3,verify}/`.
- All four explicit research C configurations compile. Their GPU behavior is
  unqualified. Log: `.tmp/m10-experiments-compile.log`.
- Generation/deployment provenance, native manifest parity, workspace checks,
  targeted Go tests and vet pass. All three SDSL-V module corpus cases pass
  through the actual shader host. Logs:
  `.tmp/m10-check-{generation,manifest,workspace,go,vet,modules}.log`.
- Both repositories pass `git diff --check`; Concept has no changes. The tracker
  has 128 unique IDs: 7 open, 11 deferred, 109 resolved and 1 superseded. FB-127
  and FB-128 are resolved; the next identifier is FB-129.

From the Oct checkout, a self-contained native reproduction is:

```powershell
go run ./tools/prometheus_concept -build-package
cmd /d /c internal\prometheus\native\build_windows_launcher.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
$env:VK_LAYER_VALIDATE_SYNC = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptWeightsInitialPartialUploadsCannotExecuteAndRetryCleanly
out/prometheus/native/marionette_tests.exe PrometheusConceptContextRefinerHasIndependentNonzeroWitnessesAndSharedPortfolios
out/prometheus/native/marionette_tests.exe PrometheusConceptMainTransformerRunsIndependentJointWitnessesAndChains
```

Asynchronous prefetch retains its C transfer queue/window owner. At its existing
activation seam, a completed borrowed binding view is imported into Concept;
foreign window allocations are not adopted or double-destroyed. M11 can replace
that transfer/acquire protocol using the same binding authority. This milestone
makes no claim about actual prefetch GPU execution or queue overlap.

The existing replacement policy still temporarily needs a candidate set; the
minimum-memory in-place path does not acquire one. No throughput, latency,
whole-model, Linux, noncoherent-heap or experimental Main GPU qualification is
claimed. The original full payload/model remains outside this bounded witness.
Generated C/headers, JSON/provenance, archives, binaries and logs stay ignored.
Full Oct/Concept Go and frozen Zig suites are skipped for this application-only
change; the affected generation, native execution, targeted Go and SDSL-V lanes
are qualified below. The Concept sibling has no source changes.
