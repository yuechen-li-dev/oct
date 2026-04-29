# P8f Report — Judgment Engine Seam for Reactor Path Selection

## Why this seam was introduced

SGEMM mode policy had grown beyond a trivial branch inside `reactor_vulkan.c` (direct/staged/readback plus baseline/tiled compute). P8f introduces a small standalone **judgment engine** so reactor policy does not keep accreting in Vulkan execution code.

## What was extracted from `reactor_vulkan.c`

The following inline policy was extracted into `reactor_judgment_engine.c`:

- requested path policy (force-direct, force-staged, auto staged-for-large)
- capability gating for direct vs staged paths
- staged→direct fallback behavior under capability constraints
- compute mode choice (baseline vs tiled via force-tiled or tiled-shape)
- final observable path detail mapping (including tiled variants and fallback-to-direct)

The reactor now gathers facts, calls the judgment engine, and executes the selected mode.

## Inputs and outputs

### Inputs (`prom_judgment_facts`)

- shape/workload facts (`m`, `n`, `k`, `work_units`)
- capability facts (`can_stage`, `can_direct`)
- policy flags (`allow_fallback`, `readback_required`, `force_direct`, `force_staged`, `force_tiled`)
- eligibility facts (`tiled_shape`)
- environment hint (`software_vulkan`)

### Outputs (`prom_judgment_decision`)

- success/error (`success`, `error_detail`)
- requested vs selected path (`requested_path`, `selected_path`)
- selected compute mode (`compute_mode`)
- final reactor-observable detail code (`final_detail`)
- observability/debug fields (`used_fallback_to_direct`, `winning_candidate_index`, `winning_score`)

## Deterministic utility selection design

The judgment engine uses explicit candidate enumeration over the currently-supported SGEMM mode space:

1. direct + baseline
2. direct + tiled
3. staged upload + baseline
4. staged upload + tiled
5. staged upload + readback + baseline
6. staged upload + readback + tiled

Each candidate is filtered for feasibility and scored deterministically. Highest score wins; ties resolve by fixed enumeration order, so repeated identical facts produce identical decisions.

## Observability and testing

Observability is preserved and expanded:

- existing detail codes still flow to reactor status (`PROM_DETAIL_PATH_*`)
- additional diagnostics (`winning_candidate_index`, `winning_score`) are now available for focused policy tests

Focused Marionette tests were added for:

- determinism (same facts, same winner)
- candidate discrimination across direct/staged/readback/tiled cases
- parity with key reactor policy scenarios (capability mismatch, forced-path degradation, fallback behavior)
- observability consistency via explicit detail assertions

## Reuse potential beyond SGEMM

The seam is reusable for future reactor domains because it cleanly separates:

- domain fact gathering (reactor)
- deterministic utility selection (judgment engine)
- execution mechanics (Vulkan command path)

Future FFT policy can reuse the same shape/facts → candidate selection pattern without embedding decision logic into Vulkan execution files.

## Intentionally out of scope

P8f intentionally does **not** introduce:

- HFSM/runtime orchestration
- DragonGod-scale architecture
- persistence/mailboxes/stateful policy loops
- broad SGEMM policy redesign

This remains a small deterministic seam, scoped to policy extraction and reuse readiness.

## Behavior preservation and cleanup note

Behavior was preserved for existing reactor-visible policy outcomes and detail codes. One small cleanup was made intentionally:

- policy thresholds now use shared judgment constants (`PROM_JUDGMENT_STAGING_WORK_THRESHOLD`, `PROM_JUDGMENT_TILED_WORK_THRESHOLD`) from the judgment-engine header to avoid drift between call-site and selector.
