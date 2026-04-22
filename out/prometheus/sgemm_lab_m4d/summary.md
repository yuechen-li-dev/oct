# M4d Windows-Native Prometheus Summary

Environment:
- Windows
- Native NVIDIA Vulkan environment required
- `OCT_PROMETHEUS_REACTOR=internal\prometheus\reactor\prometheus_reactor.dll`
- `CGO_ENABLED=1`
- `CC=C:\Users\yuech\mingw64\bin\gcc.exe`
- `CXX=C:\Users\yuech\mingw64\bin\g++.exe`
- Fresh CLI built from current source: `C:\Users\yuech\source\repos\oct\out\\prometheus\\sgemm_lab_m4d\oct_m4d.exe`

Retained bridged strategy families tested:
- `SingleCallRep` benchmark for single-call delegation (`Baseline`/`IKJ`/`KIJ`)
- `BlockedRep` benchmark for output-block decomposition (`Blocked`/`BlockedIKJ`)
- `KDecompositionRep` benchmark for K-decomposition (`KBlocked`/`KBlockedIKJ`/`StagedIKJ`)

Note:
- `StagedIKJ` is not benchmarked as a separate family because M4c made it an execution alias of `KBlocked`; staged viability in this summary is therefore judged through the shared K-decomposition family rather than a fake distinct row.

## Shape 16x16x16

ChosenStrategy: `KBlocked`
ChosenFamily: `KDecomposition` (represented by `KBlocked` in the retained M4d surface)
FastestObserved: `Blocked` via `BlockedRep` representative at median reported wall `1.182ms`
RankingStability: winner varies across runs: Blocked > KDecomposition > SingleCall; Blocked > SingleCall > KDecomposition; SingleCall > Blocked > KDecomposition

| Family | Representative | BackendUsed | Status | Environment | ReportedWallNs | Median wall | Median outer |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Blocked | BlockedRep | prometheus | ok | windows_native_vulkan | `1015900, 1182200, 1235800` | 1.182ms | 904.686ms |
| SingleCall | SingleCallRep | prometheus | ok | windows_native_vulkan | `1200100, 1328000, 1191900` | 1.200ms | 695.919ms |
| KDecomposition | KDecompositionRep | prometheus | ok | windows_native_vulkan | `1153500, 3424900, 1801400` | 1.801ms | 709.476ms |

- Current chooser is directionally wrong on this shape: it selects KDecomposition but the fastest retained family is Blocked.
- Variance note: the fastest family stays reasonably tight across the three measured runs.

## Shape 32x32x32

ChosenStrategy: `KBlockedIKJ`
ChosenFamily: `KDecomposition` (represented by `KBlocked` in the retained M4d surface)
FastestObserved: `SingleCall` via `SingleCallRep` representative at median reported wall `1.018ms`
RankingStability: winner varies across runs: Blocked > KDecomposition > SingleCall; SingleCall > KDecomposition > Blocked; SingleCall > KDecomposition > Blocked

| Family | Representative | BackendUsed | Status | Environment | ReportedWallNs | Median wall | Median outer |
| --- | --- | --- | --- | --- | --- | --- | --- |
| SingleCall | SingleCallRep | prometheus | ok | windows_native_vulkan | `1611400, 1017900, 1016400` | 1.018ms | 702.062ms |
| KDecomposition | KDecompositionRep | prometheus | ok | windows_native_vulkan | `1151000, 1030500, 1145200` | 1.145ms | 767.702ms |
| Blocked | BlockedRep | prometheus | ok | windows_native_vulkan | `1144100, 1159200, 2547700` | 1.159ms | 1.734s |

- Current chooser is directionally wrong on this shape: it selects KDecomposition but the fastest retained family is SingleCall.
- Variance note: the fastest family shows visible wall-time spread, so the ordering should be treated as suggestive rather than absolute.

## Shape 16x64 * 64x8

ChosenStrategy: `KBlockedIKJ`
ChosenFamily: `KDecomposition` (represented by `KBlocked` in the retained M4d surface)
FastestObserved: `KDecomposition` via `KDecompositionRep` representative at median reported wall `1.021ms`
RankingStability: winner varies across runs: KDecomposition > Blocked > SingleCall; Blocked > SingleCall > KDecomposition; KDecomposition > SingleCall > Blocked

| Family | Representative | BackendUsed | Status | Environment | ReportedWallNs | Median wall | Median outer |
| --- | --- | --- | --- | --- | --- | --- | --- |
| KDecomposition | KDecompositionRep | prometheus | ok | windows_native_vulkan | `1020600, 1936500, 1017400` | 1.021ms | 904.066ms |
| Blocked | BlockedRep | prometheus | ok | windows_native_vulkan | `1103600, 1026600, 2060200` | 1.104ms | 779.491ms |
| SingleCall | SingleCallRep | prometheus | ok | windows_native_vulkan | `1892400, 1820200, 1023500` | 1.820ms | 692.497ms |

- Current chooser is directionally right on this shape once M4c alias collapse is respected.
- Variance note: the fastest family shows visible wall-time spread, so the ordering should be treated as suggestive rather than absolute.

## Shape 8x128 * 128x16

ChosenStrategy: `KBlockedIKJ`
ChosenFamily: `KDecomposition` (represented by `KBlocked` in the retained M4d surface)
FastestObserved: `KDecomposition` via `KDecompositionRep` representative at median reported wall `1.024ms`
RankingStability: winner varies across runs: KDecomposition > SingleCall > Blocked; Blocked > KDecomposition > SingleCall; Blocked > SingleCall > KDecomposition

| Family | Representative | BackendUsed | Status | Environment | ReportedWallNs | Median wall | Median outer |
| --- | --- | --- | --- | --- | --- | --- | --- |
| KDecomposition | KDecompositionRep | prometheus | ok | windows_native_vulkan | `1017200, 1023700, 1392700` | 1.024ms | 1.176s |
| Blocked | BlockedRep | prometheus | ok | windows_native_vulkan | `1093500, 1013300, 1222400` | 1.094ms | 764.368ms |
| SingleCall | SingleCallRep | prometheus | ok | windows_native_vulkan | `1031500, 1290300, 1228500` | 1.229ms | 700.788ms |

- Current chooser is directionally right on this shape once M4c alias collapse is respected.
- Variance note: the fastest family stays reasonably tight across the three measured runs.

## Shape 8x8 * 256

ChosenStrategy: `KBlockedIKJ`
ChosenFamily: `KDecomposition` (represented by `KBlocked` in the retained M4d surface)
FastestObserved: `KDecomposition` via `KDecompositionRep` representative at median reported wall `1.156ms`
RankingStability: winner varies across runs: KDecomposition > Blocked > SingleCall; Blocked > KDecomposition > SingleCall; SingleCall > KDecomposition > Blocked

| Family | Representative | BackendUsed | Status | Environment | ReportedWallNs | Median wall | Median outer |
| --- | --- | --- | --- | --- | --- | --- | --- |
| KDecomposition | KDecompositionRep | prometheus | ok | windows_native_vulkan | `1017000, 1175900, 1155600` | 1.156ms | 1.758s |
| Blocked | BlockedRep | prometheus | ok | windows_native_vulkan | `1204400, 1018400, 1559500` | 1.204ms | 693.569ms |
| SingleCall | SingleCallRep | prometheus | ok | windows_native_vulkan | `1675700, 1926100, 1064800` | 1.676ms | 700.771ms |

- Current chooser is directionally right on this shape once M4c alias collapse is respected.
- Variance note: the fastest family stays reasonably tight across the three measured runs.

## Shape 16x16 * 512

ChosenStrategy: `KBlockedIKJ`
ChosenFamily: `KDecomposition` (represented by `KBlocked` in the retained M4d surface)
FastestObserved: `KDecomposition` via `KDecompositionRep` representative at median reported wall `1.019ms`
RankingStability: winner varies across runs: SingleCall > KDecomposition > Blocked; KDecomposition > Blocked > SingleCall; KDecomposition > SingleCall > Blocked

| Family | Representative | BackendUsed | Status | Environment | ReportedWallNs | Median wall | Median outer |
| --- | --- | --- | --- | --- | --- | --- | --- |
| KDecomposition | KDecompositionRep | prometheus | ok | windows_native_vulkan | `1018900, 1054300, 1016900` | 1.019ms | 2.852s |
| Blocked | BlockedRep | prometheus | ok | windows_native_vulkan | `1067300, 1155700, 1544400` | 1.156ms | 904.023ms |
| SingleCall | SingleCallRep | prometheus | ok | windows_native_vulkan | `1016200, 1218100, 1155900` | 1.156ms | 705.745ms |

- Current chooser is directionally right on this shape once M4c alias collapse is respected.
- Variance note: the fastest family stays reasonably tight across the three measured runs.

## Overall

- Chooser mismatches: 2/6 required shapes
- K-decomposition wins on: 16x64 * 64x8, 8x128 * 128x16, 8x8 * 256, 16x16 * 512
- StagedIKJ remains non-distinct after M4c; M5 should not score it separately from the shared K-decomposition family unless the backend path becomes structurally different again.
