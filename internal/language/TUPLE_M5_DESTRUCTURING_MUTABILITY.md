# Tuple M5 — Destructuring Mutability Audit

## Audit results

1. Ordinary assignment (`name = expr`) requires an existing mutable binding (`var`). Immutable bindings (`let`) are rejected.
2. Mutable bindings created by `var` are assignable by ordinary assignment.
3. Destructuring assignment now applies per-target rules:
   - new target: newly bound,
   - existing immutable target: rejected,
   - existing mutable target: updated,
   - mixed existing/new: handled independently per target.
4. Before M5, destructuring diverged between checker and runtime:
   - checker allowed new names,
   - runtime required every target to already be mutable.
5. M5 resolves the divergence by aligning runtime with checker and aligning checker default mutability for new names with immutable `let`-style defaults.

## Final semantic rule

Destructuring assignment uses ordinary assignment mutability rules per target:

- existing immutable target cannot be reassigned,
- existing mutable target can be reassigned,
- missing target is defined as a new immutable binding.

## Valid and invalid patterns

Valid (new targets):

```oct
a, b = TupleProbe()
```

Invalid (immutable reassignment):

```oct
let a = 0
a, b = TupleProbe()
```

Valid (mutable reassignment + mixed targets):

```oct
var a = 0
a, b = TupleProbe()
```

## Random canonical usage pattern

Tuple-threaded RNG state remains explicit and requires mutable RNG binding:

```oct
var rng = Random.RngSeed(42)
rng, x = Random.RandInt(rng, 1, 6)
rng, y = Random.RandFloat01(rng)
```

## Tests added

- `Language/Types/Tuples/valid/destructuring_binds_new_names.octest`
- `Language/Types/Tuples/invalid/destructuring_rejects_immutable_reassignment.octfail`
- `Language/Types/Tuples/valid/destructuring_allows_mutable_reassignment_and_mixed_targets.octest`
- `Libraries/Random/random_tuple_threading_mutability.octest`

## Random M1 status

Random tuple-threading compatibility is restored under explicit mutability (`var rng = ...`).
No hidden mutable RNG behavior was introduced.

## Reference inconsistency surfaced

Task framing referenced `with` mutable bindings. Current reference authority (`Language/reference/language/14-variables.md`) specifies `let`/`var` mutability. M5 follows the reference authority and uses `var`.
