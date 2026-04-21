# P4G REPORT — Prometheus Reactor Safety/Correctness Hardening

## Scope

P4g is a focused hardening pass on audited issues only:

- SGEMM size arithmetic overflow safety.
- Cleanup-stage status clarity.
- Memory-type failure reporting clarity.
- Handle-registry thread-safety.

## Fixes

1. **Overflow hardening**
   - Added checked helpers for `uint32_t` multiplication and float-buffer byte-size derivation.
   - SGEMM now computes checked sizes for A/B/C before any Vulkan buffer creation or memcpy.
   - Overflow is reported deterministically as `PROM_STAGE_TRANSFER_IN` with `PROM_DETAIL_SIZE_OVERFLOW`.

2. **Cleanup/status behavior**
   - Removed ambiguous cleanup self-assignment/no-op status write.
   - Cleanup now preserves the existing stage/detail unless a preceding step explicitly changed them.

3. **Memory-type reporting clarity**
   - `find_memory_type` miss now reports `VK_ERROR_FEATURE_NOT_PRESENT` instead of `VK_ERROR_MEMORY_MAP_FAILED`.
   - Added test flag `PROM_TESTCFG_FORCE_NO_MEMORY_TYPE` to validate this path deterministically.

4. **Registry thread-safety**
   - Added a process-global mutex around handle registry `contains/add/remove`.
   - Added concurrent lifecycle test coverage (multi-threaded create/destroy loops).

## Tests proving the changes

- Overflow rejection test asserts explicit stage/detail overflow failure.
- Cleanup semantics test asserts dispatch failure remains `PROM_STAGE_SUBMIT` with original detail after cleanup.
- Memory-type mapping test injects no-memory-type path and asserts `VK_ERROR_FEATURE_NOT_PRESENT`.
- Concurrent lifecycle test exercises registry operations across threads.
- Existing Go-side Prometheus tests continue to pass.
