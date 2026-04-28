# P12 M4 — Reactor Family Stub Files Report

## 1) M3 handoff summary

P12 M3 completed the SGEMM identity rename (`reactor_vulkan.c` → `reactor_vulkan_sgemm.c`) and kept behavior/ABI stable while `reactor_vulkan_common.c` held only low-risk shared Vulkan helpers. M4 continues the topology-only plan by adding inert future-family reactor source files with no behavioral claims.

## 2) Files added

Added topology placeholder source files:

- `internal/prometheus/native/reactor_vulkan_fft.c`
- `internal/prometheus/native/reactor_vulkan_fused_reduction.c`

Both files include `reactor_vulkan.h` and contain comment-only placeholder scope documentation.

## 3) FFT stub scope

`reactor_vulkan_fft.c` is intentionally inert.

Allowed content in this milestone:

- include of `reactor_vulkan.h`,
- comments documenting future intended FFT family scope.

Future intended scope documented:

- FFT kernels/pipelines,
- FFT resource ownership,
- FFT-specific diagnostics/facts,
- FFT ABI only if introduced in a later milestone.

## 4) Fused-reduction stub scope

`reactor_vulkan_fused_reduction.c` is intentionally inert.

Allowed content in this milestone:

- include of `reactor_vulkan.h`,
- comments documenting future fused-reduction family scope.

Future intended scope documented:

- softmax/layernorm/reduction/scan-style kernels,
- fused-reduction family resource ownership,
- fused-reduction diagnostics/facts,
- fused-reduction ABI only if introduced in a later milestone.

## 5) Behavior/capability non-claims

Both stubs intentionally do **not**:

- define new public ABI symbols,
- alter `PrometheusCaps`,
- change runtime probe behavior,
- report FFT or fused-reduction availability,
- alter SGEMM execution/diagnostics,
- alter batch/concurrency behavior,
- alter Dominatus, typed arena, or selector behavior.

Topology shape changed; runtime semantics did not.

## 6) Build integration updates

Updated active native build source lists to compile both stubs (Linux + Windows helper paths, including Marionette test binary source lists):

- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`

Compiled source topology now includes:

- `reactor_vulkan_common.c`
- `reactor_vulkan_sgemm.c`
- `reactor_vulkan_fft.c`
- `reactor_vulkan_fused_reduction.c`

## 7) Validation results

Executed:

- `bash internal/prometheus/native/build_stub.sh`
- `out/prometheus/native/marionette_tests P11_M20`
- `out/prometheus/native/marionette_tests PrometheusReactor_Sgemm`
- `out/prometheus/native/marionette_tests`
- `rg "reactor_vulkan_fft"`
- `rg "reactor_vulkan_fused_reduction"`

Observed:

- build succeeded with new stubs in active source lists,
- targeted and full Marionette runs succeeded,
- grep/search confirms active build references,
- no new public capability claims or fake ABI references introduced.

## 8) Deferred scope

Explicitly deferred (unchanged):

- FFT implementation,
- fused-reduction implementation,
- softmax / LayerNorm kernels,
- FFT/fused-reduction public ABI,
- FFT/fused-reduction capability flags,
- family-specific Dominatus facts,
- family-specific diagnostics,
- any SGEMM behavior changes.
