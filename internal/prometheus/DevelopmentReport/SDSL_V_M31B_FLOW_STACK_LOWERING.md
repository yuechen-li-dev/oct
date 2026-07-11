# SDSL-V M31b - bounded flow-stack lowering

M31b converts the M31a `ValidatedFlow` handoff to executable VD-MIR and shared
HLSL. State bodies use the normal lowering path; M31a's IDs, explicit
terminators, return successors, depth contract, spans, and barrier metadata are
preserved without source reparsing or backend target lookup.

| Flow shape | Lowering |
| --- | --- |
| no transitions | direct structured HLSL |
| goto/finish only | state dispatcher |
| push/pop present | dispatcher + fixed return stack |

The dispatcher uses declaration-order `uint` state IDs and `0xffffffffu` for
the non-colliding private completion sentinel. Push writes M31a's `ReturnTo`;
pop resumes the saved state LIFO. Goto does not alter the stack. Finish exits
only the dispatcher, preserving ordinary test-body execution and ABI-v1 test
epilogues.

The shared emitter reports impossible VD-MIR boundary metadata, including bad
or duplicate state IDs, bad targets, missing push stack contracts, zero-depth
push/pop, and unknown terminators. M31a remains owner of source-invalid
diagnostics and barrier conservatism.

Legacy flows stay flattened: no dispatcher, state variable, stack array, stack
pointer, or new runtime branch. M31b deliberately excludes inlining, cloning,
tail-call elimination, stack compression, and dispatcher elimination.

## Source mapping

Dispatcher-emitted HLSL now leaves explicit generated comments for:

- flow dispatcher
- fixed stack contract
- state entry
- push/pop/goto/finish terminators
- generated stack push/pop operations

State and terminator markers retain original flow source spans. Dispatcher-level
markers fall back to the entry state's span when the outer flow span is not
present in the lowered statement envelope.

## Hardware evidence

Fresh M31b `.sdslvtest` execution used the existing verified native host at:

- `out/prometheus/native/sdslv_test_host.exe`

Fresh toolchain/runtime environment:

- DXC: `C:\VulkanSDK\1.4.341.1\Bin\dxc.exe`
- Vulkan loader: `1.4.350`
- Device: `NVIDIA GeForce RTX 3070`
- Device Vulkan API: `1.4.329`
- NVIDIA driver: `596.36`

Stable replay proof:

- `go run ./cmd/oct sdslv test examples/SDSL-V/M31b/FlowStacks.sdslvtest --list`
  produced deterministic stable IDs
- `TestSdslvM31bStableCaseReplay` replayed `NestedPushPopLifo` by stable ID and
  passed
- `TestSdslvM31bFlowStackNativeHostCasesAreDeterministic` reran every M31b case
  twice and observed byte-identical PASS JSON

Committed M31b GPU suite:

| Stable ID | Case | Workgroup / Groups | MaxStackDepth | Expected trace | Result |
| --- | --- | --- | --- | --- | --- |
| `sdslv-af914781029bf783b230886c` | `LinearFallthroughLegacy` | `1x1x1 / 1x1x1` | `0` | `123` | PASS |
| `sdslv-209d080eb86306d7c37e0674` | `GotoOnlyTransfer` | `1x1x1 / 1x1x1` | `0` | `132` | PASS |
| `sdslv-b0e778bfa153eac85a33dfba` | `FinishOnlyFlowPreservesEpilogue` | `1x1x1 / 1x1x1` | `0` | `12` | PASS |
| `sdslv-b9f0fe00c4a0e69f5878e7a2` | `OnePushPop` | `1x1x1 / 1x1x1` | `1` | `123` | PASS |
| `sdslv-96ac234a9ef201400f06895a` | `NestedPushPopLifo` | `1x1x1 / 1x1x1` | `2` | `12345` | PASS |
| `sdslv-a7ffe139cf75943a749da4e2` | `SharedSubflowCallerSpecificReturn` | `1x1x1 / 1x1x1` | `1` | `12345` | PASS |
| `sdslv-8533a9861deed00787cb6fb4` | `FinalStatePushReturnsToCompletion` | `1x1x1 / 1x1x1` | `1` | `123` | PASS |
| `sdslv-68439a714b4bafbd15e712fa` | `FinishWithNonemptyStack` | `1x1x1 / 1x1x1` | `1` | `123` | PASS |
| `sdslv-3470fd02efd743243bd51157` | `FlowFollowedByAssert` | `1x1x1 / 1x1x1` | `1` | `123` | PASS |
| `sdslv-7e7f43627065c655df706b65` | `MultipleInvocationsRemainDeterministic` | `4x1x1 / 2x1x1` | `1` | `123` | PASS |
| `sdslv-63bbb1737ed65be25d866ea7` | `TheoryRowFlow[0]` | `1x1x1 / 1x1x1` | `1` | `1231` | PASS |
| `sdslv-72e39963fd12663e2b2e1030` | `TheoryRowFlow[1]` | `1x1x1 / 1x1x1` | `1` | `1237` | PASS |
| `sdslv-ca7528c054e13fef90899d06` | `BarrierSubflowRemainsUniform` | `4x1x1 / 1x1x1` | `1` | `1234` | PASS |

`BarrierSubflowRemainsUniform` executes a real GPU workgroup barrier inside the
flow dispatcher. In `.sdslvtest`, the barrier is expressed through inline HLSL
because `WorkgroupMemoryBarrierWithSync()` remains restricted to compute-stage
functions by validator design; compiler-owned barrier metadata is still covered
by the dedicated compute-shader fixture
`internal/sdslv/testdata/language/m31b-valid/BarrierSubflow.sdslvvalid` plus
the validator/emitter flow tests.

## Regression coverage

Permanent focused coverage now includes:

- flow lowering to executable VD-MIR
- HLSL dispatcher/no-stack/fixed-stack shapes
- caller-specific push return emission
- pop LIFO resume emission
- final push to `FlowComplete`
- malformed VD-MIR flow rejection at the backend boundary
- `.sdslvtest` GPU execution, deterministic replay, theory rows, and epilogue
  preservation
- SGEMM HLSL regression proof that the six pre-M31 shaders still emit no
  dispatcher, state variable, stack pointer, or return-stack array

## Validation

Passed in this workspace:

- `go test ./internal/source`
- `go test ./internal/diagnostic`
- `go test ./internal/sdslv/...`
- `go test ./cmd/oct`
- `go test ./internal/... ./cmd/oct`
- `go run ./cmd/oct sdslv test examples/SDSL-V/M31b/FlowStacks.sdslvtest`
- `go run ./tools/prometheus_native_manifest -check`
- `bash -n internal/prometheus/native/build_linux.sh`
- `git diff --check`

Windows native rebuild from the current shell still fails before compilation of
M31b code with missing MSVC SDK headers (`stdint.h`, `stddef.h`). No native C
sources changed for M31b, so hardware proof used the existing verified host
binary instead of claiming a rebuild from an unsupported shell.
