# SDSL-V M33b: fixed-shape Fill and Generate construction

Status: complete on 2026-07-11.

## Outcome

M33b is now complete as a real compiler and hardware-backed feature set.
SDSL-V supports:

- target-typed `Fill(value)` for fixed-shape `ndarray` declaration
  initialization;
- target-typed `Generate[i, j, ...](body)` with immutable `u32` binders;
- binder-order loop lowering and row-major ndarray linearization reuse;
- ordinary body composition through helper calls, guarded reads, inline HLSL,
  outer scalar captures, and tensor consumers;
- DXC/SPIR-V generation and Vulkan execution through the existing `.sdslvtest`
  host path.

M33c storage-class work did not begin.

## Proof matrix

Compiler proof:

- Parser, span, validation, lowering, and HLSL tests cover first-class
  `Fill(...)` and `Generate[...] (...)` syntax.
- Fill now proves one argument materialization and shared reuse.
- Generate now proves one destination offset materialization and one body
  materialization per coordinate tuple.
- Malformed Fill/Generate VD-MIR metadata is rejected by the HLSL emitter.
- Unrelated shaders remain free of accidental Fill/Generate lowering paths.

Hardware proof:

- `Examples/SDSL-V/M33b/TensorConstruction.sdslvtest` is a dedicated Vulkan
  execution suite.
- Green cases cover rank-1/2/4 Fill, rank-1/2/3/4 Generate, identity
  generation, Fill exactly-once, Generate exactly-once-per-coordinate,
  guarded-read composition, inline-HLSL composition, outer capture, ordinary
  call, conditional body, dense-literal parity, tensor consumption,
  generated-input matmul, multiple invocations, and stable replay.
- Stable replay executed with `--case sdslv-9de6ff0e1e3f80bc55565623`
  (`MultipleInvocationsRemainStable`) and returned the same PASS record.

Representative stable case IDs:

| Case | Stable ID |
| --- | --- |
| Fill exactly once | `sdslv-ae3fd75f5c93824a0838ccf7` |
| Generate once per coordinate | `sdslv-a916c6408a70a6dd9c6e6efe` |
| Guarded read composition | `sdslv-f94a148c6316515d09b5522a` |
| Generated matmul | `sdslv-fbe0459f3d60b8a8689db65f` |
| Stable replay | `sdslv-9de6ff0e1e3f80bc55565623` |

## Hardware evidence

Execution host:

- Date: 2026-07-11
- GPU: NVIDIA GeForce RTX 3070
- Driver: NVIDIA 596.36
- Vulkan loader instance version: 1.4.350
- Device API version: 1.4.329
- DXC: `C:\VulkanSDK\1.4.341.1\Bin\dxc.exe`
- Native host: `out/prometheus/native/sdslv_test_host.exe`

Representative execution cases:

| Case | Shape | Kind | Binder order | Workgroup | Dispatch | Expected / actual |
| --- | --- | --- | --- | --- | --- | --- |
| `FillExactlyOnceSemantics` | `[2,2]` | Fill | n/a | `1x1x1` | `1x1x1` | all `7u`; PASS |
| `GenerateExactlyOncePerCoordinate` | `[2,3]` | Generate | `i, j` | `1x1x1` | `1x1x1` | `100,201,302,410,511,612`; PASS |
| `GuardedReadGenerateBody` | `[4]` | Generate | `i` | `1x1x1` | `1x1x1` | `5,7,11,99`; PASS |
| `Rank4CoordinateEncoding` | `[2,2,2,3]` | Generate | `a, b, c, d` | `1x1x1` | `1x1x1` | final axis contiguous; PASS |
| `MatrixMultiplicationUsingGeneratedOperands` | `A[2,3] B[3,4] C[2,4]` | Generate + tensor | `i,k` / `k,j` | `1x1x1` | `1x1x1` | `10,14,17,5 / 25,29,38,14`; PASS |
| `MultipleInvocationsRemainStable` | `[2]` | Generate | `i` | `4x1x1` | `2x1x1` | replay-stable PASS |

The dedicated suite ran green end-to-end and the replay command:

```text
go run ./cmd/oct sdslv test Examples/SDSL-V/M33b/TensorConstruction.sdslvtest --case sdslv-9de6ff0e1e3f80bc55565623
```

returned a stable PASS record.

## DXC / SPIR-V evidence

Representative proof source:

- `Examples/SDSL-V/M33b/TensorConstructionProofs.sdslv`

Compiled entries and SPIR-V sizes:

| Entry | Scenario | SPIR-V bytes |
| --- | --- | ---: |
| `FillProof_CS` | Fill | 184 |
| `GenerateRank2Proof_CS` | rank-2 Generate | 200 |
| `GenerateRank4Proof_CS` | rank-4 Generate | 200 |
| `GeneratedMatmulProof_CS` | generated-input matmul | 200 |

`spirv-val` accepted all four outputs. The emitted HLSL showed:

- flat local ndarray storage (`uint values[...]`, `uint A[...]`, `uint B[...]`,
  `uint C[...]`);
- one `__sdslv_fill_*` temporary for Fill;
- one `__sdslv_tensor_offset_*` temporary per generated destination site;
- no placeholder comments and no runtime shape machinery;
- Vulkan-targeted DXC invocation with `-spirv -T cs_6_0 -fspv-target-env=vulkan1.0`.

## Boilerplate migration

Two real proof fixtures now use M33b construction instead of manual payload
spelling:

| Fixture | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| `Examples/SDSL-V/M33a/NDArrayComputeProof.sdslv` | 13 lines | 8 lines | 5 lines |
| `Examples/SDSL-V/M32b2/TensorExecution.sdslvtest` | 256 lines | 251 lines | 5 lines |

The migrated rank-4 proofs now use:

```sdslv
Generate[b, h, i, j](b * 1000u + h * 100u + i * 10u + j)
```

which preserves the same observable row-major values while removing literal
initialization boilerplate.

## Regression matrix

Green evidence preserved:

- `go test ./internal/source`
- `go test ./internal/diagnostic`
- `go test ./internal/sdslv/...`
- `go test ./cmd/oct`
- `go test ./internal/... ./cmd/oct`
- `go run ./tools/prometheus_native_manifest -check`
- `bash -n internal/prometheus/native/build_linux.sh`
- `git diff --check`
- M29 directory execution suite
- M30 TestInput suite
- M31b flow stack suite
- M32b2 tensor suite
- M33a ndarray suite
- M33b construction suite
- M33a and M33b fixture corpus tests
- representative DXC/SPIR-V compilation and validation

`go test ./cmd/oct` continues to cover the six production SGEMM source/HLSL
checks, including the “no flow dispatcher overhead” guard on the production
shader set.

## Remaining limitations

M33b intentionally stops at fixed-shape ndarray value construction:

- no shape inference
- no broadcasting
- no dynamic shape or reshape
- no alternate layouts
- no whole-value assignment construction outside typed declaration init
- no M33c storage-class or resource-construction work

## Handoff

M33c remains untouched. Any follow-up should start from the now-complete M33b
contracts rather than reopening Fill/Generate semantics, row-major traversal,
or hardware proof.
