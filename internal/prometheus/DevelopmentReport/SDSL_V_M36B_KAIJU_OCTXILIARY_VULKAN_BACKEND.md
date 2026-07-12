# SDSL-V M36b Kaiju Octxiliary Vulkan backend

Status: **COMPLETE — typed Kaiju Vulkan sidecar wired through `oct sdslv bench`** (2026-07-12)

Kaiju is an optional, process-isolated alternate Vulkan witness. It is not a
Prometheus replacement: Prometheus remains an independent native runtime and
SDSL-V remains an Oct/Go compiler facility. No compiler/runtime package may
import Kaiju.

## Canonical M36b ndarray/tensor inputs

The deleted temporary Godot files were never canonical. The authority is now
checked-in source, isolated compilation and recorded toolchain identity:

| Case | Stable ID | SPIR-V | Bytes | SHA-256 |
|---|---|---|---:|---|
| NDArrayMaterializeStorage | `sdslvbench-8b1f66233dd54390f518e9c7` | `examples/SDSL-V/M36a/artifacts/ndarraymaterialize.spv` | 1560 | `bd3ea90711adaad03e98923d7397d5b3e259497e437918bd444c46a0c46dc083` |
| TensorContractionStorage | `sdslvbench-a2b7fd8383074dd673a365d5` | `examples/SDSL-V/M36a/artifacts/tensorcontraction.spv` | 2820 | `9c14708fb37490d3f0f776a2cd4b156dbf00936fb8a4d6f5db159718f393a3a7` |

Generate with `go run ./tools/generate_m36b_canonical`. The manifest records
DXC 1.9.0.5180 (`e35541826`), `-spirv -T cs_6_0 -E main
-fspv-target-env=vulkan1.0`, `spirv-val`, compiler identity, resource
contracts and source digest. The parity test rebuilds and checks the committed
bytes/hashes when the tools are installed.

## Current hardware evidence

The pinned Kaiju spike (`ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2`) first
accepted both exact canonical byte streams on Windows/RTX 3070. The production
typed sidecar now reproduces that path through OCTWRAP/Octagon framing:

| Case | Backend path | SHA-256 | Result | Timing source |
|---|---|---|---|---|
| DefaultCounts | `oct sdslv bench --backend kaiju` | `61189616e925769a76cdc44901f1cb1d65ef0c8afd44358e610bbde19063d9e5` | pipeline, dispatch, cleanup | Vulkan query pool |
| ScalarArithmetic | `oct sdslv bench --backend kaiju` | `1582e565d23f6217cdc5de56af97f4ab55aca7218584bd0f9d246840fca4346e` | pipeline, dispatch, cleanup | Vulkan query pool |
| VectorDotStorage | `oct sdslv bench --backend kaiju` | `6f8fb736086ae9437ba8107b8b86cb888f4ea60b3334af10da44c5c3ffc3927e` | pipeline, dispatch, readback | Vulkan query pool |
| NDArrayMaterializeStorage | `oct sdslv bench --backend kaiju` | `bd3ea90711adaad03e98923d7397d5b3e259497e437918bd444c46a0c46dc083` | pipeline, dispatch, readback | Vulkan query pool |
| TensorContractionStorage | `oct sdslv bench --backend kaiju` | `9c14708fb37490d3f0f776a2cd4b156dbf00936fb8a4d6f5db159718f393a3a7` | pipeline, dispatch, readback | Vulkan query pool |

Representative measured samples from the production sidecar:

- ndarray: 3,264–3,584 ns across eight measured iterations
- tensor: 3,296–3,520 ns across eight measured iterations
- vector dot: 3,136–3,648 ns across one hundred measured iterations

Godot 4.7 was not installed in the current verification environment, so no
new Godot result is claimed. M36b must run these exact paths when Godot is
available and update the capability record based on that result.

## Production boundary

The production binary is `dist/sidecars/octxiliary-kaiju-vulkan.exe`. It is an
optional nested module under `Sidecars/KaijuVulkan`, built explicitly by
`go run ./tools/build_sidecars --kaiju --out dist/sidecars`. The root Oct
compiler/runtime packages do not import Kaiju, Prometheus remains unchanged,
and the canonical wire protocol is typed OCTWRAP/Octagon `kaiju-vulkan`
protocol version `1`.

Implemented operations:

- `compute.capabilities`
- `compute.dispatch`
- `compute.benchmark`

Implemented transport/data scope:

- `Bytes` for SPIR-V, push constants, resource payloads, and readbacks
- typed records for requests, readbacks, device info, timing, validation
  status, diagnostics, and limits
- set `0` storage buffers
- explicit entry points
- readonly/readwrite storage-buffer access
- `bytes`, `u32`, `i32`, `f32`, `float2`, and `float4` element metadata
- real Vulkan query-pool GPU timestamps for benchmark timing

CLI integration:

- `oct sdslv bench file.sdslvbench --backend kaiju`
- `oct sdslv bench file.sdslvbench --backend godot`
- `oct sdslv bench file.sdslvbench --backend auto`
- `auto` prefers Kaiju when the sidecar is installed and advertising
  dispatch/benchmark support; it falls back to Godot only when Kaiju is absent
  or explicitly unsupported
- `--list` does not launch any backend

Windows proof:

- host GPU: NVIDIA GeForce RTX 3070
- sidecar binary size after production wiring: 5,426,777 bytes
- process and in-process hardware tests passed for capabilities, dispatch, and
  canonical benchmark execution

Linux status:

- source/build status only; no Linux runtime proof is claimed in this report
