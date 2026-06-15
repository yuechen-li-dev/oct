# Optimization Library — Surfaced Friction Report

Issues discovered during implementation that require upstream fixes in the Oct
compiler/interpreter. Ordered by severity.

---

## CRITICAL — Float[] Assignment is Reference Copy, Not Value Copy

**Symptom:** `var out = v` where `v: Float[]` creates an alias, not a copy.
Mutating `out[i]` also mutates `v[i]`. This caused:
- `Scale(g, -1.0)` mutating `g` while building `d`, making `Dot(g, d)` return
  `+|g|²` instead of `-|g|²` → GradientDescent panicked in Armijo line search
- JacobianCD rows sharing the same underlying array → all J[i][j] reads/writes
  would corrupt all rows simultaneously → singular JtJ matrix
- `InitialSimplex` vertices aliasing x0 → simplex mutation corrupted starting point

**Workaround applied:** All vector operations (Scale, VecAdd, VecSub, Centroid,
SimplexCombine, InitialSimplex, JacobianCD, JtJ, GaussElim, GaussNewton, LM)
now build new arrays using `Append` from scratch rather than starting with
`var out = v`.

**Fix needed:** `var out = v` where `v: Float[]` should deep-copy the array,
not alias it. This is the standard value-semantics expectation from an immutable
record language. The same issue would affect any Oct code that copies an array
and mutates the copy.

**Upstream task:** Fix Float[] assignment to perform value copy in the generated
Go code. Currently `var out = v` emits `out := v` in Go which creates a slice
header copy (same underlying array). Should emit `out := append([]float64{}, v...)`
or equivalent to create an independent copy.

---

## MEDIUM — Float[][] Row Aliasing

**Symptom:** When building a 2D matrix using `var firstRow = [0.0, ...]` then
`H = Append(H, firstRow)` multiple times, all rows of H reference the same
underlying array. Setting `H[0][j] = val` also sets `H[1][j] = val` etc.

**Cause:** Consequence of Float[] reference copy semantics above. `firstRow`
is a single array; Append copies the slice header (pointer + len + cap) not
the data.

**Workaround applied:** Build each matrix row with a fresh `var ri = [0.0]` +
Append loop before appending to the matrix.

**Fix needed:** Same as above — value copy semantics for Float[].

---

## MEDIUM — fn(Float[]) -> X Parameters Broken in Interpreter

**Symptom:** When a function takes `fn(Float[]) -> Float` or
`fn(Float[]) -> Float[]` or `fn(Float[], Float) -> Float` as a parameter and
calls it internally, the interpreter raises:
`runtime invariant violation: undefined function Package.paramName`

The interpreter looks up the parameter name as a global function rather than
calling the function value.

**Example:**
```oct
fn applyVec(f: fn(Float[]) -> Float, v: Float[]) -> Float {
    return f(v)   // interpreter looks up "Package.f", not the passed fn
}
```

**Workaround:** These functions work correctly in COMPILED mode. All tests in
this library pass in compiled mode (40/40 compiled, 0 interpreted fallback).
The interpreter fallback path is not triggered because compiled succeeds.

**Affected functions:** GradientDescent, GradientDescentMomentum,
NelderMead (all calls to objective function f), ArmijoLineSearch,
GaussNewton, LevenbergMarquardt, FitCurve.

**Note:** `fn(Float) -> Float` parameters work correctly in both compiled and
interpreted. The issue is specific to `Float[]` in the parameter list of the
function type.

**Upstream task:** Fix interpreter's function call resolution for function-typed
parameters. When calling `f(v)` and `f` is a local variable of function type,
the interpreter should call the function value stored in `f`, not look up a
global function named `f`.

---

## LOW — Nested Index Assignment Not Supported (board fields and local Float[][])

**Symptom:** `board.Simplex[i] = val` (indexed assignment into a board field)
is rejected at parse time: "expected statement near '='".

**Workaround applied:** NelderMead was redesigned as a plain `fn` instead of a
`flow` to avoid Float[][] board fields. The `fn` version with local Float[][]
variables works correctly.

**Note:** Local `Float[][]` variable indexed assignment (`matrix[i] = row`)
IS supported. Only board field indexed assignment is not.

**Upstream task:** Support `board.ArrayField[i] = val` as an assignment target
in flow state blocks.

---

## LOW — Nested Index Read Requires Row Copy in Some Contexts

**Symptom:** In compiled mode, `simplex[i][j]` (nested double-index read on
Float[][]) can produce incorrect values in certain call contexts.

**Workaround applied:** All nested reads use explicit row copies:
```oct
let row = simplex[i]
let val = row[j]    // safe
// NOT: let val = simplex[i][j]  // potentially unsafe in compiled
```

**Upstream task:** Verify compiled code generation for chained index reads
`a[i][j]` and fix if needed.

---

## LOW — fn(Float[]) -> Float[] Parameter Limitation (Future Work)

**Symptom:** `PartialDiff` and `Gradient` in Numerics.Differentiation were
removed because `fn(Float[]) -> Float` function-typed parameters broke in
the interpreter (see MEDIUM issue above). Once the interpreter fix lands,
these can be added back.

**Impact:** Multi-variable differentiation requires manual implementation.
Users can approximate ∂f/∂xᵢ by varying one component at a time using
the scalar CentralDiff function.

---

## INFORMATIONAL — Float[] Assignment Semantics Are Go Slice Semantics

The reference-copy behaviour of Float[] is consistent with Go slice assignment.
Oct compiles Float[] to `[]float64` in Go, where `s2 := s1` copies the slice
header (pointer, length, capacity) not the data. This is correct Go semantics
but surprising for Oct users who expect value-type arrays (as in Swift, Rust,
or Oct's record types which are value types).

The right fix is at the Oct language level: Float[] should have value semantics
matching Oct's overall immutability-first design, with the compiler emitting
deep copies where needed.
