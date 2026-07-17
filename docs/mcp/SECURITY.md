# Oct MCP security boundary

## Threat model and controls

| Threat | Application mitigation | Deployment requirement / residual risk |
| --- | --- | --- |
| Host path traversal or symlink input | Source files must be slash-relative and cannot contain `..`, absolute paths, or backslashes; a fresh owned workspace is used and deleted. | Mount only runtime assets read-only; reject symlink creation at the container boundary. |
| Infinite loop/cancellation | Each execution is a child process with context timeout and kill on cancellation. | Enforce CPU and PID cgroups; child process trees are platform-dependent. |
| Oversized input/output | Fixed source/file/output/artifact limits are applied before or while buffering. | Enforce HTTP body, memory and disk quotas. |
| Artifact exfiltration | Only allowlisted output formats are registered; opaque IDs are scoped to execution and TTL. | Keep the artifact store process-local or use authenticated, expiry-enforcing storage. |
| Protocol injection/log corruption | MCP stdout is owned by the SDK; server logs go to stderr. | Avoid request-body logging and redact authorization headers. |
| Network/package/native escape | Public tool surface has no shell, package sync, GPU, native-library or host-path parameter. Child environment is minimized. | **Required:** deny egress, mount a read-only runtime image, drop privileges/capabilities, no Docker socket, no host namespaces. Oct build can start Go tooling, so this is not safe without OS/container isolation. |
| DoS/concurrency | Semaphore applies a concurrency limit and request timeout. | Ingress rate limiting, request body limit and autoscaling/queue policy. |
| Stale/guessed IDs | Random IDs, execution scoping, hash metadata and expiry. | Use authenticated storage or a signed session binding if artifacts become distributed. |

## Explicit non-claims

The process-local guards are not a complete sandbox, do not enforce memory limits, and do not make arbitrary Oct code safe on a workstation. Public deployment requires a Linux container/VM sandbox with no unrestricted network, no writable host mount, non-root UID, CPU/memory/PID limits, ephemeral workspace and an ingress rate limit. Prometheus GPU, package installation, arbitrary native libraries, Python fallback and arbitrary process execution are unsupported.

## Defaults and hard maximums

The MVP defaults are also the binary hard maximums: 256 KiB source, 32 files, 15 seconds, 64 KiB each of stdout/stderr, 8 artifacts, 2 MiB each artifact, two concurrent executions, and ten-minute artifact retention. Deployment configuration may reduce them, not increase them, until a reviewed configuration layer is added.
