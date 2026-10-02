# Random v2 — M2: builtin registry consolidation

Date: 2026-10-01
Ladder: `internal/random/RANDOM_V2_LADDER.md`
Base commit: `3d1cdfff`

## Verdict

**SUCCESS.** Every Random builtin is now described once, in
`internal/builtin/random.go`. The typechecker, the interpreter and the compiled
backend read that table instead of keeping their own name lists. The v1 API is
still the one being served, and its observable behavior is unchanged: every
test lane and 94 differential probe outputs match the pre-change compiler.

## What changed

| File | Change |
|---|---|
| `internal/builtin/random.go` (new) | The table: symbol, implementing builtin, kind (seed / draw / entropy), argument count, v1 arity diagnostic, result type, fallibility. Lookup helpers for each spelling policy. |
| `internal/builtin/builtin.go` | The 20 hand-listed Random names are gone; both spellings of each builtin are reserved from the table. |
| `internal/typecheck/typecheck.go` | `isRandomBuiltinAlias`, the normalization switch, the ten-way `||` chain and the per-name signature switch are replaced by one table lookup and `checkRandomBuiltinCall`. The typechecker no longer names any Random builtin. |
| `internal/interpret/interpret.go` | The dispatch condition and the artifact / capability-discovery guards read the table. The implementation switch has one case per implementing builtin instead of two spellings each. |
| `internal/interpret/random_builtin.go` | `isEntropyRandomBuiltin`, replacing a substring match on `"CryptoRand"`. |
| `internal/build/lower.go` | `Gaussian -> RandNormal` comes from the table. One unreachable Random name list removed. |
| `internal/build/lower_expr.go` | Three per-name signature switches replaced by table lookups. No Random builtin is named here any more. |
| `internal/build/emit_go.go`, `emit_go_random.go` (new) | The helper/import conditions and the five "destructuring ... is not supported" cases read the table. |
| `internal/builtin/coverage_test.go`, `random_test.go` (new) | See "Guard tests". |

Net: 8 files changed, 142 insertions, 163 deletions, plus 3 new files.

### Where Random builtin names still appear

| Location | Why |
|---|---|
| `internal/builtin/random.go` | The table |
| `internal/interpret/interpret.go` | Implementation: one `case` per implementing builtin (9) |
| `internal/build/emit_go.go` | Implementation: one emission `case` per implementing builtin (9) |
| `internal/build/emit_go_runtime.go`, `internal/interpret/random_builtin.go` | The xoshiro / crypto helper code itself |

`typecheck`, `lower.go` and `lower_expr.go` name none. Adding a builtin is now
one table row plus its two implementations.

### One deviation from the ladder text

The ladder said the table would hold "lane hooks". It does not hold execution
functions. `internal/builtin/definition.go` states that execution functions and
generated-code templates stay in their owning packages, and the Random table
follows that rule: it records which builtin implements each name, and the
lanes switch on that. The ladder text is updated to say so.

## Dead code removed

Two blocks were unreachable and were deleted rather than converted:

- `lower.go`, reachability scan: a `targetPkg == "Random"` name list placed
  after `if builtin.IsName(targetPkg + "." + name) { continue }`. Every name in
  that list is reserved in its qualified spelling, so the earlier `continue`
  always fired first.
- `lower_expr.go`, identifier calls: Random cases in the `builtin.IsName`
  switch. For code inside package Random an earlier block already returned for
  the same ten names; for code outside it the unqualified name never matched a
  `"Random.*"` case.

## Guard tests

`TestBuiltinDefinitionsHaveImplementationCoverage` required every builtin name
to appear as a string literal in the typechecker. That is no longer true for
Random, by design, so the test now skips table-driven Random names and a new
test covers them more strictly:

- `TestRandomBuiltinsHaveImplementationCoverage`: the typechecker must call
  `builtin.LookupRandom` and must **not** name any Random builtin; every
  builtin with its own implementation must appear in both the interpreter and
  the compiled backend. Before M2 the Random builtins had no interpreter or
  backend coverage check at all.
- `random_test.go`: table well-formedness, alias targets and signature
  agreement, both spellings reserved, lookup spelling rules, and the rule that
  the execution lanes resolve the unqualified spelling only inside package
  Random.

Nine deliberate faults were injected to confirm these tests fail when they
should (implementation case removed in either lane, a name re-hardcoded in the
typechecker, table lookup removed, alias pointing at a missing or mismatched
builtin, duplicate row, non-fallible entropy, unqualified names leaking outside
package Random). All nine were caught.

## Evidence of no behavior change

All runs on linux/amd64, Go 1.25.0, in a clone at `3d1cdfff`. "Baseline" is the
compiler built from that commit before any M2 edit.

| Check | Baseline | After M2 |
|---|---|---|
| `go test ./internal/builtin ./internal/typecheck ./internal/interpret ./internal/build` | ok | ok |
| `go test ./...` | 69 ok, 1 fail | 69 ok, 1 fail (same package, same tests) |
| `oct test Libraries/Random` (auto) | 22 passed | 22 passed, identical output |
| `oct test Libraries/Random --execution interpreted` | 22 passed | 22 passed, identical output |
| `oct test Libraries/Random --execution compiled` | 22 passed | 22 passed, identical output |
| `oct test Language --all-packages`, interpreted and compiled | 392 passed, 4 failed | identical pass/fail set |
| `oct test Experiments/FmBrownNoiseKalman`, both lanes | M1 fails (and M6 interpreted) | identical |
| `oct test Experiments/PrometheusMeasurementFilteringLab`, both lanes | 28 passed interpreted; 7 fail compiled | identical |
| `oct test Language/Types/Tuples`, both lanes | 5 passed | identical |

No `.oct`, `.octest` or `.octfail` file was edited.

**Differential probes.** 29 single-file programs, 4 artifact programs and 11
package-Random fixtures were run through both compilers with `oct run`,
`oct build` plus the built executable, `oct artifact`, and `oct test` in three
execution modes: 94 captured outputs (stdout, stderr and exit code). After
normalizing temporary directory names and Go stack-trace line numbers, all 94
are byte-identical. The probes target each edited site: drawn values in both
lanes, every arity diagnostic, type arguments, qualified calls without an
import, unqualified calls outside and inside package Random, redeclaring a
builtin name, destructuring, runtime precondition failures, entropy calls with
right and wrong argument counts, the artifact ambient-randomness guard, and the
capability-discovery guard. The probes lived only in a scratch copy of the
tree and are not committed.

### Failures that exist before and after

These are on `main` at `3d1cdfff` and are not caused by M2:

- `internal/sdslv/test`: 7 tests fail because `dxc` is not installed in the
  environment the tests ran in. Not a code failure.
- `Language`: 4 `.octfail` fixtures fail in both lanes
  (`ArrayWhere` generic-type-arg and wrong-arity, `ArrayCrossSection`
  wrong-arity).
- `Experiments/PrometheusMeasurementFilteringLab`, compiled lane: 7 M4 tests
  panic with `index out of range [-1]`. The interpreted lane passes.
- `Experiments/FmBrownNoiseKalman`: milestone M1 fails in both lanes, and M6
  in the interpreted lane.

The two experiment failures matter for M6, which migrates those files.

## v1 defects found by the probes

All of these are preserved exactly by M2, as the milestone requires. They are
listed because M3 must not carry them into v2.

1. **Argument types are never checked.** `Random.RandInt(1, 2, 3)` typechecks.
   The interpreter then runs it and prints a value; the compiled lane fails
   inside `go build` with a Go type error shown to the user.
2. **Entropy builtins have no argument-count check.** `Random.CryptoRandInt(3)!`
   crashes both the interpreter and the compiler with a Go
   `index out of range` panic. `Random.CryptoRandFloat01(1, 2)!` is accepted
   silently.
3. **Qualified calls skip the import check.** `Random.RngSeed(42)` typechecks
   without `import Random`; the failure surfaces later as
   `type 'Random.RandIntResult' has no field 'Value'`.
4. **Unqualified names typecheck in every package.** `RngSeed(42)` in package
   `Main` passes the typechecker and then fails with
   `unsupported built-in function RngSeed` (interpreted) or
   `compiled mode does not yet support builtin RngSeed`.
5. **A qualified call inside package Random does not resolve its result type.**
   `Random.RandInt(...)` written inside `package Random` yields
   `type 'Random.RandIntResult' has no field 'Value'`.
6. **Arity diagnostics are inconsistent.** `RandInt` reports
   `expects 3 arguments, got 2`; `RandFloat01`, `RandFloatRange`, `RandNormal`
   and `Gaussian` report only `arity mismatch`.

Carried into the ladder: M3's table rows for the v2 natives carry parameter
types and a regular argument count, so the typechecker checks both.

## Not run

- Windows. Everything above ran on linux/amd64.
- The slow wrapper lanes (`OCT_SLOW_TESTS=1`); no wrapper or Octxiliary code
  was touched.

## Addendum (2026-10-01, from M3)

The `oct test Language --all-packages` figure above (392 passed, 4 failed)
covers `.octfail` contracts only. A run at a root directory executes the
`.octfail` fixtures below it but only the facts of the root package. M3 re-ran
the comparison one directory at a time over every directory under `Libraries`
and `Language` that holds an `.octest`, with the pre-M2 compiler (`3d1cdfff`)
and the M2 compiler (`51e53a2`): compiled 1447 pass / 125 fail in both;
interpreted 1539 / 33 and 1540 / 32, the one difference being a test that runs
close to its 30 s cycle limit and timed out under load. The M2 conclusion
stands. Details: `RANDOM_V2_M3.md`.
