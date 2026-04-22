> **Status (2026-04-22):** Historical cloud timing artifact only. Do not use as active controller/algorithm scoring authority in the current pure-Oct phase.

# M4 Cloud Measurement Output

Source artifacts:
- `out/prometheus/sgemm_lab_m4/cloud_m4_bench.octagon`
- `Experiments/PrometheusSgemmAlgorithmLab/M4/m4_choose_strategy.octagon`

## Per-shape strategy durations (single run, ns)

### Shape K16x16x512 (16,16,512)
- ChosenStrategy: `KBlockedIKJ`
- FastestObserved: `Baseline` (13479947 ns)

| Strategy | DurationNs |
|---|---:|
| Baseline | 13479947 |
| IKJ | 14383083 |
| KIJ | 15796311 |
| KBlocked | 15970488 |
| KBlockedIKJ | 17712790 |
| Blocked | 19557213 |
| BlockedIKJ | 22164837 |
| StagedIKJ | 23931398 |

### Shape K8x8x256 (8,8,256)
- ChosenStrategy: `KBlockedIKJ`
- FastestObserved: `KBlocked` (12581032 ns)

| Strategy | DurationNs |
|---|---:|
| KBlocked | 12581032 |
| KIJ | 12694629 |
| Blocked | 14040366 |
| Baseline | 14152125 |
| KBlockedIKJ | 14740420 |
| IKJ | 15147932 |
| StagedIKJ | 15326868 |
| BlockedIKJ | 19857676 |

### Shape R16x64x8 (16,8,64)
- ChosenStrategy: `KBlockedIKJ`
- FastestObserved: `KIJ` (11147120 ns)

| Strategy | DurationNs |
|---|---:|
| KIJ | 11147120 |
| KBlocked | 11215444 |
| IKJ | 13068037 |
| Baseline | 13102342 |
| KBlockedIKJ | 13256054 |
| BlockedIKJ | 13755386 |
| StagedIKJ | 14297674 |
| Blocked | 16648783 |

### Shape R8x128x16 (8,16,128)
- ChosenStrategy: `KBlockedIKJ`
- FastestObserved: `Baseline` (11710700 ns)

| Strategy | DurationNs |
|---|---:|
| Baseline | 11710700 |
| KIJ | 12546614 |
| KBlockedIKJ | 13079465 |
| IKJ | 13863817 |
| KBlocked | 13957905 |
| BlockedIKJ | 14397144 |
| Blocked | 14398549 |
| StagedIKJ | 16138023 |

### Shape S16x16 (16,16,16)
- ChosenStrategy: `KBlocked`
- FastestObserved: `Baseline` (11696035 ns)

| Strategy | DurationNs |
|---|---:|
| Baseline | 11696035 |
| IKJ | 12032044 |
| KBlockedIKJ | 12050758 |
| KIJ | 13269492 |
| KBlocked | 13468957 |
| Blocked | 13895520 |
| StagedIKJ | 15240650 |
| BlockedIKJ | 15384852 |

### Shape S2x2 (2,2,2)
- ChosenStrategy: `IKJ`
- FastestObserved: `IKJ` (11374587 ns)

| Strategy | DurationNs |
|---|---:|
| IKJ | 11374587 |
| KBlocked | 12702395 |
| Blocked | 13391952 |
| BlockedIKJ | 13576456 |
| KBlockedIKJ | 13830439 |
| KIJ | 14205080 |
| Baseline | 15572877 |
| StagedIKJ | 15681640 |

### Shape S32x32 (32,32,32)
- ChosenStrategy: `KBlockedIKJ`
- FastestObserved: `KIJ` (12722031 ns)

| Strategy | DurationNs |
|---|---:|
| KIJ | 12722031 |
| IKJ | 14085124 |
| KBlockedIKJ | 14354984 |
| Baseline | 15569724 |
| Blocked | 16674732 |
| StagedIKJ | 16934794 |
| KBlocked | 18153360 |
| BlockedIKJ | 21715110 |

### Shape S4x4 (4,4,4)
- ChosenStrategy: `IKJ`
- FastestObserved: `KBlocked` (11236997 ns)

| Strategy | DurationNs |
|---|---:|
| KBlocked | 11236997 |
| Baseline | 11339188 |
| StagedIKJ | 11586050 |
| KBlockedIKJ | 12445014 |
| IKJ | 12846139 |
| Blocked | 12875789 |
| KIJ | 13774271 |
| BlockedIKJ | 15120178 |

## Mispredictions (cloud directional signal)
- K16x16x512: chooser=KBlockedIKJ, observed-fastest=Baseline
- K8x8x256: chooser=KBlockedIKJ, observed-fastest=KBlocked
- R16x64x8: chooser=KBlockedIKJ, observed-fastest=KIJ
- R8x128x16: chooser=KBlockedIKJ, observed-fastest=Baseline
- S16x16: chooser=KBlocked, observed-fastest=Baseline
- S32x32: chooser=KBlockedIKJ, observed-fastest=KIJ
- S4x4: chooser=IKJ, observed-fastest=KBlocked
