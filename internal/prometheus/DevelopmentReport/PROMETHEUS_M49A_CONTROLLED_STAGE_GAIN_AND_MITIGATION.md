# Prometheus M49a controlled stage gain and mitigation closeout

Date: 2026-07-17
Device: NVIDIA GeForce RTX 3070
Backend/compiler: Vulkan with validation / MSVC 19.51.36231
Convergence outcome: **SUCCESS**
Milestone state: **COMPLETE**
M49b state: **READY TO IMPLEMENT**
M48 EVT state: **POSTPONED UNTIL M49B**

## PM decision

M49a ends here. The numerical plant is deterministic, reduced precision has depth-correlated drift, A2x4 FP32 is the accuracy witness, local matched-input disturbance is small, and the retained-Hidden/Wdown branch has poor error-reduction-per-cost. The final bounded matrix now shows that a fixed periodic complete-block checkpoint materially improves the primary trajectory and improves both error norms on one held-out shape. That is enough to choose a safe, reversible, tunable MVP. Thresholds and weights remain experimental; no certification or product authority is implied.

Normal product execution remains unchanged. The only runtime seam added for this closeout is an audit-only, fixed four-entry precision pattern in the existing M48 owner. Product planning rejects nonzero entries. This is not M49b, a graph scheduler, or automatic precision authority.

## Selected architecture

- **Control unit:** complete transformer block.
- **Nominal path:** cooperative FP16.
- **Primary mitigation:** execute every second complete block through the existing A2x4 FP32 path; the initial four-block pattern is `[cooperative, A2x4, cooperative, A2x4]`.
- **Fallback:** full A2x4 FP32 for at least `FallbackCooldown` completed blocks after a confirmed unsafe condition, then recover through the checkpoint pattern before returning to nominal.
- **Output policy:** no correction, compensation, learned coordinate update, or tensor rewrite.
- **Authority:** observer-only by default in M49b; checkpoint and fallback actions are compiled but staged.

The complete-block unit won because it fits the existing recorder/resource/lifecycle boundary, changes replay identity deterministically, uses already proven paths, and avoids heterogeneous intermediate storage. Interval 2 cut primary final L2 and L-infinity error by about 55% versus cooperative FP16 while costing 36% less GPU time than full A2x4. Interval 4 barely changed the primary result. FP32 Hidden plus FP32-consuming Wdown previously improved tiny-case L2 only 0.98%, regressed L-infinity 2.74%, and roughly doubled suffix cost. An FFN-only checkpoint would require another heterogeneous recorder path without better evidence. Full A2x4 remains the safe fallback rather than the nominal policy because it is the most accurate and slowest measured path.

## Final bounded hardware A/B

All error rows compare exact matched inputs and weights against the GPU full-A2x4 output witness. Timings are five-run medians. Each policy repeated bitwise (`hash_changes=0`), warm execution allocated zero Vulkan buffers, and validation stayed clean.

### Primary: 128 tokens, width 1024, FFN 4096

| Policy | L2 by layer 0/1/2/3 | L-infinity by layer 0/1/2/3 | GPU | E2E | Audit-retained bytes | Dispatches | Submits |
|---|---|---|---:|---:|---:|---:|---:|
| cooperative FP16 | 0.03014 / 2.627 / 440.60 / 1325.63 | 0.00160 / 0.1388 / 6.971 / 9.459 | 15.233 ms | 22.643 ms | 696,288,256 | 396 | 1 |
| conventional FP16 | 0.03013 / 2.625 / 441.55 / 1383.34 | 0.00160 / 0.1391 / 7.248 / 9.381 | 32.775 ms | 40.446 ms | 696,288,256 | 396 | 1 |
| full A2x4 FP32 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | 48.765 ms | 54.917 ms | 696,288,256 | 296 | 1 |
| **checkpoint interval 2** | 0.03014 / 2.710 / 403.12 / **592.05** | 0.00160 / 0.1470 / 2.931 / **4.059** | **31.089 ms** | **37.775 ms** | 696,288,256 | 346 | 1 |
| checkpoint interval 4 | 0.03014 / 2.627 / 440.60 / 1316.04 | 0.00160 / 0.1388 / 6.971 / 9.684 | 22.529 ms | 29.790 ms | 696,288,256 | 371 | 1 |

Interval 2 improves final L2 by 55.3% and L-infinity by 57.1% versus cooperative FP16. GPU cost is 104% above cooperative but 36.3% below full A2x4. The FP32 blocks require fewer dispatches because they omit reduced-path packing; the mixed policy adds no submit, readback, or retained-memory delta versus the same audit owner.

### Held-out: 128 tokens, width 1024, FFN 2048

| Policy | L2 by layer 0/1/2/3 | L-infinity by layer 0/1/2/3 | GPU | E2E |
|---|---|---|---:|---:|
| cooperative FP16 | 0.01984 / 1.601 / 13.648 / 147.42 | 0.00116 / 0.0842 / 1.072 / 3.267 | 13.557 ms | 21.189 ms |
| conventional FP16 | 0.01983 / 1.601 / 13.657 / 125.14 | 0.00116 / 0.0843 / 1.074 / 4.455 | 22.714 ms | 30.243 ms |
| full A2x4 FP32 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | 42.473 ms | 49.157 ms |
| **checkpoint interval 2** | 0.01984 / 1.610 / 13.915 / **131.41** | 0.00116 / 0.0852 / 1.116 / **2.487** | **27.438 ms** | **34.655 ms** |
| checkpoint interval 4 | 0.01984 / 1.601 / 13.648 / 137.92 | 0.00116 / 0.0842 / 1.072 / 3.464 | 19.923 ms | 27.196 ms |

Interval 2 improves cooperative final L2 by 10.9% and L-infinity by 23.9%. Conventional FP16 happens to have lower final L2 on this shape but materially worse L-infinity, is much slower than cooperative, and does not reverse the strong primary result. The held-out case supports the direction without establishing universal optimality.

## Complete-block D, controlled G, and stopped branches

The final per-layer A/B is the complete-block matched-input discrepancy evidence needed for control selection. Earlier suffix perturbations covered ten deterministic families and found near-neutral global L1/L2 gain with direction-local L-infinity amplification; complete-depth divergence therefore is not one catastrophic suffix defect. The periodic whole-block A/B is the higher-value controlled trajectory test and supersedes more suffix permutations.

Stopped branches are deliberate: retained FP32 Hidden, FP32-consuming Wdown, an FFN-only heterogeneous checkpoint, per-coordinate compensation, and interval-4 checkpoints are not the MVP. More perturbation families, magnitudes, and shapes are tuning work, not an M49a blocker.

## Numerical control parameters

M49b owns one validated record; constants must not be duplicated in recorder or shader code. All defaults are experimental. Every field is live-tunable without shader recompilation; `AuditSampleCount` is bounded by a preallocated maximum of 64.

| Field | Default | Allowed range | Meaning / scope | Invalid-value behavior |
|---|---:|---:|---|---|
| `CanaryInterval` | 4 blocks | 1–64 | Cadence for host-visible canary evaluation; backend-specific calibration, not device-name-specific | use 1 and request audit |
| `CheckpointInterval` | 2 blocks | 1–32 | Complete-block A2x4 cadence; workload/operator-specific | use 1 (all A2x4) while authority permits, otherwise observer-only |
| `EnterHighGainCount` | 2 | 1–16 | Consecutive unsafe observations before checkpoint recommendation | use 1 |
| `ExitHighGainCount` | 3 | 1–32 | Consecutive safe observations before one-step recovery | use 8 |
| `MaxL2Error` | 0.02 | 0–1 | Normalized relative L2 envelope, never raw shape-dependent L2; backend-family initial default | zero means no reduced-path authority |
| `MaxLInfError` | 0.05 | 0–1 | Normalized relative L-infinity envelope; backend-family initial default | zero means no reduced-path authority |
| `MaxGainEstimate` | 1.25 | 1–8 | Maximum accepted estimated inherited-error gain; operator-specific evidence may override | use 1 |
| `ConfidenceFloor` | 0.75 | 0–1 | Minimum evidence confidence for action; backend/device calibration overlay | use 1 and request audit |
| `FallbackCooldown` | 8 blocks | 1–256 | Minimum full-A2x4 residency after confirmed unsafe evidence | use 256 |
| `AuditSampleCount` | 16 coordinates | 4–64 | Deterministic coordinate sample count in addition to projections | use 64 |

Parameter validation is atomic. Reject NaN/Inf, inverted or out-of-range values, and partial records. A rejected update preserves the last valid record, emits an audit event, and cannot increase authority. With no valid record, run Stage 0 observer-only; if evidence is already unsafe, recommend full A2x4.

## Shadow-HSFM

Exactly nine states are retained because each has a different safety action. Transitions are deterministic, scored from a bounded evidence record, and included in replay/audit identity.

| State | Entry evidence | Exit/hysteresis | Permitted actions | Forbidden actions |
|---|---|---|---|---|
| `Unidentified` | reset, shape/backend/weight identity change, missing valid parameters | one valid audit at/above confidence floor | cooperative only in Stage 0; request audit | checkpoint/fallback authority from stale evidence |
| `Nominal` | valid evidence below all limits for `ExitHighGainCount` | first bounded breach -> `BoundedDrift`; immediate fault -> `Quarantined` | continue cooperative | correction or learned updates |
| `BoundedDrift` | one L2/L∞/gain breach below severe envelope | `EnterHighGainCount` safe -> `Nominal`; repeated breach -> `HighGain` | continue, increase audit cadence | silent fallback |
| `HighGain` | gain/concentration breach for `EnterHighGainCount` | safe for `ExitHighGainCount` -> `BoundedDrift`; envelope breach -> `CheckpointRecommended` | request audit/checkpoint | direct nominal recovery |
| `CheckpointRecommended` | confirmed drift/high gain with valid A2x4 path | post-checkpoint safe for `ExitHighGainCount` -> `BoundedDrift`; still unsafe -> `FallbackRecommended` | interval-2 complete-block checkpoint when Stage 2 permits | per-operator or output correction |
| `AuditRequired` | canary disagreement, low confidence, stale evidence, or checkpoint outcome unavailable | valid full audit selects `BoundedDrift`, `CheckpointRecommended`, `FallbackRecommended`, or `ReferenceSuspect` | audit; conservative current authority | increase authority |
| `FallbackRecommended` | severe envelope breach or unsafe post-checkpoint audit | after `FallbackCooldown` plus `ExitHighGainCount` safe audits -> `CheckpointRecommended` | full A2x4 when Stage 3 permits | direct return to nominal |
| `ReferenceSuspect` | A2x4/CPU/audit identities disagree or witness is stale | only a new coherent reference audit -> `Unidentified` | quarantine evidence; continue last known-safe path or A2x4 | train/tune from suspect evidence |
| `Quarantined` | nonfinite value, replay mismatch, lifecycle fault, uncertain completion | explicit resource reap/reset and new valid audit -> `Unidentified` | isolate slot/path, emit fault artifact | reuse quarantined resources or output |

Timeout is bounded by `CanaryInterval * 2` blocks without fresh evidence, which moves any non-quarantined state to `AuditRequired`. Recovery is one step at a time: full A2x4 -> checkpoint pattern -> bounded drift -> nominal. Authority stage can suppress an action but cannot suppress its shadow recommendation.

## Canary contract

Every complete block writes one bounded GPU canary record: 16 shape-derived deterministic coordinates, two seeded signed projections, one absolute projection, maximum absolute value, finite flag, and output/replay hash. Seeds and coordinates derive from shape plus replay identity, never one learned coordinate. The record is at most 256 bytes and does not require full tensor readback.

- Every block: device-side record production and hash/finite status.
- Every `CanaryInterval` blocks: asynchronous small-record readback and HSFM evaluation.
- Low cadence: an alternate A2x4 sample only from `AuditRequired`, checkpoint validation, or an explicit audit budget.
- Full audit trigger: two consecutive envelope breaches, any nonfinite/replay fault, low-confidence disagreement, reference suspicion, or an unsafe post-checkpoint record.
- Budget: canary observation target <=1% median GPU time, one added dispatch maximum, no submit, <=256-byte readback per interval, and no steady-state allocation. Exceeding budget disables canary authority and requests audit; it never silently drops a confirmed breach.
- False positive: at worst an audit/checkpoint/full-A2x4 interval; never tensor corruption.
- False negative: interval-2 checkpoints occur independently of the canary once Stage 2 is enabled, and periodic audits remain mandatory.

Research calibration used full readback differences only to establish correlation. Across 20 policy/layer samples, Pearson correlation was 0.997703 primary and 0.993052 held-out; the provisional `1e-6` paired thresholds produced zero observed false positives/negatives. The host full-tensor calibration cost was about 1.38 ms/sample and is **not** the M49b canary cost or a production threshold claim.

## Controller actions and authored utility

The bounded action set is: continue cooperative FP16, execute the interval-2 complete-block checkpoint, switch to full A2x4, request audit, quarantine suspect evidence/path, and recover after hysteresis. There is no shader backend, correction pass, online learning, or graph scheduler.

The Oct M1 utility features are normalized L2 disturbance, normalized L-infinity disturbance, and time. Authored weights are `[-0.60, -0.30, -0.10]` with zero bias. The current fitted candidate is `[+0.1516106014, +0.0607656952, -0.3622479649]` with bias `-0.529778`. It violates both error sign constraints and the leave-one-out stability gate (`2.63107 > 1.0`) and has zero held-out rows in its own comparable suffix corpus. Therefore authored conservative weights are the M49b initial default; the fitted model runs in shadow only and has no transition or selector authority. The interval-2 architecture is selected by hard safety eligibility plus the hardware A/B, not by the uncertified fit.

Later tuning appends comparable primary/held-out action rows, refits with nonpositive signs enforced, rejects unstable fits, compares against authored/uniform models, and promotes weights only through an explicit artifact/version change. Zero-held-out models remain uncertified.

## Product authority staging

1. **Stage 0 — observer only (M49b default):** emit canary, state, shadow action, cost, and replay identity; execute the existing product path.
2. **Stage 1 — shadow recommendation:** compare recommended versus executed action and record counterfactual timing/error audits; still no action authority.
3. **Stage 2 — bounded checkpoint authority:** only `CheckpointRecommended` may apply the fixed interval-2 complete-block pattern; invalid/low-confidence evidence remains observer-only or audit-required.
4. **Stage 3 — bounded fallback authority:** `FallbackRecommended` may select full A2x4 for the cooldown. Quarantine and lifecycle faults remain higher authority than numerical policy.

M49b implements all four stages in one structure, with Stage 0 as the default. Promotion is configuration/artifact authority, not another architecture milestone. No device-name branch is allowed.

## Exact M49b one-shot implementation specification

Runtime owner: the M48 fixed-stack runtime owns one numerical-control instance per logical stack stream. Add `reactor_numerical_control.h/.c`; do not put control state in shaders or Go user space. M48 consumes a bounded decision (`cooperative`, fixed interval-2 pattern, or full A2x4) before recording each complete block. The existing per-layer path seam becomes controller-owned only when authority >= Stage 2.

Implementation checklist:

1. Define `prom_num_control_parameters`, validator/defaults, authority stage, the nine-state enum, evidence, state counters, decision, and fixed-size audit record. No heap growth or map keyed by coordinates.
2. Implement pure deterministic transition/action functions with named considerations and traceable scores; unsafe/impossible candidates are explicitly ineligible. Tie-break order is full A2x4, checkpoint, cooperative.
3. Add the bounded GPU canary record using existing reduction/hash facilities, one preallocated per-slot buffer, and asynchronous <=256-byte readback at cadence. No full tensor readback in normal operation.
4. Bind lifecycle to logical stream, shape, backend semantic identity, weight generations, slot generation, and parameter version. Reset to `Unidentified` on identity change; quarantine uncertain completion.
5. Extend replay identity with authority stage, parameter-record hash, HSFM state/counters, evidence identity, selected fixed path pattern, and decision trace.
6. Record performance counters: canary GPU/CPU/readback time, checkpoint/fallback block count, dispatch/submit/readback delta, state dwell/transitions, suppressed actions, audits, quarantines, and output-hash changes.
7. Emit `prometheus.m49b.numerical-control.v1` JSON/Octagon audit artifacts containing parameters, evidence, state transition, candidates/eligibility/score, executed versus shadow action, replay identity, timings, and unsupported claims.
8. Add fault injection for canary breach, nonfinite, high gain, low confidence, stale parameter version, reference disagreement, missing A2x4, known recording failure, and uncertain completion.
9. Add pure transition/parameter/canary/replay tests; fixed-stack hardware tests for all actions and recoveries; repeatability, no-fallback, validation, zero-warm-allocation, overhead, and artifact identity tests. Keep language semantics out of Go tests.
10. Demonstrate primary trajectory improvement and no severe held-out regression, then leave authority at Stage 0 until a human promotes it.

M49b succeeds with one observer, one state machine, one bounded mixed policy, deterministic decisions, safe fallback, centralized tuning, measured overhead, clean lifecycle, shadow artifacts, primary improvement, bounded held-out support, and no generic scheduler. It does not require final thresholds, formal proof, cross-vendor calibration, or production authority.

## Validation and DVT handoff

Closeout hardware facts passed for the primary and smaller-expansion held-out cases. The audit pattern is replay-identified, product planning rejects it, warm allocation is zero, all outputs repeat bitwise across five runs, and no new shader was added. The completed validation matrix was:

- authoritative Windows MSVC build: passed;
- validation-enabled native lane: 427 facts, 395 passed, 32 skipped, 0 failed;
- focused `PrometheusM49` lane: 22 passed, 0 skipped, 0 failed;
- primary and held-out hardware A/B facts: passed, including interval-2 repeatability and clean Vulkan validation;
- aggregate `go test ./...`: passed;
- Linux build plus smoke lane: passed (5/5 build stages and 1/1 smoke fact); the pre-existing `sdslv_test_host.c:312` pointer-comparison warning remains visible;
- native manifest and SDSL workspace checks: passed;
- M1 Oct tests in compiled/no-fallback mode: 2/2 passed;
- M1 artifact generation in interpreted mode: 4/4 passed twice with identical output hashes;
- Linux shell syntax, JSON parsing, source/artifact identity checks, and `git diff --check`: passed.

The Oct artifact command itself is not claimed to support compiled execution: a compiled attempt stopped truthfully at the existing `ArtifactWriteMarkdown` unsupported boundary. This does not affect the compiled/no-fallback model tests or the deterministic interpreted artifact used here.

AMD DVT begins only after M49b. Run the same replay-fixed primary, smaller-expansion, awkward, more-token, and token-boundary matrix on the named AMD device/driver; verify cooperative feature/fallback semantics; measure canary overhead and interval 1/2/4; calibrate normalized thresholds and hysteresis; inject every controller fault; test Stage 0 through Stage 3 without device-name branching; and publish an AMD-specific parameter overlay only if evidence supports it. Do not copy RTX raw thresholds.

## Known uncertainty and unsupported claims

- A2x4 is an accuracy witness, not mathematical truth.
- One RTX held-out shape supports direction, not cross-shape, cross-family, cross-vendor, or production certification.
- Interval 2 is the MVP default, not a proven optimum.
- The canary correlation corpus is small and audit-derived; its thresholds and <=1% budget remain M49b work.
- Authored utility weights are conservative defaults; fitted weights remain uncertified shadow evidence.
- AMD behavior, final hysteresis, live tuning safety under concurrency, and product authority are not established.
- M48 EVT is not resumed, DVT is not started, and no CUDA/PTX result exists.

## Recommendation

**M49b READY TO IMPLEMENT. M48 EVT POSTPONED UNTIL M49B.** The selected complete-block interval-2 architecture is correct, bounded, reversible, inspectable, and tunable. Remaining uncertainty is explicit calibration work after the controller exists, not a reason to keep M49a open.
