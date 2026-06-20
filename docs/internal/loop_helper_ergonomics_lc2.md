# LC2 ergonomics report: explicit Loop helper library

Date: 2026-06-19

## Summary

The LC2 fixtures show that `Loop.Range` is usable in real Oct syntax for local
range state that must be visible and explicitly advanced. The helper is more
verbose than `for`, but that verbosity is aligned with the goal: no hidden
`continue`, `break`, iterator, generator, or Octomata lowering semantics.

## Findings

1. `Loop.Range` usage feels reasonable when a loop needs explicit state. For
   ordinary counted iteration, `for i in start..end` remains clearer.
2. Rebinding `loop = Loop.Advance(loop)` is noisy but legible. It makes the
   state transition visible and fits Oct's immutable record update style.
3. `Loop.Current(loop)` is somewhat verbose. It is acceptable for M0 because it
   avoids method syntax, hidden mutation, and nested namespaces.
4. `Advance` feels better than `Resume` for this helper. `Resume` would imply
   Octomata's remembered flow-state jump, while `Advance` describes a pure
   record transition.
5. `Stop` is clear enough for early termination. It reads as an explicit state
   transition rather than a non-local statement signal.
6. Users may eventually want `Loop.IsDone` or `Loop.Done` to avoid negative
   condition phrasing in non-`while` contexts, but the current `while
   Loop.IsActive(loop)` shape is sufficient for M0. `Loop.ShouldAdvance` does
   not appear necessary yet.
7. Skip-body examples do not require awkward nested `if` when written in guard
   style: perform work under `if not ShouldSkip(i)` and call `Loop.Advance` once
   at the bottom of the loop. The explicit early-advance variant is noisier.
8. This likely solves the Claude lab friction well enough for v0.1 when paired
   with dedicated diagnostics: users who try `continue`/`break` are pointed to
   guard conditions, `Loop`, or Octomata.
9. The next minimal ergonomic improvement would be documentation and examples,
   not more API. If evidence accumulates, consider a small `Loop.IsDone` alias.
10. LC3 should not add `continue`/`break` sugar for v0.1 by default. Diagnostics
    plus helpers are enough unless real workloads show repeated awkwardness that
    cannot be solved by guards, `Loop.Stop`, or Octomata.
