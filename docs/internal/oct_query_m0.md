# OCT-QUERY-M0 — FLOW-backed read-only query algebra

## 1. Verdict

Success

## 2. Problem

Oct had eager array operations and general yielding FLOWs, but no compact way to
express a source-bound filter/map/take pipeline or terminal `First`, `Any`, and
`Count`. Building a second iterator runtime would have duplicated the language's
existing continuation authority.

## 3. Existing collection/FLOW model

Arrays already provide deterministic index order, `Len`, indexing, `Append`,
and eager `Array.Where`; ordinary loops cover eager map/filter construction.
Record tables already expose `Len`, logical row indexing, and compiled-data
layout facts. `batch` is an eager collection transform lowered through
`MIRBatchMap`. Octomata already supplies FLOW construction, private board state,
`yield`, `Step`, `DidYield`, `Yielded`, completion, and generated state machines.
No existing eager helper was replaced.

## 4. Query semantic model

Every query begins at its first explicit `T[]` or record-table parameter. It
walks source order with a cursor, tests one item, optionally transforms it,
yields, and resumes. An ordinary function computes one return value. A query
FLOW may yield zero or more values. A behavioral FLOW models persistent turn
input and control progression; a query FLOW is a read iterator that resumes on
the next request and eventually completes.

## 5. Operator set

The source form uses `filter`, `map`, and `take`; terminal builtins are
`Query.First`, `Query.Any`, and `Query.Count`. `Filter`/`Map` match Oct's
scientific collection vocabulary better than LINQ's `Where`/`Select`. `Take`
with `N <= 0` reads nothing. `First` is fallible on empty input, `Any` returns a
boolean, and `Count` consumes all remaining yields. Skip and all broader query
operators are absent.

## 6. FLOW lowering/runtime reuse

The contextual top-level `query` parser lowers immediately into an ordinary
`ast.FlowDecl`: one `Scan` state and a `{Cursor, Emitted}` board. The normal
FLOW typechecker lowers that to `MIRFlow`; the normal interpreter and compiled
FLOW implementation execute it. The terminal builtins drive the existing
`__octStep`, `__octDidYield`, `__octYielded`, and `__octComplete` interface.
There is no query AST, `QueryPlan`, iterator MIR, generator VM, task, coroutine,
channel, goroutine, or runtime query object.
LayoutContract and StaticFacts are not required for correctness and M0 performs
no fact-based shortcut. A future proven exact extent or key-access fact may
specialize the same query only when it preserves observable order and terminal
behavior.

## 7. Authoring/API decision

Bounded syntax was chosen over nested library combinators because Oct's
function values intentionally do not provide a broad closure language. Named
top-level predicates/projections cover nominal arrays. A single-expression
`{ item ... }` block covers compiler-owned record-table row types without
adding closures. Clauses are optional, unique, and ordered filter → map → take.
The first parameter is unconditionally the source.

The authoring study compared all three bounded shapes. Explicit hand-written
yielding FLOWs already worked but repeated cursor/length/yield bookkeeping in
every query. Library combinators returning FLOWs would need ergonomic predicate
and projection function values plus nested machine composition that M0 does not
otherwise justify. Thin syntax won because it removes that repetition and
fuses before MIR while preserving ordinary FLOW as the inspectable result.

Record-shaped `concept Job` proved helpful in `query_m0.octest`: it gives the
application record nominal domain identity and preserves natural `with`
updates, while the query remains a separate FLOW concern. It does not turn a
Concept into a data source or create relationship discovery.

## 8. Generated implementation

The compiled structural test finds one emitted `__octFlow_..._ReadyIDs` state
machine, one `Scan` state, and cursor/emitted board fields. It rejects generated
shapes containing an iterator/query runtime, channels, or goroutine closures.
The filter, map, and take clauses are fused in that one state: no intermediate
filter or map array and no state machine per operator is emitted. Compiler work
also completed record-table `Len`/index materialization, local record-field
access, and reachability for named functions used only by FLOWs.

## 9. Failure/order/short-circuit semantics

Array index order and record-table logical row order are preserved through
filter and map. One hard evaluation/type failure fails consumption; prior
yields are already observable. `First` and `Any` return after the first yield.
`take` checks its bound before source access and after each resumption, so it
does not inspect a later item once the bound is reached. `Count` exhausts the
source. Invalid contracts cover missing sources, invalid clause order,
non-boolean predicates, and terminals applied to non-FLOW values.

## 10. Oct array/record-table proof

`Language/Data/QueryM0` proves empty, one item, filter none/some, map,
filter+map, take 0/1/beyond-length/negative, First found/missing, Any false/true,
Count, order preservation, immutable `with`, repeat determinism, and
record-table filter/map/order. The application type is a record-shaped Concept.
Both execution modes pass six facts; compiled execution reports six compiled
and zero interpreted fallback. Four `.octfail` contracts pass in both command
lanes.

## 11. Catalog/Dataset integration

The language layer has no dependency on OctetDB. OctetDB separately exposes a
dataset-scoped KeyedJSON scan and uses the same filter/map/take semantics in
ordinary Go product code. Catalog identity remains the source authority; Oct
does not receive arbitrary backend maps or filesystem topology.

## 12. Query snapshot semantics

Oct arrays and record tables are explicit values observed by the constructed
FLOW. Mutation is not a query stage. The OctetDB companion implementation
serializes a Dataset scan at its database admission boundary; that product
limitation does not alter Octomata semantics.

## 13. Ready-job product proof

The companion OctetDB golden job application opens `workers/jobs`, decodes
jobs, selects `Ready`, and stops at the requested limit. Claimed, completed,
and failed jobs are excluded; requeue makes a failed job discoverable again;
restart preserves the deterministic result.

## 14. Second golden-app proof

The companion inventory application opens `inventory/items`, selects stock at
or below an explicit threshold, applies a limit, and proves deterministic
results after restart. No job-specific language feature was needed.

## 15. Benchmarks/allocations

On Windows/amd64, Ryzen 7 7700X, the generated compiled FLOW benchmark measured:

| Records | Filter | Filter+Take(10) | Filter+Map | Count |
| ---: | ---: | ---: | ---: | ---: |
| 1,000 | 4.239 µs | 228 ns | 4.741 µs | 11.951 µs |
| 10,000 | 43.140 µs | 233 ns | 49.645 µs | 84.188 µs |
| 100,000 | 463.213 µs | 256 ns | 485.982 µs | 848.322 µs |

All compiled cases reported 0 B/query and 0 allocations/query. Full scans cost
about 4.2–12 ns/record depending on operator. Take examined exactly 37 records
at every scale for a one-in-four predicate, proving scale-independent early
stop. The integration evidence lane is
`go test -tags integration ./internal/build -run '^TestCompiledQueryM0Benchmarks$' -v`.

## 16. WAL/read-only proof

Oct query execution has no database/WAL authority. In the companion product
test, scanning leaves WAL bytes, durable sequence, and dedupe identities
unchanged.

## 17. LLM legibility

The fresh-agent check is recorded in the companion product report because it
tests the candidate Go API. The intended path requires only documented catalog
opening, `Bucket`, `Dataset`, and `ScanDataset`; no Oct compiler concept is
needed by the Go user.

## 18. Rejected SQL/LINQ/planner features

M0 adds no SQL expressions, implicit/global relation namespace, join, group,
arbitrary sort, distinct, reducer, window, fluent LINQ builder, closures,
dynamic projection, cost planner, secondary index declaration, optimizer,
MVCC, parallel pipeline, or new Dataset kind. `Array.Where` remains the eager
mask operation; `with` remains a one-value immutable transform.

## 19. Required architecture decision

2. Thin query syntax/IR sugar is justified but lowers completely to FLOW

The implementation is thinner than a persistent query IR: syntax lowers to
FLOW in the parser, before MIR, which makes FLOW the only runtime authority.

## 20. Required product decision

A. Scan-based Dataset query is sufficient for v0.2

The companion evidence shows correct bounded scans and explicitly records
long-read write blocking; an index is not required for the candidate release.

## 21. Remaining limitations

M0 supports only arrays and record tables, a single filter/map/take chain,
named functions or non-capturing inline expressions, and no-input terminals.
There is no terminal collection builtin, query planner, index specialization,
general closure support, or cancellation facility for pure in-process Oct
values. Generated FLOW host-facade ABI remains experimental.

## 22. Exactly one next recommendation

After v0.2 query usage is measured, add a proof-directed source specialization
only for a demonstrated hot predicate while keeping this exact FLOW semantic
contract and deterministic order.
