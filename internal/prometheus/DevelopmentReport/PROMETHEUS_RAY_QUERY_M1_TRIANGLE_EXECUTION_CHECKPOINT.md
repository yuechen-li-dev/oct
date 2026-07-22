# Prometheus Ray Query M1 — Triangle execution checkpoint

Status: meaningful progression.  This is a real hardware traversal checkpoint,
not the complete analytic-sphere renderer qualification.

## Milestone identity

The historical roadmap reserves `P17` for PVT evidence tuning.  Ray-query work
therefore uses the explicit `PROMETHEUS_RAY_QUERY_M0` /
`PROMETHEUS_RAY_QUERY_M1` namespace.  The M0 report was renamed from
`P17_M0_RAY_QUERY_TOOLCHAIN_ADMISSION.md` to
`PROMETHEUS_RAY_QUERY_M0_TOOLCHAIN_ADMISSION.md`; no unrelated historical
milestone or roadmap entry was changed.

## Verified M0 starting state

M0 remains coherent and reproducible:

- SDSL-V has `readonly acceleration_structure` and the compute-only,
  compiler-owned `RayQueryAny(...)` intrinsic;
- the production M0 module is
  `shaders/sdslv/production/rayquery/ray_query_capability_probe.sdslv`;
- its DXC route is `-spirv -T cs_6_5 -fspv-target-env=vulkan1.3 -O3`;
- `spirv-val` accepts the result under the settled Vulkan 1.4 contract;
- disassembly contains `OpCapability RayQueryKHR`,
  `OpTypeAccelerationStructureKHR`, `OpTypeRayQueryKHR`,
  `OpRayQueryInitializeKHR`, `OpRayQueryProceedKHR`, and
  `OpRayQueryGetIntersectionTypeKHR`;
- the runtime conditionally enables buffer-device-address,
  acceleration-structure, and ray-query features and loads the required AS
  entry points without making optional ray query a global compute requirement.

The earlier report named `oct sdslv compile`, which is not a command in this
tree.  It has been corrected to the reproducible canonical route:

```text
go run ./cmd/oct sdslv compile-spv <source.sdslv> -o <temporary-output.spv> --validate --require-spirv-val
```

## Implemented execution baseline

`reactor_vulkan_ray_query.c` owns a narrow triangle-scene execution route.  It
does not introduce a second global Vulkan owner or an RT pipeline/SBT path.

The public checkpoint ABI is deliberately small:

```text
create triangle scene -> opaque scene id
probe retained scene -> boolean hardware hit and AS/device-address evidence
destroy scene
```

The scene owns a host-visible coherent, device-addressable vertex buffer; one
triangle BLAS and one identity-instance TLAS; TLAS instance storage; a mapped
evidence buffer; and its descriptor/pipeline objects.  It builds BLAS and TLAS
in separate submit-and-wait phases.  Scratch lives only through its completed
build submission.  Scene destruction orders TLAS, instance storage, BLAS, and
geometry backing storage safely; runtime destruction retires any still-live
scenes before destroying the sole device owner.

The compute pipeline uses production shader asset 54, not a private HLSL or
handwritten SPIR-V route.  Binding zero is
`VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR`; binding one is the mapped
storage evidence buffer.  The M0 canonical ray is `(0,0,0) + t(0,0,1)` over
`[0,100]`, so the execution checkpoint uses a triangle at `z=2` for its hit
witness and a separate off-axis triangle for its miss witness.

## RTX 3070 evidence

On the owner host, `vulkaninfo --summary` reports an NVIDIA GeForce RTX 3070,
driver 596.36.0.0, Vulkan device API 1.4.329.  With
`PROMETHEUS_VK_VALIDATION=1`, the focused native corpus passed:

```text
PrometheusReactor_RayQueryAdmissionIsOptionalAndCompleteWhenEnabled
PrometheusReactor_RayQueryTriangleBlasTlasProbeUsesPersistentHardwareTraversal
PrometheusReactor_RayQueryTriangleScenesRemainIsolated
```

The execution test proves all of the following on the real device:

- nonzero vertex, BLAS, and TLAS device addresses;
- successful triangle BLAS and identity-instance TLAS builds;
- acceleration-structure descriptor binding;
- production SDSL-V compute dispatch and mapped readback of a hit;
- a warm repeat using the same BLAS and TLAS addresses;
- no new Vulkan validation errors during the build/traversal route;
- two live scenes produce independently correct hit and miss results;
- destroyed scene handles are rejected.

## Deliberate boundary and next block

This does **not** yet provide a raw committed-hit record, arbitrary ray input,
triangle barycentrics, a procedural-AABB BLAS, analytic sphere candidate
commitment, shadows, camera rendering, CPU oracle, canonical image, or timing
benchmark.  `RayQueryAny(...)` remains only an M0 boolean convenience and is
not incorrectly presented as exact procedural geometry handling.

The current renderer has no CPU production traversal and no RT-pipeline/SBT
path.  Its bounded risk is equally explicit: a boolean probe cannot diagnose
candidate identity, nearest-hit ordering, or a procedural AABB false positive.

The next sound implementation block is SDSL-V's stateful inline-query surface:
opaque query values, candidate/committed identity and distance accessors, and
explicit procedural commitment.  That is required before a procedural AABB can
be distinguished from an analytic sphere surface or used as a shadow occluder.

## Deferred experiment register

| Experiment | Claimed advantage | Smallest falsification test | Required baseline |
| --- | --- | --- | --- |
| Additional analytic primitives | richer procedural scenes | candidate identity plus exact root agreement | procedural-sphere raw-hit corpus |
| Instance transforms | reusable BLAS geometry | transformed triangle agrees with CPU oracle | identity-instance TLAS |
| BLAS compaction | lower persistent AS memory | compacted and uncompacted traversal agree | persistent uncompressed BLAS |
| TLAS refit versus rebuild | cheaper updates | update never changes stable static-scene result | static rebuild path |
| Full RT pipeline/SBT | alternate execution topology | same raw-hit contract without regressions | compute ray-query route |
| Multi-bounce path tracing | indirect-light realism | deterministic one-bounce numerical replay | direct-light image route |
| Adaptive sample budget | lower render cost | bounded error against fixed sampling | deterministic one-sample image |
| Aurelian scene integration | shared scene inputs | identical canonical scene identity | standalone native scene ABI |

## Checks run

```text
go test ./internal/sdslv/lower ./internal/sdslv/emit/hlsl ./internal/sdslv/toolchain ./internal/sdslv/validate
go run ./cmd/oct sdslv check internal/prometheus/shaders/sdslv/production/rayquery/ray_query_capability_probe.sdslv
go run ./cmd/oct sdslv compile-spv internal/prometheus/shaders/sdslv/production/rayquery/ray_query_capability_probe.sdslv -o <temporary-output.spv> --validate --require-spirv-val
go run ./tools/prometheus_native_manifest -check
cmd /c internal\prometheus\native\build_windows.cmd
PROMETHEUS_VK_VALIDATION=1 out\prometheus\native\marionette_tests.exe PrometheusReactor_RayQuery
```

No commit, tag, push, release, or pull request was created.
