# Prometheus M49a native evidence import

## Authority

| key | value |
| --- | --- |
| native evidence | native-rtx3070-vulkan |
| synthetic evidence | prometheus.m49.synthetic-design-lab-only |
| source JSON | internal/prometheus/DevelopmentReport/artifacts/M49a/controlled_stage_gain_and_mitigation_rtx3070.json |
| source SHA-256 | e1cebf242a99d1a06cd761ba1541fc20fe87efd2eb8dd94c81b199718d1d1406 |
| product authority changed | false |

## Envelope evaluation

| key | value |
| --- | --- |
| identification support | 10 |
| held-out support | 0 |
| held-out failures | 0 |
| D bound | 1.48932e-06 |
| G bound | 1.00064 |
| certified | false |

## Evidence-fitted shadow utility

| key | value |
| --- | --- |
| algorithm | ridge-normal-equations-lu-v1 |
| training rows | 10 |
| held-out rows | 0 |
| training MSE | 0.0651465839129063 |
| held-out MSE | 0 |
| learned selected path | gpu_conventional_fp16 |
| authored selected path | gpu_conventional_fp16 |
| uniform selected path | gpu_conventional_fp16 |
| certified | false |
| shadow only | true |

## Interpretation

native identification imported; no envelope or mitigation is certified without held-out hardware support

reject any path, stage, shape, family, or magnitude absent from held-out support
