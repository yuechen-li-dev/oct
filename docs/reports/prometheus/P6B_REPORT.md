# P6b Report — Windows Bridge Loader + First Real Windows Prometheus Execution

## What changed

P6b closes the Windows runtime gap left by P6a.

The Prometheus bridge now has a real Windows-native dynamic loader behind
`windows && cgo`, using:

- `LoadLibraryA`
- `GetProcAddress`
- ABI version validation
- runtime create/destroy/probe wiring
- SGEMM call wiring

No C ABI shape changes were required.

## Windows loader behavior

The bridge now resolves the same exported symbols already used on Linux:

- `prometheus_reactor_abi_version`
- `prometheus_reactor_runtime_create`
- `prometheus_reactor_runtime_destroy`
- `prometheus_reactor_runtime_probe`
- `prometheus_reactor_runtime_sgemm`

Discovery remains explicit and local. Candidate paths are:

- `OCT_PROMETHEUS_REACTOR` when set
  - when this variable is present, the bridge treats it as the explicit reactor
    path and does not silently fall through to other candidates
- `internal/prometheus/reactor/prometheus_reactor.dll`
- `out/prometheus/native/prometheus_reactor.dll`
- next to the current executable
- repository-local absolute paths derived from `internal/prometheus`

## Fallback truthfulness

Pre-dispatch Windows availability failures now remain truthful CPU fallback
rather than pretending Prometheus execution happened.

This includes:

- missing DLL
- loader failure
- missing symbol
- ABI mismatch
- runtime create failure
- runtime probe failure / unavailable probe

Reported shape remains:

- `backend_requested=prometheus`
- `backend_used=cpu`
- `status=fallback(prometheus_unavailable)`

Native execution failures after successful load/probe still remain explicit
Prometheus errors rather than fallback.

## Environment classification

When the Reactor reports hardware Vulkan on Windows, the runtime now surfaces:

- `windows_native_vulkan`

This keeps Windows-native execution distinct from:

- `vulkan_wsl_dzn`
- `software_vulkan_llvmpipe_or_cpu`
- `unavailable`

## Validation environment

Validation was run on Windows-native Vulkan with:

- `vulkaninfo --summary`
- `driverName = NVIDIA`
- `deviceName = NVIDIA GeForce RTX 3070`
- `deviceType = PHYSICAL_DEVICE_TYPE_DISCRETE_GPU`

For the Go runtime path, validation used:

- `CGO_ENABLED=1`
- Windows-native `prometheus_reactor.dll`
- official benchmark surface `Experiments/PrometheusBenchmarkHarness/M0`

## Sample benchmark stdout

Successful official benchmark run:

```text
RUN  Main.MatrixMulCPUReferenceM0 (matrix_mul_cpu_reference.octest)
[8 15]
0
PASS Main.MatrixMulCPUReferenceM0 1.684872s (matrix_mul_cpu_reference.octest)
RUN  Main.MatrixMulPrometheusM0 (matrix_mul_prometheus.octest)
backend_requested=prometheus backend_used=prometheus status=ok correctness=true vulkan_env=windows_native_vulkan wall=1328600ns
[[19 22] [43 50]]
0
PASS Main.MatrixMulPrometheusM0 1.968703s (matrix_mul_prometheus.octest)
Result: 2 benchmark(s) passed, 0 failed
```

Representative direct SGEMM run:

```text
SGEMM M=32 N=32 K=32 backend_requested=prometheus backend_used=prometheus status=ok correctness=true cpu=0ns vulkan=698400ns vulkan_env=windows_native_vulkan wall=698400ns
```

## Sample `.octagon` output

`out/prometheus/native/p6b_m0_run5.octagon` captured:

```text
BenchmarkRun {
    Cases: [BenchmarkCaseResult {
        Name: "Main.MatrixMulCPUReferenceM0"
        DurationNs: (1684872300)
        BackendRequested: "cpu"
        BackendUsed: "cpu"
        Status: "ok"
        Environment: "not_applicable"
    }, BenchmarkCaseResult {
        Name: "Main.MatrixMulPrometheusM0"
        DurationNs: (1968703300)
        BackendRequested: "prometheus"
        BackendUsed: "prometheus"
        Status: "ok"
        Environment: "windows_native_vulkan"
    }]
}
```

## Correctness and stability

Correctness remained enabled and passed:

- benchmark stdout reported `correctness=true`
- direct SGEMM runs under the real DLL also reported `correctness=true`
- no silent wrong-answer path was introduced

Repeated-run stability was validated with:

- direct Windows loader tests calling the real DLL three times
- sequential official M0 benchmark reruns

Observed repeated official benchmark Prometheus lines:

- `backend_used=prometheus status=ok ... wall=1281000ns`
- `backend_used=prometheus status=ok ... wall=1155100ns`
- `backend_used=prometheus status=ok ... wall=1328600ns`

## Remaining limitations

P6b intentionally does not change:

- SGEMM algorithm
- optimization strategy
- new kernels
- auto mode
- packaging/distribution design

This milestone is strictly the Windows runtime-loader and execution milestone.

Local note:

- the Reactor DLL exercised here is the Windows-native artifact produced by the
  P6a path
- Go-side `cgo` validation in this shell used MinGW `gcc` to build the bridge
  test binaries because `go test` with `CC=cl.exe` failed in `runtime/cgo`
  before package code compiled
