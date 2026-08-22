# BATCH-SPECIALIZE-M0 — homogeneous batch lowering

## 1. Verdict

**Success**

The existing `batch` semantics and `MIRBatchMap` authority are preserved. Tiny
batches execute as direct loops; large batches use bounded contiguous ranges,
direct indexed output, one join, and deterministic lowest-index failure
selection. The compiled helper no longer contains per-item jobs/results
channels. Both interpreted and compiled Batch language lanes pass 11/11.

## 2. Motivation

TIGER-COMPARE-M0 showed that TigerBeetle continued scaling at large offered
batches while direct Go and Oct plateaued. TigerBeetle transaction batching is
not Oct `batch`; the result was pressure evidence that retaining homogeneous,
contiguous structure can matter. This milestone tests only Oct's deterministic
data-parallel map lowering.

## 3. Existing language semantics

`Language/reference/runtime/22-batch.md` remains authority: array input; one
output per input; exact length/order; implicit join; any item failure fails the
whole batch; no partial output. M0 specifies the previously unstated concurrent
failure rule: the lowest failing input index supplies the visible error.

## 4. Existing implementation

Before M0, both paths selected `min(GOMAXPROCS, len(items))` workers, sent every
index through a jobs channel, sent every indexed value/error through a results
channel, joined, and reassembled results. The interpreter additionally cloned
the complete visible environment once per item. The compiled worker captured
every surrounding user local, whether referenced or not.

## 5. Baseline measurements

Host: Windows/amd64, AMD Ryzen 7 7700X, `GOMAXPROCS=16`. Go benchmark numbers
below are execution-shape controls matching the old emitted helper. `ns/item`
includes exact output allocation. The committed benchmark also reports bytes,
allocations, goroutines, and known channel sends.

| N | trivial old | trivial specialized | moderate old | moderate specialized |
|---:|---:|---:|---:|---:|
| 1 | 644.7 | 10.9 | 1,093 | 26.5 |
| 2 | 511.0 | 7.3 | 849 | 22.7 |
| 4 | 415.7 | 4.5 | 662 | 17.7 |
| 8 | 318.1 | 2.9 | 504 | 17.0 |
| 16 | 294.8 | 7.2 | 396 | 18.8 |
| 32 | 163.7 | 4.2 | 264 | 16.1 |
| 64 | 125.1 | 2.0 | 189.2 | 16.4 |
| 128 | 94.4 | 1.9 | 146.0 | 14.9 |
| 256 | 90.2 | 2.2 | 138.1 | 15.3 |
| 512 | 130.4 | 8.6 | 142.3 | 23.2 |
| 1024 | 126.9 | 6.9 | 142.0 | 18.7 |

The moderate body performs 24 arithmetic rounds. Captured-call and fallible
large-size curves are in section 14. The old path performs `2*N` channel sends;
the specialized path performs zero.

## 6. MIR architecture

The path remains:

```text
BatchExpr -> MIRBatchMap -> deterministic BatchPlan -> Go emitter
```

No second batch IR/runtime was introduced. `MIRBatchMap` gained only a `Nested`
lowering fact used to disable inner parallelism.

## 7. BatchPlan

`internal/batchplan` owns four deterministic facts: item count, worker count,
chunk size, and strategy. M0 selects sequential through 256 items. Above 256 it
uses `min(GOMAXPROCS, ceil(N/128))` workers, so every planned worker owns roughly
128 contiguous items. The threshold is documented beside the policy and is
backed by the baseline: cheap sequential bodies remain far below goroutine cost
through N=256.

## 8. Sequential specialization

The sequential path allocates the output once, invokes the existing synthetic
worker in index order, writes directly by index, and records the first/lowest
failure while still completing all items. It uses no goroutine, channel,
WaitGroup, or result envelope. At N=64 trivial work fell from 125.1 to 2.0
ns/item and from 20 to 1 allocations/batch.

## 9. Chunked parallel specialization

At N=512 the plan owns four ranges; at N=1024 it owns eight on this host. Each
goroutine receives one `[start,end)` range and writes only `ordered[index]` in
that range. There is one WaitGroup join. There is no centralized dispatch,
sorting, append, or per-item result transport.

## 10. Failure semantics

The old compiled/interpreted implementations selected the first error received
from a channel, so concurrent failure identity depended on scheduling. M0 uses
one failure slot per range. Each slot retains its lowest failure; after join the
runtime selects the global lowest index. Empty error messages are also handled
correctly because failure presence is a Boolean rather than `err != ""`.
Internal output writes remain unpublished when any failure exists.

## 11. Capture analysis

`internal/batchcapture` lexically walks statements/expressions with block,
loop, match, and nested-batch scopes. The compiler intersects free names with
outer locals, sorts them, and emits distinct synthetic capture parameters. The
language specimen deliberately has `offset` and unrelated `unrelated`; only
`offset` appears in `MIRBatchMap.Captures`. The interpreter uses the same free
name authority to snapshot only referenced bindings.

## 12. Nested batch

An outer batch may parallelize. A batch lowered/evaluated inside an active item
uses a sequential plan. This prevents multiplicative goroutine creation and
cannot deadlock because there is no persistent pool. The nested language
contract passes in both execution modes with exact ordered output.

## 13. Generated Go audit

Representative shape:

```go
ordered := make([]U, len(items))
for slot := 0; slot < plan.workerCount; slot++ {
    start := slot * plan.chunkSize
    end := min(start+plan.chunkSize, len(items))
    go func(slot, start, end int) {
        for index := start; index < end; index++ {
            out := worker(items[index])
            ordered[index] = value(out)
        }
    }(slot, start, end)
}
wg.Wait()
```

Structural integration tests reject `make(chan`, `jobs :=`, `results :=`, and
the old item-result envelope; require BatchPlan, range iteration, direct indexed
writes, and failure slots; prove a nested node exists; and compare two emitted
sources byte-for-byte.

## 14. Performance curves

Large-size control results (`ns/item`):

| body / strategy | 64 | 128 | 256 | 512 | 1024 |
|---|---:|---:|---:|---:|---:|
| trivial old | 125.1 | 94.4 | 90.2 | 130.4 | 126.9 |
| trivial planned | 2.0 | 1.9 | 2.2 | 8.6 | 6.9 |
| moderate old | 189.2 | 146.0 | 138.1 | 142.3 | 142.0 |
| moderate planned | 16.4 | 14.9 | 15.3 | 23.2 | 18.7 |
| captured call old | 182.6 | 142.7 | 133.4 | 132.6 | 127.5 |
| captured call planned | 3.5 | 2.9 | 3.8 | 9.9 | 6.6 |
| fallible old | 181.3 | 140.5 | 126.6 | 129.2 | 128.0 |
| fallible planned | 4.1 | 2.9 | 4.8 | 9.7 | 7.2 |

At N=512 trivial throughput rises from about 7.7M to 116.6M items/s (15.2x);
moderate rises from 7.0M to 43.1M (6.1x). At N=1024 trivial reaches about
145.9M items/s versus 7.9M old (18.5x). These are compiler-helper mechanism
controls, not a claim of TigerBeetle parity.

Handwritten controls are committed for sequential, old worker pool, and
chunked range execution. On cheap bodies handwritten sequential remains faster
than range parallelism; M0 intentionally keeps it through N=256.

## 15. Allocation results

At N=512 trivial work changes from 23,843 B and 20 allocations/batch to 4,756 B
and 11 allocations; bytes/item fall from 46.6 to 9.3. The remaining allocations
are exact output, bounded failure metadata, WaitGroup/goroutine closures, and
the synthetic worker closure when captures exist. Jobs/results channel storage
and result-envelope storage disappear. Sequential batches allocate only exact
output in the control (one allocation).

Record-result controls show the same shape at N=512: 32,811 B old, 8,852 B
planned; 20 versus 11 allocations.

## 16. Hardware-counter results

Not available on this Windows host. Linux `perf` IPC/cache/branch counters were
not fabricated and are not required for Success. The structural removal of
`2*N` channel sends is directly measured; CPU utilization was not sampled by a
portable counter. Benchmarks record `GOMAXPROCS` and created goroutines/batch.

## 17. Interpreter parity

The interpreter now uses the same plan, range ownership, direct output, lowest
failure rule, and nested policy. Reusing free-variable capture analysis removed
an additional O(N²) artifact: the full input array was cloned into every item
environment even when unreferenced.

| interpreted body | N=64 before/after ns/item | N=512 before/after | N=1024 before/after |
|---|---:|---:|---:|
| trivial | 7,974 / 907 | 26,133 / 499 | 39,932 / 442 |
| moderate | 57,905 / 50,595 | 50,255 / 15,420 | 55,548 / 14,742 |
| captured | 11,819 / 3,259 | 29,082 / 1,694 | 46,194 / 1,982 |
| fallible | 10,129 / 2,649 | 28,452 / 1,838 | 45,555 / 1,907 |

At N=1024 trivial allocated bytes fall from about 548 MB/batch to 2.06 MB.
Interpreter result cloning remains required by Oct value semantics.

## 18. Semantic compatibility

`Language/Concurrency/Batch/valid/batch_valid.octest` passes 11/11 interpreted
and 11/11 compiled with zero fallback. Contracts cover empty, single, ordinary
and large order, immutable capture, body locals, calls, fallibility, repeated
multi-failure, no partial result, and nested batch. Existing invalid fixtures
for non-array input, missing/early return, and inconsistent result type remain
unchanged and green in the full Batch lane.

## 19. Side-effect/purity findings

The typechecker does not restrict batch calls to pure functions. Runtime/file/
wrapper effects can therefore be potentially observable, while pure arithmetic,
immutable values, and ordinary pure calls are safe independent work. This gap
predates M0: the old worker pool already scheduled effects nondeterministically.
The reference now explicitly distinguishes nondeterministic internal execution
order from deterministic output/failure order. No effect system or silent
source-placement restriction was added.

## 20. Future SIMD/tensor/GPU interpretation

Contiguous ranges and direct typed slices are a better future vectorization
shape than channels. The remaining scalar synthetic worker call can inhibit Go
inlining/vectorization; range-worker fusion is a bounded future compiler
optimization. No SIMD, tensor, Prometheus, or GPU path was added here.

## 21. TigerBeetle interpretation

Yes: preserving homogeneous structure eliminated the class of per-item channel
dispatch/result transport overhead identified by TIGER-COMPARE-M0. It does not
show that Oct `batch` is equivalent to transaction batching, nor that the
external database benchmark will improve by the same factor without rerunning.

## 22. Pressure findings

| finding | classification |
|---|---|
| arrival-order multi-failure identity | Bug / language-semantic gap, resolved |
| all-locals compiled capture | Compiler artifact, resolved |
| full-environment interpreter clone | Compiler/runtime artifact, resolved |
| per-item channel transport | Optimization opportunity, resolved |
| observable side-effect ordering | Language-semantic gap, documented |
| range-worker fusion/SIMD | Optimization opportunity, deferred |

## 23. Remaining limitations

The fixed threshold is evidence-backed on one CPU, not an adaptive cost model.
Cheap work can still prefer sequential execution beyond 256; M0 prioritizes a
simple deterministic range policy. Parallel batches do not cancel after error,
preserving completion semantics and deterministic selection. Interpreter
captures and results still clone to preserve value isolation. Side-effect order
is unspecified rather than statically rejected. Hardware counters and an
external Database-Scheduler rerun are not part of this repository-local pass.

## 24. Exactly one next recommendation

Rerun the Tiger safe-Go/Oct batch pressure test in `Database-Scheduler` against
this Oct compiler to measure how much of the external plateau was caused by the
now-removed per-item lowering overhead.
