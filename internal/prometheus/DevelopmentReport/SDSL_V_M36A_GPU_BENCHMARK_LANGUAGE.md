# SDSL-V M36a GPU benchmark language

`.sdslvbench` files describe repeatable GPU performance experiments.
`[Benchmark]` declarations use the compiler-owned benchmark model, separate
from `.sdslvtest` assertion cases and the M34a audit registry.

The declaration attributes are `[Benchmark]`, `[Warmup(N)]`,
`[Iterations(N)]`, `[DispatchGroups(X,Y,Z)]`, and optional
`[WorkgroupSize(X,Y,Z)]`. Warmup defaults to 10, iterations to 100, and
dispatch geometry is required. Parameters, non-void returns, assertions, and
test attributes are rejected. IDs are `sdslvbench-` plus a hash of normalized
source identity, name, and dispatch geometry; timing data and device metadata
are intentionally excluded.

Use `oct sdslv bench file.sdslvbench --list` to inspect manifests without
executing Vulkan, and `--case <stable-id>` for exact selection. `--json`
emits schema version 1 manifest data. Benchmark results are performance
observations and do not replace correctness tests.

Execution began in M36a with `tools/sdslv_benchmark_host`, a one-shot Godot
4.7 C# backend. M36b adds `octxiliary-kaiju-vulkan`, a typed Octxiliary
Vulkan witness selected with `oct sdslv bench ... --backend kaiju` or `auto`.
Godot's installed bindings explicitly make RenderingDevice unavailable under
`--headless`, so the Godot host deliberately launches without it in a
noninteractive 1x1 Forward+ window. It creates a local RenderingDevice,
pipeline, and compute dispatch outside the measured loop, warms up, returns
one raw sample per iteration, frees RIDs, writes its response, and exits. The
current Godot API does not provide a usable per-dispatch timestamp path, so
its timing is explicitly `synchronized_host_elapsed` (dispatch, submit, and
GPU synchronization), not GPU timestamp timing. The Kaiju sidecar returns real
Vulkan query-pool GPU timestamps. Go owns min/median/max for both.

RTX 3070 proof: `ScalarArithmetic` (`sdslvbench-91c86b9349e1ce473ec6640d`),
SPIR-V `1582e565d23f6217cdc5de56af97f4ab55aca7218584bd0f9d246840fca4346e`,
10 warmups and 100 iterations, reported min/median/max 39,100 / 42,300 /
377,400 ns with replay ID `sdslvbench-replay-5b0e91124ba84d74d6ad0d79`.

Each case now compiles as an isolated module, preventing independently declared
shader resource bindings from colliding. The supported Godot 4.7 M36a corpus is
resource-free plus ordinary scalar/vector storage buffers. Ndarray-generated
and tensor-generated modules remain valid SDSL-V/SPIR-V but are deferred after
Godot 4.7 Mono access-violated in `ComputePipelineCreate`; native Vulkan paths
remain the evidence for those language forms. Default benchmark DXC target is
still Vulkan 1.0.

## M36b canonical ndarray/tensor authorities

The deleted Godot-crashing temporary files were never source-controlled and
are not historical authorities. M36b therefore declares two regenerated,
checked-in benchmark artifacts authoritative from this milestone forward:
`NDArrayMaterializeStorage` (`sdslvbench-8b1f66233dd54390f518e9c7`) and
`TensorContractionStorage` (`sdslvbench-a2b7fd8383074dd673a365d5`). Their
source is `examples/SDSL-V/M36a/BasicBenchmarks.sdslvbench`; isolated HLSL,
SPIR-V, and the toolchain/resource manifest are in
`examples/SDSL-V/M36a/artifacts/`.

The manifest records source digest, compiler identity, DXC version and flags,
Vulkan target, entry point, launch contract, resources, sizes and SHA-256.
`go run ./tools/generate_m36b_canonical` regenerates them, and the benchmark
package parity test rejects byte/hash drift when DXC and spirv-val are present.
The authorities are reproducible source plus this recorded toolchain identity;
they make no byte-identity claim about deleted files.
