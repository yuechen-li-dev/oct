# M6 Findings

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

## Mean behavior

| key | value |
| --- | --- |
| scalarMeanDeltaOutputSNRDb | -0.10323719568627238 |
| windowedMeanDeltaOutputSNRDb | -0.01281063531133515 |
| scalarMeanWhitenessRatio | 0.4599244600302107 |
| windowedMeanWhitenessRatio | 0.9206405349794723 |
| scalarFinalAMean | 0.9727125543420385 |
| windowedFinalAMean | 0.5052912639818644 |
| scalarClampTotal | 12512 |
| windowedClampTotal | 0 |

## Scientific interpretation

Result pattern: scalar incremental adaptation preserves much stronger whitening than windowed adaptation (lower mean whiteness ratio), while windowed adaptation occasionally improves recovery relative to scalar (10/27 cases) but rarely wins jointly against fixed.

The data therefore supports the M6 design hypothesis: adaptation objective choice is a primary issue. Residual whitening and recovered-message quality are related but non-identical objectives, so optimizing only whitening can miss recovery quality goals.

Clamp behavior is also informative: scalar adaptation frequently saturates the A clamp while windowed adaptation does not. This suggests windowing regularizes dynamics but may underfit colored residual structure in this toy setup.

## What this does and does not show

> **Note:** This is synthetic direct-message brown-noise evidence only. It is not real FM/IQ/audio receiver validation, not deployable-policy evidence, and not a robustness theorem.
