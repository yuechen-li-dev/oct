# FLOW-backed queries

## Purpose and semantic category

QUERY-M0 provides bounded, read-only iteration over one explicit source. A
query is not an ordinary function: a function computes one returned value,
while a query FLOW may yield zero or more values before completing. A query is
also not a persistent behavioral controller. It uses cursor state, resumes
immediately when stepped again, and eventually completes; a behavioral FLOW
accepts turns and models durable control progression.

`query` is a contextual top-level authoring form. The parser lowers it directly
to one ordinary yielding FLOW with `Cursor` and `Emitted` board fields and one
`Scan` state. There is no Query value, query AST/MIR node, lazy sequence,
coroutine scheduler, channel pipeline, or second runtime.

`template query Name<T>(...)` is specialized first. Its concrete declaration
then follows this same Query-M0-to-FLOW lowering path; FLOW never handles open
type parameters.

## Sources and syntax

The first parameter is always the explicit source. QUERY-M0 accepts `T[]` and
record tables. Array order is index order. Record-table order is its observable
logical row order.

```oct
concept Job {
    ID: Int
    Status: Int
}

fn IsReady(job: Job) -> Bool { return job.Status == 1 }
fn Normalize(job: Job) -> Job { return job with { Status: 1 } }

query ReadyJobs(jobs: Job[], limit: Int) yields Job {
    filter IsReady
    map Normalize
    take limit
}
```

The record-shaped `concept Job` is useful when jobs have nominal domain
meaning; query iteration remains separate from that nominal identity. Ordinary
records work equally well. Record-table rows use a bounded inline expression
when their compiler-owned row type is not a source-visible named callback type:

```oct
query ReadyRows(rows: JobTable, limit: Int) yields JobView {
    filter { item.Status == 1 }
    map { JobView { ID: item.ID Status: item.Status } }
    take limit
}
```

Named top-level functions are the normal predicate/projection form. Inline
blocks contain exactly one expression and bind the current value as `item`.
They are bounded compiler lowering, not closures: they capture no lexical
environment. `filter`, then `map`, then `take` is the only stage order; every
stage is optional and may appear at most once. `take` must be final.

## Operators

- `filter` (`Filter` vocabulary, equivalent to Where) produces zero or one
  output per input and preserves relative order. Its expression must be `Bool`.
- `map` (`Map` vocabulary, equivalent to Select) produces one projected value
  per surviving input and preserves order. Its type must equal `yields`.
- `take N` yields at most the first `N` upstream values. `N <= 0` completes
  before reading the source.
- `Query.First(flow)` returns the first yielded value as a fallible result. An
  empty flow is a not-found error.
- `Query.Any(flow)` returns true after the first yield and false at completion.
- `Query.Count(flow)` counts yields and consumes the flow to completion.

The three terminals require a no-input yielding FLOW. `First`, `Any`, and
`Take` stop upstream work immediately when their answer is known. Terminals
consume the supplied flow's current continuation; create a new query instance
to repeat a query.

One hard evaluation error fails the query operation. Values yielded before an
error have already been observed and cannot be revoked. QUERY-M0 predicates and
projections are ordinary Oct evaluation and should be deterministic, local,
and free of external side effects whose repetition matters.

## Relationship to eager collection operations

`Array.Where` is an eager mask operation that materializes a new array.
`Append` and ordinary functions remain the right tools for eager/local data
transforms. Query `filter`/`map` are yielding stages: each record is tested,
optionally transformed, and yielded without an intermediate collection per
stage. `with` remains the immutable transformation of one nominal value; it is
not a query operator.

QUERY-M0 has no joins, grouping, arbitrary ordering, distinct, reducers,
windows, planner, cost model, SQL expressions, general lambdas, or optimizer
contract. Future source specialization may change the scan algorithm without
changing query results or base order.
