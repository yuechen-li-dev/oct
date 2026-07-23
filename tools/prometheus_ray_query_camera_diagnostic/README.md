# Prometheus ray-query camera diagnostic

This is a bounded foreign-style RQ-M1 client. It includes only the public
`prometheus_ray_query.h` semantic contract; it contains no Vulkan, SPIR-V, or
private Prometheus headers.

It receives a verified shader-package root and an existing output directory,
then writes six deterministic binary PPM diagnostics for one fixed mixed
triangle/analytic-sphere scene: hit, distance, identity, geometry kind, normal,
and triangle barycentrics. PPM is used here to keep the client dependency-free;
PNG encoding is deliberately left outside the runtime boundary.
