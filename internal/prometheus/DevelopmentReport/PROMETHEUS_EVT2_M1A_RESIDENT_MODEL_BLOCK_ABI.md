# Prometheus EVT-2 M1a — Resident Compiled Model-Block ABI

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
EVT-2 state: **READY FOR M1B**  
Compiled-model status: **RESIDENT BLOCK ABI COMPLETE**

## Result

M1a adds one production-owned, fixed resident command vessel. It is not a graph runtime: it accepts one declared resource plan, the exact seven low-level command categories, the registered resident proof shader, and declared input/output/audit bindings only. Runtime topology, shaders, callbacks, graph nodes, and arbitrary dispatch are not accepted.

The first consumer is the existing `Tongyi-MAI/Z-Image-Turbo` `noise_refiner.0` contract. The adapter validates the M0.75 thirteen-tensor bundle, binds cache-relative identities, and produces the M1a resident declaration. It reserves FP32 model-width external I/O for M1b but does not implement AdaLN, normalization, RoPE, QKV, attention, or FFN.

## Identities and ownership

- Model contract: `c845e87fe9269ed35d49d144d08d9fdf71522c9647e03cb4c29ed829752d3a94`
- Weight identity: `4ed54d0ff05fcaa67c53a7690369b452f5cf92e5b9fac779bdd883ea31201ead`
- Cache aggregate: `a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e`
- Cache checkpoint: `2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6`
- Shader portfolio: `c19d12d32321ed55dbc6cbc8f63a14911ce28db0b7077e888a75e61d68aa4676`
- Fixed execution plan: `d84c5b3a5168bfd186fef22652361dfa9693444265eb7cf2d795be5e118ae20a`
- Resident replay seed: `a4768c56bb8ff34d05ae26b4fd0dc6cb7810a83883adc5a358ad0417de2deb18`

The public native ABI is in `reactor_api.h`: create, upload immutable weights, execute, obtain evidence, and destroy. Its owner record is embedded in the established reduction runtime state, so it reuses the existing device, command-pool, descriptor, teardown, quarantine, and reap lifetime authority rather than creating a parallel Vulkan lifecycle.

The plan is exactly: bind pipeline; bind declared resources; push declared constants; dispatch; barrier; audit copy; output copy. Command buffers are deterministically recorded into the module’s preallocated command resource each execution, because external input/output bindings vary. No internal host readback occurs before the final declared output and optional audit copies.

## Shader portfolio

The proof shader is production SDSL-V at `internal/prometheus/shaders/sdslv/production/model_block/resident_identity.sdslv`:

- Registry ID: `23` (`model-block-resident-identity`)
- Entry point: `ResidentModelBlockIdentity_CS`
- Source SHA-256: `5c3010d829c99d20865f9c0c64365eb4ff87dc73f8e9aceacbbe724150d673b1`
- Generated header SHA-256: `2792e156d1817ac086ce002f473f704123445d6752631fc358864ed77037a4f4`
- SPIR-V words: `371`
- Descriptor bindings / push constants: `3` / `8` bytes

The normal production generator regenerated the header and completed `spirv-val`; registry and workspace checks pass. The proof has no semantic-space boundary because it is an identity copy. M1b will place `ModelEmbedding`, `QueryHead`, `KeyHead`, `ValueHead`, `PositionedQueryHead`, `PositionedKeyHead`, `AttentionScore`, and `AttentionProbability` in its canonical production SDSL-V source; those types erase before HLSL/SPIR-V.

## Weight and memory evidence

The local M0.75 cache was revalidated: 13 FP16 tensors, 361,820,672 bytes, individual tensor hashes, shapes, layouts, and aggregate identity. M1a’s hardware proof uploads a deliberately bounded representative bundle: 13 immutable bindings × 64 bytes = 832 bytes. This establishes resident binding, immutable upload, repeat execution, and lifecycle evidence without duplicating M1b’s real weight-consumption evidence. The full cache is represented in the compiled Z-Image plan and will be attached to this owner in M1b.

| Resource class | Bytes |
| --- | ---: |
| Persistent real-weight declaration | 361,820,672 |
| Reusable plan buffers (including largest 88,473,600-byte upload staging buffer) | 151,388,160 |
| Audit buffers | 8,192 |
| Total committed M1a declaration | 513,217,024 |
| M1a declaration ceiling | 536,870,912 |

The resident proof cold-creates 20 Vulkan buffers (seven I/O/staging/audit resources plus thirteen immutable bindings), one descriptor set, and one pipeline. Ten warm executions allocate zero Vulkan buffers/device allocations, create zero pipelines, grow zero descriptor pools, and upload zero weights.

## RTX execution and timing

Clean-process preflight selected `NVIDIA GeForce RTX 3070` (vendor `4318`), with Vulkan availability and device diagnostics both successful. The focused resident suite passed: identity output, 64-element bounded audit, ten warm executions, deterministic replay identity, destruction/recreation, and fault recovery.

The proof-shader host-observed execute samples after five warmups were `72700, 75300, 73800, 75800, 74200, 73500, 73200, 75700, 74900, 76000` ns: min `72,700`, median `74,550`, mean `74,510`, p95 `76,000`, standard deviation `1,126.45`. These are host-observed proof-vessel durations, not GPU timestamp or real Z-Image-block timings; M1b introduces the first model-stage timing boundaries.

## Faults and first discrepancies

The native suite covers malformed plan, wrong/missing binding, upload failure, pipeline creation failure with later creation recovery, queue-submit failure, uncertain-completion quarantine/reap, audit-copy failure, destroy-before/after execution, repeated-destroy rejection, and runtime teardown after partial creation.

Two defects were localized and fixed rather than masked:

1. Model-block entry points compared the boolean runtime-handle validator against `PROM_OK`; valid handles were rejected as invalid requests. All M1a call sites now use its boolean convention.
2. Queue-submit failure was collapsed to command-record failure. Execution recording now returns its specific detail code, preserving `-6909` for queue-submit failure and `-6910` for uncertain completion.

## Artifacts and M1b handoff

Deterministically generated twice, with identical hashes:

- `resident_model_block_contract.json` — `c1f4f8071ef206f1d0626a9e896bdb49d69ff372c3db4b43cdb4082721ebf2ae`
- `resident_model_block_memory.json` — `10e937dfc1da7f7a207c2c4b8495523f1684a96034d20a05d5627d726a8e738a`
- `resident_model_block_replay.json` — `712afbbe67cd01eab0b131eaa7e933aef25d0223d9a998b35808cc2d02d9bb9a`
- `resident_model_block_faults.json` — `b358f274d6a1eb6c598851d3fb2148506d0bc56d2c7ddb9448dd55bfad5e6f39`
- `evt2_m1b_handoff.json` — `e554a3f052084bcc88899c06029e27801124f635c15e6242038e610045a831f7`

M1b should add the timestep linear/AdaLN split, scale/gate application, attention-input RMSNorm/modulation, Q/K RMSNorm, and exact three-axis RoPE as new fixed shader portfolio entries and fixed execution-plan steps. It consumes the existing captured input/timestep plus the AdaLN, normalized-input, normalized-Q/K, and positioned-Q/K witnesses. Attention, FFN, scheduler, and all other block families remain intentionally absent.

## Validation

- `go test ./internal/prometheus/zimage -v` with validated local payload roots
- Windows native build
- `marionette_tests.exe ResidentModelBlock`
- `marionette_tests.exe PrometheusVulkanRuntimePreflight`
- `marionette_tests.exe ShaderRegistry`
- production SDSL-V generation and `spirv-val`
- `go run ./tools/prometheus_native_manifest -check`
- `go run ./tools/sdslv_workspace_check`
- `bash -n internal/prometheus/native/build_linux.sh`
- `git diff --check` and the Git large-file scan
- `go test ./internal/source`
- `go test ./internal/diagnostic`
- `go test ./internal/sdslv/...`
- `go test ./internal/octxiliary/...`
- `go test ./pkg/octxiliary/...`
- `go test ./internal/cli`
- `go test ./cmd/oct`
- `go test ./tools/build_sidecars`
- `go test ./internal/... ./cmd/oct`
- deterministic artifact generation twice
