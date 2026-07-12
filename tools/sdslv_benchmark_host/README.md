# SDSL-V Godot benchmark host

This is a one-shot Godot 4.7 C# execution backend for `.sdslvbench`. It owns
RenderingDevice setup, dispatch, synchronized timing, and RID cleanup only;
the Go compiler owns benchmark semantics, IDs, manifests, replay, and
statistics.

The installed Godot 4.7 bindings document that `RenderingDevice` is unavailable
under `--headless`, so the CLI deliberately launches a normal Forward+ process
without that flag. The project creates a noninteractive 1x1 window and quits
after one request; it has no UI, scene behavior, or persistent service.

`GODOT4` selects the Godot executable. If it is unset, the runner searches
`godot4` then `godot` on `PATH`. The Go parent uses non-shell process launch,
captures stdout/stderr, applies a 60-second timeout, and kills the complete
process tree on Windows.

The protocol is deterministic JSON files, schema version 1:

- request: selected benchmark metadata plus SPIR-V path/hash;
- response: device information, raw `samplesNs`, timing source, and errors.

Timing is currently `synchronized_host_elapsed`: each sample includes compute
dispatch, queue submission, and GPU synchronization. It is not labeled GPU
timestamp timing. The host uses `RenderingServer.CreateLocalRenderingDevice`,
`ShaderCreateFromSpirV`, a compute pipeline/list/dispatch, `Submit`, and
`Sync`, then frees all created RIDs and the device.

M36a executes isolated modules, one Godot process per benchmark. Proven
capabilities are resource-free and ordinary scalar/vector storage-buffer
modules. Current ndarray-generated and tensor-generated SPIR-V is valid SDSL-V
and passes SPIR-V validation, but Godot 4.7 Mono terminates during pipeline
creation for those forms; they are deferred backend compatibility work, not a
language restriction. The default DXC target remains Vulkan 1.0.
