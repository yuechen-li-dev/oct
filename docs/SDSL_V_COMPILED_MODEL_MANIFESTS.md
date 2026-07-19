# SDSL-V compiled model manifests

Compiled GPU models have two authorities. `manifest.oct` is human-authored,
compiler-parsed source declaring model intent. `lock-tagon.octagon` is the
generated immutable, data-only resolved projection consumed by a runtime
descriptor adapter. Prometheus executes a resolved descriptor; it does not
define model topology in mutable C tables.

The initial PoC is [Z-Image Turbo](../internal/prometheus/models/zimage-turbo/manifest.oct).
It declares one `NoiseRefiner` assembly family, the two closed parameter
instances `NoiseRefiner0` and `NoiseRefiner1`, and their ordered resident FP32
flow. A parameter set is not an assembly name: it is an immutable cache and
oracle binding compatible with that family.

Authored facts are the model revision/checkpoint, requested runtime and
precision, assembly membership, named parameter instances, and execution
order. The linker resolves the manifest identity, model-local IDs, shader,
ABI, memory, execution and replay contracts, cache aggregates, and canonical
authorities into [lock-tagon.octagon](../internal/prometheus/models/zimage-turbo/lock-tagon.octagon).
The lock intentionally excludes local paths, timestamps, Vulkan handles,
pointers, comments, and payload bytes.

Generate or verify the lock without GPU execution:

```powershell
go run ./tools/compiled_model_lock
go run ./tools/compiled_model_lock -check
```

`-check` rejects a missing or stale lock. The current runtime seam compares
the resolved family, parameter-set aggregate, ABI and generation contract with
the closed native NoiseRefiner rebind API. Full native registry consumption is
deferred to EVT-2 M2A closeout; context refiners, arbitrary graphs, package
download, hot reload, weight embedding, and model arithmetic remain excluded.
