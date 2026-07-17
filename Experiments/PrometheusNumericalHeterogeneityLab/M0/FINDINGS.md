# Prometheus M49 numerical heterogeneity synthetic lab

## Authority

This Oct experiment is a deterministic design simulation. It does not replace native matched-input CPU/GPU evidence and cannot authorize a product-path change.

## Corpus

| key | value |
| --- | --- |
| cases | 12 |
| splits | identification + held-out |
| families | bounded-random, cancellation, sparse-outlier, near-zero |
| paths | conventional, cooperative, A2x4 |

## Selected bounded intervention

| key | value |
| --- | --- |
| mitigation | FP32-checkpoint |
| baseline held-out mean error | 0.4822114810305888 |
| selected held-out mean error | 0.03932125908594749 |
| held-out improvement percent | 91.8456402153865 |
| shadow action | request-checkpoint-audit |
| product authority changed | false |

## Evidence-fitted utility policy

| key | value |
| --- | --- |
| algorithm | ridge-normal-equations-lu-v1 |
| training rows | 24 |
| held-out rows | 24 |
| training MSE | 0.018899154578680608 |
| held-out MSE | 0.027259904063623705 |
| authored baseline held-out MSE | 0.5623550212884366 |
| uniform baseline held-out MSE | 7.23645743727502 |
| learned selection | FP32-checkpoint |
| authored selection | FP32-checkpoint |
| uniform selection | backend-route |
| weight: initial error | 0.08545325596168596 |
| weight: effective gain | -0.045407476919057055 |
| weight: effective injection | -0.2171366883710915 |
| weight: latency | 0.024757095229672194 |
| weight: memory | 0.0017660971880111888 |
| certified | true |

## Interpretation

Synthetic recurrence study only. Identification and held-out cases are deterministic, but native matched-input evidence remains authoritative.

M48 EVT remains postponed until the real hardware matrix and held-out source-side mitigation evidence converge.
