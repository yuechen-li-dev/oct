# M6a Windows-Native Prometheus Summary

Environment:
- Windows
- `OCT_PROMETHEUS_REACTOR` pointed at the real Prometheus reactor DLL
- `CGO_ENABLED=1`
- `CC=C:\msys64\ucrt64\bin\gcc.exe`
- `CXX=C:\msys64\ucrt64\bin\g++.exe`
- Fresh CLI built at `C:\Users\yuech\source\repos\oct\out\prometheus\sgemm_lab_m6a\oct_m6a.exe`

Probe regions:
- small-square
- balanced-medium
- rectangular-k-heavy

## 8x8x8 (small-square)

- chooser: `Blocked(blockSize=2)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `Blocked(blockSize=2)`
- fastest family: `KDecomposition`
- fastest coarse parameter choice: `KDecomposition(kBlock=32)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=8), Blocked(blockSize=8), KDecomposition(kBlock=32)
- chooser directionality: family choice is directionally wrong on this shape (`Blocked` vs `KDecomposition`)
- coarse-parameter directionality: chosen config differs from fastest observed config (`Blocked(blockSize=2)` vs `KDecomposition(kBlock=32)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| KDecomposition(kBlock=32) | 504.100us | 1060500, 503500, 504100 | 628.868ms |
| KDecomposition(kBlock=8) | 1.019ms | 1015300, 1021600, 1019100 | 649.211ms |
| KDecomposition(kBlock=4) | 1.028ms | 1016800, 1028300, 1308500 | 733.388ms |
| SingleCall | 1.029ms | 1029200, 1537200, 542800 | 644.352ms |
| Blocked(blockSize=16) | 1.032ms | 1288200, 1029000, 1032400 | 658.113ms |
| Blocked(blockSize=8) | 1.035ms | 1197000, 503300, 1034800 | 626.798ms |
| Blocked(blockSize=4) | 1.078ms | 1153600, 1078100, 1023100 | 855.725ms |
| Blocked(blockSize=2) | 1.108ms | 1107800, 1017000, 1538000 | 1.698s |
| KDecomposition(kBlock=16) | 1.552ms | 2044100, 1027500, 1552200 | 652.092ms |

## 12x12x12 (small-square)

- chooser: `Blocked(blockSize=2)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `Blocked(blockSize=2)`
- fastest family: `Blocked`
- fastest coarse parameter choice: `Blocked(blockSize=4)`
- ranking stability: winner varies across runs: Blocked(blockSize=4), Blocked(blockSize=4), KDecomposition(kBlock=4)
- chooser directionality: family choice is directionally correct on this shape
- coarse-parameter directionality: chosen config differs from fastest observed config (`Blocked(blockSize=2)` vs `Blocked(blockSize=4)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| Blocked(blockSize=4) | 588.200us | 1017000, 502500, 588200 | 1.169s |
| KDecomposition(kBlock=16) | 1.018ms | 1017400, 1017600, 1019300 | 634.652ms |
| KDecomposition(kBlock=4) | 1.019ms | 1019000, 1634000, 502500 | 784.181ms |
| Blocked(blockSize=16) | 1.020ms | 1020000, 1020300, 503200 | 622.223ms |
| Blocked(blockSize=8) | 1.035ms | 1035300, 1024000, 1379900 | 845.205ms |
| KDecomposition(kBlock=8) | 1.239ms | 1529300, 1053800, 1239300 | 694.689ms |
| SingleCall | 1.284ms | 1348800, 1283600, 504000 | 624.013ms |
| Blocked(blockSize=2) | 1.285ms | 1285100, 1290700, 784600 | 2.977s |
| KDecomposition(kBlock=32) | 1.317ms | 1357600, 1317200, 1017600 | 640.499ms |

## 16x16x16 (small-square)

- chooser: `Blocked(blockSize=4)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `Blocked(blockSize=4)`
- fastest family: `Blocked`
- fastest coarse parameter choice: `Blocked(blockSize=4)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=8), KDecomposition(kBlock=16), Blocked(blockSize=16)
- chooser directionality: family choice is directionally correct on this shape
- coarse-parameter directionality: chosen coarse parameter matches the fastest observed config
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| Blocked(blockSize=4) | 1.017ms | 1017000, 2022200, 746400 | 1.684s |
| KDecomposition(kBlock=8) | 1.022ms | 1014700, 1027600, 1021600 | 687.565ms |
| Blocked(blockSize=8) | 1.023ms | 1019700, 1022900, 1285800 | 854.760ms |
| KDecomposition(kBlock=16) | 1.023ms | 3173400, 1015300, 1023100 | 643.949ms |
| KDecomposition(kBlock=32) | 1.024ms | 1016300, 1024100, 1063400 | 630.847ms |
| KDecomposition(kBlock=4) | 1.027ms | 1018300, 1052700, 1026700 | 860.453ms |
| Blocked(blockSize=16) | 1.028ms | 1027700, 1549900, 503100 | 629.398ms |
| SingleCall | 1.110ms | 1169800, 1110300, 1068600 | 637.690ms |
| Blocked(blockSize=2) | 1.202ms | 2557500, 1018400, 1202100 | 4.957s |

## 20x20x20 (small-square)

- chooser: `SingleCall`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `SingleCall`
- fastest family: `KDecomposition`
- fastest coarse parameter choice: `KDecomposition(kBlock=4)`
- ranking stability: winner varies across runs: SingleCall, KDecomposition(kBlock=32), KDecomposition(kBlock=4)
- chooser directionality: family choice is directionally wrong on this shape (`SingleCall` vs `KDecomposition`)
- coarse-parameter directionality: chosen config differs from fastest observed config (`SingleCall` vs `KDecomposition(kBlock=4)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| KDecomposition(kBlock=4) | 503.200us | 1019300, 502600, 503200 | 891.509ms |
| KDecomposition(kBlock=32) | 503.400us | 1279600, 502100, 503400 | 623.819ms |
| SingleCall | 1.018ms | 1019300, 502200, 1017600 | 627.259ms |
| KDecomposition(kBlock=16) | 1.019ms | 1041700, 658800, 1018900 | 699.716ms |
| Blocked(blockSize=4) | 1.026ms | 1025600, 1096000, 504800 | 2.243s |
| Blocked(blockSize=8) | 1.029ms | 1028900, 1139400, 716500 | 1.178s |
| Blocked(blockSize=16) | 1.037ms | 2566900, 1037400, 1031300 | 840.239ms |
| Blocked(blockSize=2) | 1.163ms | 2532300, 1030900, 1163100 | 8.213s |
| KDecomposition(kBlock=8) | 1.279ms | 1278700, 502600, 1279200 | 757.545ms |

## 24x24x24 (balanced-medium)

- chooser: `SingleCall`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `SingleCall`
- fastest family: `KDecomposition`
- fastest coarse parameter choice: `KDecomposition(kBlock=16)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=16), KDecomposition(kBlock=32), Blocked(blockSize=2)
- chooser directionality: family choice is directionally wrong on this shape (`SingleCall` vs `KDecomposition`)
- coarse-parameter directionality: chosen config differs from fastest observed config (`SingleCall` vs `KDecomposition(kBlock=16)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| KDecomposition(kBlock=16) | 574.300us | 1015800, 503700, 574300 | 709.424ms |
| Blocked(blockSize=2) | 675.100us | 1028400, 675100, 504600 | 11.837s |
| Blocked(blockSize=16) | 1.016ms | 1222400, 654400, 1015700 | 814.056ms |
| KDecomposition(kBlock=4) | 1.019ms | 1018600, 1031400, 582000 | 976.974ms |
| Blocked(blockSize=4) | 1.025ms | 1017000, 1317600, 1024800 | 3.000s |
| KDecomposition(kBlock=8) | 1.034ms | 1254100, 1033500, 1020800 | 788.425ms |
| KDecomposition(kBlock=32) | 1.078ms | 1078300, 503600, 2011200 | 627.986ms |
| Blocked(blockSize=8) | 1.129ms | 1179000, 1101500, 1128700 | 1.180s |
| SingleCall | 1.299ms | 2551000, 1298800, 1035700 | 647.258ms |

## 32x32x32 (balanced-medium)

- chooser: `SingleCall`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `SingleCall`
- fastest family: `Blocked`
- fastest coarse parameter choice: `Blocked(blockSize=8)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=4), Blocked(blockSize=4), Blocked(blockSize=8)
- chooser directionality: family choice is directionally wrong on this shape (`SingleCall` vs `Blocked`)
- coarse-parameter directionality: chosen config differs from fastest observed config (`SingleCall` vs `Blocked(blockSize=8)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| Blocked(blockSize=8) | 503.000us | 1239100, 503000, 501800 | 1.651s |
| Blocked(blockSize=4) | 566.700us | 1306300, 501600, 566700 | 5.216s |
| KDecomposition(kBlock=4) | 1.016ms | 1015500, 1162700, 503500 | 1.109s |
| SingleCall | 1.016ms | 1015700, 1252000, 1016400 | 631.864ms |
| KDecomposition(kBlock=8) | 1.026ms | 1026300, 1060200, 1017100 | 852.647ms |
| KDecomposition(kBlock=16) | 1.030ms | 2190100, 1019700, 1030000 | 714.136ms |
| Blocked(blockSize=2) | 1.087ms | 1532800, 1087000, 1022700 | 22.549s |
| KDecomposition(kBlock=32) | 1.152ms | 1152200, 2047000, 1023700 | 616.387ms |
| Blocked(blockSize=16) | 1.239ms | 1247200, 503200, 1238700 | 864.643ms |

## 40x40x40 (balanced-medium)

- chooser: `SingleCall`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `SingleCall`
- fastest family: `Blocked`
- fastest coarse parameter choice: `Blocked(blockSize=8)`
- ranking stability: winner varies across runs: SingleCall, KDecomposition(kBlock=4), Blocked(blockSize=8)
- chooser directionality: family choice is directionally wrong on this shape (`SingleCall` vs `Blocked`)
- coarse-parameter directionality: chosen config differs from fastest observed config (`SingleCall` vs `Blocked(blockSize=8)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| Blocked(blockSize=8) | 1.017ms | 1019000, 1017100, 501900 | 2.263s |
| KDecomposition(kBlock=4) | 1.017ms | 1017300, 609500, 1024200 | 1.225s |
| KDecomposition(kBlock=16) | 1.020ms | 1020400, 1017500, 1194600 | 770.002ms |
| SingleCall | 1.021ms | 1013200, 1020700, 1241400 | 626.221ms |
| KDecomposition(kBlock=32) | 1.027ms | 1027200, 1140200, 504700 | 695.944ms |
| KDecomposition(kBlock=8) | 1.037ms | 1017500, 1548600, 1036600 | 901.115ms |
| Blocked(blockSize=16) | 1.038ms | 1026900, 1060300, 1037500 | 1.174s |
| Blocked(blockSize=4) | 1.203ms | 1203400, 1230100, 1015800 | 7.519s |
| Blocked(blockSize=2) | 1.239ms | 1239400, 2446700, 1024100 | 33.527s |

## 48x48x48 (balanced-medium)

- chooser: `SingleCall`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `SingleCall`
- fastest family: `Blocked`
- fastest coarse parameter choice: `Blocked(blockSize=16)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=8), Blocked(blockSize=16), Blocked(blockSize=16)
- chooser directionality: family choice is directionally wrong on this shape (`SingleCall` vs `Blocked`)
- coarse-parameter directionality: chosen config differs from fastest observed config (`SingleCall` vs `Blocked(blockSize=16)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| Blocked(blockSize=16) | 516.800us | 1029200, 504000, 516800 | 1.153s |
| KDecomposition(kBlock=8) | 1.016ms | 1015700, 799500, 1017700 | 969.664ms |
| Blocked(blockSize=4) | 1.016ms | 1016200, 1018400, 581300 | 10.454s |
| SingleCall | 1.020ms | 1018000, 1020100, 1581500 | 620.315ms |
| KDecomposition(kBlock=4) | 1.022ms | 1021800, 504000, 1646200 | 1.375s |
| KDecomposition(kBlock=32) | 1.023ms | 1054700, 1022900, 1015500 | 728.697ms |
| KDecomposition(kBlock=16) | 1.029ms | 2109900, 1029400, 1016800 | 782.337ms |
| Blocked(blockSize=8) | 1.167ms | 1167100, 1218100, 668700 | 2.917s |
| Blocked(blockSize=2) | 9,223,372,036.855s |  | 9,223,372,036.855s |

## 16x64 * 64x8 (rectangular-k-heavy)

- chooser: `KDecomposition(kBlock=4)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `KDecomposition(kBlock=4)`
- fastest family: `KDecomposition`
- fastest coarse parameter choice: `KDecomposition(kBlock=8)`
- ranking stability: winner varies across runs: Blocked(blockSize=2), KDecomposition(kBlock=32), KDecomposition(kBlock=4)
- chooser directionality: family choice is directionally correct on this shape
- coarse-parameter directionality: chosen config differs from fastest observed config (`KDecomposition(kBlock=4)` vs `KDecomposition(kBlock=8)`)
- variance note: winner spread stays reasonably tight across the three measured runs

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| KDecomposition(kBlock=8) | 1.018ms | 1016500, 1021700, 1017700 | 1.126s |
| KDecomposition(kBlock=4) | 1.018ms | 1018200, 1279400, 669300 | 1.665s |
| Blocked(blockSize=4) | 1.020ms | 1019600, 503800, 1491200 | 1.122s |
| Blocked(blockSize=8) | 1.023ms | 1046900, 1022700, 1019900 | 713.691ms |
| KDecomposition(kBlock=32) | 1.025ms | 1143100, 503200, 1025200 | 713.307ms |
| Blocked(blockSize=2) | 1.028ms | 1010800, 1028000, 1031200 | 2.764s |
| Blocked(blockSize=16) | 1.029ms | 1200100, 503500, 1029400 | 644.952ms |
| SingleCall | 1.031ms | 1031300, 1064300, 1026300 | 727.701ms |
| KDecomposition(kBlock=16) | 1.160ms | 1159600, 1548600, 1019800 | 876.779ms |

## 8x128 * 128x16 (rectangular-k-heavy)

- chooser: `KDecomposition(kBlock=8)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `KDecomposition(kBlock=8)`
- fastest family: `KDecomposition`
- fastest coarse parameter choice: `KDecomposition(kBlock=4)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=16), SingleCall, Blocked(blockSize=16)
- chooser directionality: family choice is directionally correct on this shape
- coarse-parameter directionality: chosen config differs from fastest observed config (`KDecomposition(kBlock=8)` vs `KDecomposition(kBlock=4)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| KDecomposition(kBlock=4) | 502.400us | 502300, 1022300, 502400 | 2.675s |
| Blocked(blockSize=2) | 503.300us | 503300, 503900, 501900 | 2.790s |
| SingleCall | 503.900us | 502800, 503900, 786900 | 647.897ms |
| Blocked(blockSize=8) | 762.500us | 762500, 1019400, 502000 | 698.447ms |
| KDecomposition(kBlock=16) | 798.700us | 501500, 1014200, 798700 | 1.181s |
| Blocked(blockSize=4) | 1.013ms | 1013300, 1130300, 688400 | 1.121s |
| Blocked(blockSize=16) | 1.023ms | 1023200, 1023200, 501800 | 656.560ms |
| KDecomposition(kBlock=8) | 1.027ms | 1027000, 702000, 1031200 | 1.608s |
| KDecomposition(kBlock=32) | 1.027ms | 1019700, 1027300, 1223200 | 850.091ms |

## 8x8x256 (rectangular-k-heavy)

- chooser: `KDecomposition(kBlock=16)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `KDecomposition(kBlock=16)`
- fastest family: `Blocked`
- fastest coarse parameter choice: `Blocked(blockSize=16)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=16), KDecomposition(kBlock=16), Blocked(blockSize=16)
- chooser directionality: family choice is directionally wrong on this shape (`KDecomposition` vs `Blocked`)
- coarse-parameter directionality: chosen config differs from fastest observed config (`KDecomposition(kBlock=16)` vs `Blocked(blockSize=16)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| Blocked(blockSize=16) | 503.600us | 1034300, 503600, 502300 | 623.202ms |
| KDecomposition(kBlock=32) | 1.016ms | 1032400, 504900, 1016100 | 1.109s |
| KDecomposition(kBlock=16) | 1.017ms | 1017300, 502800, 1154400 | 1.673s |
| KDecomposition(kBlock=4) | 1.018ms | 1017900, 629500, 1020700 | 5.201s |
| Blocked(blockSize=8) | 1.020ms | 1026300, 504600, 1019800 | 642.292ms |
| KDecomposition(kBlock=8) | 1.033ms | 1032900, 503000, 1246100 | 2.626s |
| SingleCall | 1.112ms | 1111700, 1249200, 1030600 | 648.434ms |
| Blocked(blockSize=4) | 1.155ms | 1154900, 505300, 1241700 | 895.781ms |
| Blocked(blockSize=2) | 1.171ms | 3089600, 1019700, 1170700 | 1.649s |

## 12x12x192 (rectangular-k-heavy)

- chooser: `KDecomposition(kBlock=8)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `KDecomposition(kBlock=8)`
- fastest family: `KDecomposition`
- fastest coarse parameter choice: `KDecomposition(kBlock=16)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=4), KDecomposition(kBlock=16), SingleCall
- chooser directionality: family choice is directionally correct on this shape
- coarse-parameter directionality: chosen config differs from fastest observed config (`KDecomposition(kBlock=8)` vs `KDecomposition(kBlock=16)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| KDecomposition(kBlock=16) | 502.100us | 1194200, 502100, 502100 | 1.481s |
| Blocked(blockSize=4) | 1.008ms | 1019500, 1008400, 502000 | 1.274s |
| Blocked(blockSize=8) | 1.017ms | 1568900, 1016800, 503800 | 858.253ms |
| KDecomposition(kBlock=4) | 1.019ms | 1015700, 1071600, 1018800 | 3.943s |
| KDecomposition(kBlock=32) | 1.020ms | 1105200, 1018900, 1019700 | 1.006s |
| KDecomposition(kBlock=8) | 1.022ms | 1136300, 1022300, 502200 | 2.250s |
| SingleCall | 1.023ms | 1293500, 1022700, 501500 | 658.374ms |
| Blocked(blockSize=16) | 1.024ms | 1052500, 1019300, 1024000 | 630.886ms |
| Blocked(blockSize=2) | 1.077ms | 1077000, 1504600, 1071600 | 2.937s |

## 16x16x384 (rectangular-k-heavy)

- chooser: `KDecomposition(kBlock=16)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `KDecomposition(kBlock=16)`
- fastest family: `KDecomposition`
- fastest coarse parameter choice: `KDecomposition(kBlock=8)`
- ranking stability: winner varies across runs: Blocked(blockSize=2), KDecomposition(kBlock=8), KDecomposition(kBlock=8)
- chooser directionality: family choice is directionally correct on this shape
- coarse-parameter directionality: chosen config differs from fastest observed config (`KDecomposition(kBlock=16)` vs `KDecomposition(kBlock=8)`)
- variance note: winner spread is large enough that sub-family ordering should be treated cautiously

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| KDecomposition(kBlock=8) | 502.100us | 1023500, 502100, 502100 | 3.927s |
| KDecomposition(kBlock=16) | 503.700us | 1259500, 503700, 502400 | 2.169s |
| Blocked(blockSize=4) | 1.019ms | 1024600, 1019400, 746700 | 1.665s |
| Blocked(blockSize=2) | 1.020ms | 1020400, 1328500, 503000 | 5.253s |
| Blocked(blockSize=16) | 1.024ms | 1023600, 1024200, 723100 | 613.031ms |
| KDecomposition(kBlock=32) | 1.026ms | 1029200, 1025500, 664600 | 1.442s |
| SingleCall | 1.058ms | 1277100, 1057600, 1019500 | 668.656ms |
| KDecomposition(kBlock=4) | 1.130ms | 1129500, 503900, 1193000 | 8.052s |
| Blocked(blockSize=8) | 1.260ms | 1259900, 1047400, 2033300 | 844.275ms |

## 16x16x512 (rectangular-k-heavy)

- chooser: `KDecomposition(kBlock=32)`
- stabilized chooser (`hysteresis=12`, `min_commit=2`): `KDecomposition(kBlock=32)`
- fastest family: `KDecomposition`
- fastest coarse parameter choice: `KDecomposition(kBlock=4)`
- ranking stability: winner varies across runs: KDecomposition(kBlock=4), KDecomposition(kBlock=8), KDecomposition(kBlock=4)
- chooser directionality: family choice is directionally correct on this shape
- coarse-parameter directionality: chosen config differs from fastest observed config (`KDecomposition(kBlock=32)` vs `KDecomposition(kBlock=4)`)
- variance note: winner spread stays reasonably tight across the three measured runs

| Config | Median wall | Runs (ns) | Median outer |
| --- | --- | --- | --- |
| KDecomposition(kBlock=4) | 502.100us | 502100, 503000, 501400 | 10.685s |
| KDecomposition(kBlock=8) | 503.300us | 503300, 502300, 765800 | 5.526s |
| KDecomposition(kBlock=32) | 1.003ms | 1012900, 1003300, 501500 | 1.605s |
| Blocked(blockSize=4) | 1.017ms | 503500, 1016700, 1017000 | 1.648s |
| KDecomposition(kBlock=16) | 1.020ms | 505000, 1029300, 1019500 | 2.678s |
| Blocked(blockSize=2) | 1.026ms | 1025800, 770900, 1501500 | 5.212s |
| Blocked(blockSize=16) | 1.030ms | 1039000, 1029900, 502600 | 648.592ms |
| SingleCall | 1.033ms | 1032800, 1239700, 669600 | 675.540ms |
| Blocked(blockSize=8) | 1.069ms | 1069400, 1027700, 1135300 | 844.184ms |

## Overall Conclusions

- `Blocked` survives this Windows-native probe. It wins on: 12x12x12, 16x16x16, 32x32x32, 40x40x40, 48x48x48, 8x8x256
- `SingleCall` wins on: 
- `KDecomposition` wins on: 8x8x8, 20x20x20, 24x24x24, 16x64 * 64x8, 8x128 * 128x16, 12x12x192, 16x16x384, 16x16x512
- meaningful blocked `blockSize` regimes in this run: 2, 4, 8, 16
- meaningful K-decomposition `kBlock` regimes in this run: 4, 8, 16, 32
- optional `hysteresis` / `min_commit` do not change any measured one-shot M6 decision in this probe surface, so they are not justified yet for this chooser.
- M6 remains hosted inside the M4 experiment directory in this checkout; the runner and summary treat that as an explicit repository seam rather than silently inventing a new experiment root.
