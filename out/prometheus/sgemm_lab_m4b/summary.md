# M4b Windows-Native Prometheus Summary

Environment:
- Windows
- NVIDIA GeForce RTX 3070
- `OCT_PROMETHEUS_REACTOR=out/prometheus/native/prometheus_reactor.dll`
- `CGO_ENABLED=1`
- `CC=gcc`
- `CXX=g++`
- `BackendUsed=prometheus`
- `Environment=windows_native_vulkan`

Note:
- This M4b slice validates the real Prometheus Vulkan path on the six suspicious shapes.
- It does not yet provide native per-strategy rankings for the M3 kernels, because only `PROMETHEUS { a @ b }` currently lowers to native Prometheus in compiled mode.

Shape: (16, 16, 512)

ChosenStrategy: KBlockedIKJ
FastestObserved: Prometheus builtin matmul (`@`) at 517900 ns reported wall time

Observations:
- All three runs reported `BackendUsed=prometheus`, `Status=ok`, `Environment=windows_native_vulkan`
- ReportedWallNs across runs: `684100`, `1019000`, `517900`
- This contradicts the idea that the cloud M4 CPU sweep already represented Prometheus reality, but it does not yet confirm or reject the cloud K-block mismatch itself

Shape: (8, 8, 256)

ChosenStrategy: KBlockedIKJ
FastestObserved: Prometheus builtin matmul (`@`) at 501600 ns reported wall time

Observations:
- All three runs reported `BackendUsed=prometheus`, `Status=ok`, `Environment=windows_native_vulkan`
- ReportedWallNs across runs: `501600`, `501700`, `569500`
- Native path behavior is relatively stable here, but native ranking between `KBlocked` and `KBlockedIKJ` remains unavailable

Shape: (16, 8, 64)

ChosenStrategy: KBlockedIKJ
FastestObserved: Prometheus builtin matmul (`@`) at 501500 ns reported wall time

Observations:
- All three runs reported `BackendUsed=prometheus`, `Status=ok`, `Environment=windows_native_vulkan`
- ReportedWallNs across runs: `503400`, `501500`, `1049500`
- One run showed a larger spike, so this rectangular case is not perfectly stable yet

Shape: (8, 16, 128)

ChosenStrategy: KBlockedIKJ
FastestObserved: Prometheus builtin matmul (`@`) at 681700 ns reported wall time

Observations:
- All three runs reported `BackendUsed=prometheus`, `Status=ok`, `Environment=windows_native_vulkan`
- ReportedWallNs across runs: `1213800`, `1157600`, `681700`
- Variance is visible; the native path is real, but strategy-family conclusions should not be inferred from this path-validation surface

Shape: (32, 32, 32)

ChosenStrategy: KBlockedIKJ
FastestObserved: Prometheus builtin matmul (`@`) at 599500 ns reported wall time

Observations:
- All three runs reported `BackendUsed=prometheus`, `Status=ok`, `Environment=windows_native_vulkan`
- ReportedWallNs across runs: `664100`, `599500`, `636500`
- Native execution is reasonably tight here, but the cloud `KIJ` vs chooser mismatch remains unvalidated natively

Shape: (16, 16, 16)

ChosenStrategy: KBlocked
FastestObserved: Prometheus builtin matmul (`@`) at 501700 ns reported wall time

Observations:
- All three runs reported `BackendUsed=prometheus`, `Status=ok`, `Environment=windows_native_vulkan`
- ReportedWallNs across runs: `501700`, `635100`, `559700`
- This serves as the baseline sanity anchor for real Prometheus path validation, not as a native ranking of M3 kernels

Overall:
- Windows-native Prometheus execution is confirmed for all six suspicious shapes
- The cloud M4 strategy sweep should not be treated as native Prometheus evidence
- Staged viability is still unresolved on real Prometheus because `StagedIKJ` does not currently lower to the native path
- K-block usefulness is still unresolved on real Prometheus for the same reason
- The strongest M4b signals for M5 are path truthfulness and environment gating, not native per-strategy scoring
