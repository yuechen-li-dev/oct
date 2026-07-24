# PROMETHEUS G4-E2B-M1 — raw-score stabilization handoff

## Current repository handoff — Stage 3

The repository/generated-authority hygiene pass is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE1_REPOSITORY_AND_GENERATED_AUTHORITY.md`.
The mechanical ABI and vocabulary pass is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE2_ABI_AND_VOCABULARY_CONSOLIDATION.md`.
The common Vulkan runtime ownership extraction is recorded in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE3_VULKAN_RUNTIME_OWNERSHIP_EXTRACTION.md`.
The compact corpus index is
`internal/prometheus/DevelopmentReport/PROMETHEUS_DEVELOPMENT_EVIDENCE_INDEX.json`,
and the canonical repository-authority check is:

```powershell
powershell -NoProfile -File .\tools\prometheus_authority.ps1
```

Stage 0 remains the frozen lifecycle characterization and vocabulary authority, and Stage 2
remains the ABI/layout/detail preservation authority. Stage 3 establishes the concrete
`prom_vk_runtime` owner in `native/reactor_vulkan_runtime.[ch]`; SGEMM now borrows its device,
queues, command pools, capabilities, validation state, and package through that internal owner
while retaining SGEMM execution resources and policy locally. Construction failure cleanup and
reverse destruction order are explicit in the owner and SGEMM wrapper.
The Stage 3 M34b classification is **INHERITED DETERMINISTIC FAILURE**: the exact
validation-enabled production-variant command failed three times at both Stage 2
(`1f2fc2a5c95cbbefcb3dcc0b76751b752d5fed46`) and Stage 3
(`2bb06095a1755a30f5d5ab8f140838f59ee51be4`).  With `m=3`, `n=17`, and `k=7`,
variant 4 returns zero for each final-column cell where the CPU oracle expects
`1.6458333730697632`; variants 3 and 5 complete through the direct path.  The
independently rebuilt Stage 2/Stage 3 package manifests had identical SHA-256
values.  Do not change M34b, variants, shader bytes, descriptor/pipeline behavior,
or expected results to conceal this inherited behavioral authority.  The related
A2x4 footprint mismatch is inherited too.  Command pools remain in the concrete
Vulkan owner because they are queue-family-bound mechanical resources; SGEMM keeps
command buffers, reset/reuse, fences, submissions, and execution synchronization.
Dominatus decides and coordinates. Vulkan mechanisms execute and report facts.
The three required-live Gemma lanes remain outstanding because the external
checkpoint is unavailable. Stage 3 makes no live Gemma equivalence claim and does not repair
`-7406`, the repeated `MainTransformer1` topology, generated-header provenance, or registry/package
projection differences. The available Windows Vulkan preflight and lifecycle tests are separate
from the deferred payload-backed lanes.

## Stage 0 supersession

Stage 0 has frozen the current implementation vocabulary and executable characterization in
`internal/prometheus/DevelopmentReport/PROMETHEUS_STAGE0_CHARACTERIZATION_AND_VOCABULARY.md`.
This handoff's historical evidence remains valuable, but its immediate-next-milestone advice is
superseded: a future session must begin with the Stage 0 witnesses, not automatic Gemma feature
implementation.

The three independent Gemma lanes are:

- `TestGemma4E2BM1FreshSessionQFirstAuthority`
- `TestGemma4E2BM1FreshSessionKFirstAuthority`
- `TestGemma4E2BM1SameSession7406Characterization`

Run the required-live wrapper with a validated reactor and checkpoint:

```powershell
$env:OCT_PROMETHEUS_REACTOR = "<validated reactor DLL path>"
$env:G4E2B_CHECKPOINT_ROOT = "<validated external Gemma checkpoint root>"
powershell -NoProfile -File .\tools\prometheus_stage0_required_live.ps1
```

The wrapper fails on an environmental skip, missing PASS case, or zero meaningful work. The
ordinary test command may still skip when setup is absent, but a skip is never Stage 0 success.
The exact same-session boundary remains: first chain succeeds; second-chain M46 preparation
succeeds; immediately following M49 required-weight validation rejects with
`PROM_M46_DETAIL_STALE_WEIGHT_GENERATION` (`-7406`); positional dispatch has not begun. Do not
fix, bypass, weaken, move, or reinterpret `-7406` automatically.

The generated-authority check is:

```powershell
go run ./tools/prometheus_stage0 -check
```

It characterizes the `prometheus.core@1` package, source/package counts, kernel 68/69 identities,
partial generated/static projections, and the disputed repeated `MainTransformer1` topology
without repairing it. Accepted numerical authorities remain 1,800/1,800 bit-exact Q-first and
K-first scores, resident kernel-68 → kernel-69 execution, and no host Q/K detour. Accepted model
ceilings remain `643,587,076` bytes MinimumMemory and `1,005,407,748` bytes Prefetch. ABI/detail,
allocation, residency, teardown, and Windows policy witnesses are documented in the Stage 0 report.

Windows is the accepted live platform. Linux remains unclaimed. Do not start another reactor or
add more handwritten model execution. The first future implementation pass recommended by the
full audit is to establish the common Vulkan runtime/device ownership boundary, then make later
owner decisions for slots, sessions, weight snapshots, and generated authority using these frozen
witnesses.

## Proven

- The authority-verified checkpoint-backed Vulkan path runs package-backed
  `kernel-69-default` against the exact resident kernel-68 Q and K outputs.
- Fresh-session Q-first and K-first chains each match all `1,800 / 1,800`
  FP32 raw scores bit-exactly against the accepted sequential stage-local
  authority.
- Q, K, and score use bounded reusable resident roles: Q slot 0 (122,880
  bytes), K slot 1 (15,360 bytes), and distinct FP32 score slot 3 (7,200
  bytes). The ring depth is 3. Q/K have no host detour; only the final score
  readback occurs.
- Kernel-69 arithmetic, geometry, indexing, GQA mapping, BF16 decoding,
  sequential coordinate order, and post-sum `1/16` scaling are accepted. Do
  not reopen them without contradictory evidence.

## Open boundary

In one reusable native session, the first chain succeeds. On the second chain,
M46 weight preparation succeeds, but the immediately following M49 required-
weight validation rejects with `PROM_M46_DETAIL_STALE_WEIGHT_GENERATION`
(`-7406`) before any positional dispatch. The failure is the M46-to-M49
required-weight generation/hash handoff after prior score completion; it is
not initial M46 admission, Q/K ordering or binding, resident-ring depth, or
kernel-69 arithmetic.

`observed_weight_generation` and `requested_weight_generation` have been added
to the closed HeadRmsNormRope and score results. They are zero at this M49
early-return boundary because current propagation covers M46 rejection or full
M46/M49 completion, not M49's required-weight rejection. Possible causes
remain intentionally unproved: completion invalidation, missing reacquisition,
cached source metadata, stale slot role metadata, or asymmetric generation
advancement. The exact handoff to inspect is M46 `weight_result.generation` /
`weight_result.hash` into M49 `required_weight_generation` /
`required_weight_hash`.

## Relevant files and commands

- Native lifecycle and diagnostics: `native/reactor_api.c`,
  `native/reactor_api.h`, `native/reactor_vulkan_transformer.c`,
  `native/reactor_vulkan_runtime.[ch]`, and `native/reactor_vulkan_sgemm_internal.h`.
- Closed Go seam and live harness: `bridge.go`, `bridge_dlopen_windows.go`,
  `bridge_dlopen_linux.go`, `gemma4e2b_m1_rtx.go`, and
  `gemma4e2b_m1_rtx_test.go`.
- Resume command (with validated checkpoint, DLL, hardware, and validation
  environment variables configured):
  `go test -run TestGemma4E2BM1CanonicalQKVRTX -count=1 -v ./internal/prometheus`.

The next payload-enabled action is to rerun the existing Q-first, K-first,
same-session `-7406`, allocation/teardown, and canonical Z-Image witnesses
against the reorganized checkpoint. A live failure is a regression
investigation, not an automatic attribution to the historical checkpoint.
Do not automatically resume implementation of the `-7406` defect.

The exact Stage 4 candidate boundary is the SGEMM-to-reactor execution handoff: slots,
weight ownership, required generations/hashes, and any snapshot or preparation contract
that a second reactor would genuinely consume. It is not begun here; no slots, sessions,
leases, weight registries, binding snapshots, plans, or reactor registries were introduced.
