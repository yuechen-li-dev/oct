# octxiliary-kaiju-vulkan

`octxiliary-kaiju-vulkan` is the optional M36b SDSL-V validation sidecar. It
is a short-lived OCTWRAP process that accepts typed `kaiju-vulkan` protocol
requests and executes compiler-owned SPIR-V through Kaiju's raw Vulkan binding.

Scope:

- `compute.capabilities`
- `compute.dispatch`
- `compute.benchmark`
- set `0` storage buffers only
- explicit entry points
- push constants
- typed `Bytes` payloads for SPIR-V, resources, push constants, and readbacks
- Vulkan query-pool GPU timestamps for benchmark runs

Build:

```powershell
go run ./tools/build_sidecars --kaiju --out dist/sidecars
```

The build verifies the pinned local Kaiju checkout before compiling the nested
module. Required output on Windows is:

- `dist/sidecars/octxiliary-kaiju-vulkan.exe`

Requirements:

- Go 1.26.x in the current proof environment
- CGO enabled
- GCC-compatible toolchain for Kaiju's raw Vulkan binding
- Vulkan loader/runtime and headers from the local Vulkan SDK path

Validation layer:

- set `OCT_KAIJU_VULKAN_VALIDATION=1` to request `VK_LAYER_KHRONOS_validation`
- the sidecar reports requested/available/enabled state in typed responses

Limits:

- max SPIR-V bytes: 8 MiB
- max resources: 16
- max bytes per resource: 8 MiB
- max aggregate payload bytes: 32 MiB
- max aggregate readback bytes: 32 MiB
- max push constants: 256 bytes
- max warmup: 256
- max iterations: 512
- max dispatch dimension: 1,048,576
- process timeout: 30,000 ms
- max response bytes: 40 MiB

Provenance and licensing live in `KAJU_PROVENANCE.md` and the M36b report.
