# Prometheus R3 — shader asset and compute implementation registry

## Summary

R3 adds an explicit static shader registry for the current SGEMM SPIR-V assets
and a separately typed compute implementation table. The registry has no Vulkan
handles. Vulkan pipeline instances are mutable runtime data and are initialized
from the already-created pipelines after descriptor validation.

The current public implementation IDs 1–11 are preserved. The registry replaces
the repeated SGEMM dispatch-metadata and wired-eligibility cascades and now owns
the tiled runtime pipeline lookup. The judgment engine still selects candidates;
the registry only reports static dispatchability and metadata. Numerical
eligibility, including the conservative FP16 bound, remains request-specific.

## Manifest and generation

`native/shaders/manifest.json` is the reviewed declarative inventory of all
current shader assets and compute implementations. The SDSL-V generator reads
that manifest, iterates SDSL-V assets deterministically by stable ID, and keeps
the existing generated header symbols. A future Make.oct implementation can
read the same manifest to validate IDs, invoke SDSL-V generation, check drift,
build/package native assets, and run focused registry tests; R3 does not migrate
the build to Make.oct.

## Acceptance validation

`prom_shader_registry_validate` rejects malformed immutable data before runtime
pipeline instances are populated. The permanent Marionette registry suite now
covers all required registry contracts and injected duplicate ID, missing
reference, wrong-stage, invalid-size, nondispatchable-selector, missing-metadata,
and stale-output cases.

Windows native compilation passed. The full Marionette suite completed with 308
passes, 31 explicit hardware skips, and zero failures. Both required Go lanes,
native-manifest validation, SDSL-V manifest regeneration/drift validation, and
Linux shell syntax checks passed.

`nvidia-smi` identifies an NVIDIA GeForce RTX 3070 (driver 596.36), but the
native Vulkan runtime was unavailable to Marionette. Consequently the required
hardware authorities—per-implementation SGEMM correctness and benchmarks,
selector trace, FP16/packed4, resident, public async/batch, M29/M30/M30a/M31,
and EVT—did not execute. No performance parity comparison is available.

## Conclusion

R3 BLOCKED
