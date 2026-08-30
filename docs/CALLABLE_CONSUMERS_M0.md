# Typed callable consumers M0

## Friction inventory

The audit used current library and compiler code rather than historical docs.

| Area | Existing shape | Classification | M0 action |
| --- | --- | --- | --- |
| `Algorithms.FirstWhere<T>` | exact `fn(T) -> Bool`, fallible no-match | keep | retained |
| `Algorithms.CountWhere<T>` | exact `fn(T) -> Bool` | keep | retained and capture-tested |
| `Array.Where` | eager compiler-owned boolean-mask selection | unrelated to captures | retained; no predicate overload |
| `Matrix.tabulate` | compiler-owned `fn(Int, Int) -> T` generator | keep / callable cleanup | capture, order, and parity coverage added |
| Vector construction | no indexed generator | replace missing awkward surface | added `Vector.tabulate` |
| Optimization solvers | canonical `fn(Float[]) -> Float` and related exact callables | keep | direct captured objective coverage expanded |
| `NelderMeadWithContext`, `CoordinateDescentWithContext` | packed context plus `fn(Float[], Float[]) -> Float` | compatibility wrapper | retained as thin adapters; direct captured objective documented as preferred |
| Numerics integration, roots, differentiation, ODE, least squares, simulation sweep, tensor finite differences | ordinary exact function values already | keep | no duplicate APIs |
| Database query predicates | ordinary `fn(Record) -> Bool` template seam | keep | no redesign |
| String and IO | no callback-only context threading found | unrelated | no change |
| `batch` body syntax | scheduling, ordering, failure, and worker semantics | unrelated to ordinary eager map | no adapter or rewrite |
| OctGo function parameters | callable environment transport explicitly unsupported | out of scope | diagnostic retained |

## Selected API surface

The established user-authored generic collection home is `Algorithms`, while
`Array` is currently a compiler-owned namespace for storage primitives such as
mask selection and cross-sections. M0 therefore adds:

```oct
template fn Map<T, U>(values: T[], transform: fn(T) -> U) -> U[]
template fn MapIndexed<T, U>(values: T[], transform: fn(Int, T) -> U) -> U[]
template fn Filter<T>(values: T[], predicate: fn(T) -> Bool) -> T[]
template fn Any<T>(values: T[], predicate: fn(T) -> Bool) -> Bool
template fn All<T>(values: T[], predicate: fn(T) -> Bool) -> Bool
template fn FindIndex<T>(values: T[], predicate: fn(T) -> Bool) -> Int
template fn Fold<T, A>(values: T[], initial: A, reducer: fn(A, T) -> A) -> A
```

`CountWhere<T>` remains the existing canonical implementation. `FindIndex`
uses `-1` for no match, matching the established `Algorithms.BinarySearch`
convention. The new mathematical constructor is:

```oct
Vector.tabulate(length: Int, generator: fn(Int) -> T) -> Vector<T>
```

`Matrix.tabulate` remains the canonical matrix generator.

## Semantics and implementation

All collection consumers are eager and sequential. `Map`, `MapIndexed`,
`Filter`, `CountWhere`, and `Fold` visit ascending source indices exactly once.
`Fold` is left associative. `Any` stops at the first true result, `All` stops at
the first false result, and `FindIndex` stops at the first match. `Any([])` is
false, `All([])` is true, and an empty fold returns its initial accumulator.
Fresh result arrays avoid hidden source mutation.

M0 callbacks are infallible. A fallible callback is an exact type mismatch;
future error-propagating operations would require explicit names such as
`TryMap`, not hidden behavior in these APIs.

Templates perform generic specialization before execution. The specialized
consumer has an ordinary concrete function parameter. In the interpreter,
function-typed locals and parameters resolve to `FunctionValue` and all calls
use `invokeFunctionValue`; an anonymous capture environment is cloned once at
construction and is not inspected by the consumer. In compiled MIR, callback
calls are ordinary `MIRCall` nodes marked `FunctionValue`; captured and
noncaptured origins do not create different consumer operations. Generated Go
uses typed `func` values. `Vector.tabulate` and `Matrix.tabulate` remain narrow
representation-boundary builtins, but they invoke the same typed callable and
never reflect on captures.

## Compatibility and deliberate omissions

No public API was removed. Optimization's contextual functions remain wrappers
because they may be used by existing callers. New code should capture model
data directly and call the canonical solver.

M0 does not add `FilterIndexed`, predicate overloads on compiler-owned
`Array.Where`, lazy iterators, fluent pipelines, streams, shared mutable
captures, implicit closures, fallible consumer overloads, new numerical
cathedrals, batch adapters, function reflection, or OctGo callable transport.
`FilterIndexed` lacked independent current usage evidence; the index-aware gap
was covered by `MapIndexed` and both tabulation APIs.

## Dogfood and diagnostics

Coverage includes captured callbacks for every selected consumer, an escaped
callable returned by `MakeScale`, a template-to-captured-callable-to-generic-
consumer composition, scientific array normalization, parameterized matrix
generation, source isolation, deterministic index/order behavior, and
Optimization objectives. Invalid contracts cover callback parameter type,
return type, arity, fallibility, captured-function mismatch, template
specialization mismatch, vector length type, and vector generator type.

## Compiled performance

The repository benchmark lane ran each compiled Oct workload in a separate
process on 2026-08-30. Workloads use 4,096-element arrays (500 map/filter or
1,000 fold repetitions) and 96 by 96 matrices (200 repetitions). Times include
process startup, so small differences are directional rather than a
microbenchmark claim.

| Operation | Direct loop | Named callback | Captured callback |
| --- | ---: | ---: | ---: |
| Map | 184.023 ms | 180.297 ms | 179.642 ms |
| Filter | 172.858 ms | 171.491 ms | 176.504 ms |
| Fold | 154.575 ms | 157.376 ms | 159.709 ms |
| Matrix tabulate | 198.261 ms | 170.064 ms | 173.283 ms |

This run shows no catastrophic compiled callable overhead. Capturing and named
callbacks are close; capture environments are constructed outside the measured
inner loop. No special-case inlining was added.
