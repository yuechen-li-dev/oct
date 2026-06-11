# Prometheus FFT Algorithm Lab M1 (Oct correctness tapeout)

## Representation + Semantics

Complex values use explicit parallel arrays (Re/Im). Forward FFT is unnormalized; inverse FFT divides by N.

## Test Matrix

N in {1,2,4,8,16}; families: impulse, constant, alternating sign, single-bin tone, optional two-bin tone (N>=4), deterministic arbitrary. Non-power-of-two and N=0 are explicit rejection checks.

## Summary

| key | value |
| --- | --- |
| total | 28 |
| passed | 28 |
| failed | 0 |
| tolerance | 0.0001 |
| rejectN0 | true |
| rejectN3 | true |

## Limitations

Reference path prioritizes determinism and clarity over performance. Twiddle and stage traces are captured for representative N=8/N=16 cases only.
