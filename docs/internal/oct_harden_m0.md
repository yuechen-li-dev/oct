# OCT-HARDEN-M0 — module packaging, Octomata parity, and OctGen hygiene

## Verdict

Success

## BUG-1

The starting revision `1ee679c6d6967e9c56f98334e4e81e0420722b58`
tracked both `Examples/` and `examples/`. A clean-cache module download of that
revision resolved pseudo-version `v1.0.1-0.20260822151914-1ee679c6d696` and
failed with `create zip` case-insensitive collisions beginning with
`examples/Octagon/laser_experiment.octagon: ... "Examples" and "examples"`.

`Examples/` is now canonical. Repository architecture already documented that
spelling as the curated public root, and 183 of 191 tracked example files used
it. The eight lowercase-only files were moved without deleting unique content.
All repository path references, commands, tests, scripts, checked artifacts,
and manifests were normalized. Path-derived SDSL-V IDs and generated artifact
hashes were regenerated through their owning tools.

`tools/check_case_collisions` checks every tracked/unignored path component for
case-insensitive aliases and constructs a module ZIP with `x/mod/zip`. Ubuntu CI
runs it before builds. After the migration, the guard and `go list -m -json`
pass. A clean local file proxy served `v0.0.0-hardenm0`; clean caches passed
`go list -m`, `go mod download`, and
`go install github.com/yuechen-li-dev/oct/cmd/oct@v0.0.0-hardenm0`. The installed
binary ran and reported `oct dev`.

## BUG-2

The existing centralized `resolveFlowBoardFieldType` predicate is authoritative:
board fields accept `Bool`, `String`, `Int`/`Int<D>`, `Float`/`Float<D>`, and
arrays of any depth over those scalar types. It rejects named records, arrays
of records, vectors, matrices, enums, complex values, and other runtime shapes.
Its duplicate diagnostic construction now uses one helper.

The Octomata reference, compiled-support matrix, authoring guidance, and
checkpoint reconnaissance now state that truth. The language corpus explicitly
passes `Bool[]`, `String[]`, `Int[]`, `Int<D>[]`, `Float[]`, `Float<D>[]`, and a
nested scalar array in interpreted and compiled modes. Record-valued and
record-array board fields have rejection fixtures. A focused reference-content
test prevents the stale scalar-only wording from returning.

## BUG-3

OctGen formerly emitted
`github.com/yuechen-li-dev/oct/internal/octxiliary`. Every required generated
symbol was already exposed by `github.com/yuechen-li-dev/oct/pkg/octxiliary`, so
the renderer now imports that public facade and no transport type was copied.
The committed Time output was regenerated and remains deterministic.

`TestGeneratedTimeDispatchCompilesInExternalModule` creates the unrelated
module `example.com/octgenconsumer`, generates the supported Time model, rejects
any Oct `internal/` import in the result, adds the public Oct module dependency
with a repository replacement, and compiles it with `go test -mod=mod .`. This
real external-module compile passes.

The internal-import audit classified remaining occurrences as:

- valid internal build artifacts: Oct commands, tools, public-facade
  implementations, and compiled execution artifacts intentionally built or
  staged beneath the Oct module;
- test-only: package tests and the new external-output prohibition assertion;
- external generated artifact bug: the OctGen Time renderer/output fixed here.

No other checked external-consumer generator emits an Oct `internal/` import.

## BUG-4

OctGen was not generalized. `GeneratedTimeDispatch` and
`GeneratedAuditStages` remain a closed, typed semantic vocabulary. Oct chooses
semantic intent; Go owns decoding, validation, imported APIs, Go AST syntax,
and rendering. Adding a built-in semantic model still requires a host decoder
and renderer in Oct.

## OctGen architecture boundary

OctGen is externally usable, experimental, bounded to supported host generation
models, and outside the stable Oct 1.0 ABI. It is neither monorepo-only nor an
arbitrary Go metaprogramming protocol. External hosts with different typed
models may own decoding/rendering through the experimental execution seam; Oct
does not author Go syntax or imports.

## Future renderer extensibility recommendation

typed downstream host-renderer extension

This best fits the existing public execution seam and avoids central renderer
bottlenecks without making record schemas, reflection, templates, or Go syntax
into a second Oct metaprogramming language. No registration or extension
mechanism was implemented in M0.

## FLOW checkpoint ABI

The reference already described generated checkpoint bytes as deterministic
typed JSON for an experimental host boundary. It now also states explicitly
that the byte encoding and generated names may change across compiler revisions
and are not a permanent Oct 1.0 ABI. No compatibility machinery was added.

The adjacent Database-Scheduler M8/M9 experiment reports now directly pin their
retained generated FLOW/checkpoint artifact to exact Oct commit
`309da01b60ec0f7917d4fd5efd1707bd71d2d40f` and distinguish provenance from an
ABI guarantee. No Database-Scheduler code changed.

## Verification

- `go run ./tools/check_case_collisions`: pass; no case-insensitive path alias,
  valid module ZIP.
- `go list -m -json`: pass for the working module.
- clean local proxy/cache `go list -m`, `go mod download`, and `go install
  github.com/yuechen-li-dev/oct/cmd/oct@v0.0.0-hardenm0`: pass.
- `go test ./...`: pass.
- `go test -race -count=1 ./internal/octgen ./internal/typecheck
  ./pkg/octxiliary ./tools/check_case_collisions`: pass.
- `go vet ./...`: pass.
- Octomata indexed-board valid corpus, interpreted: 8 passed.
- Octomata indexed-board valid corpus, compiled: 8 passed, compiled 8,
  interpreted fallback 0.
- Octomata compiled-boundary invalid corpus: 3 passed, including complex array,
  record, and record-array rejection.
- focused OctGen/public facade/case-guard tests: pass, including the external
  temporary-module compile and deterministic regeneration.
- Concept-Vulkan checked outputs and SDSL-V conformance after canonical-path
  regeneration: pass.
- `git diff --check` and `git diff --cached --check`: pass.

## Remaining limitations

The clean module proxy/install proof ran on the available Windows host. Linux
execution is represented by the new Ubuntu CI guard and was not executed
locally. The working tree is untagged, so the install proof used a local proxy
version rather than claiming an unpublished Git tag. OctGen remains a small
experimental model set, and FLOW checkpoint bytes remain compiler-revision
sensitive by design.

## Exactly one next recommendation

Write a bounded design proposal for typed downstream host-renderer composition,
using one additional real external consumer to test the API shape before any
implementation.
