# P4b Report — Prometheus Reactor Minimal Stub (Marionette + C ABI)

## What was implemented

P4b introduces the first real native Prometheus Reactor artifact as a dynamically loadable library.

Implemented ABI surface:

- `prometheus_reactor_abi_version`
- `prometheus_reactor_runtime_create`
- `prometheus_reactor_runtime_destroy`
- `prometheus_reactor_runtime_probe`

A minimal C-compatible `PrometheusCaps` POD struct and simple status-code constants are defined in `internal/prometheus/native/bridge.h`.

## Stub runtime behavior

- Runtime create allocates a small opaque runtime handle and tracks active handles.
- Runtime destroy is defensive:
  - null destroy returns success
  - second destroy on the same pointer returns `PROM_INVALID_HANDLE` rather than crashing
- Runtime probe is deterministic for P4b:
  - `available = 0`
  - `backend_type = PROM_BACKEND_STUB`
  - `reason_code = PROM_REASON_STUB_UNAVAILABLE`

This means the bridge can load and exercise the ABI, while still treating the Reactor as unavailable for execution.

## ABI boundary properties

- Exported functions use a C ABI (`extern "C"` compatibility in the header).
- No C++ types cross the boundary.
- No STL types cross the boundary.
- ABI data structures are C-compatible POD.

## Marionette coverage added

Native Marionette tests now cover:

1. ABI version stability.
2. Runtime create/destroy lifecycle, including defensive repeated destroy behavior.
3. Runtime probe deterministic output and struct fill.
4. Invalid-usage paths (null output pointers, null handle probe, null caps pointer).

## Build/test flow

`internal/prometheus/native/build_stub.sh` now builds:

- shared reactor library artifact (`.so` on Linux, `.dll` on Windows-like environments)
- Marionette native test binary

The script also copies the built library to `internal/prometheus/reactor/`, matching the bridge discovery path used by Go.

## Explicitly deferred (still non-goals in P4b)

- Vulkan SGEMM kernels
- Real GPU execution
- Async execution/streams
- Multi-kernel scheduling
- Performance work
- Memory residency and allocator strategy beyond minimal handle lifetime
