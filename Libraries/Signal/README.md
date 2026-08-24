# Signal (M0a)

## Shelf boundary

`Signal` owns generic finite-sequence convolution, correlation, and moving averages. Use [`Mathematics`](../Mathematics/README.md) or the built-in `FFT` for transforms, [`RF`](../RF/README.md) for radio-frequency channel mathematics, and [`Wireless`](../Wireless/README.md) for communication/link calculations. Start with `Convolve` and `Correlate` below.

Time-domain, deterministic signal helpers for **dimensionless `Float[]`** inputs.

## Scope

M0a intentionally includes only foundational finite-sequence operations:

- `Convolve(x, h)`
- `Correlate(x, y)`
- `MovingAverage(x, window)`
- `AutoCorrelate(x)` (thin helper over `Correlate(x, x)`)

Out of scope for M0a: FFT, spectral analysis, windowing families, filtering frameworks, and resampling.

## Conventions

### `Convolve(x, h)`

- Finite linear convolution (not circular)
- Requires `Len(x) > 0` and `Len(h) > 0`
- Output length: `Len(x) + Len(h) - 1`
- Indexing: `y[n] = Σ_i x[i] * h[n-i]` over valid indices

### `Correlate(x, y)`

- Discrete cross-correlation with explicit lag ordering
- Requires `Len(x) > 0` and `Len(y) > 0`
- Output length: `Len(x) + Len(y) - 1`
- Output index `0` corresponds to lag `k = -(Len(y)-1)`
- Last index corresponds to lag `k = Len(x)-1`
- Definition: `rxy(k) = Σ_i x[i] * y[i-k]` over valid indices

### `MovingAverage(x, window)`

- Simple moving average, **valid-window only**
- No padding and no edge extension
- Requires:
  - `Len(x) > 0`
  - `window > 0`
  - `window <= Len(x)`
- Output length: `Len(x) - window + 1`
