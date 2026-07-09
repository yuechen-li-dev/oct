# SDSL-V M2: DXC, SPIR-V, and Header Generation

SDSL-V M2 adds the next opt-in backend lane on top of the existing `VD-MIR` boundary:

```text
SDSL-V source
  -> lex
  -> parse
  -> validate
  -> lower to VD-MIR
  -> emit HLSL
  -> DXC -spirv
  -> SPIR-V
  -> generated C header
```

This milestone is intentionally scoped to toolchain generation and a tiny compute shader proof such as `examples/SDSL-V/M0/VectorAdd.sdslv`. It does not wire new Prometheus SGEMM kernels, change Prometheus runtime dispatch, or require DXC for ordinary Go/native builds.

## Process-call pattern used

Before adding DXC invocation, the implementation inspected the existing production process runner in `internal/makecmd/makecmd.go`.

That `oct make` runner:

- resolves a working directory relative to the repository root;
- uses `exec.Command(...)`;
- merges environment overrides onto `os.Environ()`;
- captures stdout and stderr separately;
- reads the process exit code from `ProcessState`.

SDSL-V M2 reuses that exact shape in `internal/sdslv/toolchain/process.go` instead of scattering ad hoc process calls through the CLI or backend code.

## New commands

```powershell
go run ./cmd/oct sdslv compile-spv examples/SDSL-V/M0/VectorAdd.sdslv -o out/sdslv/vector_add.spv
go run ./cmd/oct sdslv generate-header examples/SDSL-V/M0/VectorAdd.sdslv -o out/sdslv/vector_add_spirv.h --symbol k_sdslv_vector_add_spirv
```

Supported options:

- `--entry <EmittedEntryName>`
- `--dxc <path>`
- `--hlsl-out <path>`
- `--spv-out <path>` on `generate-header`
- `--extra-dxc-arg <arg>` repeated
- `--validate`
- `--require-spirv-val`

Existing commands remain unchanged:

- `oct sdslv check`
- `oct sdslv emit-vdmir`
- `oct sdslv emit-hlsl`
- `oct sdslv test`

## DXC discovery order

DXC resolution is deterministic and Windows-first:

1. explicit CLI flag: `--dxc <path>`
2. environment variable: `SDSLV_DXC`
3. `PATH` lookup for `dxc` / `dxc.exe`
4. `%VULKAN_SDK%\Bin\dxc.exe`
5. otherwise fail with a clear diagnostic

The diagnostic tells the caller to pass `--dxc`, set `SDSLV_DXC`, add `dxc` to `PATH`, or install a Vulkan SDK that provides `dxc`.

`spirv-val` follows the simpler opt-in lookup path:

1. explicit path if plumbed later
2. `PATH` lookup for `spirv-val` / `spirv-val.exe`
3. `%VULKAN_SDK%\Bin\spirv-val.exe`

Validation is only attempted when the caller passes `--validate` or `--require-spirv-val`.

## Current DXC invocation

For compute shaders, M2 invokes DXC with:

```text
dxc -spirv -T cs_6_0 -E <Entry> -Fo <output.spv> -fspv-target-env=vulkan1.0 -O3 <input.hlsl>
```

Extra DXC args can be appended with repeated `--extra-dxc-arg`.

DXC stderr is surfaced directly on failure. The command summary printed by the CLI includes the resolved DXC path, emitted HLSL path, SPIR-V path, entry point, and final argument list.

## Entry point rules

- If the module contains exactly one compute entry point, M2 uses it by default.
- If the module contains multiple compute entry points, the caller must pass `--entry`.
- If `--entry` is provided, M2 validates that the requested emitted entry exists.

The selected entry is the emitted HLSL/DXC name, such as `VectorAdd_CS`.

## HLSL/Vulkan emission details

M2 keeps `VD-MIR` as the backend boundary. HLSL still emits from `VD-MIR`; there is no direct AST-to-HLSL bypass.

Two small backend changes were needed so DXC accepts the M0 compute subset:

- resources now emit deterministic Vulkan bindings:
  - `[[vk::binding(0, 0)]]` for the first resource
  - `[[vk::binding(1, 0)]]` for the second
  - and so on
- a compute entry parameter record now emits as a Vulkan push-constant global:
  - `[[vk::push_constant]] ConstantBuffer<RecordType> params;`

The current binding policy is deliberately provisional M2 policy:

- all resources use set `0`
- bindings are assigned in shader resource declaration order
- the single compute parameter record is lowered as a push constant

This is enough for deterministic toolchain generation and the `VectorAdd` proof. Full binding policy remains deferred.

## Generated header format

`generate-header` converts SPIR-V bytes into a deterministic C header suitable for Prometheus-native inclusion.

The header includes:

- an include guard
- `#include <stdint.h>`
- source-file comment
- generator-command comment
- entry-point comment
- `static const uint32_t <symbol>[]`
- `<symbol>_word_count`
- `<symbol>_byte_length`

The current format intentionally matches the broad style of existing checked-in Prometheus SPIR-V headers while adding explicit provenance comments and deterministic wrapping.

## Example proof

On a machine with DXC available, the current proof lane is:

```powershell
go run ./cmd/oct sdslv emit-hlsl examples/SDSL-V/M0/VectorAdd.sdslv -o out/sdslv/vector_add.hlsl
go run ./cmd/oct sdslv compile-spv examples/SDSL-V/M0/VectorAdd.sdslv -o out/sdslv/vector_add.spv
go run ./cmd/oct sdslv generate-header examples/SDSL-V/M0/VectorAdd.sdslv -o out/sdslv/vector_add_spirv.h --symbol k_sdslv_vector_add_spirv
```

Ordinary repository builds still consume checked-in native assets directly and do not require DXC unless the user explicitly invokes shader regeneration.

## What M2 does not do

M2 does not:

- wire generated headers into `reactor_vulkan_sgemm.c`
- add SGEMM kernel families
- change Prometheus selector logic or runtime dispatch
- change FFT/P16 work
- require DXC for `go test`, ordinary native builds, or normal `oct` language workflows

## Next step

The next natural milestone is to use this opt-in generation lane to produce source-backed Prometheus shader artifacts intentionally, then wire those artifacts into the Prometheus runtime in a separate milestone.
