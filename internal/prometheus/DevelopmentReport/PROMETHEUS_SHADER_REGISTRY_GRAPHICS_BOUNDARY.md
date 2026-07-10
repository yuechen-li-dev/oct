# Prometheus shader registry graphics boundary

One shared shader asset registry. No undifferentiated implementation record.
Compute and graphics remain separately typed.

SPIR-V bytes, stage, entry point, provenance, and capability facts are shared
because both compute and graphics consume SPIR-V. A compute implementation adds
binding and dispatch facts; it cannot express graphics pipeline state. A future
`prom_graphics_implementation` will instead reference compatible graphics-stage
shader IDs plus vertex layout, render targets, raster, depth/stencil, and blend
state. It will have its own implementation-ID namespace and mutable graphics
pipeline instances.

R3 adds no graphics pipelines, rendering state, draw submission, or fake
graphics table. This keeps the boundary concrete without manufacturing behavior
that has not been designed or validated.
