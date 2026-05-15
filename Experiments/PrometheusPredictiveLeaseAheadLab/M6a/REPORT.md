# P15 M6a — Pre-Stage Actuator Readiness Rake Report

## 1) Problem statement
M6a evaluates whether simulated pre-stage actuation is safe enough to enable beyond the M5 reservation-only baseline, and identifies required gate conditions and diagnostics.

## 2) Relation to P15 M0–M5
M0–M5 established predictive lease-ahead semantics, bounded lookahead, delayed prediction rings, future lease request seams, reservation actuation, and correction reconciliation. M6a stays simulation-only and does not alter native actuator code.

## 3) Pre-stage lifecycle model
The lab models the pre-stage state machine: `None -> Eligible -> Submitted -> Ready -> Matured` with miss/error exits `Cancelled|Expired|Wasted`.

## 4) Scenario definitions
The model includes:
- stable high-confidence,
- warmup/low-confidence,
- prediction miss,
- step-change latency shift,
- resource pressure,
- false-positive spike,
- mixed hostile.

## 5) Policy candidates
- A: reservation-only baseline
- B: greedy pre-stage on every reservation
- C: confidence-gated
- D: confidence + correction-gated
- E: full Dominatus gated policy

## 6) Rake parameters
Rake dimensions include:
- confidence threshold: 0.60 / 0.75 / 0.85
- recent miss window: 3 / 5 / 8
- max pre-stage lead ticks: 1 / 2
- pre-stage cost level: low / medium

## 7) Scoring metrics
The lab computes:
- opportunity benefit,
- waste,
- false-stage rate,
- miss amplification,
- recovery time,
- safety compliance,
- composite score with small implementation complexity penalty.

## 8) Computed results (summary)
Observed directional results from the deterministic scenarios:
- Greedy policy wins opportunity in stable windows but incurs highest waste during miss, step-change, and false-positive spikes.
- Reservation-only is safest but leaves opportunity unrealized.
- Confidence-only gating improves over greedy but still stages through correction turbulence.
- Confidence + correction-gated policy substantially reduces false-stage and waste.
- Full Dominatus policy gives the best safety/composite balance under hostile mixed conditions while preserving high opportunity in stable windows.

## 9) Final recommendation
Proceed to native M6 pre-stage only with strict gating defaults:
- confidence >= 0.75,
- warmup = false,
- reservation already RESERVED,
- recent miss window clear (window 5),
- hard safety gates clear,
- resource pressure low,
- lead time <= 2 ticks.

If any gate signal is missing in native diagnostics, defer native enablement until instrumented.

## 10) Proposed native M6 diagnostics contract
Minimum fields:
- `prestage_valid`
- `prestage_state`
- `prestage_requested`
- `prestage_submitted`
- `prestage_ready`
- `prestage_cancelled`
- `prestage_expired`
- `prestage_wasted`
- `prestage_block_reason`
- `prestage_confidence`
- `prestage_target_tick`
- `prestage_lead_ticks`
- `prestage_cost_estimate`
- `prestage_benefit_estimate`

## 11) Required safety gates
- confidence gate
- warmup gate
- recent miss gate
- hard safety gate
- resource pressure gate
- lead-time gate
- reservation-state gate

## 12) Limitations
- synthetic deterministic traces only (no hardware traces),
- pre-stage modeled as abstract cost/benefit signals,
- no transfer/dispatch side effects,
- no native scheduling contention model.

## 13) Next milestone recommendation
M6 should implement diagnostics-first native scaffolding with Dominatus gates default-on and pre-stage action default-off behind feature flags; follow with narrow canary validation before any broader enablement.
