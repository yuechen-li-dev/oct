# SDSL-V M33a: first-class fixed-shape ndarray types

M33a completes the user-facing fixed-shape tensor value model for SDSL-V.
Programs can now describe rank-N static row-major tensor values directly with:

```sdslv
ndarray<ElementType, [Extent0, Extent1, ...]>
```

instead of recursive nested fixed-array boilerplate.

## User-facing surface

Supported examples:

- `ndarray<u32, [8u]>`
- `ndarray<f32, [4u, 4u]>`
- `ndarray<u32, [2u, 2u, 2u, 3u]>`

Dense literals are flat and source ordered:

```sdslv
let input: ndarray<u32, [2u, 2u]> = [
    1u, 2u,
    3u, 4u
];
```

The literal payload is interpreted in compiler-owned row-major order. Exact
element count is required, element types are checked exactly, and nested
literal syntax remains deferred in M33a.

## Semantic model

`ndarray` is a distinct first-class type identity. It is not a public alias for
recursive `array<...>` syntax.

Type equality requires:

- equal element type;
- equal ordered shape;
- equal rank (derived from shape length);
- equal layout.

So:

- `ndarray<u32, [2u, 3u]> != ndarray<u32, [3u, 2u]>`
- `ndarray<u32, [2u, 3u]> != array<array<u32, 3u>, 2u>`

No implicit conversion, flattening coercion, reshape, or broadcasting is added.

## Compiler ownership

The compiler now retains ndarray metadata through:

- parser/AST type shape metadata with exact source spans for the keyword,
  element type, full shape list, shape delimiters, and each extent;
- validator-owned static extent and total-count checks;
- backend-neutral VD-MIR ndarray type/literal nodes;
- shared row-major `IndexNExpr` linearization;
- shared flat HLSL fixed-shape storage emission.

Initial layout contract:

- layout: row-major only;
- rank: derived from shape length only;
- storage: one flat fixed-size HLSL array;
- indexing: existing rank-general indexing path.

## Validation and diagnostics

The dedicated invalid ndarray corpus now covers:

- missing shape;
- empty shape;
- nonconstant extent;
- noninteger extent;
- zero extent;
- negative extent;
- extent overflow;
- total-size overflow;
- unsupported element type;
- wrong index count;
- wrong index type;
- too few literal elements;
- too many literal elements;
- wrong literal element type;
- nested-array/ndarray implicit conversion in both directions;
- shape mismatch assignment;
- immutable mutation;
- tensor rank mismatch boundary behavior;
- deferred nested literals.

Each invalid fixture verifies phase, diagnostic code, and exact source location.

## Tensor integration

`ndarray` now participates in tensor notation through the same tensor-shape
provenance path used for nested fixed arrays. No ndarray-only tensor lowering
path was introduced.

Covered tensor cases:

- elementwise copy;
- rank-2 matrix multiplication;
- rank-3 batched matmul;
- rank-4 copy;
- reduction extent inference from ndarray axes;
- destination/source shape provenance from ndarray shape metadata.

## Backend lowering

Lowering keeps ndarray as distinct semantic type metadata while reusing the
shared fixed-shape row-major addressing path.

For shape `[D0, D1, ..., Dn-1]` and indices `[i0, i1, ..., in-1]`, the linear
offset remains:

```text
(((i0 * D1 + i1) * D2 + i2) ...)
```

The shared HLSL emitter produces one flat array and source-ordered scalar
stores for dense literals. Example:

```hlsl
uint input[24];
input[0] = 1u;
...
input[23] = 10u;
```

No nested HLSL bracket syntax is used as semantic authority.

## Dedicated corpora and tests

Added dedicated fixture corpora:

- `internal/sdslv/testdata/language/m33a-valid`
- `internal/sdslv/testdata/language/m33a-invalid`

Added focused parser/type/backend coverage for:

- ndarray parsing and dense literals;
- exact ndarray source spans;
- type identity and ordered-shape equality;
- whole-value assignment under existing exact-type semantics;
- row-major literal/source ordering;
- distinct ndarray lowering and flat HLSL emission.

## Hardware proof

Fresh M33a ndarray execution ran on the existing Windows native SDSL-V host.

Hardware:

- GPU: `NVIDIA GeForce RTX 3070`
- NVIDIA driver: `596.36`
- Vulkan loader: `1.4.350`
- Vulkan device API: `1.4.329`
- Device type: discrete GPU

Dedicated ndarray `.sdslvtest` suite:

- path: `Examples/SDSL-V/M33a/NDArrayExecution.sdslvtest`
- workgroup geometry: `[1, 1, 1]`
- dispatch geometry: `[1, 1, 1]`

Stable case IDs:

- `sdslv-8cb88b810cacae7533404f81` — `Rank1DenseLiteralIndexing`
- `sdslv-df152e0beebd14a253e06737` — `Rank2RowMajorLayout`
- `sdslv-e9a528967df7c8cea3949aac` — `Rank3TensorUse`
- `sdslv-5150d466a942e4841a4036b8` — `Rank4TensorCopy`
- `sdslv-d9314618b866c89edbdcd47d` — `Rank2Matmul`
- `sdslv-dce23e4959fbf86e5a5a364d` — `ElementAssignment`
- `sdslv-4f43897e0cebbc63f163ee9b` — `DenseLiteralSourceOrder`

Observed result:

- expected/actual assertion values matched for every case;
- all recorded host results returned `status:"PASS"`;
- row-major dense literal ordering, indexing, tensor contraction, and mutation
  executed successfully on real hardware.

Representative direct SPIR-V proof:

- `Examples/SDSL-V/M33a/NDArrayComputeProof.sdslv`
- `oct sdslv compile-spv ...`
- entry: `NDArrayComputeProof_CS`
- target: `cs_6_0`, `-spirv`, `vulkan1.0`, `-O3`

## Proof-source cleanup

At least one existing M32b.2 proof source now uses ndarray directly.

Two committed cases in `Examples/SDSL-V/M32b2/TensorExecution.sdslvtest` were
refactored:

- `Rank3BatchedMatmul`
- `Rank4RowMajorLayout`

Example source reduction:

Before:

```sdslv
array<array<array<array<u32, 3u>, 2u>, 2u>, 2u>
```

After:

```sdslv
ndarray<u32, [2u, 2u, 2u, 3u]>
```

The proof cases now express the real tensor shape directly and use dense
row-major literals instead of repetitive element-by-element setup.

## Compatibility

M33a preserves the completed M29-M32 ownership boundaries:

- nested fixed arrays still parse, validate, lower, and execute;
- ordinary arrays remain unchanged;
- tensor lowering still accepts nested fixed arrays and ndarray through shared
  shape metadata;
- production SGEMM shader sources remain unchanged;
- native/test ABI remains unchanged.

Validated regression lanes:

- `go test ./internal/sdslv/...`
- `go run ./cmd/oct sdslv test Examples/SDSL-V/M29`
- `go run ./cmd/oct sdslv test Examples/SDSL-V/M30/FixedTestInputResources.sdslvtest`
- `go run ./cmd/oct sdslv test Examples/SDSL-V/M31b/FlowStacks.sdslvtest`
- `go run ./cmd/oct sdslv test Examples/SDSL-V/M32b2/TensorExecution.sdslvtest`
- six production SGEMM `sdslv check` passes

## Non-goals retained

Still deferred after M33a:

- dynamic shapes;
- runtime-rank values;
- reshape;
- slicing;
- broadcasting;
- transpose syntax;
- sparse tensors;
- storage classes;
- tensor templates;
- production SGEMM migration to ndarray.

## M33b handoff

M33b should build only on the settled first-class ndarray metadata:

- element type;
- ordered static shape;
- rank derived from shape length;
- row-major layout contract;
- exact dense literal payload order;
- tensor-shape provenance through the shared tensor path.

Do not collapse ndarray back into nested arrays as semantic identity while
building M33b features.
