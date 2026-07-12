# SDSL-V Kaiju Octxiliary Vulkan runtime evaluation

Date: 2026-07-12  
Convergence state: **SUCCESS**  
Kaiju authority: `https://github.com/KaijuEngine/kaiju` at commit
`ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2` (`26.07.12.1-nightly`)  
Test host: Windows amd64, NVIDIA GeForce RTX 3070, Vulkan API 1.4.329,
Vulkan SDK 1.4.341.1, Go 1.26.2, CGO with GCC

## Executive answer

**Yes, with an important ownership qualification.** A small dedicated process
can import only Kaiju's exported raw Vulkan binding packages, create a
compute-only Vulkan device without a surface or window, load compiler-owned
SPIR-V, bind manifest-owned set-0 storage buffers, use an explicit entry point,
push constants and dispatch geometry, synchronize, read back data, and return
JSON with real GPU timestamp samples.

The isolated proof did all of that on Windows. It passed resource-free, scalar
storage-buffer, vector storage-buffer, ndarray-generated, and tensor-generated
SPIR-V. The ndarray and tensor probes returned the SDSL-V test result ABI with
`abi_version=1` and `failed=0`. This materially crosses the current Godot 4.7
pipeline-creation failure boundary.

This result does **not** justify adding Kaiju to Oct production dependencies.
Kaiju has no reusable high-level headless compute API, its module path cannot
currently be resolved by the Go tool, and its raw Vulkan package is a large,
low-level, pre-1.0 ownership surface. The appropriate next step is a pinned
Kaiju fork containing a dedicated compute command, not a dependency from Oct
compiler/runtime packages.

## Oct architecture inspected

The evaluation used these Oct authorities:

- `pkg/octxiliary` and `internal/octxiliary`: public SDK over the `OCTWRAP`
  handshake, framed Octagon messages, typed values, deterministic request IDs,
  and explicit sidecar errors;
- `tools/build_sidecars`: explicit sidecar lifecycle; optional commands are
  built into `dist/sidecars` and are not ambient compiler dependencies;
- wrapper manifests: `Family`, `Protocol`, `SidecarCommand`, `GoModuleDir`, and
  explicit wire function/type declarations;
- `internal/sdslv/bench` and `tools/sdslv_benchmark_host`: schema-v1 benchmark
  requests, isolated per-case SPIR-V, manifest-owned resources, warmup and
  iterations, one host process per case, and synchronized host elapsed timing;
- M36a benchmark manifest: resource-free `ScalarArithmetic`, default-count
  resource-free coverage, and `StorageBench_VectorDotStorage` with set 0,
  bindings 0/1;
- the Godot 4.7 C# host: `CreateLocalRenderingDevice`, shader/pipeline creation,
  storage buffers/uniform sets, dispatch, submit/sync, readback, and RID cleanup;
- M34a audit registry and native runtime: the audit accepts file or embedded
  SPIR-V plus an entry point and deterministic replay identity, but its concrete
  execution override is SGEMM-specific (A/B/C buffers, push ABI, dispatch
  metadata). It is evidence for compiler-owned arbitrary modules, not a generic
  manifest-defined resource executor;
- Prometheus native: production-owned instance/device/pipeline/descriptor/
  command/query machinery and real native Vulkan timing remain authoritative
  for production compute.

The existing generic Octxiliary typed transport is not itself the current
`.sdslvbench` schema. Large binary payloads, repeated resources, uint triplets,
and sample arrays would be awkward if forced into today's generic wrapper value
set. A dedicated benchmark-backend process may still follow Octxiliary's
discovery, isolation, explicit build, failure, and ownership conventions while
using the existing benchmark JSON envelope. If this later becomes a manifest
wrapper, add a versioned record/byte transport deliberately; do not encode the
schema as positional strings.

## Part A: Kaiju Vulkan architecture audit

Kaiju is a Go module under repository `src/`, with module path
`kaijuengine.com` and `go 1.25.0`. The checked commit has 45 commits in the
preceding month and frequent dated nightly tags, so active development is
directly verified. It is also API-churn evidence, not stability evidence.

| Capability | Exists | Public/reusable | Location | Integration concern |
|---|---:|---|---|---|
| Go module | Yes | Awkward | `src/go.mod` | `kaijuengine.com` is not Go-resolvable; exact source checkout/replace or fork is required. |
| Vulkan loader/binding | Yes | Public, raw | `src/rendering/vulkan` | Exported wrappers, types, vendored headers, CGO and generated C bridge; no semantic stability promise. |
| Vulkan constants | Yes | Public, raw | `src/rendering/vulkan_const` | Large generated-looking API surface tied to binding layout. |
| Platform wrappers | Yes | Public, raw | `vulkan_windows.go`, `vulkan_linux.go`, `vulkan_xlib.go`, Darwin/Android files | Windows proven. Linux source path includes Xlib wrapper even for no-window imports. macOS requires MoltenVK flags. |
| Default loader | Yes | Reusable | `vk_default_loader.c`, `init.go` | Dynamically loads the platform Vulkan loader; no window library needed on Windows proof. |
| Instance creation | Yes | Raw reusable; high-level not headless | raw `CreateInstance`; `gpu_instance_vulkan.go` | High-level setup asks the window for extensions and enables debug report in debug mode. |
| Validation/debug | Partial | High-level only policy | `vk_config.go`, `gpu_instance_vulkan.go`, raw debug-report wrappers | Uses `VK_LAYER_KHRONOS_validation` plus legacy `VK_EXT_debug_report`; raw proof used implicit validation layer and emitted no errors. Debug-utils types exist, but a complete debug-utils high-level path was not found. |
| Physical device enumeration | Yes | Raw reusable | `vulkan.go`; `gpu_physical_device_vulkan.go` | High-level selection is surface-aware. Proof selects a compute family directly. |
| Compute queue discovery | Yes | Public high-level helper | `GPUPhysicalDevice.FindComputeFamiliy` | Typo is public API. High-level logical-device setup creates graphics/present queues only, then retrieves the compute queue; a distinct compute family is not safely created. |
| Logical device | Yes | Raw reusable; high-level graphics-coupled | `gpu_logical_device_vulkan.go` | High-level path requires graphics features/extensions and assumes presentation. Proof creates a minimal compute-only device. |
| Shader module | Yes | Raw reusable | `CreateShaderModule`; `GPUDevice.createSpvModule` | Raw accepts arbitrary bytes/words. High-level loads through asset DB. |
| Entry-point selection | Partial | Raw reusable | `PipelineShaderStageCreateInfo.PName` | High-level compute hardcodes `main`; proof accepts explicit names. |
| Compute pipeline | Yes | Raw reusable; high-level coupled | `CreateComputePipelines`; `createComputeShader` | High-level requires `ShaderDataCompiled`, asset DB, render IDs, descriptor metadata, and engine cleanup tracking. |
| Descriptor-set layout | Yes | Raw reusable | descriptor wrappers; `createDescriptorSetLayout` | High-level descriptor shape comes from Kaiju shader metadata, not an external manifest. |
| Descriptor pool/set/update | Yes | Raw reusable | Vulkan wrappers and rendering helpers | Go 1.25+ pointer pinning matters. The proof had to use `runtime.Pinner`, matching newer Kaiju rendering code. |
| Storage buffers | Yes | Raw and high-level primitive | `GPUDevice.CreateBuffer`; Vulkan buffer wrappers | High-level names allocation errors as vertex-buffer errors and does not form a generic external resource ABI. |
| Host-visible/coherent memory | Yes | Raw/high-level primitive | `createBufferImpl`, memory wrappers | Proof uses mapped host-visible coherent storage for minimality. Device-local plus staging is possible but not packaged as a reusable compute resource. |
| Device-local/staging copy | Yes | High-level primitive | `CopyBuffer`, `copyBufferImpl` | Uses graphics single-time command machinery; generic compute host would need its own policy. |
| Command pools/buffers | Yes | Raw reusable; helper private | command wrappers; private `beginSingleTimeCommands` | High-level helper submits to graphics queue and waits synchronously. |
| Dispatch | Yes | Raw reusable; high-level queued | `CmdDispatch`; `QueueCompute`; `executeCompute` | High-level dispatch is painter-frame based and uses one descriptor set/current frame. |
| Fences/semaphores | Yes | Raw reusable | Vulkan wrappers; swapchain synchronization | Proof uses a fence. High-level semaphore ownership is swapchain-oriented. |
| Barriers/synchronization | Yes | Raw reusable; narrow high-level policy | `CmdPipelineBarrier`; `executeCompute` | Existing compute barrier targets later graphics reads; not a generic host-readback synchronization API. Fence completion made host-coherent proof reads visible. |
| Query pools/timestamps | Yes | Raw only | `CreateQueryPool`, `CmdWriteTimestamp`, `GetQueryPoolResults`; physical limits | No high-level timing API/use found. Proof cleanly exposed timestamp period conversion and per-dispatch samples. |
| SPIR-V loading | Yes | Raw reusable | shader module wrappers; asset DB high-level path | No reflection. Correctly leaves descriptor ABI ownership to Oct. |
| Headless compute executable/tag | No | No | no `headless` or `compute` build tag/command found | `vk_wrapper_compute.c` is implementation plumbing, not an engine compute mode. Dedicated command is required. |
| Window-free raw path | Yes | Proven on Windows | raw binding import graph | No surface, swapchain, editor/game loop, audio, physics, assets, or UI imported. |
| Cleanup | Yes | Mixed | many explicit `Destroy*`; `GPUApplicationInstance.Destroy` | High-level cleanup is engine-object ordered. Proof owns and destroys every raw object in reverse dependency order. |
| Game plugins/mods | Yes | Lua scripting, in-process | `src/plugins`, embedded Lua C source | Not an external process plugin API. Reflected Go engine objects are exposed to sandboxed Lua. |
| Editor plugins | Yes | Go/editor-specific | editor plugin registry/build paths | Does not provide a runtime-neutral Oct host boundary. |

The word "public" above means Go-exported and importable from a source checkout.
It does not mean stable, documented as a library contract, or independently
versioned.

## Part B: integration-form decision

| Approach | Feasibility | Initialization/build cost | Stability/maintenance | Isolation and pin/update behavior | Decision |
|---|---|---|---|---|---|
| 1. Import Kaiju packages directly | Technically proven for the raw binding, not via canonical module resolution | Minimal import graph, CGO/GCC/Vulkan headers | Raw API and module layout are unstable; `kaijuengine.com` lookup fails | Compiler can remain isolated, but consumers need a local replace | Reject for production form |
| 2. Vendor a minimal Vulkan subset | Technically possible | Would copy binding, constants, headers and C loader/bridge, not a genuinely small API | Oct would own a large Kaiju-derived Vulkan binding and merge upstream manually | Exact snapshot easy; safe updates expensive | Reject; violates the intended maintenance boundary |
| 3. Pinned fork plus compute-host command | Proven shape | Fork builds only raw packages for sidecar; no unrelated engine systems | Explicit patch surface and commit pin; ownership cost visible | Strong process/compiler isolation; controlled reviews per update | **Selected spike approach** |
| 4. Upstream reusable compute API | Best possible long-term package boundary, not present today | Would require upstream design for device/resource/timing/lifetime | Lower downstream cost if accepted; schedule/API unknown | Cleanest updates after an upstream contract exists | Follow-up to selected spike, not this spike approach |
| 5. Existing executable/plugin | Not suitable | Full engine/window/game lifecycle; Lua plugin is in-process | Couples to engine runtime and reflected API | Poor fit for arbitrary compiler-owned SPIR-V | Reject |
| 6. Retain another backend | Always feasible | Existing Godot/Prometheus costs | Avoids new dependency but leaves Godot boundary | Strong isolation | Contingency, not selected spike |

### Exactly one recommended spike approach

Use a **pinned Kaiju fork plus a tiny dedicated compute-sidecar command**. Keep
the command in that fork (or a separate sidecar repository with a checked-out
fork replace), pin both commit and patch, and expose only the versioned process
contract. No Oct compiler package may import Kaiju. Before productionization,
offer the compute-device/resource/timestamp API upstream; acceptance can later
reduce the fork patch, but the production decision must not depend on it.

## Part C: sidecar contract

Recommended executable name: `octxiliary-kaiju-vulkan`.

The spike accepts one request file and emits one JSON response. A production
backend should preserve the existing M36a benchmark fields and add only the
fields needed for generic dispatch:

```json
{
  "schemaVersion": 1,
  "operation": "compute.dispatch",
  "spirvPath": "module.spv",
  "spirvHash": "lowercase-sha256",
  "entryPoint": "StorageBench_VectorDotStorage",
  "workgroupSize": [16, 1, 1],
  "dispatchGroups": [1, 1, 1],
  "pushConstantsBase64": "",
  "resources": [
    {
      "set": 0,
      "binding": 0,
      "access": "readonly",
      "kind": "storage_buffer",
      "elementType": "f32",
      "byteLength": 64,
      "payloadBase64": "...",
      "readback": false
    }
  ],
  "warmup": 10,
  "iterations": 100
}
```

```json
{
  "schemaVersion": 1,
  "success": true,
  "runtime": {
    "name": "kaiju",
    "commit": "ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2",
    "device": "NVIDIA GeForce RTX 3070",
    "driver": 2500395008,
    "vulkanApi": "1.4.329",
    "timingSource": "vulkan_query_pool_gpu_timestamp",
    "headless": true
  },
  "spirvHash": "...",
  "samplesNs": [3456, 3232, 3456],
  "readbacks": [{"set": 0, "binding": 1, "payloadBase64": "..."}],
  "warnings": [],
  "errors": []
}
```

Oct owns descriptor coordinates, payloads, entry point, workgroup declaration,
dispatch groups, push ABI, warmup/iterations, replay identity, expected values,
and result interpretation. The sidecar validates rather than infers those
semantics. Kaiju owns Vulkan instance/device/queue, memory and descriptors,
pipeline, commands, synchronization, timing, readback, and cleanup.

Production hardening must add limits, device selection, feature/capability
checks, non-coherent memory handling, device-local staging policy, multi-set
support if manifests need it, structured Vulkan diagnostics, cancellation, and
an explicit maximum request/payload size. `workgroupSize` is identity/validation
metadata; Vulkan local size remains shader-owned.

## Parts D/E/F: proof and compatibility matrix

The isolated proof is in `tools/octxiliary_kaiju_vulkan_spike`. Its `go.mod`
uses a local replace to the ignored pinned source checkout. It is intentionally
absent from Oct's root module, normal sidecar list, and production build.

The M36a vector probe was regenerated through the normal Oct command:

```text
go run ./cmd/oct sdslv compile-spv ... --entry StorageBench_VectorDotStorage
DXC: -spirv -T cs_6_0 -fspv-target-env=vulkan1.0 -O3
```

| Module | SHA-256 | spirv-val | Kaiju pipeline | Dispatch | Readback | Notes |
|---|---|---:|---:|---:|---:|---|
| Resource-free SDSL-V `M13IfDemo_CS` | `e3942a241384d69964f3c8553e9ee9d209c43c5fa35042d0e1e92f9eaaf5d242` | Pass | Pass | Pass | N/A | Explicit non-`main` entry point; 1x1x1. |
| Scalar storage SDSL-V | `fabf2425e4d8065bde84a64f622b6b9a11eed3a6a5ea559c4a2c865180ee4ff5` | Pass | Pass | Pass | Pass | Input 3.0, output 9.0. |
| M36a `VectorDotStorage` shape | `e691700384742a196d859b1e982170c59f8616bf448447b13b72087b6224a4b6` | Pass | Pass | Pass | Pass | 1..16 read back as 1,4,9,...,256. Exact current function body and ABI regenerated in isolation. |
| Existing M29/M33 ndarray execution group 0 | `4f73e95933c95dde523e35e4795795fea39ffefc719dd02bd391d849340ab5db` | Pass | Pass | Pass | Pass | Result ABI `abi_version=1`, `failed=0`. |
| Existing M29/M33 tensor execution group 0 | `da482280157d1c7bcda9f083f209b6406118278140e33d996dde3982484c1f2a` | Pass | Pass | Pass | Pass | Result ABI `abi_version=1`, `failed=0`. |

The removed Godot-crashing M36a ndarray/tensor benchmark binaries were temporary
per-case files and were not available as preserved artifacts/hashes. The proof
therefore used the exact checked local SDSL-V-generated M29/M33 corpus artifacts
that exercise the same ndarray/tensor lowering class and push/result ABI. This
is strong backend evidence, but it is not a claim that the unavailable M36a
binary hashes were reproduced. A productionization spike should preserve those
exact M36a per-case artifacts and rerun them before enabling a backend flag.

## M36b canonical-artifact update (2026-07-12)

The temporary binaries described above were not recoverable authorities. M36b
instead generated and checked in new canonical M36a benchmark artifacts from
the current benchmark source, isolated per benchmark with DXC Vulkan 1.0 and
`spirv-val`. The canonical ndarray artifact is
`7ee4a36be544f4742c30aa13d159ce8312d31a527403c00d7b67ab0d377a397a`
(1,560 bytes); the canonical tensor artifact is
`9c14708fb37490d3f0f776a2cd4b156dbf00936fb8a4d6f5db159718f393a3a7`
(2,820 bytes). Their stable IDs, exact source/toolchain provenance and resource
contracts are in `examples/SDSL-V/M36a/artifacts/manifest.json`.

The existing Kaiju JSON proof executable ran these exact bytes successfully on
the RTX 3070, with explicit `main`, set-0 bindings 0/1, readback and eight real
query-pool samples each. This is evidence only for the retired spike envelope;
the production M36b binary still must replace it with typed OCTWRAP/Octagon.

That replacement is now implemented as `octxiliary-kaiju-vulkan`: a typed
OCTWRAP sidecar with `compute.capabilities`, `compute.dispatch`, and
`compute.benchmark`. `oct sdslv bench ... --backend kaiju` now runs the exact
canonical ndarray/tensor artifacts through that sidecar on the RTX 3070 and
returns real query-pool GPU timestamp samples.

SPIR-V observations:

| Module class | SPIR-V | Capabilities/extensions | Interface representation | Entry/local size | Accepted |
|---|---|---|---|---|---:|
| Resource-free | 1.0 | `Shader`; no extension | none | `M13IfDemo_CS`, 1x1x1 | Yes |
| Scalar/vector storage | 1.0 | `Shader`; no extension | Vulkan 1.0 `BufferBlock`, set 0 bindings 0/1 | explicit names, 1x1x1 or 16x1x1 | Yes |
| Ndarray test | 1.0 | `Shader`; no extension | result `BufferBlock`; unused input optimized out | `main`, 1x1x1 | Yes |
| Tensor test | 1.0 | `Shader`; no extension | result/input `BufferBlock`, set 0 bindings 0/1; push constants | `main`, 1x1x1 | Yes |

The default remains `vulkan1.0`. Existing preserved HLSL environment probes for
`vulkan1.0`, `1.1`, `1.2`, and `1.3` all passed their matching `spirv-val`
target and Kaiju pipeline/dispatch/readback. Those optional target probes were
DXC environment checks, not authorization to change Oct's default.

## Part G: timing

Kaiju's raw binding exposes everything needed for correct GPU timestamps:

- query-pool creation/destruction and result retrieval;
- command query reset and timestamp writes;
- queue-family `TimestampValidBits`;
- physical-device `TimestampPeriod`.

No high-level Kaiju timing API or query-pool use was found. The proof adds a
two-query pool, records timestamps immediately around each dispatch, waits on a
fence, retrieves 64-bit results, and converts ticks with `TimestampPeriod`.
Pipeline, descriptor, buffer, and command-pool setup occur outside timing.
Warmups are executed but excluded from `samplesNs`. Representative samples were
roughly 2.3-3.5 microseconds on the tested RTX 3070.

The current prototype uses top-of-pipe/bottom-of-pipe bounds. A production
patch should use the narrowest stages allowed by the targeted Vulkan version,
handle timestamp wrap with `TimestampValidBits`, and record availability/error
state. This is still materially better than Godot's synchronized host elapsed
timing.

## Part H: headless/platform status

**Windows: proven truly headless.** The command imports only
`rendering/vulkan`, `rendering/vulkan_const`, and the Go standard library. Its
CGO set is `cgo_helpers.go`, `init.go`, `types.go`, `util.go`, `vulkan.go`,
`vulkan_windows.go`, `vk_bridge.c`, `vk_default_loader.c`, and
`vk_wrapper_desktop.c`. It never creates a window, surface, swapchain, editor,
game loop, or render pass. No build tag is required.

**Linux: source-supported but not executed in this Windows spike.** The raw
loader path uses `-ldl`; the package also selects `vulkan_xlib.go` on ordinary
Linux even when the caller never creates a surface, so Xlib headers may remain
a build-time dependency. Runtime use of a display/surface is not required by
the proof design. This must be verified in Linux CI before productionization.

There is no Kaiju `headless`, `compute`, or `compute-only` build tag. Relevant
engine tags are `debug`, `editor`, `filedrop`, `generator`, `rawsrc`, platform
tags, and optional runtime tags such as `steam`. The file named
`vk_wrapper_compute.c` is not evidence of a compute-mode executable; the normal
desktop build compiled `vk_wrapper_desktop.c`.

macOS and Android are engine-supported source paths, but were outside this
Windows/Linux question and were not tested. macOS build instructions require
MoltenVK CGO flags.

## Part I: build and dependency cost

| Item | Finding |
|---|---|
| Go | Kaiju declares Go 1.25.0; proof built with Go 1.26.2. |
| CGO | Required by the Vulkan binding. `CGO_ENABLED=1`. |
| C compiler | 64-bit GCC-compatible compiler required on Windows; tested with MSYS2 UCRT64 GCC. Kaiju docs recommend 64-bit MinGW and explicitly reject windows/386. |
| Vulkan | Vendored Vulkan headers are compiled; a Vulkan loader/runtime is required. SDK tools were used for DXC and validation. |
| Sidecar first build | 17.4 s with uncached Kaiju packages in this checkout. |
| Sidecar incremental build | 1.4 s. |
| Sidecar size | 5,705,240 bytes unstripped. |
| Normal Kaiju runtime build | Succeeded on Windows in 18.3 s; 27,059,763 bytes unstripped. |
| Sidecar Go dependency graph | Standard library plus only Kaiju raw Vulkan and constant packages. |
| Audio/physics/editor/UI/assets | Not imported or linked by the sidecar. |
| Kaiju submodules | Normal engine uses `src/libs` prebuilt archive submodule and a binary content-tools submodule. Compute sidecar needs neither. |
| Full engine native deps | Current `src/libs` commit contains SoLoud archives; docs also mention optional Steam files and platform runtimes. Bullet was removed at that prebuilt commit. |
| Module resolution | `go list -m -versions kaijuengine.com` fails with an HTML/XML parse error. A pinned checkout/replace, fork, or module-path upstream fix is mandatory. |

The raw binding does not depend on the prebuilt SoLoud archives. This is a
major positive result: compute-only packaging can exclude unrelated native
systems. Partial import is nevertheless awkward because the only Go module is
the whole `src` tree and there is no separately versioned Vulkan module.

## Part J: license and provenance

Kaiju's root license is MIT and requires preservation of its copyright and
permission notice in copies/substantial portions. A fork distribution must
ship that license and record the pinned commit and Oct/sidecar modifications.

The root `licenses/third-party` directory contains notices for FreeType,
JetBrains Mono (OFL), Material Icons (Apache 2.0), Artery Font Format,
msdf-atlas-gen, msdfgen, and tinyxml2. Those systems are not in the compute-only
sidecar import graph, but a full Kaiju game-host distribution would need to
carry the applicable notices.

The pinned prebuilt `src/libs` submodule commit is
`14ec049b64064ce7a2abd4679acbd2b5b00ba3ae`. It contains SoLoud archives for
Windows/Linux/macOS/Android and a README identifying the source, but it contains
no `LICENSE`, `COPYING`, or `NOTICE` file. Main-repository MIT licensing is not
evidence for those archives. This is a provenance/redistribution gap that must
be resolved before distributing the full engine or prebuilts. It does not
affect the proof binary because the compute-only import/link graph excludes
`src/libs`.

Required production provenance set: Kaiju MIT text and authors, exact Kaiju
commit, fork patch/commit, sidecar source, Go module lock/checksums, generated
binary build metadata, Vulkan loader expectation, and every actually linked or
bundled third-party notice. Do not ship the prebuilt submodule merely because
it is convenient.

## Part K: future game-host assessment

Kaiju can be an **optional host process**, but the present source does not offer
an external deterministic application protocol. Normal games implement the
Kaiju game interface and are compiled into the Go binary/build output. Runtime
plugins are sandboxed Lua scripts with reflected Go object access; editor
plugins are Go/editor-specific. Neither is an Octxiliary process bridge.

A future experiment could add a narrow host command that accepts deterministic
Oct-owned state/event/UIIR messages and projects them into Kaiju entities,
rendering, input, and audio. Oct would retain state-transition and language
semantics; Kaiju would own presentation and host services. SDSL-V graphics
shaders could eventually be packaged as explicit Kaiju shader assets after
graphics-stage/interface compatibility work, but the compute proof does not
establish that path.

Such a host would almost certainly require a fork or upstream host API today.
It should remain one backend among Godot, native Prometheus, and future hosts,
never a mandatory runtime for all Oct applications. Process messages are the
clean semantic boundary; reflecting Oct semantics into Kaiju's Lua/Go object
model would create the coupling this spike is meant to avoid.

## Part L: backend comparison and recommendations

| Property | Godot C# | Kaiju Go | Prometheus native |
|---|---|---|---|
| Language | C# | Go + CGO raw binding | C/C++ |
| Arbitrary SPIR-V | Bounded | Proven in spike for tested generic compute forms | Yes, production plus bounded audit interfaces |
| Resource buffers | Ordinary scalar/vector proven | Set-0 storage buffers proven; generic API is spike-owned | Yes |
| Ndarray/tensor modules | Godot 4.7 pipeline creation crashes for current forms | Existing generated forms passed pipeline/dispatch/result ABI | Yes |
| Headless compute | No in current Godot 4.7 host | Yes on Windows raw path; Linux execution pending | Yes |
| GPU timestamps | Synchronized host elapsed | Raw query-pool timestamps proven | Yes |
| Packaging | Self-contained engine host | 5.7 MB proof plus CGO/Vulkan runtime; fork/source pin needed | Native build/runtime |
| Cross-platform | Strong engine distribution | Engine claims broad support; compute sidecar proven only Windows, Linux source path pending | Current native supported platforms |
| Maintenance | External engine/API boundary | External engine plus explicit fork patch/raw-binding ownership | Internal production ownership |

Recommendations by use:

- generic `.sdslvbench`: productionize the pinned Kaiju sidecar only as an
  optional backend after Linux CI and exact preserved M36a module replay;
- production Prometheus compute: retain Prometheus native; Kaiju offers no
  reason to transfer production ownership;
- future Oct game host: exploratory only, separate milestone and process
  protocol; do not infer suitability from compute success;
- CI smoke backend: useful on GPU-enabled Windows/Linux runners after packaging
  and deterministic capability reporting; not a software Vulkan fallback;
- graphics experimentation: potentially useful because Kaiju already owns a
  renderer, but requires separate shader/interface and host-lifecycle work.

## Validation evidence

Completed on the test host:

- pinned Kaiju checkout and source audit;
- full normal Kaiju Windows runtime build;
- isolated sidecar build and cached rebuild;
- validation-layer vector run with empty validation stderr;
- `spirv-val --target-env vulkan1.0` for resource-free, scalar, vector,
  ndarray, and tensor proof modules;
- matching target validation and dispatch for preserved Vulkan 1.0/1.1/1.2/1.3
  environment modules;
- resource-free explicit-entry pipeline/dispatch;
- scalar storage output 9.0 from input 3.0;
- vector storage output squares 1 through 256;
- ndarray/tensor pipeline, push constants, dispatch, and SDSL-V result ABI;
- warmup exclusion and per-dispatch GPU timestamps;
- repeated vector executions with identical identity/readback JSON (timestamps
  correctly vary and are not replay identity);
- explicit cleanup under validation;
- spike `go test`/build checks and repository `git diff --check` are recorded in
  the final task validation.

Not executed: Linux build/run, macOS/Android, exact unavailable temporary M36a
ndarray/tensor hashes, multi-set descriptors, non-storage resources,
device-local staging, device loss/restart, or full `.sdslvbench` backend wiring.

## Rollback/deletion

No production package, root module dependency, wrapper manifest, normal sidecar
list, or runtime selector changed. To delete the experiment:

1. remove `tools/octxiliary_kaiju_vulkan_spike`;
2. remove ignored `out/kaiju-audit` and `out/kaiju-spike` scratch/build data;
3. remove this report.

## Final recommendation

The proof answers the core technical question positively and crosses the Godot
failure class, while the module-resolution, high-level API, Linux-build, and
fork ownership costs are explicit. Do not replace Prometheus native and do not
make Kaiju mandatory for Oct applications.

**RECOMMEND KAIJU SIDECAR SPIKE FOR PRODUCTIONIZATION**
