# Build Week eligibility and commit ledger

- Event start: **2026-07-13 09:00:00 Pacific (-07:00)**.
- Last pre-event commit: `8c029d6d8f8d5f698276edfda138fa96f5fb305e` (2026-07-12 12:39:01 Pacific).
- Audited submission head: `3e41eb67e40445754dfc9e057cc95e4efb471c07` (2026-07-21 11:41:53 Pacific), closing DVT2-M6A.
- Audit time: **2026-07-21 12:10:04 Pacific (-07:00)**.
- Counting method: `git rev-list 8c029d6..HEAD`; **83 reachable commits**. First-parent count: **79**.

The older July 17 head and 23-commit total are superseded. `BUILD_WEEK_COMMITS.json` remains a historical per-commit map through its own head; this document is the current ledger authority.

## Pre-existing foundation

Oct’s compiler/runtime and language contracts, SDSL-V compute compiler foundation, Prometheus Vulkan/SGEMM foundation, and the owner’s review workflow predate the event. They are context, not Build Week inventions.

## Eligible additions at this head

| Addition | Evidence |
| --- | --- |
| Complete transformer and canonical workflow | [EVT2 shipping JSON](../../internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping/zimage_python_smoke.json) |
| Persistent owner, bounded streaming, Prefetch | [M1](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M1_SHARED_OWNER_RETARGETING.md), [M2](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M2_DOUBLE_BUFFERED_PREFETCH.md) |
| Profiling, tiled SGEMM, production attention | [M3](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M3_CRITICAL_PATH_ACCOUNTING.md), [M4](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M4_OBVIOUS_SHADER_OPTIMIZATION.md), [M5B](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M5B_BUILTIN_TOPOLOGY.md) |
| Vulkan 1.4, SDSL-V graphics, OctMake, Oct laboratory | [Mx5](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_MX5_VULKAN14_MIGRATION.md), [graphics](../../internal/prometheus/DevelopmentReport/SDSL_V_M41_CANONICAL_FULL_LANGUAGE_IMPLEMENTATION.md), [Oct lab](../../internal/prometheus/DevelopmentReport/PROMETHEUS_EVT2_OCT_ORACLE_EXPERIMENTS.md) |
| M6A cooperative-matrix feasibility | [M6A](../../internal/prometheus/DevelopmentReport/PROMETHEUS_DVT2_M6A_COOPERATIVE_MATRIX_FEASIBILITY.md) |

M6B and the discarded M2D execution are not part of this submission story.
