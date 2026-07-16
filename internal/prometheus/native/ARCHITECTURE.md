# Prometheus native batch architecture

The current production batch route is deliberately small:

```text
public batch API -> supported-mask validation -> reactor_batch.c
  -> M30 task ownership -> M29 shared physical ring -> Vulkan SGEMM
```

## Optional cooperative-matrix negotiation

M40a adds an optional device-creation branch, not a production SGEMM route.
The instance requests at most Vulkan 1.3 (bounded by loader support). After the
ordinary compute device is selected, the runtime enumerates
`VK_KHR_cooperative_matrix`, queries KHR tuples, and selects only the audited
subgroup 16x16x16 FP16-input/FP32-accumulator tuple. The device chain enables
`cooperativeMatrix`, `shaderFloat16`, and `vulkanMemoryModel` only when the
extension, tuple, all features, Vulkan 1.3, and a 32-lane subgroup are present.

`PROMETHEUS_VK_DISABLE_COOPERATIVE_MATRIX=1` is a test-only absence
simulation. Unsupported devices retain the ordinary device-create chain and
all production pipelines. The cooperative module is reachable only through
the audit descriptor, requires full subgroups, rejects non-16-aligned M/N/K,
and never enters the production selector or registry.

## Fused reduction family

M39b adds an independent row-wise reduction family through the same Vulkan
runtime/device/compute-queue ownership. It has a dedicated persistent physical
ring because its fixed multi-dispatch plans and temporary buffers are not SGEMM
batch entries. One logical request records all reduction stages into one
slot-owned command buffer and submit; compute barriers connect stages and one
timestamp query pair measures the GPU interval.

The public contract supports FP32 sum, max, and stable softmax only. Widths up
to 1024 use one reduction group per row (and fused softmax); larger widths use
1024-element partials followed by a bounded final stage. Ring buffers and five
pipelines persist and grow/reuse by slot. Logical failure, quarantine, reap, and
physical recyclability are separately diagnosed. This family does not alter
the SGEMM shared ring, async tokens, batch engine, or selector.

`reactor_batch.c` is the production batch authority. It owns immutable
caller-order plans, logical-lane metadata, per-entry runtime facts,
deterministic failure selection, admission stop/refill, staging, diagnostics
preparation, and ordered commit. It delegates physical slot generation and
submission evidence to M29 and task lifetime, feedback, quarantine, and reap
to M30/M30a.

Logical width and contiguous partition are planning hints only. They do not
represent CPU workers, Vulkan queues, command resources, or physical slots.
There is one actual compute queue for the batch engine. The transfer queue is
a separate real capability and is never batch width.

P11's executable worker architecture was deleted in R2e2. Public v1
diagnostic and numeric flag tombstones remain ABI-compatible but are neutral
where they formerly described P11 workers or slots. Historical rationale is
preserved in
`../DevelopmentReport/PROMETHEUS_P11_ARCHITECTURE_RETROSPECTIVE.md`.
The R2e2 deletion evidence is in
`../DevelopmentReport/PROMETHEUS_R2E_P11_REMOVAL.md`. Future concurrency ideas
are documented only—none are implemented or authorized—in
`../DevelopmentReport/PROMETHEUS_CONCURRENCY_FUTURE_DIRECTIONS.md`.
