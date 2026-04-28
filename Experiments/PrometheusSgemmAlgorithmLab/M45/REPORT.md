# P13 M1 / Prometheus SGEMM Algorithm Lab M45 — Occupancy Band Control Strategy Lab

## 1) Hypothesis and control-theory framing

### Restated hypothesis

P13 M1 hypothesis:

> A small device capability band model can select practical SGEMM occupancy/WIP recipes (`device_band + shape_class -> kernel_variant`) without requiring first-pass per-GPU response-surface characterization.

### Occupancy in GPU terms

Occupancy is the amount of resident executable work (active warps/wavefronts) the GPU can keep scheduled on an SM/CU while waiting on dependent instructions and memory. It is constrained by finite per-SM resources, especially register file capacity and shared-memory/LDS allocation.

### WIP / inventory mapping

M45 models occupancy with a process-control WIP lens:

- warp = production line,
- register/shared-memory footprint per thread/block = inventory depth per line,
- larger accumulator/register tiles increase work-in-progress depth,
- deeper per-line WIP can raise line efficiency, but often reduces number of simultaneously resident lines.

The control problem is choosing the tradeoff between:

- **depth per line** (arithmetic intensity, unroll, accumulator count), and
- **number of active lines** (occupancy margin).

### Why per-device response surfaces may be overkill in first pass

Per-device profiling can improve raw predicted throughput in this structural model, but it carries systematically higher tuning, maintenance, and portability costs. For Prometheus first implementation, this is likely too expensive unless gains are consistently large enough to dominate product score penalties.

### Why capability bands may be enough

Banding uses stable architectural features (register/shared-memory/bandwidth/compute/workgroup/queue classes) to derive safe ceilings and choose bounded variants. This usually captures the highest-value occupancy constraints while keeping integration complexity and maintenance bounded.

### What M45 must prove before native work

Before any Vulkan/kernel implementation, M45 must prove with executable logic that:

1. band classification is deterministic,
2. constrained devices avoid high-register variants by default,
3. richer devices can safely use deeper variants where shapes justify them,
4. banding improves product score over static default selection,
5. profile/autotune alternatives are compared with realistic process penalties,
6. recommendation is computed from model metrics, not hardcoded.

---

## 2) Device band model

The executable model encodes device capability classes:

- register file capacity class,
- shared memory capacity class,
- memory bandwidth class,
- FP32 throughput class,
- max workgroup/subgroup class,
- queue capability class,
- exact profile availability,
- runtime autotune allowed flag,
- optional manual override variant.

Derived features include:

- register pressure tolerance,
- shared-memory pressure tolerance,
- compute-vs-memory bias,
- safe accumulator ceiling,
- safe tile ceiling.

Band classifier outputs one of:

- `register-constrained`,
- `balanced`,
- `compute-rich`,
- `memory-rich`.

---

## 3) Shape class model

M45 classifies shapes as:

- small square,
- medium square,
- large square,
- tall-skinny,
- wide-short,
- K-heavy,
- ML/FFN-like.

Each shape includes modeled classes affecting selection/scoring:

- memory pressure,
- arithmetic intensity need,
- tile suitability,
- occupancy sensitivity.

---

## 4) Variant vocabulary

M45 uses a compact discrete variant vocabulary:

- `baseline-scalar`,
- `memory-conservative`,
- `small-register-tile`,
- `balanced-2x2-accum4`,
- `aggressive-4x4-accum8`.

Each variant includes:

- register tile class,
- accumulator count,
- shared tile class,
- prefetch depth placeholder,
- estimated register/shared pressure classes,
- arithmetic intensity class,
- occupancy class,
- complexity score,
- portability risk score.

---

## 5) Candidate strategies

M45 compares:

1. **No occupancy-aware selection** (`baseline-scalar` everywhere).
2. **Device-band feedforward** (`device_band + shape_class -> variant`, then safety clamp).
3. **Per-device response profile** (when profile exists; includes tuning burden).
4. **Runtime autotune** (adaptive candidate with startup overhead).
5. **Manual override seam** (optional exact override, else band fallback).

---

## 6) Scoring model

M45 computes structural (non-hardware-timing) metrics per strategy:

- throughput score,
- occupancy adequacy and occupancy risk,
- register/shared pressure penalties via adequacy,
- tuning cost,
- portability risk,
- implementation complexity,
- maintenance cost,
- aggregate product score.

Important: these are relative model scores only, not GFLOP/s claims.

---

## 7) Findings

From executable tests and generated artifacts:

1. **Band classification is deterministic.**
2. **Register-constrained devices avoid aggressive high-pressure variants by default.**
3. **Compute/register-rich bands can select aggressive variants when shape class supports it (e.g., FFN-like).**
4. **Shape class materially affects variant selection.**
5. **Per-device profile can improve raw score, but with higher tuning/maintenance burden.**
6. **Runtime autotune carries explicit startup/tuning overhead.**
7. **Manual override seam correctly supersedes band mapping when override exists, else falls back.**
8. **Band strategy beats no-selection baseline across representative matrix in modeled product score.**
9. **Per-device profile is not chosen as first default unless score justifies process cost.**
10. **Final recommendation is computed, not hardcoded.**
11. **Model outputs are deterministic for repeated scenarios.**

---

## 8) Final recommendation

Direct answers requested by M45:

1. **Is device-band occupancy control viable first strategy?** **Yes, in this model.**
2. **What device bands initially?** `register-constrained`, `balanced`, `compute-rich`, `memory-rich`.
3. **What shape classes initially?** small/medium/large square, tall-skinny, wide-short, K-heavy, FFN-like.
4. **What first variant vocabulary is sufficient?** five discrete variants (`baseline-scalar` through `aggressive-4x4-accum8`).
5. **Does banding beat baseline?** **Yes** on aggregate modeled product score.
6. **Does per-device response-surface tuning beat banding enough for first implementation?** **Not as first default** after cost penalties.
7. **Should runtime autotune be implemented now?** **No** for first pass; keep deferred.
8. **Should manual override seam be preserved?** **Yes.**
9. **What should P13 M2 do next?** Implement band classifier + shape classifier + variant selector seam in production path.
10. **What remains deferred?** Per-device response-surface commissioning, runtime autotune, native kernel variant implementation/benchmarking.

---

## 9) Implementation direction for P13 M2

P13 M2 should implement the minimum viable control law:

- production device capability classifier to M45 bands,
- production shape classifier to M45 shape classes,
- deterministic selector from `(device_band, shape_class)` to first variant vocabulary,
- safety clamping by derived ceilings (register/tile/accumulator),
- explicit manual override seam,
- diagnostics fields mirroring model outputs (selected variant, band, shape class, reason).

No per-device commissioning as default in M2.

---

## 10) Deferred scope

Explicitly deferred from M45:

- Vulkan/native SGEMM kernel implementation,
- runtime autotune implementation,
- benchmark-based performance claims,
- automatic response-surface fitting/commissioning,
- full kernel generation systems.

---

## Language/reference consistency note

This milestone adds executable experiment model/tests/artifacts under `Experiments/` and does not alter language semantics under `Language/`.

Observed documentation/implementation gap surfaced during M45: `Language/reference` documents enum types as first-class, but in this experiment package path the model had to use `String` class tags instead of enum-typed record fields to keep the lab executable. This should be validated and reconciled in language/tooling docs or compiler behavior.
