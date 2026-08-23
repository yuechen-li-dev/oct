# Algorithms

Small, readable algorithms that make their contracts and complexity visible.

- `FirstWhere<T>` and `CountWhere<T>` use explicit templates so exact predicate types are preserved without per-type copies.
- `BinarySearch` is the O(log n) search for ascending `Int[]`; sortedness remains the caller's contract.
- `GCD` and `LCM` show Euclid's algorithm and its sign/zero conventions.
- `IsPrime` is a bounded O(sqrt(n)) reference test.
- `PrimesThrough` is the O(n log log n) Sieve of Eratosthenes.

Textbook example:

```oct
let primes = PrimesThrough(20)!
// [2, 3, 5, 7, 11, 13, 17, 19]
let index = BinarySearch(primes, 13)
// 5
```

These routines favor transparent moderate-input implementations. They are not a big-integer or cryptography package.
