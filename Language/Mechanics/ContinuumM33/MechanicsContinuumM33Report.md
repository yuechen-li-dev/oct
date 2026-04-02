# Mechanics Continuum M33 — Strengthened Global Consistency Probe (Result-Tightened)

M33 keeps the M32 execution regime frozen (same 4 probes, fixed cap 8, same `Delta(k) < 0.5 * Delta(1)` horizon rule, same correction family with `alpha=0.2`) and changes only the global consistency operator.

## Aggregate comparison

| Metric | M32 Path A (baseline) | M33 Path B (strengthened) |
|---|---:|---:|
| MeanDelta1 | 2.073194 | 0.444702 |
| MeanDelta2 | 1.325704 | 0.309848 |
| PracticalHorizonK (`k*`) | 8 | 8 |
| MeanResidualAtK | 4.773679 | 1.331320 |

Direct reading:
- Early gain **did not improve** (`Delta1`, `Delta2` both smaller in M33).
- Practical horizon **did not improve** (`k*` unchanged at 8).
- Residual-at-horizon is lower for M33, but this sits on a reweighted residual definition, so this alone is not enough to claim a stronger operational regime.

## Per-probe comparison

| Probe | Path | Step0 | Step1 | Step2 | Step8 (final) | Probe Horizon | Early gain vs M32 |
|---|---|---:|---:|---:|---:|---:|---|
| Horizontal | M32 | 8.762508 | 7.883590 | 7.093341 | 3.767721 | 8 | — |
| Horizontal | M33 | 2.117382 | 1.904583 | 1.713305 | 0.909093 | 8 | Worse (`Delta1` -0.666118, `Delta2` -0.598970) |
| Vertical | M32 | 8.762508 | 7.883590 | 7.093341 | 3.767721 | 8 | — |
| Vertical | M33 | 2.110301 | 1.899735 | 1.710166 | 0.910204 | 8 | Worse (`Delta1` -0.668352, `Delta2` -0.600679) |
| Diagonal | M32 | 16.653506 | 13.384905 | 11.524877 | 5.779636 | 3 | — |
| Diagonal | M33 | 4.333022 | 3.646382 | 3.208852 | 1.754117 | 4 | Worse (`Delta1` -2.581961, `Delta2` -1.422498) |
| Opposite diagonal | M32 | 16.653506 | 13.387168 | 11.524877 | 5.779636 | 3 | — |
| Opposite diagonal | M33 | 4.256256 | 3.587455 | 3.166440 | 1.751865 | 4 | Worse (`Delta1` -2.597537, `Delta2` -1.441277) |

## Diagonal-vs-axial judgment (concrete)

Using step-1 early-gain change (`M33 Delta1 - M32 Delta1`):
- Axial average (horizontal + vertical): **-0.667235**
- Diagonal-family average (diagonal + opposite): **-2.589749**

Conclusion: **No**, diagonals did not benefit more; they regressed more strongly than axial probes on early-gain metrics.

## Stability

Across the full fixed cap (`0..8`):
- Residual is monotone decreasing for all 4 probes in both paths.
- Per-step deltas remain nonnegative and diminishing for all 4 probes in both paths.
- No oscillation or instability was observed.

## Blunt verdict

**M33 is a wash.**

- Early trajectory behavior is worse by the required early-gain metrics (`MeanDelta1` and `MeanDelta2` both drop).
- Practical horizon `k*` does not improve (8 → 8).
- Residual-at-horizon is lower, but this comes with the strengthened operator’s reweighting, not faster practical saturation.
- The pass stays architecturally clean and stable, but it does not deliver the target “smarter consistency beats iteration” win on early-horizon efficiency.

## Next-step recommendation

Move to the first explicit convergence-boundary pass rather than adding more operator complexity. M33 shows stable behavior and preserved structure, but it does not improve early gain or horizon efficiency under the frozen M32 schedule.
