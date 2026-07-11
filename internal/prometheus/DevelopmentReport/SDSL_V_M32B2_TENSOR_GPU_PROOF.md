# SDSL-V M32b.2: tensor GPU execution proof and SGEMM parity

## Outcome

M32b.2 closes the execution proof for indexed tensor notation on the real
Vulkan path:

```text
tensor source
  -> validated tensor semantics
  -> tensor VD-MIR
  -> shared HLSL
  -> SPIR-V
  -> native Vulkan execution
  -> exact assertion results
```

The dedicated executable suite is
`examples/SDSL-V/M32b2/TensorExecution.sdslvtest`. It stays within the settled
M29-M32b.1 ownership split:

- parser/validator own tensor/test syntax and static metadata;
- lowering owns `vdmir.TensorAssign`/`TensorReductionExpr`;
- shared HLSL owns tensor loop/address/materialization emission;
- `.sdslvtest` owns compilation groups, manifests, replay IDs, and ABI v1;
- the native host owns Vulkan execution, readback, and deterministic JSON.

## Proof matrix

Fresh green hardware cases on 2026-07-11:

- `sdslv-7c8e328918686f81732e35fb` — `Rank1ElementwiseMap`
- `sdslv-bbe0730c475e2980a4b2dfc9` — `Rank2ElementwiseRowMajorLayout`
- `sdslv-28f3d975fd337c949f1fb243` — `DotProductAndTypedZeroFloat`
- `sdslv-a5ea246a528e60c65cef522d` — `TypedZeroIntAndUInt`
- `sdslv-19b3c45f9c7db2e846cacd03` — `Rank2Matmul`
- `sdslv-5c23546e922c83f3164142a4` — `Rank3BatchedMatmul`
- `sdslv-0aae48740302cd090dc728fa` — `Rank4RowMajorLayout`
- `sdslv-7d22c3825256ab2afc2616b1` — `MultipleReductionIndicesConvolution`
- `sdslv-0607aaf11f78e8edec30fef5` — `CompoundAddTensorAssignment`
- `sdslv-55069cfec3a9a5f8d6e262f2` — `TensorGuardedReadsUseTestInput`
- `sdslv-d9f74fcbff5f6c48ef2cc96b` — `TensorInlineHlslExpression`
- `sdslv-7229bbd93b21f770c49a0921` and `sdslv-c84083c53759173da503b8fb` — `TheoryRowsReuseTensorBody` rows 0/1
- `sdslv-a4085ecfa1d43a28278cbcfc` — `MultipleInvocationsKeepTensorStatePrivate`
- `sdslv-783e8b0535804de633c44695` — `SgemmStyleTensorParity`

Covered contract points:

- rank-1, rank-2, rank-3, and rank-4 fixed-array tensors execute on hardware;
- free-index extents come from the destination;
- `Sum[...]` uses typed zero for `f32`, `i32`, and `u32`;
- reduction execution respects declared reduction domains;
- compound tensor `+=` contributes the old destination exactly once;
- guarded reads compose with tensor lowering and real M30 `TestInput`;
- inline HLSL composes through the shared expression materializer;
- Theory rows reuse one lowered body and replay as distinct stable cases;
- multi-invocation execution is deterministic and keeps tensor locals private;
- SGEMM-style register-tile contraction matches the explicit-loop equivalent.

## Row-major layout

Rank-general fixed arrays keep the M32b.1 compiler-owned linearization:

```text
offset = (((i0 * D1 + i1) * D2 + i2) ...)
```

The rank-4 layout proof uses shape `[2, 2, 2, 3]` and uniquely encoded values.
Observed address witnesses include:

- `[0,0,0,0] -> 0`
- `[0,0,1,2] -> 12`
- `[0,1,0,2] -> 102`
- `[1,0,1,2] -> 1012`
- `[1,1,1,2] -> 1112`

That confirms no nested-bracket HLSL semantic dependence, no overlap, and no
missing logical elements.

## Order and exactly-once evidence

Permanent compiler guards added in this slice:

- `validate.ValidatedTensorAssignments` now honors `.sdslvtest`/fixture source
  kind when tensor metadata is requested directly.
- `lower.ModuleForTests` now computes tensor metadata against a test-aware copy
  of the module while preserving the real `Assert` statements for dedicated
  assert lowering.
- shared-emitter regression coverage now checks free-index order `y, x` and
  reduction order `ky, kx` in emitted HLSL.
- `.sdslvtest` lowering coverage now proves tensor bodies and hidden
  `TestInput` resources coexist in one lowered test function.

Existing M32b.1 one-shot materialization tests remain the compiler proof for:

- one destination address materialization for tensor `+=`;
- one destination read and one final write;
- left-to-right prelude order;
- guarded-read and inline-HLSL shared materialization;
- no tensor placeholders in emitted HLSL.

## SPIR-V and hardware evidence

Fresh Windows hardware evidence on 2026-07-11:

- Vulkan loader: `1.4.350`
- Device API: `1.4.329`
- GPU: `NVIDIA GeForce RTX 3070`
- Driver: `596.36`

The dedicated suite produced two real compilation groups:

- `group-0` (`[1,1,1]` workgroup, `[1,1,1]` dispatch for the rank/map/reduction/layout/guarded/theory/SGEMM cases)
  - HLSL: `55,417` bytes
  - SPIR-V: `41,740` bytes
- `group-1` (`[4,1,1]` workgroup, `[2,1,1]` dispatch for the multiple-invocation proof)
  - HLSL: `3,660` bytes
  - SPIR-V: `3,552` bytes

Because `.sdslvtest` groups cases by validated workgroup size, the
representative rank-1 map, rank-3 batched matmul, rank-4 layout, and SGEMM
parity proofs share the same `group-0` SPIR-V artifact. This is expected and
preserves the M29 grouping contract rather than forcing per-case recompilation.

Stable replay evidence:

- whole-suite replay via `oct sdslv test examples/SDSL-V/M32b2/TensorExecution.sdslvtest`
- selected-case replay via
  `oct sdslv test examples/SDSL-V/M32b2/TensorExecution.sdslvtest --case sdslv-783e8b0535804de633c44695`
- deterministic repeated host replay for
  `MultipleInvocationsKeepTensorStatePrivate`

## Validation lanes run

- `go test ./internal/source`
- `go test ./internal/diagnostic`
- `go test ./internal/sdslv/...`
- `go test ./cmd/oct`
- `go test ./internal/... ./cmd/oct`
- `go run ./tools/prometheus_native_manifest -check`
- `bash -n internal/prometheus/native/build_linux.sh`
- `git diff --check`
- `go run ./cmd/oct sdslv test examples/SDSL-V/M32b2/TensorExecution.sdslvtest`
- `go run ./cmd/oct sdslv test examples/SDSL-V/M32b2/TensorExecution.sdslvtest --case sdslv-783e8b0535804de633c44695`

## Limitations

- This is an execution-proof milestone, not a production SGEMM migration.
  Prometheus production SGEMM sources remain unchanged.
- `.sdslvtest` SPIR-V size is reported per compilation group, not per logical
  case, because group sharing is part of the M29 contract.
- No tensor templates, broadcasting, dynamic tensors, automatic tiling, or
  cooperative-matrix lowering begin here.
