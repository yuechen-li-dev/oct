# OCTGEN-M1: Second Production Dogfood and Reusable Staging Boundary

> **Experimental, post-1.0 work.** OctGen remains outside the Oct 1.0
> compatibility promise and is an explicit staging tool, never a `go build`
> compiler phase.

## Verdict

M1 supports OctGen as **compile-time computation**, not merely typed staging.
The second specimen uses normal interpreted Oct to calculate 45 concrete Go
descriptors from shape-aware stage inputs. Go validates and renders the result;
it does not expand IDs, element counts, base offsets, or audit-key projections.

## Specimen

[`tools/compiled_model_lock/audit_schedule.go`](../../tools/compiled_model_lock/audit_schedule.go)
previously contained 29 NoiseRefiner and 16 ContextRefiner concrete
`auditStageSpec` literals. They feed the production, lock-derived audit
schedule and bounded arena proof described in
[`docs/SDSL_V_COMPILED_MODEL_MANIFESTS.md`](../SDSL_V_COMPILED_MODEL_MANIFESTS.md).
This is a Go tool path only; the M1 checks do not execute Prometheus, Vulkan,
or model workloads.

Go generics cannot materialize a committed, fixed descriptor catalog from
value-level shape and lifecycle data. Runtime loops could calculate it at run
time, but would not leave the independent concrete Go source and reviewable
catalog required by this experiment.

The specimen is structurally different from M0's wrapper dispatch. It has no
host-known operation modes. It benefits from derived declaration IDs, derived
shape metadata, filtering/validation of bounded key sets, and coordinated
production-table plus existing schedule-test coverage.

## Oct computation

[`audit_stages.oct`](../../tools/compiled_model_lock/audit_stages.oct) defines
compact `StageSpec` inputs and computes:

- sequential IDs from ordered arrays;
- `Vector`, `Model`, `Hidden`, `Qkv`, and `Adaln` element counts;
- base offsets from shape plus multiplier;
- the first, last, midpoint, and valid additional projection keys;
- duplicate, out-of-range, and over-15 key filtering;
- two profile-specific concrete stage arrays.

The emitted [`audit_stages.generated.go`](../../tools/compiled_model_lock/audit_stages.generated.go)
contains 29 NoiseRefiner and 16 ContextRefiner `auditStageSpec` values. The
handwritten Go resolver now only defensively clones them before existing
validation/projection logic consumes them.

## Reused staging boundary and model

M0's shared path remains:

```text
project.Load -> typecheck.CheckProgram -> existing interpreter Generate()
  -> interpret.Value -> provenance-aware model decoder
  -> go/ast + go/format -> host-controlled atomic write / check
```

M1 extracts the interpreter invocation to return the raw existing
`interpret.Value`, then dispatches only on an explicit generated-record name.
The reusable infrastructure is loading, type checking, interpretation,
provenance, output confinement, deterministic rendering, atomic writes, and
check mode. M1 adds a small structured vocabulary for package-local typed
variable declarations containing validated composite records. It is not a Go
AST binding and does not introduce raw Go source snippets.

## Workflow and bootstrap

```powershell
go run ./tools/octgen generate -input tools/compiled_model_lock/audit_stages.oct -output tools/compiled_model_lock/audit_stages.generated.go
go run ./tools/octgen check -input tools/compiled_model_lock/audit_stages.oct -output tools/compiled_model_lock/audit_stages.generated.go
go generate ./tools/compiled_model_lock
go run ./tools/compiled_model_lock -check
```

The package has the equivalent `//go:generate` directive. `tools/octgen` does
not import `tools/compiled_model_lock`; its build uses the existing Oct
implementation only. The generated stage file is committed, so normal Go
builds and lock checks do not execute OctGen and no interpreter bootstrap cycle
is introduced. As in M0, Oct itself receives no mutation, environment, clock,
random, or network authority; the host writes one confined `.go` destination.

## Measurements

| Measure | Result |
| --- | --- |
| Concrete descriptors generated | 45 (29 NoiseRefiner, 16 ContextRefiner) |
| Handwritten stage-literal lines removed | 88 |
| Oct generator lines | 157 |
| Generated Go lines | 454 |
| M1 renderer/decoder lines | 214 |
| Cold `go run ... check` after `go clean -cache` | 17.486 s |
| Warm `go run ... generate` | 565 ms |
| Determinism | byte-identical repeated generation, covered by test |

The generated file is intentionally verbose: it is ordinary, inspectable Go
data, while the source generator keeps derivation visible and testable.

## Tests

- `go test -count=1 ./internal/octgen ./tools/compiled_model_lock ./cmd/octxiliary-time ./tools/octgen`
- `go generate ./tools/compiled_model_lock`
- both M0 and M1 `octgen check` commands
- `go run ./tools/compiled_model_lock -check` (lock validation only; no GPU)
- `go build ./tools/compiled_model_lock`
- `git diff --check`

The OctGen tests prove M1's parse/type-check/interpret path, deterministic
bytes, stale-output detection, invalid-model provenance, and M0 continued
operation. The existing compiled-model-lock tests preserve the schedule's
lock-derived header/layout behavior, malformed-stage rejection, and exact
29-stage contract.

## Ergonomics and implications

M0's collection friction recurs: each derived array needs an explicit first
element plus `for`/`Append`. Repeated record labels are more noticeable with
45 `StageSpec` values. Host-side decoding likewise grows with each record
shape, although provenance remains straightforward and reusable. M1 does not
add a language feature or a generic collection helper: two generators alone do
not yet justify one.

For Oct Make and future compiled-Oct staging, the useful lesson is that a pure
Oct data/derivation function can produce a deterministic, host-validated
concrete artifact without inheriting filesystem authority. A later bounded
collection-transform experiment would need evidence from ordinary Oct and Oct
Make—not just OctGen.

## Recommendation

Proceed to **OCTGEN-M2: one additional independent production consumer with
coordinated implementation and test declarations**, reusing the package-local
typed-table vocabulary before considering any broader Go declaration model.

For future extensibility, prefer a **typed downstream host-renderer extension**
over schema-driven arbitrary Go generation. That keeps Oct responsible for
semantic generation intent while downstream Go code owns Go syntax, imported
APIs, and rendering. M1 does not implement registration or a plugin mechanism;
the current built-in tool still requires a decoder and renderer in Oct for each
new supported semantic model.
