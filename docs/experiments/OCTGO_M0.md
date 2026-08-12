# OCTGO-M0 — Oct contracts for existing Go packages

## 1. Thesis and result

M0 answers the narrow question positively. An ordinary Go package can remain ordinary Go while an optional Oct companion adds typed structural contracts, existing Concepts, compile-time `Require`, and executable Octest research assertions. The Go compiler is not taught Oct, and normal `go build` / `go test` do not invoke Oct.

This is an experiment, not a general Go FFI. The implementation is confined to `internal/octgo`, one CLI branch, one host-selected Octest sidecar path, and `experimental/octgo/specimen`; it is intentionally easy to remove or redesign.

## 2. Specimen

`experimental/octgo/specimen` is a real importable Go package. Its ordinary API is:

```go
type Threshold int
const DefaultThreshold Threshold = 2
func StrictlyAbove(value, threshold int) bool
func Residual(trace, dimension, threshold int) int
```

The package contains no Oct imports, annotations, reflection, generated API declarations, compiler hooks, or package-init behavior.

## 3. Companion-file convention

M0 discovers exactly one `*.contracts.oct` beside the Go source and ordinary `*.octest` files in the same directory. The selected Oct package also uses a normal `manifest.oct`. The specimen therefore has:

```text
specimen.go
specimen.contracts.oct
specimen.octest
manifest.oct
octgo_bridge/main.go       # committed generated adapter
```

No Go comments carry semantic identity. The existing wrapper manifest records the intentionally selected function names and deterministic static-adapter identity.

## 4. Go semantic IR

`go/packages` loads source, compiled files, syntax, `go/types`, and positions. The bounded deterministic IR contains:

- package identity: import path and package name;
- exported named/alias types: declaration name, kind, underlying scalar or ordered exported struct fields, position, and support status;
- exported constants: typed identity, exact value, Oct literal when representable, position, and support status;
- exported functions: declaration name, typed parameter/result projections, canonical `go/types` signature, position, and support status.

All declaration lists are sorted by name. Generated output contains no timestamp, random order, environment value, or absolute path.

Semantic identity is the typed tuple of Go package path, declaration name, declaration kind, and projected `go/types` shape/signature. An Oct function alias with a different wire name is rejected in M0.

## 5. Supported and rejected Go shapes

Supported scalar mappings are deliberately exact:

| Go | Oct |
|---|---|
| `bool` | `Bool` |
| `int` | `Int` |
| `float64` | `Float` |
| `string` | `String` |
| named scalar with one of those underlying types | same-named Oct Concept over that scalar |

The IR can structurally describe structs with exported fields of supported types. M0 check accepts same-named record-shaped Concepts with fields in declared order, but the executable function bridge remains scalar-only.

Functions may be free, non-generic, non-variadic, scalar-only, and have zero or one result. Interfaces, methods, type parameters, pointers, slices/arrays, maps, channels, unsafe pointers, complex numbers, multiple results, arbitrary named records at the executable boundary, and other Go constructs are rejected with a declaration-specific diagnostic. M0 does not model initialization, method sets, interface satisfaction, goroutines, or alias analysis.

## 6. Exact meaning of `oct check`

```text
oct check <go-package-directory>
```

means: load the host Go package with typed compiler APIs; produce the bounded semantic IR; discover and type-check the Oct project and its `*.contracts.oct`; match selected Concepts and functions to typed Go identities; evaluate ordinary compile-time `Require`; admit supported typed Go constants as same-named Concept witnesses; validate the wrapper identity; validate every selected Go/Oct/manifest signature; deterministically regenerate the adapter in memory; and fail if the committed adapter is missing or stale.

It does not execute `[Fact]` or `[Theory]`, invoke a bridged Go function, run the Go package as an application, mutate the bridge, or broaden the Concept proof evaluator.

`oct check <directory> --generate` performs the same validation and then atomically writes `octgo_bridge/main.go`. Generation is explicit. Validation-only check never repairs stale output.

## 7. Meaning of `oct test`

`oct test <go-package-directory>` first performs the complete check/freshness lane, builds the committed adapter into an owned temporary directory, and then runs discovered ordinary `[Fact]` and `[Theory]` cases through existing compiled Octest. The host supplies only that temporary directory as `OCT_WRAPPER_PATH`; the generated Oct program calls the established statically described Octxiliary function seam.

Thus:

```text
check = non-executing host/compiler/interop contract validation
test  = executable behavioral/scientific evidence against real Go code
```

OCTGO tests require compiled execution in M0. This avoids an interpreted fallback silently running the placeholder Oct declaration instead of Go.

## 8. Concept integration and static honesty

The companion declares the existing language construct:

```oct
concept Threshold = Int {
    Require(Self >= 0, "a theorem threshold must be non-negative")
}
```

The host checks that the typed Go declaration `Threshold` has underlying `int`. A typed exported Go constant such as `DefaultThreshold Threshold = 2` is injected into a lifecycle-scoped ordinary Oct witness binding:

```oct
let OctGo_DefaultThreshold: Threshold = 2
```

The normal project loader, Concept expansion, typechecker, and bounded refinement evaluator decide whether it is admitted. No Go-specific Concept implementation exists.

This proves representation compatibility and the selected constant value only. It does not claim every runtime `Threshold` value is non-negative. No `Require` expression can call Go: the witness contains literals, and existing `Require` still rejects arbitrary calls, mutation, I/O, process, environment, clock, randomness, network, and unknown runtime values.

## 9. Static bridge architecture

The bridge pipeline is:

```text
go/packages + go/types IR
    + validated Oct declarations/manifest
    -> deterministic Go source
    -> committed octgo_bridge/main.go
    -> explicit temporary `go build` during `oct test`
    -> existing typed Octxiliary transport
    -> direct ordinary Go call
```

The generated switch directly invokes `specimen.StrictlyAbove` and `specimen.Residual`. There is no reflection, binary inspection, dynamic symbol lookup, generated change to the package API, or arbitrary Go execution during checking.

## 10. Signature checking and diagnostics

The manifest selects a function, but it is not trusted. M0 verifies:

1. wrapper family contains the exact Go import path;
2. Oct name and wire name equal the typed Go declaration name;
3. the `go/types.Signature` is bridgeable;
4. Go parameters/results map exactly to the Oct declaration;
5. manifest argument/result spellings equal the Oct declaration;
6. fallibility is not invented for an ordinary non-error Go signature.

A mismatch reports the Go import path and `go/types` signature, the Oct companion path and expected signature, and the precise incompatible parameter/result.

## 11. Scientific dogfood and counterexample memory

The executable specimen contains six cases:

- a Fact that equality is rejected by a strict threshold;
- a three-row Theory covering above/equal/below cases;
- a dimension-sensitive residual Fact including clamping;
- a permanent counterexample Fact showing that dropping the dimension multiplier changes the theorem seam.

These are executable invariants and regression/counterexample memory, not certified mathematical proofs.

## 12. Capability boundary

Check-time authority belongs to the Go host. It may load the selected package and, only in explicit generation mode, write the known adapter path. Oct `Require` receives no filesystem, network, process, environment, clock, randomness, reflection, compiler-object, or Go-call capability.

Test-time Go calls are limited to functions selected by the typed manifest and emitted as direct calls in the committed adapter. The generated adapter is built in a temporary scope and removed after testing.

## 13. Determinism and stale output

IR declarations and bridge functions are name-sorted. Generation uses `go/format`, a fixed header, the module import path, and no ambient metadata. Repeated generation is byte-identical. Tests corrupt a generated case name, confirm `oct check` reports staleness, and confirm check does not mutate the stale bytes.

## 14. Normal Go compatibility

Both commands succeed without invoking Oct:

```text
go build ./experimental/octgo/specimen
go test ./experimental/octgo/specimen
```

The committed adapter is a separate `main` package beneath `octgo_bridge`. Ordinary consumers of `specimen` see the unchanged Go API. There is no `go generate` directive and no hidden Oct runtime dependency in normal Go compilation.

## 15. Measurements and friction

The specimen IR contains 1 exported type, 1 exported constant, and 2 exported functions. The contract selects 1 Concept, 2 functions, and 1 constant witness. The Octest lane executes 6 cases in one compiled harness with zero interpreted fallbacks. The committed generated adapter is 46 lines / 1,376 bytes of deterministic ordinary Go.

Verification on 2026-08-12:

- focused `internal/octgo`, CLI, tester, project, typechecker, Concepts, OctGen, and experimental OctGen Go tests passed;
- compiled Concepts-M0: 12 rejection contracts and 1 executable case passed;
- compiled Concepts-M1: 10 rejection contracts and 2 executable cases passed;
- compiled Octest assertion helpers: 2 cases passed;
- `oct check ./experimental/octgo/specimen`: passed with a fresh bridge and did not run tests;
- `oct test ./experimental/octgo/specimen`: 6 passed, 0 failed, 0 skipped, 0 interpreted fallbacks;
- `go build ./experimental/octgo/specimen` and `go test ./experimental/octgo/specimen`: passed without Oct execution;
- `go vet ./...`, `go build ./...`, and `git diff --check`: passed;
- `go test ./...` ran for 44.4 seconds and all reported packages passed except the pre-existing `internal/conceptvulkan` checked-output lane, where 18 EVT1 generated artifacts report `CV3001 stale or hand-edited generated output`. OCTGO changed none of those files; focused touched-area and OctGen lanes remain green.

The main architectural friction is that existing wrapper metadata lives in `manifest.oct`, so M0's companion selection spans `*.contracts.oct` plus an established manifest rather than one file. The function declaration body is also a compiled-backend placeholder; correctness depends on the host forcing compiled execution and the static wrapper lowering. This is acceptable evidence for M0 because it reuses the real typed transport path, but it is not the final authoring experience.

Adding `go/packages` also makes `golang.org/x/tools` a direct compiler-tooling dependency and advances its compatible `x/*` dependency set. That is a concrete cost of using typed Go tooling instead of text parsing.

Rejected designs include new Go annotations, reflection, Go AST exposure to Oct, arbitrary FFI, compile-time host execution, Go compiler plugins, macros/quasiquotation, automatic Go build hooks, and a duplicate Concept or test framework.

For the Riemann compiler specifically, the result is useful for narrow theorem-helper seams: stable typed scalar signatures, explicit theorem constants, and research Facts can be checked without moving implementation out of Go. It is not yet useful for large compiler data structures, interfaces, generic utilities, or rich proof evidence.

M0 teaches that Oct can be a scientific companion language when the semantic projection is small, explicit, typed, and host-owned. Concepts add honest static constraints to representable declarations/constants, while Fact/Theory add executable research memory over selected implementation seams. The separation fails if the projection pretends to prove arbitrary runtime values or grows into a mirror of Go.

## 16. Exactly one next recommendation

**OCTGO-M1: replace the manifest-plus-placeholder authoring duplication with one typed, host-parsed companion declaration that lowers to the same existing wrapper metadata and static adapter, without expanding the supported Go type or runtime surface.**

This is the smallest evidence-based next milestone because M0's capability works; its largest measured friction is declaring the same selected function in the Oct body and wrapper manifest. M1 should improve only that authoring seam. It must not add records to calls, runtime constructors, arbitrary FFI, interfaces, generics, or Riemann-specific logic.
