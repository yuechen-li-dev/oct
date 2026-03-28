# P1a — Prometheus Sandbox / Toolchain Capability Probe

Date: 2026-03-28 (UTC)

## 1. Toolchain Status

Direct probe commands showed a working native toolchain:

- `cc`/`gcc` present at `/usr/bin/cc` and `/usr/bin/gcc` (GCC 13.3.0)
- `g++` present at `/usr/bin/g++` (G++ 13.3.0)
- `clang` present at `/root/.swiftly/bin/clang` (Clang 17.0.0)
- `cmake` 3.28.3 and `make` 4.3 present

Validation:

- compiled and ran a minimal C binary (`hello-c`)
- compiled and ran a minimal C++ binary (`hello-cpp`)

Conclusion: native C/C++ compile and run is viable in this sandbox.

## 2. Vulkan Install Status

Attempted narrow apt path only (no CUDA, no custom large dependency trees):

- `apt-get install -y --no-install-recommends libvulkan-dev vulkan-tools mesa-vulkan-drivers`

Installed successfully:

- `libvulkan-dev`
- `libvulkan1`
- `vulkan-tools`
- `mesa-vulkan-drivers`
- required runtime dependencies (xcb/wayland/drm support libraries)

Observed issue during `apt-get update`:

- one third-party mirror (`https://mise.jdx.dev/deb`) returned `403 Forbidden`
- Ubuntu archive/security indices and required Vulkan packages still resolved and installed correctly

Conclusion: Vulkan headers/loader/dev/runtime packages are installable in this environment.

## 3. Compile/Link Status

Built a minimal Vulkan probe program at:

- `testdata/p1a_prometheus_probe/minimal_vulkan_probe.c`

Compile/link command:

- `cc testdata/p1a_prometheus_probe/minimal_vulkan_probe.c -lvulkan -o testdata/p1a_prometheus_probe/minimal_vulkan_probe`

Result:

- compile succeeded
- link against `-lvulkan` succeeded

Conclusion: native Vulkan compile+link is viable.

## 4. Runtime Status

Runtime probe command:

- `testdata/p1a_prometheus_probe/minimal_vulkan_probe`

Observed runtime behavior:

- `vkCreateInstance: success`
- `physical_device_count: 1`
- enumerated device: `llvmpipe (LLVM 20.1.2, 256 bits)`

Additional runtime check:

- `vulkaninfo --summary` reported
  - `deviceType = PHYSICAL_DEVICE_TYPE_CPU`
  - `driverName = llvmpipe`
- also reported headless/session limits:
  - `'DISPLAY' environment variable not set... skipping surface info`
  - `XDG_RUNTIME_DIR is invalid or not set`

Device exposure check:

- `/dev/dri` not present

Conclusion: Vulkan runtime works, but only via CPU software rasterization (`llvmpipe`), with no exposed hardware GPU device.

## 5. Sandbox Suitability Judgment

**suitable only for compile/link validation, not runtime GPU work**

## 6. Exact Blockers

1. No hardware GPU device exposure in sandbox (`/dev/dri` absent).
2. Enumerated Vulkan device is CPU (`llvmpipe`), not a physical GPU.
3. Headless/runtime session constraints (`DISPLAY` and `XDG_RUNTIME_DIR` unset/invalid) indicate limited graphics/runtime integration.

These blockers prevent meaningful GPU-backed Prometheus runtime validation, correctness-on-GPU checks, and performance benchmarking.

## 7. Recommended Next Step

**proceed only with native compile/link scaffolding**

Rationale: current environment can validate toolchain integration and Vulkan build/link plumbing, but cannot validate actual GPU execution behavior for Prometheus.
