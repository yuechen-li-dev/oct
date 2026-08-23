# OCTETDB-PERF-M4 compiler provenance and specialization observations

## Verdict

The Oct compiler required no language or backend change for PERF-M4. Commit
`ca22ab8dfc20ac6d6c59dd34976789cd2c84ad2e` generated the W5 safe-Go FLOW
adapter through the existing experimental `internal/build.EmitGoSource` seam.
A small evidence-only generator lives at `Experiments/PerfM4`; generated code
was not hand edited.

## Chosen specialization

Profiling preceded the choice. Durable W1 was dominated by filesystem sync, so
W1–W4 remained S0. W5 default full scans were dominated by JSON validation,
decode, and allocation. Its Oct source declares a bounded `Job` record and
four query FLOWs: filter, filter+take, filter+map, and count. Generated Go is a
single fused state machine per query with array index cursor and emitted-count
board state. There are no channels, goroutines, iterator runtime, unsafe, cgo,
manual allocator, or handwritten repairs.

## Provenance and size

- Oct commit: `ca22ab8dfc20ac6d6c59dd34976789cd2c84ad2e`.
- Oct source: 34 physical / 26 nonblank lines.
- Generated safe Go: 1,473 physical / 1,339 nonblank lines, 46,598 bytes.
- Generator wall time on the recorded Windows host: 448.69 ms first measured
  run, then 172.42 and 205.47 ms with warm Go build cache.
- Runtime materialization median: 0.44 ms / 1k records, 3.89 ms / 10k, and
  45.72 ms / 100k.

## Result

In the primary WSL matrix, the mixed W5 lane measured 219.7× default. Focused
25%-selectivity full filters measured 63.4×, 68.1×, and 92.3× at 1k, 10k, and
100k. Point lookup gained only 1.44× in WSL and does not use generated code.
Mutation workloads gained no compiler-owned path and their S0 controls stayed
near 1× subject to storage variance.

## Product boundary exposed

The compiler can generate the fast loop, but OctetDB v0.2.0 has no product-owned
published read representation or transactional index update contract. PERF-M4
therefore uses S1 only for a read-only W5 dataset materialized from the durable
Dataset at initialization. Extending that mirror to W6 would introduce a
commit-to-publication race or a broad locking adapter; PERF-M4 refused both.

This is compiler success plus integration evidence: the next missing piece is
not a new query language feature. It is a product-owned coherence/publication
boundary if OctetDB later prioritizes compiled query integration.
