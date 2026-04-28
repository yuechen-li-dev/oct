# P13 M7 / Prometheus SGEMM Algorithm Lab M48 — Small-Register-Tile Variant Recipe Rake Lab

## 1) Why `small-register-tile` is ambiguous

`small-register-tile` is a category label, not an implementable shader recipe.

At least five materially different interpretations exist:

1. One thread computes two C elements in a row (`1x2` along N).
2. One thread computes two C elements in a column (`2x1` along M).
3. One thread computes one C element but splits K reduction into two independent accumulators (`2accum-K`).
4. One thread computes a `2x2` output tile.
5. Keep one-element output but increase workgroup tile/looping shape without changing per-thread accumulator count.

These are not equivalent:

- **Register pressure:** 2x2 has four accumulators and materially higher live values than 1x2/2x1/2accum-K.
- **Memory coalescing:** 1x2-row is naturally aligned with row-major contiguous stores, while 2x1-col tends to stride stores across rows.
- **Shared-memory use:** output tiling variants usually increase staging pressure; 2accum-K can preserve output mapping while changing dependency structure.
- **Tail handling:** 1x2-row mostly exposes N tails; 2x1-col mostly exposes M tails; 2x2 exposes both.
- **Implementation complexity and correctness risk:** dual-dimension tails and four-accumulator update wiring are substantially riskier for first bring-up.

Therefore M48 must choose one precise recipe before M8 implementation.

## 2) Candidate recipes

Executable candidates modeled in M48:

- `baseline-control` (reference only)
- `SRT-1x2-row`
- `SRT-2x1-col`
- `SRT-2accum-K`
- `SRT-2x2`

## 3) Recipe model fields

Each recipe model includes:

- output tile rows/cols,
- accumulator count,
- estimated registers per thread,
- A reuse factor,
- B reuse factor,
- arithmetic intensity gain,
- coalesced load risk,
- coalesced store risk,
- shared-memory pressure,
- tail dimension affected,
- masked load/store complexity,
- implementation complexity,
- correctness risk,
- benchmark harness integration complexity,
- robust usefulness base.

All values are modeled policy inputs (not measured performance claims).

## 4) Shape-class interaction

M48 evaluates all M45 shape classes:

- small-square
- medium-square
- large-square
- tall-skinny
- wide-short
- K-heavy
- ML/FFN-like

Shape profiles weight M/N/K influence and tail risk.

Key modeled effects:

- `SRT-1x2-row` gets stronger boost from N-heavy (`wide-short`, FFN-like) profiles.
- `SRT-2x1-col` gets stronger M-heavy boost (`tall-skinny`) but pays row-major store-risk penalties.
- `SRT-2accum-K` gets explicit K-heavy benefit without output-tail expansion.
- `SRT-2x2` can score well on balanced large shapes but is strongly penalized by safety and complexity.

## 5) Safety-margin model

Following M47 philosophy, M48 avoids exact per-GPU Pareto tuning and uses robust safety penalties:

- explicit safety margin input (`safetyMarginPermille`),
- register-pressure penalty scaling,
- tail-complexity penalty scaling,
- benchmark-dependence penalty scaling.

Higher safety margin increases penalties, with larger impact on aggressive recipes (especially `SRT-2x2`).

## 6) Scoring model

Per-shape score combines:

- robust usefulness base,
- shape boost from M/N/K profile,
- arithmetic intensity gain,

minus penalties for:

- register pressure (with safety margin),
- coalescing risk,
- tail complexity,
- implementation/correctness/harness complexity,
- benchmark dependence risk.

Aggregate recipe score combines per-shape scores plus:

- shape coverage count,
- benchmark readiness,
- future progression value.

Recommendation is selected only from computed top aggregate score.

## 7) Findings

From computed model outputs:

- `SRT-2x2` is consistently the most aggressive option (registers, tail complexity, implementation/correctness burden).
- `SRT-1x2-row` and `SRT-2x1-col` differ materially on coalescing risk under row-major C (column tiling has higher store risk).
- `SRT-2accum-K` improves relative suitability for K-heavy shapes while keeping output-tail behavior simple.
- `SRT-1x2-row` performs strongly on wide-short and FFN-like classes and remains competitive on medium/large square classes.
- safety margins push harder against `SRT-2x2` than against small variants.

## 8) Final recommendation

Computed M48 recommendation: **`SRT-2accum-K`**.

Why this is safest for first implementation:

- keeps accumulator count at 2 (small step up from baseline),
- improves K-loop ILP without introducing output-tile mapping changes,
- avoids row/column output-tail amplification,
- keeps coalesced store behavior close to baseline,
- has lower integration and correctness risk than `SRT-2x2`,
- remains robust across shape classes while still improving K-heavy suitability.

## 9) P13 M8 implementation contract

M8 should implement exactly:

- recipe: `SRT-2accum-K`,
- scope: **benchmark-only** variant,
- precision: **FP32-only**,
- basis: existing baseline/tiled FP32 SGEMM code path,
- integration: **new benchmark-only variant id** (not runtime dispatch actuation),
- unavailable behavior: fallback to baseline-control benchmark path with deterministic diagnostic reason,
- first shape-class support focus:
  - K-heavy,
  - ML/FFN-like,
  - medium-square,
  - large-square,
- tail policy:
  - no special output-tail path required for the variant itself,
  - retain baseline fallback for launch/edge cases and unsupported mixed-tail situations.

Benchmark harness integration requirements:

- use existing deterministic shape-case framework,
- preserve correctness-first gates,
- preserve timing confidence diagnostics,
- record variant id and fallback reason in artifact schema,
- keep manual override seam available.


## Direct answers (required)

1. **Which concrete small-register-tile recipe should be implemented first?**
   - `SRT-2accum-K` from computed aggregate scoring.
2. **Why is it safer/better than alternatives?**
   - It improves K-loop ILP while avoiding output-tile remapping tails and avoiding 4-accumulator pressure.
3. **Which shape classes should it support first?**
   - K-heavy, ML/FFN-like, medium-square, large-square.
4. **How should tails be handled?**
   - No extra output-tail specialization for this recipe; keep baseline fallback for edge/unsupported tails.
5. **New compute mode or benchmark-only variant id?**
   - Benchmark-only variant id (no dispatch actuation).
6. **FP32-only?**
   - Yes, FP32-only for first implementation.
7. **How to integrate with occupancy benchmark harness?**
   - Reuse existing deterministic shape suite, correctness-first gates, timing confidence diagnostics, and artifact diagnostics with fallback reasons.
8. **What remains deferred?**
   - Native shader/SPIR-V rollout, runtime dispatch actuation, hardware tuning, `SRT-2x2`, balanced-2x2-accum4, FP16/Packed4, and autotune/per-GPU response surfaces.

## 10) Deferred scope

Explicitly deferred beyond M48/M8:

- Vulkan shader implementation details,
- SPIR-V generation,
- runtime dispatch actuation,
- real-hardware benchmarking claims,
- `SRT-2x2` rollout,
- `balanced-2x2-accum4` implementation,
- FP16/Packed4 integration,
- runtime autotune,
- per-GPU response-surface tuning.

## Documentation consistency note

M48 uses enum-typed record fields and computed artifact generation, consistent with current `Language/reference` record/enum patterns.
