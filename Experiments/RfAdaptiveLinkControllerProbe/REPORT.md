# RF + Octomata Integration Report — RfAdaptiveLinkControllerProbe

## 1. Goal

This experiment is a deterministic systems-integration probe that composes two existing parts of Oct:

- RF helpers provide an evolving channel-quality trace.
- Octomata flows provide controller logic reacting to that trace.

The target is modest: validate that RF world modeling and Octomata control can be packaged and tested together without additional framework glue.

## 2. RF condition sequence

The RF trace is intentionally deterministic and small (12 samples):

- distance ramp via `LinearDistanceSeries`
- free-space path loss via `FreeSpacePathLossLinearSeries`
- fixed, hand-authored deterministic shadowing schedule applied through `ApplyShadowingToPathLossSeries`
- received-power sequence computed from RF link-budget helper `ReceivedPower`
- thermal-noise sequence from `ThermalNoisePowerWithNoiseFigureSeries`
- final `SNRLinearSeries` converted to compact integer quality scores

This yields a trace that starts healthy, enters a marginal band, then degrades into poor conditions.

## 3. Controller design

The controller side has two Octomata flows:

1. `ClassifyLinkStep` (small readable state machine):
   - `Healthy`
   - `Watch`
   - `Recover`

2. `RunAdaptiveLinkController` (time-indexed sequence controller):
   - consumes the deterministic RF quality sequence
   - emits chosen `LinkState` at each step
   - uses utility selection policy to preserve prior state when challenger wins are small or too early

A second baseline flow, `RunNaiveController`, uses plain thresholds for direct comparison.

## 4. Stability behavior

Anti-flapping behavior is implemented in `RunAdaptiveLinkController` with utility policy:

- `hysteresis: 9`
- `min_commit: 2`

This is applied over the marginal middle region of the sequence, where values hover near Healthy/Watch boundaries. The intent is to avoid noisy flip-flop transitions and only change mode when the challenger is clearly better and the commit window allows it.

## 5. Observed behavior

Tested behavior shows:

- RF sequence drives controller deterministically across runs.
- High quality maps to `Healthy`, degraded tail maps to `Recover`.
- Marginal segment tends to remain stable (`Healthy`/`Watch`) instead of immediate panic switching.
- Transition-count comparison demonstrates adaptive policy does not flap more than naive thresholding.

Observability is exercised with `Active`, `Complete`, `Result`, and `StateHistory`.

## 6. Observations

### What felt clean

- RF helper composition is straightforward for deterministic “world state” construction.
- Octomata utility policy expresses anti-flapping intent directly (rather than scattered threshold glue).
- End-to-end integration is testable with concise `.octest` coverage.

### What felt awkward

- Full multi-state per-timestep visual traces are easier to compare by returned mode histories than by state-history names, since sequence control here runs inside one loop state.
- Quality discretization is intentionally simple; richer controllers would likely want a more direct score modeling convention.

## 7. Conclusion

Yes—RF + Octomata now compose cleanly for a small, deterministic simulation/control experiment. The package stays first-principles and standards-agnostic, and demonstrates that RF-modeled environment traces can drive stable Octomata control logic with runnable integration tests.
