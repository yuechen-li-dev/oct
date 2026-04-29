# P13 M15 / Prometheus SGEMM Algorithm Lab M52 — Full Occupancy Variant Recipe Completion Lab

## 1) Context and goal
M52 finalizes benchmark-only recipe contracts for unresolved occupancy selector variants using an executable Oct scoring model.

## 2) Fixed variants from prior work
- `baseline-scalar -> existing baseline/current path`
- `small-register-tile -> SRT-2accum-K`

## 3) Unresolved variant classes
- `memory-conservative`
- `balanced-2x2-accum4`
- `aggressive-4x4-accum8`

## 4) Candidate recipes
Each unresolved class evaluates three candidates (MC-A/B/C, B-A/B/C, A-A/B/C) in executable model form.

## 5) Recipe fields and scoring
The model includes all required structural fields (tile shape, accumulators, register/shared-memory/local-pressure estimates, reuse, ILP, coalescing risks, tail complexity, implementation/correctness risk, shape coverage, benchmark-only suitability, production readiness risk) and computes total score from class-fit + robustness + safety + shape suitability.

## 6) Shape-class interactions
The model applies modifiers for:
- `small-square`, `medium-square`, `large-square`
- `tall-skinny`, `wide-short`
- `K-heavy`, `ML/FFN-like`

Row/column bias and K-split bias alter class suitability by shape.

## 7) Findings by variant class
- `memory-conservative`: winner is low-pressure and safety-prioritized.
- `balanced-2x2-accum4`: winner keeps true accum4 semantics with manageable complexity.
- `aggressive-4x4-accum8`: winner remains benchmark-only with explicit production-readiness risk.

## 8) Final recipe map
- `baseline-scalar -> existing baseline/current path`
- `memory-conservative -> MC-baseline-strict`
- `small-register-tile -> SRT-2accum-K`
- `balanced-2x2-accum4 -> B2x2-row-major-biased`
- `aggressive-4x4-accum8 -> A2x4-row-biased-accum8`

## 9) P13 M16 implementation contract
Implement exactly the computed map above as benchmark-only contracts for the three resolved classes, with aggressive variants shape-gated and dispatch-actuation still deferred.

## 10) Deferred scope
No Vulkan/SPIR-V work, no runtime dispatch changes, no hardware tuning, no performance claims.

## Required direct answers
1. `memory-conservative`: `MC-baseline-strict`.
2. `balanced-2x2-accum4`: `B2x2-row-major-biased`.
3. `aggressive-4x4-accum8`: `A2x4-row-biased-accum8`.
4. Better fit rationale: class-role-aligned safety/complexity tradeoffs dominate theoretical upside.
5. First shape support:
   - memory-conservative: small-square, medium-square, ML/FFN-like fallback lanes.
   - balanced: medium/large-square, wide-short, ML/FFN-like.
   - aggressive: large-square, wide-short, selected ML/FFN-like K bands.
6. Benchmark-only variants: balanced/aggressive in M52 contract; aggressive explicitly benchmark-only.
7. Possible future production candidates: memory-conservative first; balanced maybe later after validation.
8. M16 should implement selected concrete recipes and keep dispatch deferral/safety gates.
9. Deferred: all kernel-level/native execution and tuning.
