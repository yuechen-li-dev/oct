# SDSL-V M32a: indexed tensor notation semantics

M32a adds `tensor` statements as statically validated shorthand for repeated,
bounded GPU computations. The statement is deliberately separate from ordinary
indexed assignment, so no existing indexing acquires implicit tensor behavior.

```sdslv
tensor C[i, j] = Sum[k](A[i, k] * B[k, j]);
```

Conceptually this visits `i` then `j` in destination-axis order, accumulates
over `k`, and performs one final write to `C[i, j]`. `+=` adds that completed
contraction to the current destination element; it is not a repeated write in
the reduction.

Free indices come only from the destination and use its static extents.
`Sum[...]` binds reduction indices only for its body; those extents are inferred
from fixed-shape indexed source axes. There is no broadcasting, rank promotion,
reshape, dynamic shape, Unicode syntax, or implicit Einstein summation.

The initial source categories are compiler-known fixed arrays (including nested
fixed arrays), workgroup tiles, and register tiles. Index expressions retain
their ordered source list; physical tile, register-tile, and matrix-view
backends remain rank-two categories. Free indices are intentionally bounded:
may be direct or `base + index` / `index + base`; reduction indices are direct
axis uses. Guarded reads remain ordinary expressions and retain their existing
safety and fallback semantics.

M32a preserves source spans, type checks numeric `Sum`, and produces
`validate.ValidatedTensorAssign` metadata with resolved static index extents,
ordered free/reduction indices, and per-index provenance. Every inferred extent
retains its destination or source axis, source value name, source span, and the
compiler-owned constant expression/value that justified it. Conflicting
reduction extents surface both the original and conflicting occurrences through
related-span diagnostics.

Index scoping is explicit and local: free indices belong only to one `tensor`
statement, reduction indices belong only to one `Sum[...]` body, and no
unresolved identifier becomes an implicit tensor index. The accepted affine
surface is deliberately narrow: free indices may appear as `i`, `base + i`, or
`i + base`; reduction indices are direct axis uses. Nonlinear remapping,
multiple tensor indices in one axis, modulo, negative strides, and similar
computed transforms remain rejected in M32a.

Alias validation is conservative by design. A destination read is allowed only
when it uses the exact destination index tuple in destination-axis order;
permuted, offset, duplicated, or reduced destination reads are rejected. This
keeps M32a lowering-ready without starting M32b loop or backend lowering work.

It does not lower tensor statements: VD-MIR/HLSL loop generation, expansion,
reduction accumulators, and execution are M32b work. The lowerer reports that
boundary explicitly.
