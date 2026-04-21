# P6a Report — Windows-Native Prometheus Toolchain + Reactor Build Enablement

## What changed

P6a makes the Prometheus native build flow real on Windows without changing the
Oct core/runtime architecture.

The native Prometheus area now supports two additive build paths:

- `internal/prometheus/native/build_stub.sh`
  - existing POSIX/Linux helper for `libprometheus_reactor.so`
- `internal/prometheus/native/build_windows.cmd`
  - Windows-native MSVC helper for `prometheus_reactor.dll`

The Reactor ABI remains a plain C ABI with POD structs and opaque handles.
Windows support was added at the native boundary only:

- exported Reactor symbols now use a Windows-safe export macro when the DLL is
  being built
- the internal active-handle registry no longer depends unconditionally on
  `pthread`; Windows uses `SRWLOCK`, while Linux keeps the existing
  `pthread_mutex_t` path

## Windows-native build flow

Run the helper from a proper Visual Studio developer shell:

```bat
internal\prometheus\native\build_windows.cmd
```

The helper assumes:

- `cl` is already available on `PATH`
- a Vulkan SDK is installed
- if `VULKAN_SDK` is set, the helper uses `%VULKAN_SDK%\Include` and
  `%VULKAN_SDK%\Lib`
- if `VULKAN_SDK` is not set, the shell must already provide Vulkan headers and
  `vulkan-1.lib` through the normal MSVC include/library environment

The helper does not attempt to install or configure Visual Studio globally. It
fails fast if `cl` is missing.

## Artifacts produced

Windows-native outputs land under `out/prometheus/native/`:

- `prometheus_reactor.dll`
- `prometheus_reactor.lib`
- `prometheus_reactor.exp`
- `prometheus_reactor.pdb`
- `marionette_tests.exe`
- `marionette_tests.pdb`

For parity with the existing bridge discovery convention, the helper also
copies the built DLL to:

- `internal/prometheus/reactor/prometheus_reactor.dll`

Linux/native output remains:

- `out/prometheus/native/libprometheus_reactor.so`
- `out/prometheus/native/marionette_tests`

## Marionette status on Windows

Marionette native tests are enabled in the Windows toolchain path.

The same `build_windows.cmd` helper builds `marionette_tests.exe` against the
same native Reactor sources, which keeps Windows-native ABI and Reactor
compilation exercised without broadening the core Go/Oct workflow.

## What remains for the next milestone

P6a is only a build/toolchain milestone. It does not yet claim Windows runtime
execution.

Still deferred to the next Windows milestone:

- Windows bridge dynamic loading
- Windows-native `LoadLibrary`/symbol resolution path
- proof that Oct Prometheus execution uses the Windows Reactor at runtime
- optimization/performance work

The result is narrower and honest:

> the Windows-native Reactor DLL build is now enabled, but Windows runtime
> loading/execution is still a separate step.
