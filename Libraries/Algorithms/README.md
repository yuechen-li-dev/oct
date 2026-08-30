# Algorithms

Small, readable algorithms that make their contracts and complexity visible.

- `Map<T, U>`, `MapIndexed<T, U>`, and `Filter<T>` eagerly materialize fresh arrays in ascending source-index order.
- `Any<T>` and `All<T>` short-circuit at the first decisive predicate result; `Any([])` is false and `All([])` is true.
- `FindIndex<T>` returns the first matching index or `-1`, matching this package's existing `BinarySearch` convention.
- `Fold<T, A>` applies its reducer left to right and returns the initial accumulator for empty input.
- `FirstWhere<T>` and `CountWhere<T>` use explicit templates so exact predicate types are preserved without per-type copies.
- `BinarySearch` is the O(log n) search for ascending `Int[]`; sortedness remains the caller's contract.
- `GCD` and `LCM` show Euclid's algorithm and its sign/zero conventions.
- `IsPrime` is a bounded O(sqrt(n)) reference test.
- `PrimesThrough` is the O(n log log n) Sieve of Eratosthenes.

Textbook example:

```oct
let primes = PrimesThrough(20)
// [2, 3, 5, 7, 11, 13, 17, 19]
let index = BinarySearch(primes, 13)
// 5
```

These routines favor transparent moderate-input implementations. They are not a big-integer or cryptography package.

## Ordinary callable consumers

Callbacks are ordinary exact Oct function values. Consumers do not distinguish
captured from noncaptured functions and do not inspect capture environments.
Use an explicit immutable capture to carry local scientific context:

```oct
let threshold = 5
let selected = Algorithms.Filter<Int>(values, fn(value: Int) -> Bool with {
    threshold: threshold
} {
    return value >= threshold
})
```

These M0 consumers accept infallible callbacks only. Fallible callback
propagation is deliberately left to separately named future APIs rather than
hidden inside `Map`, `Filter`, or `Fold`. All operations are eager; this package
does not define iterators, streams, lazy views, or hidden in-place mutation.

`SieveLimit` is a refined Concept requiring a value of at least two. Invalid runtime values fail once at explicit `SieveLimit(raw)?` admission; `PrimesThrough` is infallible after admission.
