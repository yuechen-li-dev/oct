# Prometheus SGEMM Algorithm Lab — M12

## Predictive staging / delay-compensation simulation (pure Oct)

## 1) What was simulated

M12 simulates an SGEMM-shaped orchestration pipeline in pure Oct with bounded, deterministic abstractions:

- chunked work progression over a fixed work-unit budget
- transfer setup latency + per-unit transfer bandwidth cost
- per-unit compute cost
- overlap opportunity between staging and compute slices
- bounded lookahead and bounded outstanding staged depth
- deterministic prediction error regimes (perfect, mostly-correct, mediocre, poor)
- adaptive feedback policy that changes chunk size inside configured bounds

## 2) What was intentionally excluded

M12 intentionally does **not** simulate:

- unified-memory page migration fidelity
- cache coherence and protocol-level hardware behavior
- PCIe packet-level mechanics
- sparse/irregular access patterns
- real GPU backend execution or Vulkan timing

This keeps the milestone focused on signal finding rather than false realism.

## 3) State variables

The simulation state includes:

- remaining work units
- current chunk size (and adaptive updates)
- prefetched-ready chunk pool
- overlap carry budget (compute window remaining for transfer progress)
- prediction cursor
- accumulated timing metrics and transfer accounting
- peak outstanding depth usage

## 4) Policy/control knobs

M12 studies:

- baseline policy: `eager`
- baseline policy: `jit`
- baseline policy: `overlap`
- adaptive policy: `adaptive` (feedback chunk-size control)
- chunk size sweep (small and large chunks)
- lookahead sweep (`0..3` in sampled cases)
- outstanding depth sweep (`1..3` in sampled cases)
- prediction quality sweep (`0 perfect`, `1 mostly-correct`, `2 mediocre`, `3 poor`)

## 5) Metrics collected

Per case, M12 emits:

- total completion time
- transfer stall time
- compute idle time
- total transfer work
- wasted transfer work due to wrong prediction
- average chunk size used
- peak outstanding depth usage
- chunk transition count

## 6) Baselines compared

M12 includes all required baseline orchestration references:

1. eager stage-then-compute
2. chunked just-in-time staging
3. overlapped chunked staging

Plus one adaptive policy:

4. overlap + feedback chunk-size adaptation

## 7) Workload regimes

M12 evaluates at least four distinct parameterized regimes:

- transfer-bound
- compute-bound
- balanced
- predictability-sensitive

## 8) Results summary by regime

Deterministic output patterns from `m12_predictive_staging_study.octagon`:

- **Transfer-bound:** best sampled case was `adaptive` (`2936`) vs eager/JIT (`4032`) and overlap fixed (`3092`).
- **Compute-bound:** `overlap` and `adaptive` tied at `3094`, both better than eager/JIT (`3776`).
- **Balanced:** best sampled case was `overlap` with small chunks (`1560`) vs eager/JIT (`3072`).
- **Predictability-sensitive:** perfect prediction overlap (`1894`) beat mediocre (`2184`) and poor (`2358`); adaptive recovered part of mediocre loss (`2030`).

### Best-performing sampled policy by regime

- transfer-bound: **adaptive**
- compute-bound: **overlap** / **adaptive** (tie in sampled set)
- balanced: **overlap** (small chunk configuration)
- predictability-sensitive: **overlap with perfect prediction** (adaptive best among imperfect-prediction samples)

## 9) Is predictive/feedback staging promising?

**Yes, conditionally.**

Signal is strongest where transfer pressure is meaningful and prediction quality is at least moderate. Benefits degrade when prediction quality is poor and speculative waste climbs.

## 10) Carry-forward ideas

Most promising carry-forward items:

- bounded overlap + modest lookahead (`1..2`) as stable defaults
- adaptive chunk sizing with strict min/max bounds
- explicit tracking of speculative waste as a first-class controller guardrail

## 11) What did not help

- aggressive lookahead under poor prediction quality (waste grows)
- expecting large gains in compute-dominant regimes
- unbounded speculation (intentionally excluded; bounded depth performed more legibly)

## 12) Inconsistency / documentation-gap check

No deliberate syntax/style divergence from `Language/reference` was introduced for this milestone.
