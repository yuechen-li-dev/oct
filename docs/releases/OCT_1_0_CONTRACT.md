# Oct 1.0 Contract (RC2 candidate)

## Meaning of 1.0

Oct 1.0 is a feature-frozen, useful, statically typed scientific language with
an installable GoOct toolchain. It is not a claim that every repository
experiment, library, backend, or Oct-built subsystem is complete. The stable
contract is the language described by `Language/reference/`, exercised by the
`Language/` corpus, and implemented by GoOct (`cmd/` and `internal/`).

## Stable language surface

The intended 1.0 language surface includes Unicode lexical identifiers,
comments and scalar literals; explicit static types; functions and function
values in their documented narrow form; nominal records, record tables, enums,
exhaustive enum `match`; `let`/`var` binding; arrays, ranges, indexing, and
explicit iteration; vectors, matrices, the documented rank-2 tensor/indexed
operations; `if`, `switch`, `match`, `for`, and `while`; fallible functions,
`?`, `!`, and fallible `match`; SI dimensions; core builtins; explicit packages
and imports; and Octomata's documented flow/board/control surface.

The reference is the human-readable authority. `Language/**/*.octest` and
`Language/**/*.octfail` are the executable semantic authority. GoOct is the
reference implementation and native backend; ClrOct is not a 1.0 backend.

## Stable essential tooling

The intended stable tools are `oct run`, `oct build`, `oct test`, `oct fmt`,
`oct new`, `oct init`, `oct pkg`, `oct artifact`, `oct bench`, and `oct version`.
`oct build` produces a native executable, not `.octbin`. Package import
resolution is shared by run/build/test/artifact. `oct test --execution compiled`
must never silently fall back; `auto` may report per-case interpreted fallback.
Every stable 1.0 language construct is compiled by GoOct. The positive gate is
`tools/Test-Oct10Conformance.ps1`; compiled mode requires native execution,
at least one compiled case, and zero fallback.

Release candidates are version-injected builds: `oct version` reports the
artifact semantic version, beginning with `oct 1.0.0-rc.1`. Windows x86-64 and
Linux x86-64 archives include the executable, 13 sidecars, `LICENSE`,
`INSTALL.md`, and the minimal Go compiler runtime needed for native lowering.
The Go toolchain declared by bundled `runtime/go.mod` remains a prerequisite for
`oct build`; no source checkout is required.

## Standard library boundary

The authoritative classification is
`docs/releases/OCT_1_0_SURFACE_MANIFEST.md`. Stable wrapper-backed APIs require
their documented sidecar and discovery behavior; missing sidecars are explicit
compiled errors, never interpreter fallback.

## Exclusions and known limitations

- Prometheus, SDSL-V, Machina UI, OctMake, ClrOct, remote `oct exp run`, and
  their internal/generated formats are separately versioned or experimental.
- No user-defined generics, macros, reflection, metaprogramming, anonymous
  functions, lambdas, closures, maps, or general dynamic values are promised.
- `docs/COMPILED_SUPPORT.md` is an implementation tracker, not a separate
  compatibility boundary. A stable program that passes binding must compile and
  execute natively; unsupported experimental shapes must fail explicitly.
- Sidecar availability is an explicit environmental prerequisite for the
  wrapper APIs that use it. Hardware/GPU paths are not essential tooling.

## 1.x compatibility policy

1. Existing valid 1.0 programs retain their specified meaning throughout 1.x.
2. 1.x may add compatible syntax, APIs, diagnostics, tools, libraries, and
   optimizations.
3. Deprecation requires a warning and migration guidance before removal;
   intentional source or semantic breaks wait for 2.0.
4. A compiler defect may be corrected when behavior contradicts this written
   contract; the correction needs a regression contract and release note.
5. Unspecified behavior, experimental facilities, implementation internals,
   generated Go, test-harness layouts, `.octbin`, and sidecar protocols are not
   compatibility promises unless a later public document explicitly says so.
