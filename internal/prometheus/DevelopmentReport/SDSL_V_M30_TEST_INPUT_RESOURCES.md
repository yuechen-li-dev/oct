# SDSL-V M30 — fixed test input resources

M30 adds one compiler-owned read-only test input resource to `.sdslvtest`.
The path remains narrow and preserves the M29 ownership split:

- parser and validator own `TestInput*` attributes and `TestInput.*[index]`;
- validated metadata owns canonical 32-bit payload words and spans;
- VD-MIR owns one hidden backend-neutral resource contract;
- shared HLSL owns binding `1` decoding from `StructuredBuffer<uint>`;
- the manifest serializes input metadata only;
- the native host owns Vulkan allocation, upload, descriptor writes, and cleanup.

## Public test-only surface

`.sdslvtest` accepts:

- `[TestInputBool(...)]`
- `[TestInputInt(...)]`
- `[TestInputUInt(...)]`
- `[TestInputFloat(...)]`

Inside a test body the compiler exposes:

- `TestInput.Bool[index]`
- `TestInput.Int[index]`
- `TestInput.UInt[index]`
- `TestInput.Float[index]`
- `TestInput.Length`

This surface exists only in `.sdslvtest`. A function may declare at most one
`TestInput*` attribute. Fact and Theory both allow it. Theory rows share one
payload per function. `TestInput` is not first-class, cannot be rebound or
mutated, and typed members may only be indexed.

## Canonical payload encoding

Validated metadata stores one 32-bit word per element:

- `bool`: `0` or `1`
- `i32`: exact two's-complement bits
- `u32`: exact bits
- `f32`: IEEE-754 bits from `math.Float32bits`

Payload words are compiler authority. Stable case IDs do not include payload
contents, so payload edits do not perturb replay identity.

## Hidden execution contract

The test program always uses descriptor set `0`.

- binding `0`: existing writable result storage buffer
- binding `1`: compiler-owned read-only test input storage buffer

The shared HLSL test emitter always declares binding `1` as
`StructuredBuffer<uint> __sdslv_test_input`. Typed reads decode from that raw
word buffer with ordinary shared indexed-resource emission:

- `uint`: direct word
- `int`: `asint(word)`
- `float`: `asfloat(word)`
- `bool`: `word != 0u`

`TestInput.Length` lowers to a compiler-owned scalar constant. Indexed access
still lowers through the ordinary VD-MIR resource index path, so guarded reads
exercise the real M27 lowering and do not use a test-only fake path.

## Manifest and host policy

The manifest now carries per-case input metadata:

- `test_input_binding`
- `test_input.abi_version`
- `test_input.kind`
- `test_input.element_count`
- `test_input.payload_words`

The compiler does not consume the serialized manifest. The native host parses
and validates this metadata, allocates a host-visible storage buffer, uploads
the canonical payload words, and binds that buffer at set `0`, binding `1`.

No-input tests stay compatible through a documented dummy allocation policy:
the host allocates and binds one zero word even when logical input length is
zero. Binding `1` is never omitted from the descriptor set layout.

## Cleanup and portability

The native host now owns two buffer lifetimes instead of one and destroys both
input and result resources on success and on every failure path before device
teardown. VD-MIR still carries only shader-execution semantics, so future
Godot or other execution hosts can reuse the same payload contract without
Vulkan handles or process metadata leaking into compiler IR.

## Permanent test inventory

The committed M30 executable suite is
`examples/SDSL-V/M30/FixedTestInputResources.sdslvtest` and currently contains
seven hardware-executed Facts:

- `sdslv-562ebcfc3fd535b10496774c` — `FloatInputReads`
- `sdslv-7c9d302e5c0927838b7a0826` — `BoolInputReads`
- `sdslv-8d2b31d5ec4be844a645dd48` — `GuardedReadUsesFallback`
- `sdslv-a5f866221f90199dace4a0df` — `GuardedReadUsesSource`
- `sdslv-b71c5378d98c97d63a061800` — `NoInputCompatibility`
- `sdslv-bbe7af6af6e2082c36431fd6` — `IntInputReads`
- `sdslv-c026387628e85ee4fcd9aa65` — `GuardedReadBoundaryCases`

Typed payload coverage is explicit:

- Bool payload: `true`, `false`
- Int payload: `-7`, `9`
- UInt payload: `5u`, `7u`, `11u`
- Float payload: `-0.0`, `1.5`

The float case is bit-sensitive rather than value-only: it reads
`TestInput.Float[0u]`, routes the value through inline `asuint`, and asserts
the exact IEEE-754 bit pattern `2147483648u` for negative zero.

Boundary-index coverage is explicit:

- index `0u` reads the first UInt element
- final valid index `TestInput.Length - 1u` reads the last UInt element
- index equal to `TestInput.Length` selects the fallback branch

Guard behavior coverage is explicit:

- guard true selects the real binding-1 resource value in
  `GuardedReadUsesSource`
- guard false selects the fallback in `GuardedReadUsesFallback`
- the boundary case mixes valid and invalid indices in one real lowered body

No-input compatibility is explicit:

- `NoInputCompatibility` declares no `TestInput*` attribute
- the host still binds descriptor set `0`, binding `1`
- the host allocates one zero dummy word while logical length remains `0`
- exact replay of that no-input case is deterministic and passes repeatedly

## Compiler-negative evidence

The validator negative matrix is permanently covered in
`internal/sdslv/validate/diagnostic_test.go`. It now exercises:

- `TestInput` outside `.sdslvtest`
- `TestInput` on a non-test function
- duplicate `TestInput` attributes
- conflicting `TestInput` kinds
- wrong scalar literal type
- nonconstant payload values
- unsupported payload values
- access without payload
- unsupported members
- member/payload kind mismatch
- invalid index type
- first-class passing/returning rejection
- mutation rejection

Stable case identity preservation under payload edits is covered in
`internal/sdslv/test/discovery_test.go` and proves payload words are excluded
from stable-ID derivation.

## Manifest and native-negative evidence

Manifest/native negative coverage is permanently exercised in
`internal/sdslv/test/host_test.go`.

Malformed manifest coverage executes the real native host against emitted M30
artifacts and rejects:

- invalid input ABI version
- invalid value kind
- element-count/payload-count mismatch
- oversized payload
- truncated payload / malformed JSON

Descriptor and cleanup evidence is split deliberately:

- executable replay proves the no-input dummy-buffer path is deterministic
- source guards assert two descriptor layout bindings, two descriptor writes,
  checked result/input size multiplication, preserved M29 result classifications,
  and centralized destruction of both input and result allocations

The native host does not expose a public injection seam for every
post-allocation Vulkan failure stage. For those paths the maintained source
guards are the permanent regression mechanism.

## Architecture guards

M30 now has maintained source guards proving:

- no new `TestInput` declaration AST or descriptor-binding AST was added
- no parser-owned arbitrary `TestInput` resource declaration grammar exists
- no TestInput-as-function implementation exists
- no local-array substitution exists in generated HLSL
- no test-specific guarded-read emitter exists
- the compiler test compiler does not consume manifest DTOs
- VD-MIR does not carry Vulkan handles or host manifest metadata

## Validation and hardware evidence

The following validation lanes were rerun after the permanent hardening pass:

- `go test ./internal/source`
- `go test ./internal/diagnostic`
- `go test ./internal/sdslv/...`
- `go test ./cmd/oct`
- `go test ./internal/... ./cmd/oct`
- `go test ./internal/sdslv/validate -run TestSdslvM29DiagnosticCodesAndSpans`
- `go test ./internal/sdslv/test -run TestSdslvNativeHost`
- `go test ./internal/sdslv/test -run TestSdslvStableCaseReplayWithTestInput`
- `go run ./tools/prometheus_native_manifest -check`
- `bash -n internal/prometheus/native/build_linux.sh`
- canonical Windows native build via `internal/prometheus/native/build_windows.cmd`
- `go run ./cmd/oct sdslv test examples/SDSL-V/M29/InlineHlslFacts.sdslvtest`
- `go run ./cmd/oct sdslv test examples/SDSL-V/M29/RealAssertions.sdslvtest`
- `go run ./cmd/oct sdslv test examples/SDSL-V/M30/FixedTestInputResources.sdslvtest`
- `go run ./cmd/oct sdslv test examples/SDSL-V/M30/FixedTestInputResources.sdslvtest --case sdslv-a5f866221f90199dace4a0df`

Fresh hardware evidence on `2026-07-10`:

- GPU: `NVIDIA GeForce RTX 3070`
- NVIDIA driver: `596.36`
- Vulkan loader: `1.4.350`
- Vulkan device API: `1.4.329`
- input ABI version: `1`
- result ABI version: `1`
- workgroup size: `[1,1,1]`
- dispatch groups: `[1,1,1]`

M29 regression evidence remained green on the same host:

- `sdslv-11e3deb3d1ad94f0071f3d8d`
- `sdslv-5664efcb0ab3deb7eb8c871b`
- `sdslv-a20bf18c1aa6672e75d2b267`
- `sdslv-ea0387cf37037ceec9e4083d`
- `sdslv-39ebeaa8b059a087462c251e`
- `sdslv-9308174bfc8d78166b45d302`
- `sdslv-e82550ab5f8c124eadf272c3`

## Acceptance conclusion

M30 is accepted complete:

- typed fixed `TestInput` syntax is parser/validator owned
- canonical payload words are validated metadata authority
- VD-MIR carries the hidden backend-neutral input contract
- shared HLSL emits the fixed read-only binding-1 resource
- the native host allocates, uploads, binds, and cleans up the input buffer
- ordinary indexed-resource and M27 guarded-read lowering are exercised
- no-input compatibility is real and deterministic
- existing M29 replay identities and suites remain green
- the permanent negative/compiler/native guard matrix exists
- fresh RTX 3070 execution evidence exists for the committed M30 suite

## Non-goals

M30 does not add:

- user-authored descriptor declarations
- multiple input resources
- writable input
- vectors, matrices, records, arrays, textures, samplers, or images
- generic `TestInput<T>`
- host-owned fixture semantics
