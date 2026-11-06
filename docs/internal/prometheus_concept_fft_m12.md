# M12: Concept FFT and measured Oct dispatch

The forward `FFT()` builtin now shares one binary64 implementation contract
between interpreted and compiled execution. Ordinary calls calibrate planned
CPU and Vulkan calls. `PROMETHEUS` blocks require GPU completion for FFT and
matrix multiplication; they work outside benchmarks.

## Ownership and precision

`PrometheusFft.concept` owns reusable upload, device, ping/pong, readback, and
twiddle buffers, package pipelines, recordings, submissions, completion, and
retirement. The old C executor is deleted. Its request validation, legacy plan
diagnostics, and public Complex32 ABI remain compatibility seams. The C++23
adapter attaches an owner to the existing runtime, serializes access, supplies
binary64 twiddles, and loads package pipelines. C11 only transports spans and
generated record layouts. The executor shares the existing device/queue with
SGEMM and uses its host submission synchronization.

Shader 70 uses SDSL-V `f64` scalar arrays and arithmetic. VD-MIR records the
`shader-float64` requirement; SPIR-V carries `Float64`. The runtime queries and
enables the optional core feature. FP64 inputs never pass through the legacy
Complex32 interface. Host-generated FP64 twiddles avoid an FP32 transcendental
conversion. The forward result remains unnormalized with negative exponential
sign. GPU shape admission is a nonempty power of two through 2^20.

The legacy normalized inverse scales only the final pass. The initial native
rerun caught an omitted normalization when the optional flag was absent; the
corrected implementation passes the independent DFT and round-trip corpus.

## Selection policy and costs

Each program owns one serialized engine with one current GPU storage shape.
Both CPU and GPU cache that shape's roots of unity. Three timing windows per
candidate last at least 10 ms and measure complete calls. This avoids zero
durations from individual small calls on the local Windows clock. A GPU median
must beat CPU by 20% and agree numerically before selection. Cold setup is
recorded separately. A shape change recalibrates; a forced call never runs CPU
calibration. Engines close with the interpreted or compiled program.

The first-call calibration cost is real and can dominate a small or one-shot
workload. This implementation selects for repeated calls of the current shape;
it does not promise a first-call acceleration or a portable size threshold.
Automatic GPU unavailability/failure selects CPU with an inspectable reason.
Forced unavailability/failure is an error. Nonfinite calibration outputs keep
automatic execution on CPU.
Hardware admission reuses the runtime's authoritative backend classification;
software Vulkan is rejected by both the native FFT adapter and strict Oct
adapters. The software/unavailable classification boundary has a Go unit test;
execution on a separate software Vulkan installation is not claimed.

## Local measurements

Windows AMD64, NVIDIA RTX 3070, driver 610.88, Vulkan SDK 1.4.350.0,
MSVC 14.51.36231 and Concept's Clang C11 consumer build. Local timing is
machine-specific. Reproduce using `go run ./tools/fft_measure`.

| Complex values | CPU warm | GPU full-call warm | Automatic choice |
| ---: | ---: | ---: | --- |
| 32 | 0.221 us | 79.022 us | CPU |
| 256 | 2.152 us | 92.583 us | CPU |
| 4,096 | 46.132 us | 147.194 us | CPU |
| 65,536 | 1.200 ms | 0.557 ms | Vulkan FP64 |
| 1,048,576 | 33.518 ms | 12.010 ms | Vulkan FP64 |

These are medians of aggregate full-call windows. Cold device/pipeline setup
was about 110–133 ms for the two larger shapes; total first automatic calls
were about 176 and 315 ms. At 2^20, maximum absolute difference from the Go
binary64 control was 1.30e-10 and maximum error scaled by peak spectrum was
2.63e-16. Retained raw evidence is `.tmp/m12-measurement.json`; generated
artifacts stay ignored. The report records an observed run, not a performance
guarantee.

A subsequent repeat (`.tmp/m12-measurement-final.json`) selected CPU at 65,536:
1.152 ms CPU versus 0.999 ms GPU did not clear the 20% margin. At 2^20 it again
selected Vulkan: 35.667 ms versus 14.311 ms (about 2.49x), with the same scaled
error. The boundary is sensitive to current load, which is why selection uses
measured costs and a margin. It is cached for the engine's current shape; it
does not continuously retune a long-running workload.

## Qualification

The Oct contracts live in `Language/Prometheus`, with separate ordinary,
GPU-required and explicitly missing-reactor environments. Go tests orchestrate
those files. Native tests compare against an independent direct DFT, exercise
low-bit preservation, single elements, repeated sizes, buffer reuse, wide
dynamic range, injected failures, and coexistence with SGEMM. The legacy FP32
suite retains its strided/batched/inverse/diagnostic checks.

Completed checks:

- Full fast `go test -count=1 -parallel 8 ./...` passed. Subsequent small engine
  and emitter cleanup passed the affected FFT/build package tests.
- Targeted `go vet` passed for the compiler/runtime/shader and generation paths.
- CLI integration orchestration passed both ordinary and missing-reactor suites
  in compiled and interpreted execution. Real native CLI orchestration passed
  both GPU lanes: three facts per lane, no interpreter fallback.
- The eight Mathematics transform facts and the reference FFT example passed
  compiled execution, with no fallback.
- All 32 Modernization shader facts passed on the native host, including the
  new FP64 fact. SDSL-V conformance manifest acceptance and rejection checks
  passed.
- Normal native build passed. Normal FFT tests: 15/15; Verify FFT tests linked
  with normal companion modules: 15/15. Both ran with Vulkan synchronization
  validation and no validation errors or synchronization hazards.
- Five artifacts matched across three fresh Concept emissions. Normal/Verify
  ABI headers matched; both emitted FFT modules compiled with
  `clang -std=c11 -pedantic-errors`.
- Concept regeneration/provenance and native source inventory checks passed.
  Generated C, caches, raw reports and temporary outputs remain ignored.

A final registry rerun also exposed a pre-existing admission defect: duplicate
implementation IDs, incompatible operation/pipeline families and missing
dispatch metadata were accepted. The existing validator now rejects them; its
five facts pass. The catalog assertion was updated from the obsolete 40 to 44
embedded entries and checks that FP64 FFT remains package-only. The rebuilt
native FFT suite again passes 15/15. Initial and corrected registry logs are
retained separately; FB-137 records the repair.

Benchmark compatibility was checked separately. The old embedded fallback
program was moved to `Language/Prometheus/bench`; with a missing reactor its CPU
case passes and its strict GPU case fails. Its explicitly enabled slow CLI test
passes. The existing Windows benchmark test also passes all 28 cases and loads
the exported Octagon report with hardware backend/environment metadata. The
initial metadata mismatch is retained in `.tmp/m12-native-bench.log`; the
corrected run is `.tmp/m12-native-bench-final.log`.

The first hosted integration run exposed missing global corpus registration
for the environment-specific suites. GPU facts and benchmark entry points now
have explicit checked owners; the portable runner fixes its reactor to an
absent path. The strict benchmark test is included in ordinary integration CI.
Targeted corpus routing and benchmark-owner reruns pass. No native assertions
or compiled execution requirements were relaxed; FB-138 records this repair.

The complete local portable corpus rerun finished after 475.93 s with three
Windows executable-cleanup failures in the existing array directory (33 facts
passed there). That unchanged directory passed alone in 19.66 s. The broad
local integration run is retained as failed, not relabeled green; FB-139 leaves
the cleanup cause open. This is distinct from the passed fast Go suite and the
targeted FFT/native qualification.

This is a Windows GPU qualification; Linux GPU execution and other devices are
not claimed. Concept compiler/library sources and the frozen Zig compiler are
unchanged; the frozen Zig suites were skipped.

The legacy C FFT diagnostic registry and FP32 plan reporting remain compatibility
code. Moving their bookkeeping is separate from replacing the Vulkan executor.

The owner explicitly refuses new work after a terminal device-loss result.
The final guard was rebuilt and passed the same Normal/Verify FFT and strict
C11/determinism checks. Deliberate physical device-loss injection was not run.
