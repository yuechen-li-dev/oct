# P8d Report — Vulkan Reactor Tiled SGEMM (Shared/Local Memory) Port

## Summary

P8d ports the M9 tiled-SGEMM safety protocol into the Vulkan reactor by adding a new tiled compute pipeline that uses workgroup shared memory, explicit bounds guards, and explicit tile barriers.

This is a correctness-first port. Performance tuning (tile auto-search, subgroup tricks, async overlap) remains intentionally deferred.

## M9 hazard map → Vulkan implementation

### 1) Non-multiple dimensions / partial tiles

**M9 hazard:** tails in `M`, `N`, and/or `K` must not assume full tiles.

**P8d mapping:** tiled shader computes `tileCount = ceil(k / TILE_K)` and guards A/B shared loads and C writeback:

- A tile load guarded by `row < m && aCol < k`
- B tile load guarded by `bRow < k && col < n`
- C store guarded by `row < m && col < n`
- out-of-range cooperative loads explicitly write `0.0`

This preserves valid accumulation for partial edge tiles.

### 2) Bounds guards must be explicit

**M9 hazard:** no implicit out-of-range tolerance.

**P8d mapping:** all A/B/C bounds checks are explicit in shader control flow; no unchecked global buffer indexing is used for edge elements.

### 3) Barrier placement discipline

**M9 hazard:** missing post-load or between-tile barriers is unsafe.

**P8d mapping:** each tile iteration executes:

1. cooperative A/B load into shared memory
2. `barrier()`
3. inner `kk` MAC loop over shared tiles
4. `barrier()` before next tile overwrite

This follows the minimal safe two-barrier tile protocol from M9.

### 4) Tile size/local-size agreement

**M9 hazard:** local coverage deficit/duplication causes cooperative load holes or overlap.

**P8d mapping:** tile and local shapes are fixed to `8x8` and matched exactly:

- workgroup local size: `8x8`
- shared A tile: `[8][8]`
- shared B tile: `[8][8]`

No implicit “extra thread” or “missing thread” load ownership exists.

### 5) Row-major indexing consistency

**M9 hazard:** mismatched conventions across A load / B load / accumulate / C store.

**P8d mapping:** row-major indexing is consistent across the full path:

- `A[row * k + aCol]`
- `B[bRow * n + col]`
- accumulation over `tileA[localRow][kk] * tileB[kk][localCol]`
- `C[row * n + col]`

### 6) Vendor-sensitive assumptions forbidden

**M9 hazard:** implicit sync, lockstep, implicit zero-fill.

**P8d mapping:** synchronization is explicit (`barrier()`), out-of-range values are explicitly zeroed, and no subgroup lockstep assumptions are used.

## Path integration

P8d keeps existing paths and adds tiled compute observability:

- direct path (existing)
- staged upload / upload+readback paths (existing)
- tiled compute path (new, direct-buffer mode only)

Policy is conservative:

- tiny/small shapes stay non-tiled
- larger direct-mode shapes select tiled compute
- forced staged modes remain staged

Detail code diagnostics now expose tiled selection explicitly (`PROM_DETAIL_PATH_TILED`).

## Tile size choice

`8x8x8` was chosen because it matches the already-established local dispatch geometry and keeps the cooperative-load mapping straightforward and reviewable.

No auto-tuning is introduced in P8d.

## Correctness tests added

Marionette coverage now includes:

1. forced tiled exact-multiple shape (`32x32x32`)
2. forced tiled non-multiple shape (`35x29x19`)
3. forced tiled rectangular shape (`64x8x13`)
4. automatic policy check that small shapes do not auto-select tiled

All are verified against the CPU SGEMM oracle.

## Intentionally deferred optimizations

- tile-size tuning / auto-selection
- staged+tiled combined compute mode
- subgroup/warp-level specialization
- async overlap / queue-level scheduling
- reduced barrier experimentation

These are deferred to keep P8d protocol-preserving and correctness-first.

## Inconsistency surfaced

M9 models protocol hazards at an abstract structural level, while P8d encodes concrete Vulkan shader/runtime mechanics. This is an intentional layer translation; P8d follows M9’s protocol invariants directly rather than reusing M9’s abstraction surface verbatim.
