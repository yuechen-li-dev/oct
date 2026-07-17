# Prometheus M49a native evidence import

## Authority

| key | value |
| --- | --- |
| native evidence | native-rtx3070-vulkan |
| synthetic evidence | prometheus.m49.synthetic-design-lab-only |
| source JSON | internal/prometheus/DevelopmentReport/artifacts/M49a/controlled_stage_gain_and_mitigation_rtx3070.json |
| source SHA-256 | 668a8768fbcb42ef1c386ad73e89b85c079f8d227965408bf157d3b3347cc5f7 |
| product authority changed | false |

## Envelope evaluation

| key | value |
| --- | --- |
| identification support | 7 |
| held-out support | 0 |
| held-out failures | 0 |
| D bound | 1.48932e-06 |
| G bound | 1.00064 |
| certified | false |

## Evidence-fitted shadow utility

| key | value |
| --- | --- |
| algorithm | ridge-normal-equations-lu-v1 |
| training rows | 7 |
| held-out rows | 0 |
| training MSE | 0.002247484797368486 |
| held-out MSE | 0 |
| learned selected path | gpu_conventional_fp16 |
| authored selected path | gpu_conventional_fp16 |
| uniform selected path | gpu_conventional_fp16 |
| certified | false |
| shadow only | true |

## Interpretation

native identification imported; no envelope or mitigation is certified without held-out hardware support

reject any path, stage, shape, family, or magnitude absent from held-out support
