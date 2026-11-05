# Concept RMSNorm on the shared Prometheus device

Date: 2026-11-05

Status: **MILESTONE VERIFIED — Success (end state A).**

The production input, projection-activation, and head RMSNorm entrypoints now
use `reactors/PrometheusRmsNorm.concept`. Concept owns weight publication,
reusable buffers, pipelines, recording, submissions, completion, and retirement.
The head-normalization → BF16 → RoPE tail stays in one GPU submission and retains
Q/K for the existing attention consumer. Ordinary normalization and this tail
never initialize the shared legacy model executor.

## Ownership and intent

The existing pure RMSNorm planner remains the strategy/admission authority;
its public C packet and numerical oracle are retained. Its memory/readback plan
now describes shared allocations: no duplicate weight upload or N readback
buffer, no device copy regions, and shader-write → host-read visibility for
final observations. Public request/result layouts remain unchanged. Historical
command-plan replay hashes change with the new memory plan; numerical output
and deterministic replay across identical requests remain the contracts.

The new executor uses the existing Concept Vulkan `BorrowedContext`, `Buffer`,
`Pipeline`, `Recording`, `Submission`, declared-access barriers, and Standard
spans/math. Pipelines load the existing package variants 63, 64, 67 and 68 through
the foundational module-to-pipeline API. Three lazily created semantic slots
serve ordinary normalization, resident Q, and resident K. A synchronous API with
fixed resident roles needs no additional work scheduler; standalone reductions
continue to use DragonGod's FIFO queue.

Weights use candidate storage: upload succeeds before the new generation is
published. Stale/nonfinite candidates preserve the committed weight. Buffers
grow and then persist; an identical warm request allocates no buffers. Shared
placement prefers device-local host-visible memory when available and otherwise
uses the foundational host-visible allocation path. This milestone makes no
throughput or memory-placement performance claim.

Uncertain completion retains the owned submission and its buffers. Retry drains
that work before reuse; detach can drain it directly. Device loss is terminal.
Legacy attention borrows completed Concept Q/K and consumes their authority on
successful completion. Before replacing a still-published resident destination,
the adapter establishes shared-device idle, covering a legacy consumer whose
completion could not be observed. Destruction also establishes this boundary.
Calls on one model owner, including resident consumers and destruction, require
external serialization, as the previous model API did.

The C11 facade transports opaque generated state and packets. The C++23 adapter
validates foreign packets, translates allocation/mutex failures, loads package
modules, and projects hashes. Private generated ownership layouts stay in C11.
Concept physical slot diagnostic IDs use a separate namespace from the legacy
model ring; numeric equality across independent allocators never implies aliasing.

## Integration defects found and fixed

The first numerical GPU run passed but exposed two real integration failures:

- Owner destruction removed the registry entry before borrower cleanup. Validated
  getters then saw no attachment, leaking 17 buffers and one borrowed context in
  the resident test. Retirement now happens while the owner remains registered.
  This also fixes automatic destruction of the earlier reduction attachment.
- Each adapter defined its own submit callback. Concept Vulkan correctly rejected
  the conflicting callback identities on one device. Both now register one common
  owner callback through the existing shared queue synchronization boundary.

The failed run remains in `.tmp/prometheus-rmsnorm-gpu.log`; it was not discarded
or counted as passing evidence. The native build also initially found two
barrier helpers still needed by standalone legacy RoPE; those are retained.

## Verification

The Windows entrypoint regenerates all three Concept modules, verifies their
source/output digests, builds the C11/C++23 archives from `manifest.concept`, and
builds the DLL, Marionette binaries, and SDSL-V host. Generated C, headers,
provenance, objects, and archives stay under ignored `.native-build/`.

```powershell
cmd /d /c internal\prometheus\native\build_windows.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptRmsNorm
out/prometheus/native/marionette_tests.exe PrometheusM46
out/prometheus/native/marionette_tests.exe PrometheusM49aMatchedInputM46HardwareProof
out/prometheus/native/marionette_tests.exe PrometheusReduction_
out/prometheus/native/marionette_tests.exe PrometheusFrM0
out/prometheus/native/marionette_tests.exe PrometheusReactor_Sgemm
out/prometheus/native/marionette_tests.exe PrometheusStage0
go test ./tools/prometheus_concept ./tools/prometheus_native_manifest
go vet ./tools/prometheus_concept ./tools/prometheus_native_manifest
go run ./tools/prometheus_native_manifest
```

New GPU checks cover an independent double-sum CPU oracle, widths 31/1536/4096,
padded rows, both output strategies, all four FP32/BF16 input/output combinations,
forced staged reduction, warm allocation/hash/replay stability, rejected weight
replacement, SGEMM and reductions on the same device, both resident preparation
orders, optional positional readback, one-shot uncertain completion, retry,
detach/reattach, automatic owner destruction, and resource counters.

Final Windows/RTX 3070 evidence:

| Check | Result |
| --- | --- |
| Full native package/DLL/test/benchmark/host build | Pass |
| New Concept RMSNorm GPU facts | 3 passed, zero skips |
| Existing M46 planner/oracle + matched-input GPU proof | 3 passed, zero skips |
| Existing reduction + row-wise softmax + SGEMM + public ABI regressions | 27 passed, zero skips |
| Total selected native facts | 33 passed, zero skips/failures |
| New independent RMSNorm oracle, worst absolute error | `1.19209e-7` |
| Resident Q/K → legacy scores oracle, worst absolute error | `5.96046e-8` |
| Existing matched-input output / inverse-RMS hashes | Unchanged from C baseline |
| Warm output / replay / allocation checks | Pass |
| Shared-device identity / validation / lifetime counters | Pass |
| Concept reduction planner facts | 4 Normal + 4 Verify passed |
| Tooling tests, vet, and native manifest parity | Pass |

Validation was enabled for every selected native run. Logs are retained under
`.tmp/rmsnorm-final-*.log` and `.tmp/prometheus-rmsnorm-final-build.log`.

Full Concept/Oct compiler qualification and frozen Zig lanes are not prerequisites
for this native consumer change: no compiler or foundational library changed.
Existing Concept reduction planner facts run in Normal and Verify. Linux runtime
execution is not claimed from this Windows host.

## Next port slice

The old C RMSNorm executor, normalization pipelines/weights, temporary buffers,
and legacy Q/K ring pinning are removed. `reactor_vulkan_gemma4e2b.c` retains the
pure planner/oracles, standalone legacy RoPE, and the legacy attention score
executor. Porting that score consumer next removes the model-state dependency
from the complete head-normalization → positional encoding → scores chain.
The separate model-block/Z-Image owners remain outside this milestone.
