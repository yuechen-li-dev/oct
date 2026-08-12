# OCTGO-M1 — single-source typed Go imports

## 1. Result

M1 answers the central question positively: one typed Oct companion declaration
is now the only user-authored truth for each selected Go function. The OctGo
host parses that declaration, validates the same-named exported function with
`go/packages` and `go/types`, derives the existing wrapper metadata internally,
and emits the same deterministic static Go adapter. Ordinary Go ownership and
ordinary `go build` / `go test` independence are unchanged.

This milestone changes the authoring seam only. It does not widen the Go type
surface, add reflection, execute Go during checking, or create a general FFI.

## 2. M0 duplication and the authoritative M1 declaration

M0 selected each executable function twice: once as an ordinary Oct function
with a fake body and once as a `WrapperFunction` manifest entry. The specimen
therefore repeated the function name, argument types, and return type, and the
fake body remained a potential execution hazard.

M1 replaces both with:

```oct
go fn StrictlyAbove(value: Int, threshold: Int) -> Bool
go fn Residual(trace: Int, dimension: Int, threshold: Int) -> Int
```

`go fn` means exactly: this same-named Oct symbol is implemented by the
same-named exported free function in the one Go package being checked, with the
written Oct signature. It is accepted only in `*.contracts.oct`, has no body,
is non-fallible, and does not support aliases or package-wide selection.

## 3. Syntax decision and architectural judgment

A dedicated, narrow OctGo declaration is worth a language feature. Existing
forms were investigated first:

- ordinary `fn` requires a body, which recreates the placeholder problem;
- records or arrays would turn a function signature into stringly descriptor
  data and would still require a separately synthesized callable symbol;
- function-value types describe values, not externally implemented top-level
  symbols.

Accordingly M1 adds only the bodyless `go fn` form. It reuses ordinary parameter
and return type parsing and the ordinary function symbol/typechecker model; the
only new AST fact is that the declaration is a Go import. This boundary is
materially clearer than a record catalog and much narrower than `extern`, a
general FFI, dynamic linking, or arbitrary package imports.

## 4. Parsing, binding, and `go/types` authority

The ordinary parser recognizes `go fn` only when the source path ends in
`*.contracts.oct`. Project loading registers its signature as an ordinary typed
callable symbol but skips body checking because no Oct implementation exists.
The OctGo host then name-sorts the imports and binds each to the same-named
exported `go/types.Func` in the single checked package.

The Go semantic IR remains authoritative. It rejects missing functions,
methods, generics, variadics, multiple results, pointers, slices, arrays, maps,
channels, interfaces, complex values, unsafe pointers, and record-valued
executable boundaries exactly as before. A declaration cannot legalize an
unsupported Go signature. Parameter and result mappings are compared directly
between the Go declaration and the one OctGo import declaration; no manifest or
placeholder appears in diagnostics.

## 5. Internal wrapper lowering

After semantic validation, imports lower deterministically as:

```text
go fn declarations
    -> sorted typed import model
    -> internal WrapperMetadata
    -> deterministic adapter source
```

The internal wrapper keeps the established family, protocol, sidecar command,
module directory, wire name, argument list, and return type needed by compiled
lowering. All of those values are host-derived. `manifest.oct` contains no
wrapper schema, wrapper instance, or function catalog. Tests compare repeated
lowerings and assert canonical name order and exact derived signatures.

Source intent and bridge ABI remain separate: `go fn` does not expose protocol,
sidecar, command, or generated-module details.

## 6. Static bridge and compiled Octest binding

The adapter remains generated, formatted, reflection-free Go with a fixed
header, concrete decoding, and direct calls such as
`goPackage.StrictlyAbove(arg0, arg1)`. The Oct compiler does not emit a function
body for `IsGoImport` declarations. During `oct test`, the checked host-derived
wrapper metadata is injected into the owned compiled harness program, so calls
lower through the established static Octxiliary seam to the generated sidecar
and then directly to the real Go package.

There is no placeholder source body, string registry, symbol lookup, runtime
reflection, or second dispatch implementation.

## 7. `oct check`

`oct check <package>` loads the Go package, parses and type-checks the companion,
validates Concepts and imported constant witnesses, binds and validates every
`go fn`, derives wrapper metadata, renders the adapter in memory, and compares
it byte-for-byte with the committed bridge. It does not execute Go or Octest
and does not mutate stale output. A companion package without any `.octest`
still receives all of these checks.

Semantic validation precedes rendering/freshness. Consequently a removed Go
function or incompatible signature reports the Go-versus-OctGo contract error
before an otherwise inevitable stale-artifact error.

## 8. `oct test` and interpreted behavior

`oct test <package>` calls the same `Check` pipeline first. Only after a valid,
fresh contract does it build the committed adapter in an owned temporary scope,
inject the already-derived wrapper IR into the compiled harness, and execute
ordinary `[Fact]` and `[Theory]` cases against the real Go implementation.

Imported Go calls are compiled-only in M1. The CLI continues to reject an
interpreted OctGo test request, and the interpreter itself now reports:

```text
OctGo import Specimen.StrictlyAbove is compiled-only; interpreted execution
cannot call imported Go functions
```

This avoids a second runtime dispatch mechanism and makes a bodyless import
impossible to execute accidentally.

## 9. Compile-time `Require`, Concepts, and constants

`go fn` signatures are visible for ordinary type checking but are not
executable operations for the bounded refinement evaluator. A regression using
`Require(StrictlyAbove(2, 1), ...)` fails because the requirement cannot be
evaluated at compile time. Checking therefore never invokes Go.

Existing scalar/record Concept shape checks are unchanged. The typed Go
`DefaultThreshold` constant still becomes the lifecycle-scoped literal witness
`OctGo_DefaultThreshold: Threshold`, and the existing Concept admission path
still accepts or rejects it. M1 does not add foreign Concepts or broaden
constant import semantics.

## 10. Staleness and diagnostics

Freshness remains deterministic and non-mutating:

- a removed Go function reports a missing exported Go identity;
- a changed Go signature reports the exact count/type mismatch against the
  OctGo import declaration;
- an incompatible import edit reports the semantic mismatch before freshness;
- a valid new import changes rendered bytes and reports a stale bridge;
- a manually modified adapter reports artifact staleness and is not repaired by
  validation-only check.

The signature diagnostic now has only two meaningful sides:

```text
Go function github.com/.../specimen.StrictlyAbove
has signature:
    func(value int, threshold int) bool
Oct companion specimen.contracts.oct expects:
    fn(Int, Float) -> Bool
error:
    parameter 2 is incompatible: Go maps to Int, Oct expects Float
```

An attempted `go fn Bad(values: Int) -> Int` binding to
`func Bad(values []int) int` fails at the host type-mapping boundary and writes
no partial bridge.

## 11. Scientific specimen

The M0 specimen and its ordinary test syntax are retained. Compiled Octest runs
six cases with zero interpreted fallbacks, covering strict threshold equality
and the three-row above/equal/below theory, dimension-sensitive residual
correction with clamping, and the recorded dimension-free counterexample. The
same tests reach the real Go functions after all placeholder bodies and
handwritten wrapper function entries were removed.

## 12. Ergonomic and artifact measurements

For the two imported specimen functions:

- user-authored function-selection declarations: 4 before (2 fake `fn`
  declarations plus 2 manifest function entries), 2 after (2 `go fn`
  declarations), or 2 sites -> 1 site per function;
- placeholder implementation bodies: 2 before, 0 after;
- companion source: 19 lines before, 15 after (4 removed);
- manifest source: 59 lines before, 28 after (31 removed);
- companion plus manifest: 78 lines before, 43 after (35 removed);
- generated adapter: 50 physical lines / 1,376 bytes, unchanged in bytes from
  the M0 adapter;
- deterministic SHA-256 before and after three regenerations:
  `86DF1ED31FC7B47A395B58EE17C4069C365FF431F6D7AE1E02E941DCB3709A4B`.

Warm local median wall times over three runs on 2026-08-12 were:

- `oct check`: 80.8 ms;
- `oct test`: 1,475.0 ms;
- `oct check --generate`: 82.3 ms.

These are workstation observations, not performance guarantees. Generation of
already-identical bytes performs no rewrite.

## 13. Compatibility and boundaries

The Go package remains an ordinary importable package with no Oct dependency,
annotation, initialization hook, reflection, source rewriting, or generated API
surface. Its committed adapter remains a separate `main` package. Normal Go
build and test do not invoke Oct.

M1 intentionally rejects broader Go types, methods, interfaces, generics,
variadics, multiple results, pointers, containers, complex values, record calls,
aliases, wildcard imports, cross-package graphs, arbitrary foreign libraries,
runtime lookup, macros, IDE work, and automatic Go build integration.

## 14. Verification

Verification on 2026-08-12 produced:

- focused parser, build, OctGo, interpreter, tester, project, typechecker, CLI,
  internal OctGen, and experimental OctGen Go tests: passed;
- Concepts M0: 12 rejection contracts and 1 compiled case passed;
- Concepts M1: 10 rejection contracts and 2 compiled cases passed;
- compiled assertion helpers: 5 cases passed;
- real `oct check ./experimental/octgo/specimen`: passed with a fresh bridge;
- real `oct test ./experimental/octgo/specimen`: 6 passed, 0 failed, 0 skipped,
  and 0 interpreted fallbacks;
- ordinary `go build` / `go test ./experimental/octgo/specimen`: passed without
  Oct execution;
- `go vet ./...`, `go build ./...`, and `git diff --check`: passed;
- `go test ./...` completed in 33.5 seconds; all reported packages passed except
  the known unrelated `internal/conceptvulkan` checked-output lane, where the
  same 18 EVT1 artifacts report `CV3001 stale or hand-edited generated output`.
  M1 changes no ConceptVulkan source or artifact.

## 15. Riemann readiness and lesson

The authoring ergonomics now look ready for a narrow Riemann dogfood: a reader
can see every consumed Go function directly in the companion, each signature is
written once, and every bridge artifact is host-derived. The evidence is still
limited to stable scalar theorem-helper seams; it does not justify richer
compiler structures or a wider runtime surface.

M1 teaches that Oct can serve as Go's optional semantic/metaprogramming layer
without becoming a shadow implementation language. The useful division is:
Go owns implementation and legal host binding; Oct owns explicit typed intent,
bounded static constraints, and scientific executable memory; generated static
transport remains an inspectable implementation detail.

## 16. Exactly one next recommendation

**OCTGO-M2: narrowly dogfood M1 in Riemann with one stable scalar theorem-helper
seam and its existing scientific counterexample tests, without expanding the
type/runtime surface.**

The evidence for this next step is that M1 removed the measured declaration
paperwork while preserving all M0 execution and safety boundaries. No M2 work
is included in this milestone.
