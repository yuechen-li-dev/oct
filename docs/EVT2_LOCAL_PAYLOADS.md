# EVT-2 local payload authority

EVT-2 Z-Image work uses a durable local payload bundle. Large tensors are intentionally not committed to Git; the repository contains their identities, layouts, selected projections, and reproduction commands. The local payload directory is the large-data authority. Read this guide before reporting a payload blocker.

The Oct laboratory bundle is the canonical numerical authority. Historical ComfyUI captures are compatibility evidence only, and ComfyUI source is not required for production M1B-M1E.

## Setup

The established Windows roots are:

```text
%LOCALAPPDATA%\Oct\evt2-z-image-turbo
%LOCALAPPDATA%\Oct\evt2-z-image-turbo\oracle\f332072aa78be7aecdf3ee76d5c247082da564a6
```

Set them in each PowerShell session:

```powershell
$env:OCT_EVT2_CACHE = "$env:LOCALAPPDATA\Oct\evt2-z-image-turbo"
$env:OCT_EVT2_ORACLE = "$env:LOCALAPPDATA\Oct\evt2-z-image-turbo\oracle\f332072aa78be7aecdf3ee76d5c247082da564a6"
```

`OCT_EVT2_CACHE` must contain the hash-addressed `layers` hierarchy. `OCT_EVT2_ORACLE` must be the pinned revision directory itself, not its parent `oracle` directory.

## Identities

| Item | Required SHA-256 or revision |
| --- | --- |
| Model revision | `f332072aa78be7aecdf3ee76d5c247082da564a6` |
| Source checkpoint | `2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6` |
| FP16 cache aggregate | `a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e` |
| Captured block input | `857cea75e69d665c43779c9bc860796e76ac8b78c5c70882e02a04940e78fded` |
| Captured timestep | `bc0ba90e94f5ae98779c6f7c44e7d1346f8aa6aa1cc048f62a748d96076823b2` |
| O19 canonical stage manifest | `0cab3d8fe179e70058cb22b37994413649f257268566b2c1dfb1254d2daeae65` |
| Canonical stage projections | `f9350d37b46a26d132d4a1e6c80c984ebce87f6f3fe4fd9eb274ffbfd631f480` |
| O19 final diagnostic F32 | `4aff8bf19cfbfc9aebf2e8aa78ef91fb7bb5c117f98504080ed1bc3b206e0c43` |

## Layout and ownership

The local root has this actual structure (large `.bin` files remain local):

```text
%LOCALAPPDATA%\Oct\evt2-z-image-turbo\
  layers\2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6\noise_refiner.0\
    manifest.json
    <13 cache tensor>.fp16.bin
  oracle\f332072aa78be7aecdf3ee76d5c247082da564a6\
    run_02\capture.json
    run_02\noise_refiner_0_input.bin
    run_02\noise_refiner_0_timestep.bin
    run_02\noise_refiner_0_output.bin
    m075\noise_refiner_0_fp16_weight_output.bin
  canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\o19-fp32-reference\noise_refiner.0\
    manifest.json
    o19_stage_manifest.json
    o19_stage_projections.json
    stages\<34 FP32 stage payloads>.f32.bin
    final_output.f32.bin
  canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\noise_refiner.0\capture_04\
    ... historical compatibility evidence only
```

The cache manifest is `layers/<checkpoint>/noise_refiner.0/manifest.json`; its 13 tensor paths are the `destination_name` values in that manifest. The loader consumes the oracle's `run_02/capture.json`, `run_02/noise_refiner_0_input.bin`, `run_02/noise_refiner_0_timestep.bin`, `run_02/noise_refiner_0_output.bin`, and `m075/noise_refiner_0_fp16_weight_output.bin`.

## M2A `noise_refiner.1` authority

The second immutable package is deliberately separate from block 0:

```text
layers/2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6/noise_refiner.1/
  manifest.json
  tensor_inventory.json
  <13 FP16 cache tensors>
canonical/f332072aa78be7aecdf3ee76d5c247082da564a6/o19-fp32-reference/noise_refiner.1/
  manifest.json
  final_output.f32.bin
  stages/<34 FP32 stage payloads>.f32.bin
```

Its cache aggregate is `80c0cd75f44cc434d9306c0fd9a8f02e48b593ecc254de01c1f8fcc29f4bc7c8`.
Regenerate it only from the pinned local checkpoint:

```powershell
go run ./tools/zimage_noise_refiner1_cache -source "$env:USERPROFILE\ComfyUI\models\diffusion_models\z_image_turbo_bf16.safetensors" -cache-root $env:OCT_EVT2_CACHE
go run ./tools/zimage_canonical_reference -cache-root $env:OCT_EVT2_CACHE -oracle-root $env:OCT_EVT2_ORACLE -block noise_refiner.1 -input-f32 "$env:OCT_EVT2_CACHE\canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\o19-fp32-reference\noise_refiner.0\final_output.f32.bin" -out "$env:OCT_EVT2_CACHE\canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\o19-fp32-reference\noise_refiner.1" -capture
go run ./tools/evt2_payload_check
```

The canonical block-1 input is the accepted block-0 FP32 final payload. That
is the two-block chain reproduction boundary: do not convert it to BF16 for
the internal hand-off.

The current M2A-R native rebind/chain witness is deliberately gated on the
real payload lane. It binds block 0, executes the complete resident block,
stages and atomically commits the block-1 package, then starts block 1 from
the resident FP32 final without the BF16 ingress adapter:

```powershell
$env:PROMETHEUS_REQUIRE_VULKAN_HARDWARE = '1'
$env:PROMETHEUS_VK_VALIDATION = '1'
$env:OCT_EVT2_M1B_REAL = '1'
$env:OCT_EVT2_M1C_REAL = '1'
$env:OCT_EVT2_M1D_REAL = '1'
.\out\prometheus\native\marionette_tests.exe PrometheusM1BRealPayloadReachesTheFirstCanonicalModelWitness
```

The normative M1C-M1E local authority is derived from `OCT_EVT2_CACHE`: `canonical/<revision>/o19-fp32-reference/noise_refiner.0`. It contains the O19 stage manifest, projections, final diagnostic, and all 34 hash-addressed FP32 stage payloads. No third environment variable is needed. The older `canonical/<revision>/noise_refiner.0/capture_04` tree is historical, non-normative, incompatible with O19 production acceptance, and retained only for forensic comparison. Do not point `OCT_EVT2_M1B_CANONICAL` at it.

The committed, payload-free numerical authority is [canonical_stage_projections.json](../internal/prometheus/DevelopmentReport/artifacts/Evt2OctOracle/canonical_stage_projections.json), [canonical_stage_manifest.json](../internal/prometheus/DevelopmentReport/artifacts/Evt2OctOracle/canonical_stage_manifest.json), and the M1C inventory [m1c_canonical_stage_authority.json](../internal/prometheus/DevelopmentReport/artifacts/Evt2M1c/m1c_canonical_stage_authority.json). Relevant experiment metadata lives in `internal/prometheus/DevelopmentReport/artifacts/Evt2OctOracle/`.

The M1E payload-free compiled-block package is generated with
`go run ./tools/evt2_m1e_assembly` into
`internal/prometheus/DevelopmentReport/artifacts/Evt2M1e/`. It references the
same immutable cache and O19 identities; it does not add another payload root
or package large weights into Git.

## Fresh-author startup

1. Read this document.
2. Check cache, pinned oracle, and the derived O19 directory: `Test-Path $env:LOCALAPPDATA\Oct\evt2-z-image-turbo`; `Test-Path $env:LOCALAPPDATA\Oct\evt2-z-image-turbo\oracle\f332072aa78be7aecdf3ee76d5c247082da564a6`; `Test-Path $env:LOCALAPPDATA\Oct\evt2-z-image-turbo\canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\o19-fp32-reference\noise_refiner.0`.
3. Set `OCT_EVT2_CACHE` and `OCT_EVT2_ORACLE` using the commands above.
4. Run `go run ./tools/evt2_payload_check`; it verifies the cache/oracle identities, every O19 payload hash, manifest, projections, final diagnostic, and rejects historical substitution.
5. Run `go test ./internal/prometheus/zimage -run TestLoadNoiseRefiner0PayloadBundleWhenLocalPayloadsAreAvailable -v` and confirm the identities listed above.
6. Continue the assigned EVT-2 milestone.
7. Report a payload blocker only when the files are genuinely absent or a validated hash/contract check fails.

The existing Oct-laboratory reproduction path is:

```powershell
go run ./cmd/oct test Experiments/ZImageTurboNoiseRefiner0/M9 --execution compiled
go run ./tools/zimage_canonical_reference -cache-root $env:OCT_EVT2_CACHE -oracle-root $env:OCT_EVT2_ORACLE -out "$env:OCT_EVT2_CACHE\canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\o19-fp32-reference\noise_refiner.0" -capture -projections-out "$env:OCT_EVT2_CACHE\canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\o19-fp32-reference\noise_refiner.0\o19_stage_projections.json" -stage-manifest-out "$env:OCT_EVT2_CACHE\canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\o19-fp32-reference\noise_refiner.0\o19_stage_manifest.json"
```

## Convergence classification

- Environment variable unset: ordinary setup; set it and continue.
- Default local directory exists: validate and continue.
- Default local directory absent but regenerable: regenerate from the documented Oct authority.
- Payload genuinely missing and not regenerable: `HONEST STOP`.
- Hash or model-contract contradiction: `HONEST STOP`.

The historical `canonical/.../capture_04` manifest is an example of the last rule: it must not be promoted to a numerical authority merely because its directory exists. Use the accepted O19 artifacts and M1B replay bundle above.

Do not call an unset variable `NOT READY`, `REQUIRES RESCOPE`, or `CONTRACT BLOCKED`.
