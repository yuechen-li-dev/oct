# Prometheus native batch architecture

The current production batch route is deliberately small:

```text
public batch API -> supported-mask validation -> M31 batch engine
  -> M30 task ownership -> M29 shared physical ring -> Vulkan SGEMM
```

M31 owns immutable caller-order plans, logical-lane metadata, per-entry
runtime facts, deterministic failure selection, admission stop, staging, and
ordered commit. M29 owns physical slot generation and submission evidence;
M30/M30a own task lifetime, feedback, quarantine, and reap.

Logical width and contiguous partition are planning hints only. They do not
represent CPU workers, Vulkan queues, command resources, or physical slots.
There is one actual compute queue for the batch engine. The transfer queue is
a separate real capability and is never batch width.

P11's executable worker architecture was deleted in R2e2. Public v1
diagnostic and numeric flag tombstones remain ABI-compatible but are neutral
where they formerly described P11 workers or slots. Historical rationale is
preserved in
`../DevelopmentReport/PROMETHEUS_P11_ARCHITECTURE_RETROSPECTIVE.md`.
