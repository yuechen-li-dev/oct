# M5 Findings

## Scope

Bounded 27-case deterministic direct-message brown-noise sweep; no FM/IQ/audio realism.

## Architecture

Scalar board preserved; history arrays external to Octomata board.

## Algorithm

Windowed lag-1 estimator with windowSize=32; smoothed A update with alpha=0.2; clamp range [-0.99,0.99].

## Key counts

| key | value |
| --- | --- |
| scalarAdaptiveWin | 15 |
| scalarRecoveryOnly | 0 |
| scalarWhitenessOnly | 12 |
| scalarNoMeaningfulWin | 0 |
| windowedAdaptiveWin | 1 |
| windowedRecoveryOnly | 0 |
| windowedWhitenessOnly | 22 |
| windowedNoMeaningfulWin | 4 |
| windowedBetterRecovery | 10 |
| windowedBetterWhiteness | 0 |
