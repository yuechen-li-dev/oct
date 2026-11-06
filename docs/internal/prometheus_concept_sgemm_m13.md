# M13: one Concept SGEMM executor with a C++23 host

The former `reactor_vulkan_sgemm.c` executor is removed. Every synchronous,
resident, async, batch and placement-benchmark submission now traverses
`PrometheusSgemm.concept`. Ordinary Oct `@`, explicit Prometheus FP32 conversion,
and the public SGEMM ABI retain their existing contracts. M14 owns the later
measured `@` dispatch and binary64 GPU matrix-multiplication work.

## Ownership boundary

Concept owns five physical work slots: four shared by resident/async/batch
operations and one for synchronous/audit work. A prepared job transitions to
executing exactly once. Each slot owns its recordings, submission/fence or
transfer-to-compute handoff, completion observation, and retirement. A failed
observation leaves the owner intact for quarantine/reap. Replacing a slot waits
for prior work; dropping the executor drains all slots before its command pools.
The existing Concept Vulkan library supplies recording, hazard tracking, bounded
copies, submission, semaphore handoff and waits. Its new ordinary `Poll` operation
is supplied by [Concept PR #194](https://github.com/yuechen-li-dev/Concept/pull/194);
merge that producer change before integrating this consumer.

`reactor_sgemm.cpp` retains the shared runtime/device host, mapped buffer arenas
and their real allocation metadata, package pipeline instances, descriptor
storage, and existing judgment/diagnostic policy. These are scoped runtime
resources borrowed by a job, never adopted twice. Runtime teardown retires the
Concept executor before releasing its borrowed buffers, descriptors or pipelines.
Persistent arena mutation and descriptor updates retire their previous work
first; temporary audit scopes also establish retirement before releasing their
borrowed resources.
Pipeline creation uses a bounded portfolio and a scoped shader-module lease.
The small C++ driver adapter binds existing package instances and writes legacy
timestamp intervals. It owns no submission loop, fence or command buffer.

The C11 adapter transports generated layouts only. Size, alignment and field
offset assertions cover the job and shader packet. Both native launchers build
the C++23 host through `manifest.concept`; GNU linking groups the C/C++ consumer
archives because generated C calls the C++ driver callbacks. Both native targets
explicitly select `NativeTarget.optimize = true` (`-O2`, without fast-math).
The same producer PR supplies this optional setting and applies it to ABI
probes; existing nine-field manifests retain their old behavior. Generated C,
provenance, objects and archives remain ignored build products.

This is an intent port: existing selector/admission policy and independent
numerical/failure oracles remain useful; the old mechanical handoff-copy test
was removed with its deleted helper. The replacement integration test exercises
real GPU ownership and independent outputs across SGEMM and FFT.

## Defects exposed and corrected

The tiled, 2x2 and 2x4 dispatch metadata omitted `threads_z` in positional
initializers. Later fields shifted, inflating the supposed columns covered by
one workgroup and leaving real columns unwritten. Named fields restore the
actual footprint without changing shader arithmetic or numerical tolerances.
The original executable's SGEMM lane had 75 passes, one skip and seven failures
(126 assertions); the original occupancy and safe-policy numerical tests also
failed separately. Those failed baseline logs are retained locally.

The initial ownership rerun also exposed a polling/accounting gap. A consumer
must not set `Submission.waited` after a raw fence poll: this bypassed the
native retirement counter. Concept's new `Poll` retains not-ready/error
ownership and calls the immediate wait path only after a signaled observation.
The library's new fact proves idempotence with another independently pending
submission. Lifetime tests now account for SGEMM's own borrowed contexts while
still requiring every context and allocation to return to baseline at teardown.

Public multi-token SGEMM also wrote the old singleton async admission state,
then consumed only its task-table entry. Sync admission now reads outstanding
public ownership separately, so consuming all tasks releases admission while
unconsumed tasks still reject conflicting calls.

The broad native sweep also found a stale reduction registry test dereferencing
an intentionally null source-provenance pointer (`0xC0000005`, 188 passes and
four skips before the stop). Native checks now validate exact package identity;
the package inventory's Go test owns the source-root assertion for all seven
production reduction sources. The failed sweep is retained as evidence.

Two feedforward integration tests were also skipping their own injected no-wait
failure. They now require real GPU completion, verify the constant-input product,
and retain dispatch-local reconciliation facts separately from later planner
updates. All 14 feedforward facts pass without a skip.

## Qualification

Hardware: local RTX 3070, shared Vulkan device with compute and dedicated
transfer queues. Native checks use `PROMETHEUS_VK_VALIDATION=1` and retain
failure, quarantine, atomic batch commit, reuse and independent CPU oracles.
The new repeated-transfer test requires requested validation to be active.

The module's five emitted artifacts match across three fresh emissions.
Normal and Verify ABI headers match; both emitted implementations compile
with `clang -std=c11 -pedantic-errors -O2`.

The fast Oct Go suite, the Prometheus/tooling tests and vet pass. The compiled
matrix corpus passes 11 facts plus five negative specimens, and the forced GPU
corpus passes three facts with zero interpreted fallback. Concept's 19 Vulkan
facts pass in both modes, its quick lane passes, and four native-build boundary
tests pass, including optimized C11/C++23 execution, malformed mode rejection,
independent ABI measurement and legacy structured arguments.

The broad Normal native run passed **416 facts**, skipped **10**, and failed
none, with Vulkan validation enabled. Earlier failed runs remain recorded above.
The final recording/submission timing instrumentation and Verify consumer lane
are being checked separately before closeout. The Concept execution boundary
measures actual recording and compute-submission CPU intervals independently;
preparing a job packet is not reported as the entire recording cost.
